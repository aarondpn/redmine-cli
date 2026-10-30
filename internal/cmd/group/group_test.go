package group

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

// --- list ---

func TestGroupList_EmptyJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"groups":[],"total_count":0}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdGroupList(f)
	cmd.SetArgs([]string{"--output", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := testutil.Stdout(f); got != "[]\n" {
		t.Fatalf("stdout = %q, want %q", got, "[]\n")
	}
}

func TestGroupList_Table(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"groups":[{"id":1,"name":"Developers"},{"id":2,"name":"QA"}],"total_count":2}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdGroupList(f)
	cmd.SetArgs([]string{"--output", "table"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	stdout := testutil.Stdout(f)
	for _, want := range []string{"Developers", "QA"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
}

func TestGroupList_PaginationWarning(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"groups":[{"id":1,"name":"Developers"}],"total_count":5}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdGroupList(f)
	cmd.SetArgs([]string{"--output", "table", "--limit", "1"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if stderr := testutil.Stderr(f); !strings.Contains(stderr, "Showing 1 of 5 groups") {
		t.Fatalf("stderr = %q, want pagination warning", stderr)
	}
}

// --- get ---

func TestGroupGet_Detail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/groups/1.json" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"group":{"id":1,"name":"Developers"}}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdGroupGet(f)
	cmd.SetArgs([]string{"1"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	stdout := testutil.Stdout(f)
	if !strings.Contains(stdout, "Developers") {
		t.Errorf("detail output missing 'Developers':\n%s", stdout)
	}
}

func TestGroupGet_MembershipsShowProjectAndKeepRolesInJSON(t *testing.T) {
	const body = `{"group":{"id":1,"name":"Developers","builtin":"non_member","custom_fields":[{"id":9,"name":"Cost center","value":"42"}],"memberships":[{"id":7,"project":{"id":3,"name":"Apollo"},"roles":[{"id":4,"name":"Developer"}]}]}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("include"); got != "memberships" {
			t.Errorf("include = %q, want memberships", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdGroupGet(f)
	cmd.SetArgs([]string{"1", "--include", "memberships"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if out := testutil.Stdout(f); !strings.Contains(out, "Apollo (ID: 7)") {
		t.Errorf("detail output missing project name:\n%s", out)
	}

	f = testutil.NewFactory(t, srv.URL)
	cmd = newCmdGroupGet(f)
	cmd.SetArgs([]string{"1", "--include", "memberships", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := testutil.Stdout(f)
	for _, want := range []string{`"builtin": "non_member"`, `"Apollo"`, `"Developer"`, `"custom_fields"`} {
		if !strings.Contains(out, want) {
			t.Errorf("json missing %s:\n%s", want, out)
		}
	}
}

// --- create ---

func TestGroupCreate_Success(t *testing.T) {
	var capturedMethod string
	var capturedBody map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"group":{"id":10,"name":"QA"}}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdGroupCreate(f)
	cmd.SetArgs([]string{"--name", "QA"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if capturedMethod != "POST" {
		t.Errorf("expected POST, got %s", capturedMethod)
	}

	// Verify the request payload contains the --name flag value.
	group, _ := capturedBody["group"].(map[string]interface{})
	if group == nil {
		t.Fatal("request body missing group wrapper")
	}
	if group["name"] != "QA" {
		t.Errorf("payload name = %v, want 'QA'", group["name"])
	}

	if stderr := testutil.Stderr(f); !strings.Contains(stderr, "QA") {
		t.Errorf("stderr = %q, want success message mentioning group name", stderr)
	}
}

func TestGroupCreate_MissingName(t *testing.T) {
	f := testutil.NewFactory(t, "http://unused")
	cmd := newCmdGroupCreate(f)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing --name")
	}
}

// --- delete ---

func TestGroupDelete_Force(t *testing.T) {
	var capturedMethod, capturedPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdGroupDelete(f)
	cmd.SetArgs([]string{"1", "--force"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if capturedMethod != "DELETE" {
		t.Errorf("expected DELETE, got %s", capturedMethod)
	}
	if capturedPath != "/groups/1.json" {
		t.Errorf("expected /groups/1.json, got %s", capturedPath)
	}
	if stderr := testutil.Stderr(f); !strings.Contains(stderr, "Deleted group 1") {
		t.Errorf("stderr = %q, want success message", stderr)
	}
}

// Fixture mirrors groups/show.api.rsb: membership entries carry id, project
// and roles but no top-level name.
const groupWithMembershipsBody = `{"group":{"id":1,"name":"Developers","memberships":[
{"id":10,"project":{"id":3,"name":"Apollo"},"roles":[{"id":4,"name":"Developer"},{"id":5,"name":"Reporter","inherited":true}]}]}}`

func TestGroupGet_IncludeMembershipsTable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("include"); got != "memberships" {
			t.Errorf("include = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(groupWithMembershipsBody))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdGroupGet(f)
	cmd.SetArgs([]string{"1", "--include-memberships"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	stdout := testutil.Stdout(f)
	for _, want := range []string{"Apollo (ID: 3)", "Developer", "Reporter"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("output missing %q:\n%s", want, stdout)
		}
	}
}

func TestGroupGet_IncludeMembershipsJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(groupWithMembershipsBody))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdGroupGet(f)
	cmd.SetArgs([]string{"1", "--include-memberships", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Memberships []struct {
			Project struct {
				Name string `json:"name"`
			} `json:"project"`
			Roles []struct {
				Name string `json:"name"`
			} `json:"roles"`
		} `json:"memberships"`
	}
	if err := json.Unmarshal([]byte(testutil.Stdout(f)), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Memberships) != 1 || got.Memberships[0].Project.Name != "Apollo" || len(got.Memberships[0].Roles) != 2 {
		t.Errorf("memberships = %+v", got.Memberships)
	}
}
