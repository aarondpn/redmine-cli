package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	redmineapi "github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

// NewCmdAPI creates the `api` command for raw Redmine API access.
func NewCmdAPI(f *cmdutil.Factory) *cobra.Command {
	var (
		method    string
		fields    []string
		rawFields []string
		input     string
		include   bool
		silent    bool
		format    string
	)

	cmd := &cobra.Command{
		Use:   "api <endpoint> [flags]",
		Short: "Make an authenticated API request",
		Long: `Make an authenticated HTTP request to the Redmine API and print the response.

The endpoint argument should be a path like "/issues.json" or "issues.json"
(the leading slash is optional).

The default HTTP method is GET. When --raw-field or --input is provided,
the method defaults to POST instead.

Each --raw-field sets a top-level key of the JSON body. Numbers, true/false,
arrays and objects are sent as JSON, anything else (including invalid JSON)
as a string. A value that looks like a number is sent as one ('1.10' becomes
1.1), so put such strings inside a JSON object. Bracket keys like
'issue[subject]' are not expanded, so pass nested objects as JSON:
-F 'issue={"subject":"Bug"}'.`,
		Example: `  # GET the current user
  redmine api /users/current.json

  # GET issues with query parameters
  redmine api /issues.json -f project_id=myproject -f limit=5

  # POST with a nested JSON field
  redmine api /issues.json -F 'issue={"subject":"Bug report","project_id":1}'

  # POST from a file
  redmine api /issues.json --input body.json

  # DELETE an issue
  redmine api -X DELETE /issues/123.json

  # Show response headers
  redmine api /issues.json -i`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAPI(cmdutil.Context(cmd), f, args[0], method, fields, rawFields, input, include, silent, format)
		},
	}

	cmd.Flags().StringVarP(&method, "method", "X", "", "HTTP method (default: auto)")
	cmd.Flags().StringArrayVarP(&fields, "field", "f", nil, "Query parameter as key=value")
	cmd.Flags().StringArrayVarP(&rawFields, "raw-field", "F", nil, "JSON body field as key=value")
	cmd.Flags().StringVar(&input, "input", "", "Read request body from file (use - for stdin)")
	cmd.Flags().BoolVarP(&include, "include", "i", false, "Show response status and headers")
	cmd.Flags().BoolVar(&silent, "silent", false, "Suppress response output")
	cmdutil.AddOutputFlag(cmd, &format)

	return cmd
}

func runAPI(ctx context.Context, f *cmdutil.Factory, endpoint, method string, fields, rawFields []string, input string, include, silent bool, format string) error {
	// Validate mutual exclusion.
	if len(rawFields) > 0 && input != "" {
		return fmt.Errorf("--raw-field and --input are mutually exclusive")
	}

	// Normalise endpoint.
	if !strings.HasPrefix(endpoint, "/") {
		endpoint = "/" + endpoint
	}

	// Auto-detect HTTP method.
	if method == "" {
		if len(rawFields) > 0 || input != "" {
			method = "POST"
		} else {
			method = "GET"
		}
	}
	method = strings.ToUpper(method)

	// Build query params from -f flags.
	params := url.Values{}
	for _, fv := range fields {
		k, v, ok := strings.Cut(fv, "=")
		if !ok {
			return fmt.Errorf("invalid --field value %q: expected key=value", fv)
		}
		params.Add(k, v)
	}

	// Build request body.
	var body io.Reader
	if len(rawFields) > 0 {
		b, err := buildJSONBody(rawFields)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	} else if input != "" {
		b, err := readInput(input, f.IOStreams.In)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}

	client, err := f.ApiClient()
	if err != nil {
		return err
	}

	resp, err := client.DoRaw(ctx, method, endpoint, params, body)
	if err != nil {
		return err
	}

	printer := f.Printer(format)
	if resp.StatusCode >= 400 {
		if silent {
			return &cmdutil.SilentError{Code: 1}
		}
		if printer.Format() == output.FormatJSON && !include {
			return rawResponseError(resp, endpoint)
		}
	}

	if !silent {
		renderAPIResponse(printer, f.IOStreams.Out, f.IOStreams.IsTTY, resp, include)
	}

	if resp.StatusCode >= 400 {
		return &cmdutil.SilentError{Code: 1}
	}
	return nil
}

