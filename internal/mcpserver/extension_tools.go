package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aarondpn/redmine-cli/v2/internal/extension/runner"
	"github.com/aarondpn/redmine-cli/v2/internal/extension/spec"
)

// ExtensionTool is an MCP tool backed by a declarative command of an
// installed extension. The tool runs the extension executable with the
// execution protocol shared with the CLI (see package runner).
type ExtensionTool struct {
	// Extension is the extension name; the tool's group is ext-<Extension>.
	Extension string
	// Version is the installed version, for display only.
	Version string
	// Path is the extension executable.
	Path string
	// Command is the manifest command the tool runs. Raw commands and
	// commands with mcp: false are never tools.
	Command *spec.Command
	// Env is the invocation-level part of the environment contract, the
	// same as for a CLI run (binary, profile, config file, server, headers,
	// read-only mode, color). The per-call fields (extension, command,
	// surface, tool) are filled in for every call, and read-only mode is
	// forced for commands that do not write.
	Env runner.Env
}

// Name is the MCP tool name, <extension>_<command>.
func (t ExtensionTool) Name() string {
	return spec.ToolName(t.Extension, t.Command.Name)
}

// Group is the tool group of the extension, ext-<name>.
func (t ExtensionTool) Group() Group {
	return ExtensionGroup(t.Extension)
}

// Description is the tool description shown to MCP clients: the manifest
// description plus the same markers generated tools carry, so a host or
// model can tell destructive and writing tools apart.
func (t ExtensionTool) Description() string {
	desc := strings.TrimSpace(t.Command.Description)
	if desc != "" && !strings.HasSuffix(desc, ".") && !strings.HasSuffix(desc, "!") && !strings.HasSuffix(desc, "?") {
		desc += "."
	}
	var parts []string
	if desc != "" {
		parts = append(parts, desc)
	}
	if t.Command.Destructive {
		parts = append(parts, "Destructive.")
	}
	if t.Command.Writes {
		parts = append(parts, "Requires --enable-writes.")
	}
	return strings.Join(parts, " ")
}

func (t ExtensionTool) descriptor() ToolDescriptor {
	return ToolDescriptor{
		Name:             t.Name(),
		Description:      t.Description(),
		Group:            t.Group(),
		Writes:           t.Command.Writes,
		Extension:        t.Extension,
		ExtensionVersion: t.Version,
	}
}

// usable reports whether t describes a command that can be a tool at all.
func (t ExtensionTool) usable() bool {
	return t.Extension != "" && t.Command != nil && !t.Command.Raw && t.Command.MCPEnabled()
}

// ResolveExtensionTools drops the tools that cannot be registered: a tool
// whose name is taken by a generated tool or by an earlier extension tool
// (extensions are taken in name order, so the result does not depend on
// the input order), and entries that are not MCP commands. It returns the
// remaining tools and one warning per tool skipped because of a name clash.
// The server still starts; the warnings tell the operator what is missing.
func ResolveExtensionTools(tools []ExtensionTool) ([]ExtensionTool, []string) {
	ordered := make([]ExtensionTool, 0, len(tools))
	for _, t := range tools {
		if t.usable() {
			ordered = append(ordered, t)
		}
	}
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Extension < ordered[j].Extension })

	owner := map[string]string{}
	for _, d := range generatedToolDescriptors() {
		owner[d.Name] = ""
	}
	var (
		kept     []ExtensionTool
		warnings []string
	)
	for _, t := range ordered {
		name := t.Name()
		if by, taken := owner[name]; taken {
			what := "a core tool has the same name"
			if by != "" {
				what = "extension " + by + " already provides a tool of that name"
			}
			warnings = append(warnings, fmt.Sprintf("extension %s: skipping MCP tool %s (command %s): %s", t.Extension, name, t.Command.Name, what))
			continue
		}
		owner[name] = t.Extension
		kept = append(kept, t)
	}
	return kept, warnings
}

// registerExtensionTools registers opts.ExtensionTools after the generated
// tools, with the same gates: writing tools only with EnableWrites, then
// the group and tool filters. Clashing names are skipped here as well, so
// an extension can never replace a core tool even if the caller did not
// resolve the list first.
func registerExtensionTools(srv *mcp.Server, opts Options) {
	tools, _ := ResolveExtensionTools(opts.ExtensionTools)
	for _, t := range tools {
		if t.Command.Writes && !opts.EnableWrites {
			continue
		}
		if !opts.groupEnabled(t.Group()) || !opts.toolEnabled(t.Name()) {
			continue
		}
		readOnly := !t.Command.Writes
		annotations := &mcp.ToolAnnotations{ReadOnlyHint: readOnly}
		if !readOnly {
			destructive := t.Command.Destructive
			annotations.DestructiveHint = &destructive
		}
		srv.AddTool(&mcp.Tool{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: spec.InputSchema(t.Command),
			Annotations: annotations,
		}, extensionToolHandler(t))
	}
}

