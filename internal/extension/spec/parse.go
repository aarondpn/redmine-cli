package spec

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"gopkg.in/yaml.v3"
)

// MaxManifestSize bounds how much of a manifest file is read. Real manifests
// are a few KiB; the cap keeps a hostile repository from exhausting memory.
const MaxManifestSize = 1 << 20

// maxNodes bounds the decoded document, including nodes reached through
// aliases, so an alias bomb cannot expand into an enormous tree.
const maxNodes = 100_000

// Options carries the context-dependent parts of validation.
type Options struct {
	// File is the name used in error positions. Defaults to FileName.
	File string
	// ExtensionName, when set, must equal the manifest name.
	ExtensionName string
	// CLIVersion is the running redmine-cli version. min_cli_version is
	// only enforced when this is a release version; "dev", "" and other
	// non-semver builds skip the check.
	CLIVersion string
}

func (o Options) file() string {
	if o.File != "" {
		return o.File
	}
	return FileName
}

// ParseFile reads and validates the manifest at path. Errors name the file
// by its base name, matching how authors refer to redmine-extension.yml.
func ParseFile(path string, opts Options) (*Manifest, error) {
	m, _, err := parseFile(path, opts, false)
	return m, err
}

// ParseFileLenient is ParseFile in lenient mode; see ParseLenient.
func ParseFileLenient(path string, opts Options) (*Manifest, []*Error, error) {
	return parseFile(path, opts, true)
}

func parseFile(path string, opts Options, lenient bool) (*Manifest, []*Error, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, MaxManifestSize+1))
	if err != nil {
		return nil, nil, err
	}
	if opts.File == "" {
		opts.File = filepath.Base(path)
	}
	return parse(data, opts, lenient)
}

// Parse decodes and validates a manifest strictly: YAML syntax, then
// schema_version and min_cli_version (so a manifest written for a newer
// redmine-cli is reported as such rather than as a list of unknown fields),
// then the embedded JSON Schema and the semantic rules together. Validation
// problems are returned as a *ValidationError whose entries carry file:line
// positions, ordered by line. Unknown fields are errors; this is the mode
// for `redmine extension check`, install and the published schema.
func Parse(data []byte, opts Options) (*Manifest, error) {
	m, _, err := parse(data, opts, false)
	return m, err
}

// ParseLenient is Parse except that unknown fields are not errors: they are
// dropped before validation and returned as warnings (one *Error per field,
// ordered by line), so a manifest using fields added by a newer redmine-cli
// with the same schema_version still loads. Warnings are returned even when
// err is non-nil. Every other rule is enforced exactly as by Parse.
func ParseLenient(data []byte, opts Options) (*Manifest, []*Error, error) {
	return parse(data, opts, true)
}

func parse(data []byte, opts Options, lenient bool) (*Manifest, []*Error, error) {
	file := opts.file()
	if len(data) > MaxManifestSize {
		return nil, nil, &ValidationError{Errors: []*Error{{File: file, Message: fmt.Sprintf("manifest is larger than %d bytes", MaxManifestSize)}}}
	}
	doc, pos, err := decodeYAML(data, file)
	if err != nil {
		return nil, nil, err
	}
	if err := checkCompatibility(doc, pos, opts); err != nil {
		return nil, nil, err
	}

	root, err := manifestSchema()
	if err != nil {
		return nil, nil, err
	}
	var warnings []*Error
	if lenient {
		warnings = dropUnknownFields(doc, root, pos, file)
	}
	is := &issues{file: file, pos: pos}
	if err := newSchemaChecker().check(is, "", doc, root); err != nil {
		return nil, warnings, err
	}

	// Semantic rules run whenever the document fits the structs, even if
	// the schema found problems, so one pass reports everything.
	m, err := fromTree(doc)
	if err != nil {
		if len(is.list) == 0 {
			// The schema admits only shapes the structs can hold, so this
			// is an internal inconsistency rather than an author mistake.
			is.list = append(is.list, &Error{File: file, Message: err.Error()})
		}
		return nil, warnings, is.err()
	}
	validateSemantics(is, m, opts)
	if err := is.err(); err != nil {
		return nil, warnings, err
	}
	return m, warnings, nil
}

