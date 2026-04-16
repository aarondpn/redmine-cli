package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderActionJSON(t *testing.T) {
	var buf bytes.Buffer
	env := ActionEnvelope{
		Ok:       true,
		Action:   ActionDeleted,
		Resource: "project",
		ID:       "demo",
		Message:  "Project \"demo\" deleted",
	}

	if err := RenderActionJSON(&buf, env); err != nil {
		t.Fatalf("RenderActionJSON: %v", err)
	}

	var got ActionEnvelope
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON emitted: %v\n%s", err, buf.String())
	}
	if !got.Ok {
		t.Errorf("ok: want true")
	}
	if got.Action != ActionDeleted {
		t.Errorf("action: got %q want %q", got.Action, ActionDeleted)
	}
	if got.Resource != "project" {
		t.Errorf("resource: got %q", got.Resource)
	}
	if got.ID != "demo" {
		t.Errorf("id: got %v", got.ID)
	}
}

func TestRenderErrorJSON(t *testing.T) {
	var buf bytes.Buffer
	env := ErrorEnvelope{
		Error: ErrorDetail{
			Message: "Resource not found.",
			Code:    ErrCodeNotFound,
		},
	}

	if err := RenderErrorJSON(&buf, env); err != nil {
		t.Fatalf("RenderErrorJSON: %v", err)
	}

	var got ErrorEnvelope
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON emitted: %v\n%s", err, buf.String())
	}
	if got.Error.Code != ErrCodeNotFound {
		t.Errorf("code: got %q want %q", got.Error.Code, ErrCodeNotFound)
	}
	if !strings.Contains(got.Error.Message, "not found") {
		t.Errorf("message: got %q", got.Error.Message)
	}
}

func TestStdPrinter_Action_JSONMode_WritesEnvelopeToStdout(t *testing.T) {
	var out, errOut bytes.Buffer
	p := NewStdPrinter(&out, &errOut, false, true, FormatJSON)

	p.Action(ActionDeleted, "issue", 42, "Deleted issue #42")

	if errOut.Len() != 0 {
		t.Errorf("expected nothing on stderr, got %q", errOut.String())
	}

	var env ActionEnvelope
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("stdout not JSON: %v\n%s", err, out.String())
	}
	if env.Action != ActionDeleted || env.Resource != "issue" {
		t.Errorf("envelope: %+v", env)
	}
	if id, ok := env.ID.(float64); !ok || id != 42 {
		t.Errorf("id: got %v, want 42", env.ID)
	}
}

func TestStdPrinter_Action_NonJSONMode_WritesToStderr(t *testing.T) {
	var out, errOut bytes.Buffer
	p := NewStdPrinter(&out, &errOut, false, true, FormatTable)

	p.Action(ActionDeleted, "issue", 42, "Deleted issue #42")

	if out.Len() != 0 {
		t.Errorf("expected nothing on stdout, got %q", out.String())
	}
	if !strings.Contains(errOut.String(), "Deleted issue #42") {
		t.Errorf("stderr: %q", errOut.String())
	}
}