// extensionToolHandler validates the arguments against the command's
// params, runs the extension and turns its output into the tool result.
// Every failure is a tool error (isError), never a protocol error, so the
// model sees the message and can correct its call.
func extensionToolHandler(t ExtensionTool) mcp.ToolHandler {
	name := t.Name()
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var raw json.RawMessage
		if req != nil && req.Params != nil {
			raw = req.Params.Arguments
		}
		args, err := decodeArguments(raw)
		if err != nil {
			return extensionToolError(`validating "arguments": ` + err.Error()), nil
		}
		input, err := spec.PrepareInput(t.Command, normalizeNumbers(t.Command, args))
		if err != nil {
			return extensionToolError(`validating "arguments": ` + validationMessage(err)), nil
		}

		env := t.Env
		env.Extension = t.Extension
		env.Command = t.Command.Name
		env.Surface = runner.SurfaceMCP
		env.Tool = name
		// A read command must not write, even when the extension calls
		// `redmine api` with a mutating method by mistake.
		env.ReadOnly = env.ReadOnly || !t.Command.Writes

		res, err := runner.RunCommand(ctx, t.Path, t.Command, input, env.Environ(os.Environ()), nil)
		if err != nil {
			// The message carries the extension's stderr, which may quote
			// a response it got from Redmine.
			return extensionToolError(redactSecretText(describeRunError(name, err))), nil
		}
		return extensionToolResult(res.Stdout), nil
	}
}

// decodeArguments reads the tool arguments as a JSON object, keeping
// numbers as json.Number (see spec.DecodeArguments).
func decodeArguments(raw json.RawMessage) (map[string]any, error) {
	return spec.DecodeArguments(raw)
}

// normalizeNumbers converts the numbers of every argument except json
// params to the types the spec validation knows (see
// spec.NormalizeArguments, which `extension check --input` uses as well).
func normalizeNumbers(c *spec.Command, args map[string]any) map[string]any {
	return spec.NormalizeArguments(c, args)
}

// validationMessage joins the problems of a *spec.ValidationError into one
// line; other errors are returned as is.
func validationMessage(err error) string {
	var ve *spec.ValidationError
	if errors.As(err, &ve) && len(ve.Errors) > 0 {
		return strings.Join(ve.Details(), "; ")
	}
	return err.Error()
}

// describeRunError explains why running the tool's extension failed. The
// extension's stderr in an ExitError is already sanitized and truncated.
func describeRunError(tool string, err error) string {
	var (
		exitErr  *runner.ExitError
		timeout  *runner.TimeoutError
		tooLarge *runner.OutputTooLargeError
	)
	switch {
	case errors.As(err, &exitErr):
		msg := fmt.Sprintf("%s failed with exit status %d", tool, exitErr.Code)
		if exitErr.Message != "" {
			msg += ": " + exitErr.Message
		}
		return msg
	case errors.As(err, &timeout):
		return fmt.Sprintf("%s %s and was stopped", tool, timeout.Error())
	case errors.As(err, &tooLarge):
		return fmt.Sprintf("%s was stopped: its %s", tool, tooLarge.Error())
	default:
		return fmt.Sprintf("%s: %v", tool, err)
	}
}

// extensionToolError is a tool-level error result.
func extensionToolError(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: msg}},
	}
}

// extensionToolResult turns the extension's stdout into the tool result. A
// JSON object becomes structuredContent plus the same JSON as a text block
// (like generated tools); other JSON values are only text, since
// structuredContent must be an object; output that is not JSON is passed
// on as text. Every shape is redacted before it leaves the server, so an
// extension relaying /my/account.json cannot hand the API key to the
// client: JSON documents (one, or several in a row as NDJSON) through the
// structured redaction, anything else by masking keys in the text.
func extensionToolResult(stdout []byte) *mcp.CallToolResult {
	v, ok := runner.DecodeOutput(stdout)
	if !ok {
		text, isJSON := redactJSONStream(stdout)
		if !isJSON {
			text = redactSecretText(strings.ToValidUTF8(string(stdout), "�"))
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}
	}
	redactSecrets(&v)
	body, err := json.Marshal(v)
	if err != nil {
		return extensionToolError("failed to encode response: " + err.Error())
	}
	res := &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(body)}}}
	if obj, isObject := v.(map[string]any); isObject {
		res.StructuredContent = obj
	}
	return res
}

// redactJSONStream handles stdout that is a sequence of JSON documents
// (NDJSON, or two `redmine api` responses printed one after the other): it
// redacts each like a single result and returns them one per line. ok is
// false unless all of stdout is JSON.
func redactJSONStream(stdout []byte) (text string, ok bool) {
	dec := json.NewDecoder(bytes.NewReader(stdout))
	dec.UseNumber()
	var b strings.Builder
	n := 0
	for {
		var v any
		err := dec.Decode(&v)
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", false
		}
		redactSecrets(&v)
		line, err := json.Marshal(v)
		if err != nil {
			return "", false
		}
		b.Write(line)
		b.WriteByte('\n')
		n++
	}
	return b.String(), n > 0
}
