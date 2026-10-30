package ops

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

func TestResourceOps_SendCustomFieldValues(t *testing.T) {
	ctx := context.Background()
	cfv := models.CustomFieldValues{"3": {"a", "b"}, "4": {"x"}}
	wantValues := map[string]any{"3": []any{"a", "b"}, "4": "x"}

	tests := []struct {
		name   string
		call   func(c *api.Client) error
		method string
		path   string
		key    string
		reply  string
	}{
		{"CreateTimeEntry", func(c *api.Client) error {
			_, err := CreateTimeEntry(ctx, c, CreateTimeEntryInput{IssueID: 1, Hours: 1, CustomFieldValues: cfv})
			return err
		}, "POST", "/time_entries.json", "time_entry", `{"time_entry":{"id":1}}`},
		{"UpdateTimeEntry", func(c *api.Client) error {
			_, err := UpdateTimeEntry(ctx, c, UpdateTimeEntryInput{ID: 9, CustomFieldValues: cfv})
			return err
		}, "PUT", "/time_entries/9.json", "time_entry", ""},
		{"CreateVersion", func(c *api.Client) error {
			_, err := CreateVersion(ctx, c, CreateVersionInput{ProjectID: "demo", Name: "v", CustomFieldValues: cfv})
			return err
		}, "POST", "/projects/demo/versions.json", "version", `{"version":{"id":1}}`},
		{"UpdateVersion", func(c *api.Client) error {
			_, err := UpdateVersion(ctx, c, UpdateVersionInput{ID: 9, CustomFieldValues: cfv})
			return err
		}, "PUT", "/versions/9.json", "version", ""},
		{"CreateUser", func(c *api.Client) error {
			_, err := CreateUser(ctx, c, CreateUserInput{Login: "l", FirstName: "f", LastName: "l", Mail: "m@x.y", CustomFieldValues: cfv})
			return err
		}, "POST", "/users.json", "user", `{"user":{"id":1}}`},
		{"UpdateUser", func(c *api.Client) error {
			_, err := UpdateUser(ctx, c, UpdateUserInput{ID: 9, CustomFieldValues: cfv})
			return err
		}, "PUT", "/users/9.json", "user", ""},
		{"UpdateMyAccount", func(c *api.Client) error {
			_, err := UpdateMyAccount(ctx, c, UpdateMyAccountInput{CustomFieldValues: cfv})
			return err
		}, "PUT", "/my/account.json", "user", ""},
		{"CreateGroup", func(c *api.Client) error {
			_, err := CreateGroup(ctx, c, CreateGroupInput{Name: "g", CustomFieldValues: cfv})
			return err
		}, "POST", "/groups.json", "group", `{"group":{"id":1}}`},
		{"UpdateGroup", func(c *api.Client) error {
			_, err := UpdateGroup(ctx, c, UpdateGroupInput{ID: 9, CustomFieldValues: cfv})
			return err
		}, "PUT", "/groups/9.json", "group", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMethod, gotPath string
			var body map[string]map[string]any
			client, closeTS := newTestAPIClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod, gotPath = r.Method, r.URL.Path
				raw, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(raw, &body)
				if tt.reply == "" {
					w.WriteHeader(http.StatusNoContent)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(tt.reply))
			}))
			defer closeTS()

			if err := tt.call(client); err != nil {
				t.Fatalf("call: %v", err)
			}
			if gotMethod != tt.method || gotPath != tt.path {
				t.Errorf("request = %s %s, want %s %s", gotMethod, gotPath, tt.method, tt.path)
			}
			if got := body[tt.key]["custom_field_values"]; !reflect.DeepEqual(got, wantValues) {
				t.Errorf("custom_field_values = %v, want %v", got, wantValues)
			}
		})
	}
}

func TestCreateTimeEntry_OmitsCustomFieldValuesWhenUnset(t *testing.T) {
	var body map[string]map[string]any
	client, closeTS := newTestAPIClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"time_entry":{"id":1}}`))
	}))
	defer closeTS()

	if _, err := CreateTimeEntry(context.Background(), client, CreateTimeEntryInput{IssueID: 1, Hours: 1}); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["time_entry"]["custom_field_values"]; ok {
		t.Errorf("body should omit custom_field_values: %v", body)
	}
}
