package spec

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/aarondpn/redmine-cli/v2/internal/textsafe"
)

// MaxJSONArgSize bounds a json param read from @file or @- so a mistaken
// path (e.g. a disk image) cannot exhaust memory.
const MaxJSONArgSize = 10 << 20

// Configure turns cmd into the CLI form of c: Use, Short, positional
// argument validation, one flag per non-positional param and completion
// for enum values. It does not add --yes for destructive commands or a
// RunE; the caller owns confirmation and execution.
func Configure(cmd *cobra.Command, c *Command) error {
	cmd.Use = UseLine(c)
	// Validation rejects control characters; help is shown without running
	// the extension, so it is neutralized here as well.
	cmd.Short = textsafe.Terminal(c.Description)
	cmd.Args = PositionalArgs(c)
	return AddFlags(cmd, c)
}

// UseLine renders the cobra Use string: the command name followed by its
// positionals, <required> and [optional], with "..." for an array.
func UseLine(c *Command) string {
	parts := []string{c.Name}
	for _, p := range c.Positionals() {
		name := FlagName(p.Name)
		if p.Array {
			name += "..."
		}
		if p.Required {
			parts = append(parts, "<"+name+">")
		} else {
			parts = append(parts, "["+name+"]")
		}
	}
	return strings.Join(parts, " ")
}

// PositionalArgs validates the number of positional arguments. Validation
// guarantees required positionals come first and an array one is last, so
// the bounds follow directly from the declarations.
func PositionalArgs(c *Command) cobra.PositionalArgs {
	pos := c.Positionals()
	required := 0
	variadic := false
	for _, p := range pos {
		if p.Required {
			required++
		}
		if p.Array {
			variadic = true
		}
	}
	return func(cmd *cobra.Command, args []string) error {
		if len(args) < required {
			return fmt.Errorf("missing required argument <%s>", FlagName(pos[len(args)].Name))
		}
		if !variadic && len(args) > len(pos) {
			if len(pos) == 0 {
				return fmt.Errorf("%q accepts no arguments, received %d", cmd.CommandPath(), len(args))
			}
			return fmt.Errorf("%q accepts at most %d argument(s), received %d", cmd.CommandPath(), len(pos), len(args))
		}
		return nil
	}
}

// AddFlags registers one flag per non-positional param: name with '_' as
// '-', repeatable for arrays, the manifest default as flag default, and
// completion for enum values. Positionals get completion through
// ValidArgsFunction when the command has none yet. It fails instead of
// panicking when a flag name is reserved or already defined.
func AddFlags(cmd *cobra.Command, c *Command) error {
	fs := cmd.Flags()
	for _, p := range c.FlagParams() {
		name := FlagName(p.Name)
		if slices.Contains(ReservedFlagNames, name) {
			return fmt.Errorf("param %q: flag --%s is reserved", p.Name, name)
		}
		if fs.Lookup(name) != nil || cmd.InheritedFlags().Lookup(name) != nil {
			return fmt.Errorf("param %q: flag --%s is already defined", p.Name, name)
		}
		if err := addFlag(fs, p, name, textsafe.Terminal(flagUsage(p))); err != nil {
			return fmt.Errorf("param %q: %w", p.Name, err)
		}
		if values := completionValues(p); values != nil {
			if err := cmd.RegisterFlagCompletionFunc(name, cobra.FixedCompletions(values, cobra.ShellCompDirectiveNoFileComp)); err != nil {
				return err
			}
		}
	}
	if cmd.ValidArgsFunction == nil && len(c.Positionals()) > 0 {
		cmd.ValidArgsFunction = positionalCompletion(c)
	}
	return nil
}

func addFlag(fs *pflag.FlagSet, p *Param, name, usage string) error {
	if p.Array {
		def, _ := p.Default.([]any)
		switch p.Type {
		case TypeString:
			fs.StringArray(name, typedList(def, func(v any) (string, bool) { return defaultText(p.Type, v) }), usage)
		case TypeJSON:
			fs.StringArray(name, typedList(def, func(v any) (string, bool) { return defaultText(p.Type, v) }), usage)
			if def != nil {
				// pflag shows string arrays CSV-quoted, which mangles JSON;
				// show the default the way it would be written instead.
				if text, ok := defaultText(TypeJSON, def); ok {
					fs.Lookup(name).DefValue = text
				}
			}
		case TypeInteger:
			fs.Var(newInt64ListValue(typedList(def, func(v any) (int64, bool) {
				c, ok := coerceScalar(TypeInteger, v)
				i, _ := c.(int64)
				return i, ok
			})), name, usage)
		case TypeNumber:
			fs.Var(newFloat64ListValue(typedList(def, func(v any) (float64, bool) {
				c, ok := coerceScalar(TypeNumber, v)
				f, _ := c.(float64)
				return f, ok
			})), name, usage)
		case TypeBoolean:
			// Validation rejects these: --x --x=false would be a list.
			return errors.New("boolean parameters cannot be arrays")
		default:
			return fmt.Errorf("unknown type %q", p.Type)
		}
		return nil
	}
	switch p.Type {
	case TypeString, TypeJSON:
		def, _ := defaultText(p.Type, p.Default)
		fs.String(name, def, usage)
	case TypeInteger:
		c, _ := coerceScalar(TypeInteger, p.Default)
		def, _ := c.(int64)
		fs.Var(newInt64Value(def), name, usage)
	case TypeNumber:
		c, _ := coerceScalar(TypeNumber, p.Default)
		def, _ := c.(float64)
		fs.Var(newFloat64Value(def), name, usage)
	case TypeBoolean:
		def, _ := p.Default.(bool)
		fs.Bool(name, def, usage)
	default:
		return fmt.Errorf("unknown type %q", p.Type)
	}
	return nil
}

