package ops

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

// bodyRecorder answers every request with reply (204 when empty) and keeps
// the decoded body of the last write.
func bodyRecorder(t *testing.T, reply string) (*api.Client, *map[string]any) {
	t.Helper()
	var body map[string]any
	client, closeTS := newTestAPIClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		body = nil
		_ = json.Unmarshal(raw, &body)
		if reply == "" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(reply))
	}))
	t.Cleanup(closeTS)
	return client, &body
}

func ptrTo[T any](v T) *T { return &v }

func TestWritableFields_ReachTheRequestBody(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name  string
		reply string
		call  func(c *api.Client) error
		want  map[string]any
	}{
		{"CreateUser", `{"user":{"id":1}}`, func(c *api.Client) error {
			_, err := CreateUser(ctx, c, CreateUserInput{Login: "l", FirstName: "f", LastName: "l", Mail: "m@x.y",
				Status: ptrTo(2), Language: ptrTo("de"), Pref: models.UserPreferences{"hide_mail": "1"}})
			return err
		}, map[string]any{
			"user": map[string]any{"login": "l", "firstname": "f", "lastname": "l", "mail": "m@x.y", "status": 2.0, "language": "de"},
			"pref": map[string]any{"hide_mail": "1"},
		}},
		{"UpdateUser", "", func(c *api.Client) error {
			_, err := UpdateUser(ctx, c, UpdateUserInput{ID: 9, Language: ptrTo("fr"),
				GroupIDs: &[]int{1, 2}, NotifiedProjectIDs: &[]int{}, Pref: models.UserPreferences{"time_zone": "UTC"}})
			return err
		}, map[string]any{
			"user": map[string]any{"language": "fr", "group_ids": []any{1.0, 2.0}, "notified_project_ids": []any{}},
			"pref": map[string]any{"time_zone": "UTC"},
		}},
		{"UpdateUserPassword", "", func(c *api.Client) error {
			_, err := UpdateUser(ctx, c, UpdateUserInput{ID: 9, Password: "n3w pass"})
			return err
		}, map[string]any{
			"user": map[string]any{"password": "n3w pass", "password_confirmation": "n3w pass"},
		}},
		{"UpdateMyAccount", "", func(c *api.Client) error {
			_, err := UpdateMyAccount(ctx, c, UpdateMyAccountInput{Language: ptrTo("en"), NotifiedProjectIDs: &[]int{3},
				Pref: models.UserPreferences{"no_self_notified": "1"}})
			return err
		}, map[string]any{
			"user": map[string]any{"language": "en", "notified_project_ids": []any{3.0}},
			"pref": map[string]any{"no_self_notified": "1"},
		}},
		{"CreateGroup", `{"group":{"id":1}}`, func(c *api.Client) error {
			_, err := CreateGroup(ctx, c, CreateGroupInput{Name: "g", TwofaRequired: ptrTo(true)})
			return err
		}, map[string]any{"group": map[string]any{"name": "g", "twofa_required": true}}},
		{"UpdateGroup", "", func(c *api.Client) error {
			_, err := UpdateGroup(ctx, c, UpdateGroupInput{ID: 4, TwofaRequired: ptrTo(false)})
			return err
		}, map[string]any{"group": map[string]any{"twofa_required": false}}},
		{"CreateProject", `{"project":{"id":1}}`, func(c *api.Client) error {
			_, err := CreateProject(ctx, c, CreateProjectInput{Name: "P", Identifier: "p", DefaultIssueQueryID: 7})
			return err
		}, map[string]any{"project": map[string]any{"name": "P", "identifier": "p", "default_issue_query_id": 7.0}}},
		{"UpdateProject", "", func(c *api.Client) error {
			_, err := UpdateProject(ctx, c, UpdateProjectInput{Identifier: "p", DefaultIssueQueryID: ptrTo(7)})
			return err
		}, map[string]any{"project": map[string]any{"default_issue_query_id": 7.0}}},
		{"CreateVersion", `{"version":{"id":1}}`, func(c *api.Client) error {
			_, err := CreateVersion(ctx, c, CreateVersionInput{ProjectID: "p", Name: "v", DefaultProjectVersion: true})
			return err
		}, map[string]any{"version": map[string]any{"name": "v", "default_project_version": true}}},
		{"UpdateVersion", "", func(c *api.Client) error {
			_, err := UpdateVersion(ctx, c, UpdateVersionInput{ID: 3, DefaultProjectVersion: true})
			return err
		}, map[string]any{"version": map[string]any{"default_project_version": true}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, body := bodyRecorder(t, tt.reply)
			if err := tt.call(client); err != nil {
				t.Fatalf("call: %v", err)
			}
			if !reflect.DeepEqual(*body, tt.want) {
				t.Errorf("body = %v\nwant   %v", *body, tt.want)
			}
		})
	}
}

