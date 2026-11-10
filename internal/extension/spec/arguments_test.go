package spec

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDecodeArguments(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr string
	}{
		{name: "empty", raw: "  ", want: `{}`},
		{name: "null", raw: "null", want: `{}`},
		{name: "object keeps digits", raw: `{"a":12345678901234567890,"b":1.0}`, want: `{"a":12345678901234567890,"b":1.0}`},
		{name: "array", raw: `[1]`, wantErr: "must be a JSON object"},
		{name: "invalid", raw: `{`, wantErr: "invalid JSON"},
		{name: "trailing data", raw: `{} {}`, wantErr: "unexpected data"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecodeArguments([]byte(tt.raw))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if b, _ := json.Marshal(got); string(b) != tt.want {
				t.Fatalf("got %s, want %s", b, tt.want)
			}
		})
	}
}

// TestCLIAndMCPInputAgree sends the same input through the CLI flags and
// through MCP-style arguments: the extension must get the same stdin.
func TestCLIAndMCPInputAgree(t *testing.T) {
	c := &Command{Name: "echo", Description: "d", Params: []Param{
		{Name: "id", Type: TypeInteger, Positional: true, Required: true},
		{Name: "ratio", Type: TypeNumber},
		{Name: "extra", Type: TypeJSON},
		{Name: "items", Type: TypeJSON, Array: true},
	}}
	extra := `{"a":12345678901234567890,"f":1.0,"e":1e2,"n":[-0.5,9007199254740993]}`
	cli, err := runCommand(t, c, "", "5", "--ratio", "2.5", "--extra", extra, "--items", "1.50", "--items", `"x"`)
	if err != nil {
		t.Fatal(err)
	}
	args, err := DecodeArguments([]byte(`{"id":5,"ratio":2.5,"extra":` + extra + `,"items":[1.50,"x"]}`))
	if err != nil {
		t.Fatal(err)
	}
	mcp, err := PrepareInput(c, NormalizeArguments(c, args))
	if err != nil {
		t.Fatal(err)
	}
	cliJSON, _ := json.Marshal(cli)
	mcpJSON, _ := json.Marshal(mcp)
	if string(cliJSON) != string(mcpJSON) {
		t.Fatalf("CLI input %s\nMCP input %s", cliJSON, mcpJSON)
	}
	if !strings.Contains(string(cliJSON), `"a":12345678901234567890`) || !strings.Contains(string(cliJSON), `"f":1.0`) {
		t.Fatalf("json value lost its digits: %s", cliJSON)
	}
}