// defaultText renders a default for a string-valued flag: strings as is,
// json defaults as compact JSON (what the user would type).
func defaultText(t ParamType, v any) (string, bool) {
	if v == nil {
		return "", false
	}
	if t == TypeString {
		s, ok := v.(string)
		return s, ok
	}
	data, err := json.Marshal(v)
	if err != nil {
		return "", false
	}
	return string(data), true
}

func typedList[T any](in []any, conv func(any) (T, bool)) []T {
	if in == nil {
		return nil
	}
	out := make([]T, 0, len(in))
	for _, v := range in {
		if c, ok := conv(v); ok {
			out = append(out, c)
		}
	}
	return out
}

func flagUsage(p *Param) string {
	var notes []string
	if p.Type == TypeJSON {
		notes = append(notes, "JSON value or @file")
	}
	if len(p.Enum) > 0 {
		notes = append(notes, "one of: "+formatList(p.Enum))
	}
	if p.Array {
		notes = append(notes, "repeatable")
	}
	if p.Required {
		notes = append(notes, "required")
	}
	usage := p.Description
	if len(notes) > 0 {
		if usage != "" {
			usage += " "
		}
		usage += "(" + strings.Join(notes, "; ") + ")"
	}
	return usage
}

func completionValues(p *Param) []string {
	switch {
	case len(p.Enum) > 0:
		out := make([]string, len(p.Enum))
		for i, v := range p.Enum {
			out[i] = formatValue(v)
		}
		return out
	case p.Type == TypeBoolean && p.Positional:
		return []string{"true", "false"}
	}
	return nil
}