// checkCompatibility enforces schema_version and min_cli_version before
// anything else. A manifest for a newer schema or redmine-cli may use
// fields this build does not know; reporting those would hide the real
// fix, upgrading. Values of the wrong shape are left to the schema.
func checkCompatibility(doc any, pos positions, opts Options) error {
	obj, ok := doc.(map[string]any)
	if !ok {
		return nil
	}
	is := &issues{file: opts.file(), pos: pos}
	if v, ok := obj["schema_version"]; ok && jsonTypeOf(v) == "integer" {
		if n, ok := coerceScalar(TypeInteger, v); ok && n != int64(CurrentSchemaVersion) {
			is.addf("/schema_version", "unsupported schema_version %d (this redmine-cli supports %d)", n, CurrentSchemaVersion)
		} else if !ok {
			is.addf("/schema_version", "unsupported schema_version %s (this redmine-cli supports %d)", formatValue(v), CurrentSchemaVersion)
		}
	}
	if v, ok := obj["min_cli_version"].(string); ok {
		want, ok := parseVersion(v)
		if running, release := parseVersion(opts.CLIVersion); ok && release && compareVersions(running, want) < 0 {
			is.addf("/min_cli_version", "requires redmine-cli %s or newer (running %s)", v, opts.CLIVersion)
		}
	}
	return is.err()
}

// dropUnknownFields removes, in place, every object member the schema does
// not declare where it forbids additional properties, and returns one
// warning per removed member.
func dropUnknownFields(doc any, root *jsonschema.Schema, pos positions, file string) []*Error {
	var out []*Error
	var walk func(ptr string, v any, s *jsonschema.Schema)
	walk = func(ptr string, v any, s *jsonschema.Schema) {
		if s == nil {
			return
		}
		switch val := v.(type) {
		case map[string]any:
			if s.Properties == nil {
				return
			}
			for _, k := range slices.Sorted(maps.Keys(val)) {
				sub, ok := s.Properties[k]
				if !ok {
					if isFalseSchema(s.AdditionalProperties) {
						out = append(out, &Error{
							File:    file,
							Line:    pos.line(pointer(ptr, k)),
							Path:    displayPath(ptr),
							Message: fmt.Sprintf("unknown field %q ignored", k),
						})
						delete(val, k)
					}
					continue
				}
				walk(pointer(ptr, k), val[k], sub)
			}
		case []any:
			for i, item := range val {
				walk(pointer(ptr, i), item, s.Items)
			}
		}
	}
	walk("", doc, root)
	slices.SortStableFunc(out, func(a, b *Error) int { return cmp.Compare(a.Line, b.Line) })
	return out
}

// fromTree converts the JSON-compatible document into the typed manifest.
// Going through JSON keeps decoding identical to what the schema validated.
func fromTree(doc any) (*Manifest, error) {
	data, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var m Manifest
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	for ci := range m.Commands {
		for pi := range m.Commands[ci].Params {
			p := &m.Commands[ci].Params[pi]
			for i, v := range p.Enum {
				p.Enum[i] = normalizeNumbers(v)
			}
			p.Default = normalizeNumbers(p.Default)
		}
	}
	return &m, nil
}

// normalizeNumbers replaces json.Number with int64 for integral literals and
// float64 otherwise, so callers can type-switch on plain Go values.
func normalizeNumbers(v any) any {
	switch t := v.(type) {
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return i
		}
		f, _ := t.Float64()
		return f
	case []any:
		for i := range t {
			t[i] = normalizeNumbers(t[i])
		}
		return t
	case map[string]any:
		for k := range t {
			t[k] = normalizeNumbers(t[k])
		}
		return t
	default:
		return v
	}
}

var yamlLineRe = regexp.MustCompile(`^yaml: line (\d+): (.*)$`)