// membershipServer serves POST /projects/demo/memberships.json with
// createStatus/createReply and the project's memberships list with listReply.
func membershipServer(t *testing.T, createStatus int, createReply, listReply string, posted *map[string]any, lists *int) *api.Client {
	t.Helper()
	client, closeTS := newTestAPIClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/projects/demo/memberships.json":
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, posted)
			w.WriteHeader(createStatus)
			_, _ = w.Write([]byte(createReply))
		case r.Method == http.MethodGet && r.URL.Path == "/projects/demo/memberships.json":
			*lists++
			_, _ = w.Write([]byte(listReply))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(closeTS)
	return client
}

const (
	membership5 = `{"id":11,"project":{"id":1,"name":"Demo"},"user":{"id":5,"name":"Ann"},"roles":[{"id":3,"name":"Dev"}]}`
	membership6 = `{"id":12,"project":{"id":1,"name":"Demo"},"user":{"id":6,"name":"Bob"},"roles":[{"id":3,"name":"Dev"}]}`
	membershipG = `{"id":13,"project":{"id":1,"name":"Demo"},"group":{"id":9,"name":"QA"},"roles":[{"id":3,"name":"Dev"}]}`
)

func TestCreateMemberships_SingleUserKeepsUserID(t *testing.T) {
	var posted map[string]any
	lists := 0
	client := membershipServer(t, http.StatusCreated, `{"membership":`+membership5+`}`, "", &posted, &lists)

	m, err := CreateMembership(context.Background(), client, CreateMembershipInput{ProjectID: "demo", UserID: 5, RoleIDs: []int{3}})
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != 11 || lists != 0 {
		t.Errorf("membership = %+v, list calls = %d", m, lists)
	}
	want := map[string]any{"membership": map[string]any{"user_id": 5.0, "role_ids": []any{3.0}}}
	if !reflect.DeepEqual(posted, want) {
		t.Errorf("body = %v", posted)
	}

	// A one-element user_ids is sent as user_id too.
	if _, err := CreateMemberships(context.Background(), client, CreateMembershipInput{ProjectID: "demo", UserIDs: []int{5}, RoleIDs: []int{3}}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(posted, want) {
		t.Errorf("one-element user_ids body = %v", posted)
	}
}

func TestCreateMemberships_BulkReadsBackEveryPrincipal(t *testing.T) {
	var posted map[string]any
	lists := 0
	client := membershipServer(t, http.StatusCreated, `{"membership":`+membership5+`}`,
		`{"memberships":[`+membership6+`,`+membershipG+`,`+membership5+`],"total_count":3}`, &posted, &lists)

	ms, err := CreateMemberships(context.Background(), client, CreateMembershipInput{ProjectID: "demo", UserIDs: []int{5, 9, 6}, RoleIDs: []int{3}})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"membership": map[string]any{"user_ids": []any{5.0, 9.0, 6.0}, "role_ids": []any{3.0}}}
	if !reflect.DeepEqual(posted, want) {
		t.Errorf("body = %v", posted)
	}
	if lists != 1 || len(ms) != 3 || ms[0].ID != 11 || ms[1].ID != 13 || ms[2].ID != 12 {
		t.Errorf("memberships = %+v (list calls %d), want IDs 11, 13, 12 in input order", ms, lists)
	}

	m, err := CreateMembership(context.Background(), client, CreateMembershipInput{ProjectID: "demo", UserIDs: []int{6, 5}, RoleIDs: []int{3}})
	if err != nil || m.ID != 12 {
		t.Errorf("CreateMembership = %+v, %v; want the first requested principal's membership (12)", m, err)
	}
}