func positionalCompletion(c *Command) cobra.CompletionFunc {
	pos := c.Positionals()
	return func(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
		var p *Param
		switch {
		case len(args) < len(pos):
			p = pos[len(args)]
		case len(pos) > 0 && pos[len(pos)-1].Array:
			p = pos[len(pos)-1]
		default:
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		if values := completionValues(p); values != nil {
			return values, cobra.ShellCompDirectiveNoFileComp
		}
		if p.Type == TypeJSON {
			// @file is a path.
			return nil, cobra.ShellCompDirectiveDefault
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

// BuildInput converts parsed flags and positional args into the input
// object: param names as keys, values typed per the manifest, defaults
// applied, absent optionals omitted. A json param given as null counts as
// not given, the same as a null MCP argument. The result is validated against
// InputSchema(c), the same contract MCP input must meet. json params read
// @file from disk and @- from cmd's stdin.
func BuildInput(cmd *cobra.Command, c *Command, args []string) (map[string]any, error) {
	in := map[string]any{}
	stdinUsed := false
	readStdin := func() ([]byte, error) {
		if stdinUsed {
			return nil, errors.New("only one value can be read from stdin (@-)")
		}
		stdinUsed = true
		return readLimited(cmd.InOrStdin())
	}

	pos := c.Positionals()
	for i, p := range pos {
		label := "<" + FlagName(p.Name) + ">"
		if p.Array {
			if i < len(args) {
				list := make([]any, 0, len(args)-i)
				for _, a := range args[i:] {
					v, err := parseText(p, a, label, readStdin)
					if err != nil {
						return nil, err
					}
					list = append(list, v)
				}
				in[p.Name] = list
			}
			break
		}
		if i >= len(args) {
			break
		}
		v, err := parseText(p, args[i], label, readStdin)
		if err != nil {
			return nil, err
		}
		if v != nil { // a json null counts as not given, as over MCP
			in[p.Name] = v
		}
	}
	if len(args) > len(pos) && (len(pos) == 0 || !pos[len(pos)-1].Array) {
		return nil, fmt.Errorf("too many arguments: expected at most %d, received %d", len(pos), len(args))
	}

	fs := cmd.Flags()
	for _, p := range c.FlagParams() {
		name := FlagName(p.Name)
		f := fs.Lookup(name)
		if f == nil {
			return nil, fmt.Errorf("flag --%s is not registered", name)
		}
		if !f.Changed {
			continue
		}
		v, err := flagValue(fs, p, name, readStdin)
		if err != nil {
			return nil, err
		}
		if v != nil { // a json null counts as not given, as over MCP
			in[p.Name] = v
		}
	}

	for i := range c.Params {
		p := &c.Params[i]
		v, ok := in[p.Name]
		if !ok {
			if p.Required {
				if p.Positional {
					return nil, fmt.Errorf("missing required argument <%s>", FlagName(p.Name))
				}
				return nil, fmt.Errorf("missing required flag --%s", FlagName(p.Name))
			}
			continue
		}
		if err := checkValue(p, v); err != nil {
			return nil, fmt.Errorf("invalid value for %s: %w", paramLabel(p), err)
		}
	}

	in = ApplyDefaults(c, in)
	if err := ValidateInput(c, in); err != nil {
		return nil, err
	}
	return in, nil
}

func paramLabel(p *Param) string {
	if p.Positional {
		return "<" + FlagName(p.Name) + ">"
	}
	return "--" + FlagName(p.Name)
}

func flagValue(fs *pflag.FlagSet, p *Param, name string, readStdin func() ([]byte, error)) (any, error) {
	label := "--" + name
	if p.Array {
		switch p.Type {
		case TypeString:
			v, err := fs.GetStringArray(name)
			return toAnySlice(v), err
		case TypeJSON:
			raw, err := fs.GetStringArray(name)
			if err != nil {
				return nil, err
			}
			out := make([]any, 0, len(raw))
			for _, r := range raw {
				v, err := parseJSONArg(r, label, readStdin)
				if err != nil {
					return nil, err
				}
				out = append(out, v)
			}
			return out, nil
		case TypeInteger:
			v, ok := fs.Lookup(name).Value.(*int64ListValue)
			if !ok {
				return nil, fmt.Errorf("flag --%s is not an integer list", name)
			}
			return toAnySlice(v.list), nil
		case TypeNumber:
			v, ok := fs.Lookup(name).Value.(*float64ListValue)
			if !ok {
				return nil, fmt.Errorf("flag --%s is not a number list", name)
			}
			return toAnySlice(v.list), nil
		}
		return nil, fmt.Errorf("unknown type %q", p.Type)
	}
	switch p.Type {
	case TypeString:
		return fs.GetString(name)
	case TypeJSON:
		raw, err := fs.GetString(name)
		if err != nil {
			return nil, err
		}
		return parseJSONArg(raw, label, readStdin)
	case TypeInteger:
		v, ok := fs.Lookup(name).Value.(*int64Value)
		if !ok {
			return nil, fmt.Errorf("flag --%s is not an integer", name)
		}
		return int64(*v), nil
	case TypeNumber:
		v, ok := fs.Lookup(name).Value.(*float64Value)
		if !ok {
			return nil, fmt.Errorf("flag --%s is not a number", name)
		}
		return float64(*v), nil
	case TypeBoolean:
		return fs.GetBool(name)
	}
	return nil, fmt.Errorf("unknown type %q", p.Type)
}

// parseText converts one positional argument to p's type.
func parseText(p *Param, s, label string, readStdin func() ([]byte, error)) (any, error) {
	switch p.Type {
	case TypeString:
		return s, nil
	case TypeInteger:
		n, err := parseInteger(s)
		if err != nil {
			return nil, fmt.Errorf("invalid value %q for %s: %v", s, label, err)
		}
		return n, nil
	case TypeNumber:
		f, err := parseNumber(s)
		if err != nil {
			return nil, fmt.Errorf("invalid value %q for %s: %v", s, label, err)
		}
		return f, nil
	case TypeBoolean:
		b, err := strconv.ParseBool(s)
		if err != nil {
			return nil, fmt.Errorf("invalid value %q for %s: must be true or false", s, label)
		}
		return b, nil
	case TypeJSON:
		return parseJSONArg(s, label, readStdin)
	}
	return nil, fmt.Errorf("unknown type %q", p.Type)
}

// parseJSONArg decodes a json param given as JSON text, @file or @- (stdin).
// Numbers stay json.Number, as for an MCP tool call, so the value reaches
// the extension with its exact digits (12345678901234567890 does not turn
// into a float) and both surfaces send the same input.
func parseJSONArg(s, label string, readStdin func() ([]byte, error)) (any, error) {
	data := []byte(s)
	source := "value"
	if rest, ok := strings.CutPrefix(s, "@"); ok {
		var err error
		switch rest {
		case "":
			return nil, fmt.Errorf("invalid value for %s: @ must be followed by a file path or -", label)
		case "-":
			source = "stdin"
			data, err = readStdin()
		default:
			source = rest
			data, err = readFileLimited(rest)
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", label, err)
		}
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("invalid JSON for %s (%s): %v", label, source, err)
	}
	if dec.More() {
		return nil, fmt.Errorf("invalid JSON for %s (%s): unexpected data after the value", label, source)
	}
	return v, nil
}

func readFileLimited(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return readLimited(f)
}

func readLimited(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxJSONArgSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxJSONArgSize {
		return nil, fmt.Errorf("input is larger than %d bytes", MaxJSONArgSize)
	}
	return data, nil
}

// parseInteger parses a base-10 integer, the one syntax for integer flags
// and positionals. pflag's own integer flags accept 0x10, 0o10 and 010
// (as 8), which the MCP surface (JSON numbers) does not.
func parseInteger(s string) (int64, error) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		if errors.Is(err, strconv.ErrRange) {
			return 0, errors.New("must be an integer between -9223372036854775808 and 9223372036854775807")
		}
		return 0, errors.New("must be an integer")
	}
	return n, nil
}

// int64Value is an integer flag parsed with parseInteger. Type and String
// match pflag's Int64 so help output is unchanged.
type int64Value int64

func newInt64Value(def int64) *int64Value { v := int64Value(def); return &v }

func (v *int64Value) Set(s string) error {
	n, err := parseInteger(s)
	if err != nil {
		return err
	}
	*v = int64Value(n)
	return nil
}

func (v *int64Value) Type() string   { return "int64" }
func (v *int64Value) String() string { return strconv.FormatInt(int64(*v), 10) }

// int64ListValue is a repeatable integer flag parsed with parseInteger.
// Like pflag's Int64Slice it accepts comma-separated values, and the first
// use replaces the default.
type int64ListValue struct {
	list    []int64
	changed bool
}

func newInt64ListValue(def []int64) *int64ListValue { return &int64ListValue{list: def} }

func (v *int64ListValue) Set(s string) error {
	parts := strings.Split(s, ",")
	out := make([]int64, 0, len(parts))
	for _, part := range parts {
		n, err := parseInteger(part)
		if err != nil {
			return err
		}
		out = append(out, n)
	}
	if !v.changed {
		v.list = nil
		v.changed = true
	}
	v.list = append(v.list, out...)
	return nil
}

func (v *int64ListValue) Type() string { return "int64Slice" }

func (v *int64ListValue) String() string {
	parts := make([]string, len(v.list))
	for i, n := range v.list {
		parts[i] = strconv.FormatInt(n, 10)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// jsonNumber is the JSON number grammar (RFC 8259, section 6).
var jsonNumber = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)

// parseNumber parses a number written the way JSON writes one, the one
// syntax for number flags and positionals. strconv.ParseFloat (and so
// pflag's Float64) also takes NaN, Inf, hex floats and underscores: NaN and
// Inf cannot be encoded as JSON at all, and the others are not accepted
// over MCP either.
func parseNumber(s string) (float64, error) {
	if !jsonNumber.MatchString(s) {
		return 0, errors.New("must be a number")
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, errors.New("must be a finite number")
	}
	return f, nil
}

// float64Value is a number flag parsed with parseNumber. Type and String
// match pflag's Float64 so help output is unchanged.
type float64Value float64

func newFloat64Value(def float64) *float64Value { v := float64Value(def); return &v }

func (v *float64Value) Set(s string) error {
	f, err := parseNumber(s)
	if err != nil {
		return err
	}
	*v = float64Value(f)
	return nil
}

func (v *float64Value) Type() string { return "float64" }
func (v *float64Value) String() string {
	return strconv.FormatFloat(float64(*v), 'g', -1, 64)
}

// float64ListValue is a repeatable number flag parsed with parseNumber.
// Like pflag's Float64Slice it accepts comma-separated values, and the
// first use replaces the default.
type float64ListValue struct {
	list    []float64
	changed bool
}

func newFloat64ListValue(def []float64) *float64ListValue { return &float64ListValue{list: def} }

func (v *float64ListValue) Set(s string) error {
	parts := strings.Split(s, ",")
	out := make([]float64, 0, len(parts))
	for _, part := range parts {
		f, err := parseNumber(part)
		if err != nil {
			return err
		}
		out = append(out, f)
	}
	if !v.changed {
		v.list = nil
		v.changed = true
	}
	v.list = append(v.list, out...)
	return nil
}

func (v *float64ListValue) Type() string { return "float64Slice" }

func (v *float64ListValue) String() string {
	parts := make([]string, len(v.list))
	for i, f := range v.list {
		parts[i] = fmt.Sprintf("%f", f)
	}
	return "[" + strings.Join(parts, ",") + "]"
}
