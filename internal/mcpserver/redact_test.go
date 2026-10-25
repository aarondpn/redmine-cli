package mcpserver

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

func TestRedactSecrets(t *testing.T) {
	type wrapper struct {
		User  *models.User        `json:"user"`
		Users []models.User       `json:"users"`
		ByID  map[int]models.User `json:"by_id"`
		Any   any                 `json:"any"`
		Raw   map[string]any      `json:"raw"`
	}
	type tagged struct {
		Key string `json:"api_key,omitempty"`
	}
	type taggedPtr struct {
		Key *string `json:"api_key,omitempty"`
	}
	// outer's first field shares its address, so a visited set keyed by
	// address alone would skip outer once Inner had been walked.
	type outer struct {
		Inner tagged      `json:"inner"`
		User  models.User `json:"user"`
	}

	tests := []struct {
		name string
		in   func() any
	}{
		{"pointer to user", func() any { return &models.User{ID: 1, APIKey: "secret"} }},
		{"pointer to pointer", func() any { u := &models.User{ID: 1, APIKey: "secret"}; return &u }},
		{"slice of users", func() any { return &[]models.User{{APIKey: "secret"}, {APIKey: "secret"}} }},
		{"custom struct tagged api_key", func() any { return &tagged{Key: "secret"} }},
		{"pointer-to-string api_key", func() any { k := "secret"; return &taggedPtr{Key: &k} }},
		{"struct visited after its first field", func() any {
			o := &outer{User: models.User{APIKey: "secret"}}
			return &[]any{&o.Inner, o}
		}},
		{"nested wrapper", func() any {
			return &wrapper{
				User:  &models.User{APIKey: "secret"},
				Users: []models.User{{APIKey: "secret"}},
				ByID:  map[int]models.User{1: {APIKey: "secret"}},
				Any:   models.User{APIKey: "secret"},
				Raw:   map[string]any{"api_key": "secret", "nested": map[string]any{"api_key": "secret"}},
			}
		}},
		{"interface holding struct value", func() any { var v any = models.User{APIKey: "secret"}; return &v }},
		{"interface holding pointer", func() any { var v any = &models.User{APIKey: "secret"}; return &v }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := tt.in()
			redactSecrets(v)
			raw, err := json.Marshal(v)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if strings.Contains(string(raw), "secret") || strings.Contains(string(raw), "api_key") {
				t.Errorf("secret survived redaction: %s", raw)
			}
		})
	}
}

func TestRedactSecrets_KeepsOtherFields(t *testing.T) {
	u := &models.User{ID: 7, Login: "alice", Mail: "a@example.com", APIKey: "secret",
		CustomFields: []models.CustomFieldValue{{ID: 1, Name: "Team"}}}
	redactSecrets(&u)
	if u.APIKey != "" {
		t.Fatalf("APIKey = %q, want empty", u.APIKey)
	}
	if u.ID != 7 || u.Login != "alice" || u.Mail != "a@example.com" || len(u.CustomFields) != 1 {
		t.Errorf("unrelated fields changed: %+v", u)
	}
}

func TestRedactSecrets_NonPointerAndNil(t *testing.T) {
	// Must not panic on values it cannot modify.
	redactSecrets(nil)
	redactSecrets(models.User{APIKey: "secret"})
	var nilUser *models.User
	redactSecrets(&nilUser)
	redactSecrets((*models.User)(nil))
}

// TestRedactSecrets_ToolAndResourceExits pins the backstop wiring: toolResult
// and jsonContent must redact on their own, independently of the explicit
// RedactUser calls in the user ops, so a future tool or resource that returns
// a user cannot leak its key.
func TestRedactSecrets_ToolAndResourceExits(t *testing.T) {
	type result struct {
		Users []models.User `json:"users"`
	}

	t.Run("toolResult", func(t *testing.T) {
		res, structured, err := toolResult(result{Users: []models.User{{Login: "alice", APIKey: "secret"}}})
		if err != nil {
			t.Fatal(err)
		}
		text, err := json.Marshal(res)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(text), "alice") || strings.Contains(string(text), "secret") {
			t.Errorf("text content = %s, want the user without its key", text)
		}
		if structured.Users[0].APIKey != "" || structured.Users[0].Login != "alice" {
			t.Errorf("structured value = %+v, want login kept and APIKey empty", structured.Users[0])
		}
	})

	t.Run("jsonContent", func(t *testing.T) {
		res, err := jsonContent("redmine://test/1", &models.User{Login: "alice", APIKey: "secret"})
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Contents) != 1 || !strings.Contains(res.Contents[0].Text, "alice") {
			t.Fatalf("unexpected contents: %+v", res.Contents)
		}
		if strings.Contains(res.Contents[0].Text, "secret") || strings.Contains(res.Contents[0].Text, "api_key") {
			t.Errorf("resource body leaked the key: %s", res.Contents[0].Text)
		}
	})
}