func renderAPIResponse(printer output.Printer, out io.Writer, isTTY bool, resp *redmineapi.RawResponse, include bool) {
	if printer.Format() == output.FormatJSON {
		renderAPIJSON(printer, resp, include)
		return
	}

	if include {
		// The status line and headers are metadata the server controls, so
		// they are always sanitized like other terminal output; the body
		// below is written as received.
		fmt.Fprintf(out, "%s\n", output.SanitizeTerminal(resp.Status))
		// Sort header keys for stable output.
		keys := make([]string, 0, len(resp.Headers))
		for k := range resp.Headers {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			for _, v := range resp.Headers[k] {
				fmt.Fprintf(out, "%s: %s\n", output.SanitizeTerminal(k), output.SanitizeTerminal(v))
			}
		}
		fmt.Fprintln(out)
	}

	if len(resp.Body) > 0 {
		if isTTY && isJSON(resp.Body) {
			var buf bytes.Buffer
			if json.Indent(&buf, resp.Body, "", "  ") == nil {
				buf.WriteByte('\n')
				_, _ = buf.WriteTo(out)
				return
			}
		}
		_, _ = out.Write(resp.Body)
	}
}

func renderAPIJSON(printer output.Printer, resp *redmineapi.RawResponse, include bool) {
	body := any(nil)
	if len(resp.Body) > 0 {
		body = decodeAPIJSONBody(resp.Body)
	}
	if include {
		printer.JSON(map[string]any{
			"status_code": resp.StatusCode,
			"status":      resp.Status,
			"headers":     resp.Headers,
			"body":        body,
		})
		return
	}
	printer.JSON(body)
}

// decodeAPIJSONBody decodes a response body for JSON output. Numbers are kept
// as json.Number so large integers survive -o json and --jq unchanged.
func decodeAPIJSONBody(body []byte) any {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err == nil {
		// Reject trailing data, as json.Unmarshal would.
		if _, err := dec.Token(); err == io.EOF {
			return value
		}
	}
	return map[string]any{"body": string(body)}
}

func rawResponseError(resp *redmineapi.RawResponse, endpoint string) error {
	var payload struct {
		Errors []string `json:"errors"`
	}
	var messages []string
	if err := json.Unmarshal(resp.Body, &payload); err == nil && len(payload.Errors) > 0 {
		messages = payload.Errors
	}
	return redmineapi.NewAPIError(resp.StatusCode, endpoint, messages)
}

// buildJSONBody constructs a JSON object from key=value pairs.
// Values are parsed as JSON (numbers, bools, arrays, objects); strings are kept as-is.
func buildJSONBody(fields []string) ([]byte, error) {
	obj := make(map[string]interface{})
	for _, fv := range fields {
		k, v, ok := strings.Cut(fv, "=")
		if !ok {
			return nil, fmt.Errorf("invalid --raw-field value %q: expected key=value", fv)
		}
		obj[k] = parseJSONValue(v)
	}
	return json.Marshal(obj)
}

// parseJSONValue attempts to parse s as a JSON value. If parsing fails it
// returns s as a plain string.
func parseJSONValue(s string) interface{} {
	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return n
	}
	// NaN and infinities parse as floats but can't be encoded as JSON.
	if f, err := strconv.ParseFloat(s, 64); err == nil && !math.IsNaN(f) && !math.IsInf(f, 0) {
		return f
	}
	// Try JSON array/object. Numbers are kept as json.Number so large
	// integers survive unchanged.
	if (strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]")) ||
		(strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")) {
		dec := json.NewDecoder(strings.NewReader(s))
		dec.UseNumber()
		var v interface{}
		if dec.Decode(&v) == nil {
			// Reject trailing data, as json.Unmarshal would.
			if _, err := dec.Token(); err == io.EOF {
				return v
			}
		}
	}
	return s
}

// readInput reads the request body from a file path or stdin (when path is "-").
func readInput(path string, stdin io.Reader) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(stdin)
	}
	return os.ReadFile(path)
}

// isJSON reports whether data looks like a JSON value.
func isJSON(data []byte) bool {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return false
	}
	return data[0] == '{' || data[0] == '['
}
