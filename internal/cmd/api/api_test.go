package api

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	redmineapi "github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

func TestGetDefaultMethod(t *testing.T) {
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := NewCmdAPI(f)
	cmd.SetArgs([]string{"/test.json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if gotMethod != "GET" {
		t.Errorf("expected GET, got %s", gotMethod)
	}
}

func TestAutoPrependSlash(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := NewCmdAPI(f)
	cmd.SetArgs([]string{"issues.json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/issues.json" {
		t.Errorf("expected /issues.json, got %s", gotPath)
	}
}

func TestExplicitMethod(t *testing.T) {
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := NewCmdAPI(f)
	cmd.SetArgs([]string{"-X", "DELETE", "/issues/123.json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if gotMethod != "DELETE" {
		t.Errorf("expected DELETE, got %s", gotMethod)
	}
}

func TestQueryParamsField(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := NewCmdAPI(f)
	cmd.SetArgs([]string{"/issues.json", "-f", "project_id=demo", "-f", "limit=5"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotQuery, "project_id=demo") {
		t.Errorf("expected project_id=demo in query, got %s", gotQuery)
	}
	if !strings.Contains(gotQuery, "limit=5") {
		t.Errorf("expected limit=5 in query, got %s", gotQuery)
	}
}

func TestRawFieldsPOST(t *testing.T) {
	var gotMethod string
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := NewCmdAPI(f)
	cmd.SetArgs([]string{"/issues.json", "-F", "issue[subject]=Bug", "-F", "issue[project_id]=1"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if gotMethod != "POST" {
		t.Errorf("expected POST (auto-detected), got %s", gotMethod)
	}
	if gotBody["issue[subject]"] != "Bug" {
		t.Errorf("expected issue[subject]=Bug, got %v", gotBody["issue[subject]"])
	}
	// "1" should be parsed as number
	if gotBody["issue[project_id]"] != float64(1) {
		t.Errorf("expected issue[project_id]=1 (number), got %v (%T)", gotBody["issue[project_id]"], gotBody["issue[project_id]"])
	}
}

func TestJSONValueParsing(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		{"true", true},
		{"false", false},
		{"42", int64(42)},
		{"3.14", 3.14},
		{`[1,2,3]`, []interface{}{float64(1), float64(2), float64(3)}},
		{"hello", "hello"},
		{"NaN", "NaN"},
		{"inf", "inf"},
		{"-Infinity", "-Infinity"},
		{`{"id":9007199254740993}`, json.RawMessage(`{"id":9007199254740993}`)},
		{`[1,2] x`, `[1,2] x`},
		{`{"a":1}{"b":2}`, `{"a":1}{"b":2}`},
	}
	for _, tt := range tests {
		got := parseJSONValue(tt.input)
		gotJSON, _ := json.Marshal(got)
		expJSON, _ := json.Marshal(tt.expected)
		if string(gotJSON) != string(expJSON) {
			t.Errorf("parseJSONValue(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestInputFile(t *testing.T) {
	var gotMethod string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		buf := new(bytes.Buffer)
		buf.ReadFrom(r.Body)
		gotBody = buf.Bytes()
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	tmp := t.TempDir()
	bodyFile := tmp + "/body.json"
	os.WriteFile(bodyFile, []byte(`{"issue":{"subject":"test"}}`), 0644)

	f := testutil.NewFactory(t, srv.URL)
	cmd := NewCmdAPI(f)
	cmd.SetArgs([]string{"/issues.json", "--input", bodyFile})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if gotMethod != "POST" {
		t.Errorf("expected POST, got %s", gotMethod)
	}
	if string(gotBody) != `{"issue":{"subject":"test"}}` {
		t.Errorf("unexpected body: %s", gotBody)
	}
}

func TestInputStdin(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := new(bytes.Buffer)
		buf.ReadFrom(r.Body)
		gotBody = buf.String()
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	f.IOStreams.In = strings.NewReader(`{"test":1}`)
	cmd := NewCmdAPI(f)
	cmd.SetArgs([]string{"/issues.json", "--input", "-"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if gotBody != `{"test":1}` {
		t.Errorf("unexpected stdin body: %s", gotBody)
	}
}

func TestIncludeHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom", "value")
		w.WriteHeader(200)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := NewCmdAPI(f)
	cmd.SetArgs([]string{"/test.json", "-i"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := testutil.Stdout(f)
	if !strings.Contains(out, "200") {
		t.Errorf("expected status line with 200, got:\n%s", out)
	}
	if !strings.Contains(out, "X-Custom: value") {
		t.Errorf("expected X-Custom header, got:\n%s", out)
	}
}

// rawHTTPServer answers every connection with response verbatim, so a test
// can send a status line or header that net/http's server would not write.
func rawHTTPServer(t *testing.T, response string) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				_, _ = http.ReadRequest(bufio.NewReader(conn))
				_, _ = io.WriteString(conn, response)
			}()
		}
	}()
	return "http://" + ln.Addr().String()
}

// TestIncludeHeaders_SanitizesMetadata: the status line and header values are
// server-controlled metadata and are neutralized for the terminal, while the
// body is still written byte for byte.
func TestIncludeHeaders_SanitizesMetadata(t *testing.T) {
	body := "{\"raw\":\"\x1b[31m\"}"
	server := rawHTTPServer(t, "HTTP/1.1 200 OK\x1b]52;c;ZXZpbA==\x07\r\n"+
		// net/http rejects C0 controls in header values but passes C1
		// controls, raw 8-bit bytes (\x9b is an 8-bit CSI) and bidi marks.
		"X-Evil: a\u009b2Jb\x9b2Jc\u202Ed\r\n"+
		"Content-Length: "+strconv.Itoa(len(body))+"\r\n"+
		"Connection: close\r\n\r\n"+body)

	f := testutil.NewFactory(t, server)
	cmd := NewCmdAPI(f)
	cmd.SetArgs([]string{"/test.json", "-i"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	head, gotBody, _ := strings.Cut(testutil.Stdout(f), "\n\n")
	if strings.ContainsAny(head, "\x1b\x07\u009b\u202E") || strings.Contains(head, "\x9b") {
		t.Errorf("status line or headers contain raw control characters:\n%q", head)
	}
	if !strings.Contains(head, "200 OK^[]52;c;ZXZpbA==^G") {
		t.Errorf("expected the sanitized status line, got:\n%q", head)
	}
	if !strings.Contains(head, `X-Evil: a\u009b2Jb`+"\uFFFD"+`2Jcd`) {
		t.Errorf("expected the sanitized header value, got:\n%q", head)
	}
	if gotBody != body {
		t.Errorf("body = %q, want it unchanged %q", gotBody, body)
	}
}

func TestJSONOutput_PrettyPrintsBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ok":true,"id":42}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := NewCmdAPI(f)
	cmd.SetArgs([]string{"/test.json", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(testutil.Stdout(f)), &payload); err != nil {
		t.Fatalf("expected valid JSON output, got: %v", err)
	}
	if ok, _ := payload["ok"].(bool); !ok {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestJSONOutput_IncludeWrapsMetadata(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom", "value")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := NewCmdAPI(f)
	cmd.SetArgs([]string{"/test.json", "--output", "json", "-i"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	var payload struct {
		StatusCode int                 `json:"status_code"`
		Status     string              `json:"status"`
		Headers    map[string][]string `json:"headers"`
		Body       map[string]any      `json:"body"`
	}
	if err := json.Unmarshal([]byte(testutil.Stdout(f)), &payload); err != nil {
		t.Fatalf("expected valid JSON output, got: %v", err)
	}
	if payload.StatusCode != http.StatusCreated || payload.Body["ok"] != true {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	if values := payload.Headers["X-Custom"]; len(values) != 1 || values[0] != "value" {
		t.Fatalf("unexpected headers: %+v", payload.Headers)
	}
}

func TestSilentSuppressesOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":"secret"}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := NewCmdAPI(f)
	cmd.SetArgs([]string{"/test.json", "--silent"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if out := testutil.Stdout(f); out != "" {
		t.Errorf("expected no output with --silent, got: %s", out)
	}
}

func TestNon2xxReturnsErrorWithBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`{"errors":["not found"]}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := NewCmdAPI(f)
	cmd.SetArgs([]string{"/missing.json"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for 404")
	}
	var silentErr *cmdutil.SilentError
	if !errors.As(err, &silentErr) {
		t.Fatalf("expected SilentError, got %T: %v", err, err)
	}
	// Body should still be written.
	if out := testutil.Stdout(f); !strings.Contains(out, "not found") {
		t.Errorf("expected error body in output, got: %s", out)
	}
}

func TestJSONOutput_Non2xxReturnsAPIErrorWithoutWritingBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"errors":["not found"]}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := NewCmdAPI(f)
	cmd.SetArgs([]string{"/missing.json", "--output", "json"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for 404")
	}

	var apiErr *redmineapi.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", apiErr.StatusCode, http.StatusNotFound)
	}
	if len(apiErr.Errors) != 1 || apiErr.Errors[0] != "not found" {
		t.Fatalf("errors = %v, want [not found]", apiErr.Errors)
	}
	if out := testutil.Stdout(f); out != "" {
		t.Fatalf("expected no stdout body before main renders JSON envelope, got: %q", out)
	}
}

func TestJSONOutput_IncludeNon2xxWritesMetadataAndReturnsSilentError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"errors":["rate limited"]}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := NewCmdAPI(f)
	cmd.SetArgs([]string{"/limited.json", "--output", "json", "-i"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected non-zero result for 429")
	}

	var silentErr *cmdutil.SilentError
	if !errors.As(err, &silentErr) {
		t.Fatalf("expected SilentError, got %T: %v", err, err)
	}

	var payload struct {
		StatusCode int                 `json:"status_code"`
		Status     string              `json:"status"`
		Headers    map[string][]string `json:"headers"`
		Body       map[string]any      `json:"body"`
	}
	if err := json.Unmarshal([]byte(testutil.Stdout(f)), &payload); err != nil {
		t.Fatalf("expected valid JSON output, got: %v", err)
	}
	if payload.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", payload.StatusCode, http.StatusTooManyRequests)
	}
	if payload.Body["errors"] == nil {
		t.Fatalf("expected response body in payload, got %+v", payload.Body)
	}
	if values := payload.Headers["Retry-After"]; len(values) != 1 || values[0] != "60" {
		t.Fatalf("unexpected headers: %+v", payload.Headers)
	}
}

func TestMutualExclusion(t *testing.T) {
	f := testutil.NewFactory(t, "http://unused")
	cmd := NewCmdAPI(f)
	cmd.SetArgs([]string{"/test.json", "-F", "a=b", "--input", "file.json"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for mutual exclusion")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected mutually exclusive error, got: %v", err)
	}
}