// decodeYAML parses data into a JSON-compatible tree (map[string]any,
// []any, string, int64, float64, bool, nil) and records the line of every
// node by JSON pointer.
func decodeYAML(data []byte, file string) (any, positions, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		e := &Error{File: file, Message: err.Error()}
		if m := yamlLineRe.FindStringSubmatch(err.Error()); m != nil {
			e.Line, _ = strconv.Atoi(m[1])
			e.Message = m[2]
		}
		return nil, nil, &ValidationError{Errors: []*Error{e}}
	}
	if root.Kind == 0 || len(root.Content) == 0 {
		return nil, nil, &ValidationError{Errors: []*Error{{File: file, Message: "manifest is empty"}}}
	}
	d := &treeDecoder{file: file, pos: positions{}}
	node := root.Content[0]
	d.pos[""] = node.Line
	v := d.value(node, "")
	if err := d.is().err(); err != nil {
		return nil, nil, err
	}
	return v, d.pos, nil
}

type treeDecoder struct {
	file  string
	pos   positions
	nodes int
	errs  []*Error
}

func (d *treeDecoder) is() *issues { return &issues{file: d.file, pos: d.pos, list: d.errs} }

func (d *treeDecoder) fail(n *yaml.Node, ptr, format string, args ...any) {
	d.errs = append(d.errs, &Error{File: d.file, Line: n.Line, Path: displayPath(ptr), Message: fmt.Sprintf(format, args...)})
}

func (d *treeDecoder) value(n *yaml.Node, ptr string) any {
	d.nodes++
	if d.nodes > maxNodes {
		if d.nodes == maxNodes+1 {
			d.fail(n, ptr, "document is too large (more than %d nodes, including aliases)", maxNodes)
		}
		return nil
	}
	switch n.Kind {
	case yaml.AliasNode:
		return d.value(n.Alias, ptr)
	case yaml.DocumentNode:
		if len(n.Content) == 0 {
			return nil
		}
		return d.value(n.Content[0], ptr)
	case yaml.MappingNode:
		out := make(map[string]any, len(n.Content)/2)
		for i := 0; i+1 < len(n.Content); i += 2 {
			k, v := n.Content[i], n.Content[i+1]
			if k.Kind == yaml.AliasNode {
				k = k.Alias
			}
			if k.Kind != yaml.ScalarNode {
				d.fail(k, ptr, "mapping keys must be strings")
				continue
			}
			if k.Tag == "!!merge" {
				d.fail(k, ptr, "merge keys (<<) are not supported")
				continue
			}
			child := pointer(ptr, k.Value)
			if _, dup := out[k.Value]; dup {
				d.fail(k, ptr, "duplicate key %q (first defined on line %d)", k.Value, d.pos[child])
				continue
			}
			d.pos[child] = k.Line
			out[k.Value] = d.value(v, child)
		}
		return out
	case yaml.SequenceNode:
		out := make([]any, 0, len(n.Content))
		for i, item := range n.Content {
			child := pointer(ptr, i)
			d.pos[child] = item.Line
			out = append(out, d.value(item, child))
		}
		return out
	case yaml.ScalarNode:
		return d.scalar(n, ptr)
	default:
		d.fail(n, ptr, "unsupported YAML node")
		return nil
	}
}

// scalar resolves a scalar the way YAML 1.2 core schema does (yaml.v3's
// rules) but keeps the result JSON-compatible: timestamps and other
// non-JSON types fall back to their source text.
func (d *treeDecoder) scalar(n *yaml.Node, ptr string) any {
	var v any
	if err := n.Decode(&v); err != nil {
		d.fail(n, ptr, "%v", err)
		return nil
	}
	switch t := v.(type) {
	case nil, bool, string:
		return t
	case int:
		return int64(t)
	case int64:
		return t
	case uint64:
		if t > math.MaxInt64 {
			return float64(t)
		}
		return int64(t)
	case float64:
		if math.IsInf(t, 0) || math.IsNaN(t) {
			d.fail(n, ptr, "%s is not a valid JSON number", n.Value)
			return nil
		}
		return t
	case time.Time:
		return n.Value
	default:
		return n.Value
	}
}