func TestCreateMemberships_BulkDedupesIDs(t *testing.T) {
	var posted map[string]any
	lists := 0
	client := membershipServer(t, http.StatusCreated, `{"membership":`+membership5+`}`,
		`{"memberships":[`+membership5+`,`+membership6+`],"total_count":2}`, &posted, &lists)

	ms, err := CreateMemberships(context.Background(), client, CreateMembershipInput{ProjectID: "demo", UserIDs: []int{5, 6, 5}, RoleIDs: []int{3}})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"membership": map[string]any{"user_ids": []any{5.0, 6.0}, "role_ids": []any{3.0}}}
	if !reflect.DeepEqual(posted, want) || len(ms) != 2 {
		t.Errorf("body = %v, memberships = %+v", posted, ms)
	}

	// Duplicates of a single principal collapse to user_id.
	if _, err := CreateMemberships(context.Background(), client, CreateMembershipInput{ProjectID: "demo", UserIDs: []int{5, 5}, RoleIDs: []int{3}}); err != nil {
		t.Fatal(err)
	}
	if want := map[string]any{"membership": map[string]any{"user_id": 5.0, "role_ids": []any{3.0}}}; !reflect.DeepEqual(posted, want) {
		t.Errorf("body = %v", posted)
	}
}

func TestCreateMemberships_BulkRejectedBatch(t *testing.T) {
	for _, tt := range []struct {
		name    string
		status  int
		reply   string
		wantMsg string
	}{
		{"422", http.StatusUnprocessableEntity, `{"errors":["User has already been taken"]}`, "(Redmine: User has already been taken)"},
		{"500 for an invalid later principal", http.StatusInternalServerError, ``, BatchRejectedMessage},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var posted map[string]any
			lists := 0
			client := membershipServer(t, tt.status, tt.reply, `{"memberships":[]}`, &posted, &lists)
			_, err := CreateMemberships(context.Background(), client, CreateMembershipInput{ProjectID: "demo", UserIDs: []int{5, 6}, RoleIDs: []int{3}})
			var apiErr *api.APIError
			if !errors.As(err, &apiErr) || !apiErr.IsValidationError() {
				t.Fatalf("err = %v, want a validation error", err)
			}
			if !strings.Contains(err.Error(), BatchRejectedMessage) || !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("err = %v", err)
			}
			if lists != 0 {
				t.Errorf("a rejected batch must not be read back (list calls %d)", lists)
			}
		})
	}
}

func TestCreateMemberships_BulkReadBackMissingPrincipal(t *testing.T) {
	var posted map[string]any
	lists := 0
	client := membershipServer(t, http.StatusCreated, `{"membership":`+membership5+`}`,
		`{"memberships":[`+membership5+`],"total_count":1}`, &posted, &lists)
	_, err := CreateMemberships(context.Background(), client, CreateMembershipInput{ProjectID: "demo", UserIDs: []int{5, 6}, RoleIDs: []int{3}})
	if err == nil || !strings.Contains(err.Error(), "lists none for principal 6") {
		t.Fatalf("err = %v", err)
	}
}

func TestCreateMemberships_RejectsAmbiguousOrMissingPrincipal(t *testing.T) {
	client, _ := bodyRecorder(t, "")
	ctx := context.Background()
	if _, err := CreateMemberships(ctx, client, CreateMembershipInput{ProjectID: "demo", UserID: 5, UserIDs: []int{6}, RoleIDs: []int{3}}); err == nil || !strings.Contains(err.Error(), "not both") {
		t.Errorf("user_id + user_ids: err = %v", err)
	}
	if _, err := CreateMemberships(ctx, client, CreateMembershipInput{ProjectID: "demo", RoleIDs: []int{3}}); err == nil || !strings.Contains(err.Error(), "required") {
		t.Errorf("no principal: err = %v", err)
	}
}
