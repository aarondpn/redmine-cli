package api

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

// setLimit lowers a response size limit for the duration of a test.
func setLimit(t *testing.T, limit *int64, v int64) {
	t.Helper()
	old := *limit
	*limit = v
	t.Cleanup(func() { *limit = old })
}

func bodyServer(status int, body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

func TestGet_ResponseSizeLimit(t *testing.T) {
	setLimit(t, &maxJSONResponseBytes, 64)
	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{"at limit", `{"v":"` + strings.Repeat("a", 64-8) + `"}`, false},
		{"over limit", `{"v":"` + strings.Repeat("a", 200) + `"}`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := bodyServer(http.StatusOK, tt.body)
			defer ts.Close()

			var out map[string]string
			err := newTestClient(ts).Get(context.Background(), "/x.json", nil, &out)
			var tooLarge *ResponseTooLargeError
			if got := errors.As(err, &tooLarge); got != tt.wantErr {
				t.Fatalf("err = %v, want ResponseTooLargeError = %v", err, tt.wantErr)
			}
		})
	}
}

func TestDoRaw_ResponseSizeLimit(t *testing.T) {
	setLimit(t, &maxRawResponseBytes, 32)
	ts := bodyServer(http.StatusOK, strings.Repeat("x", 100))
	defer ts.Close()

	_, err := newTestClient(ts).DoRaw(context.Background(), http.MethodGet, "/x", nil, nil)
	var tooLarge *ResponseTooLargeError
	if !errors.As(err, &tooLarge) {
		t.Fatalf("err = %v, want ResponseTooLargeError", err)
	}
	if !strings.Contains(err.Error(), "32 byte limit") {
		t.Errorf("err = %q, want the limit in the message", err)
	}
}

func TestUpload_ResponseSizeLimit(t *testing.T) {
	setLimit(t, &maxJSONResponseBytes, 32)
	ts := bodyServer(http.StatusCreated, `{"upload":{"token":"`+strings.Repeat("t", 100)+`"}}`)
	defer ts.Close()

	c := newTestClient(ts)
	c.Attachments = &AttachmentService{client: c}
	_, err := c.Attachments.Upload(context.Background(), "f.bin", strings.NewReader("x"), 1)
	var tooLarge *ResponseTooLargeError
	if !errors.As(err, &tooLarge) {
		t.Fatalf("err = %v, want ResponseTooLargeError (not a decode error)", err)
	}
}

func TestResponseTooLargeError_Message(t *testing.T) {
	err := &ResponseTooLargeError{Limit: 256 << 20}
	if got := err.Error(); got != "response body exceeds the 256 MiB limit" {
		t.Errorf("Error() = %q", got)
	}
}

func TestAttachmentDownload_NotSizeLimited(t *testing.T) {
	setLimit(t, &maxJSONResponseBytes, 16)
	setLimit(t, &maxRawResponseBytes, 16)
	payload := strings.Repeat("z", 4096)
	ts := bodyServer(http.StatusOK, payload)
	defer ts.Close()

	c := newTestClient(ts)
	c.Attachments = &AttachmentService{client: c}
	var buf bytes.Buffer
	if _, err := c.Attachments.Download(context.Background(), &models.Attachment{ID: 1, Filename: "z"}, &buf); err != nil {
		t.Fatalf("download failed: %v", err)
	}
	if buf.Len() != len(payload) {
		t.Errorf("downloaded %d bytes, want %d", buf.Len(), len(payload))
	}
}

func TestParseErrorResponse_TruncatesOversizedBody(t *testing.T) {
	setLimit(t, &maxErrorBodyBytes, 64)
	body := `{"errors":["Subject is blank","Tracker is invalid","` + strings.Repeat("x", 4096) + `"]}`
	ts := bodyServer(http.StatusUnprocessableEntity, body)
	defer ts.Close()

	var out map[string]any
	err := newTestClient(ts).Get(context.Background(), "/issues.json", nil, &out)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want *APIError (an oversized error body must not fail the parse)", err)
	}
	want := []string{"Subject is blank", "Tracker is invalid", "…"}
	if strings.Join(apiErr.Errors, "|") != strings.Join(want, "|") {
		t.Errorf("Errors = %q, want %q", apiErr.Errors, want)
	}
	if apiErr.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("StatusCode = %d, want 422", apiErr.StatusCode)
	}
}

func TestParseErrorResponse_OversizedNonJSONBody(t *testing.T) {
	setLimit(t, &maxErrorBodyBytes, 64)
	ts := bodyServer(http.StatusInternalServerError, strings.Repeat("<html>", 1000))
	defer ts.Close()

	err := newTestClient(ts).Get(context.Background(), "/issues.json", nil, nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusInternalServerError || len(apiErr.Errors) != 0 {
		t.Fatalf("err = %#v, want a plain 500 APIError", err)
	}
}

func TestParseErrorResponse_SanitizesAndScrubs(t *testing.T) {
	body := `{"errors":["Subject \u001b[31mcan't\u001b[0m be blank\r\n\tsee docs\u009b2J\u0007"]}`
	ts := bodyServer(http.StatusUnprocessableEntity, body)
	defer ts.Close()

	c := newTestClient(ts)
	var out map[string]any
	err := c.Get(context.Background(), "/issues.json", map[string][]string{"key": {"SECRET"}, "token": {"TOKEN"}}, &out)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want *APIError", err)
	}
	if got, want := apiErr.Errors[0], "Subject ^[[31mcan't^[[0m be blank\n\tsee docs\\u009b2J^G"; got != want {
		t.Errorf("Errors[0] = %q, want %q", got, want)
	}
	if strings.Contains(apiErr.URL, "SECRET") || strings.Contains(apiErr.URL, "TOKEN") {
		t.Errorf("URL = %q leaks credentials", apiErr.URL)
	}
	if !strings.Contains(apiErr.URL, "/issues.json") {
		t.Errorf("URL = %q, want the request path preserved", apiErr.URL)
	}
}

// TestNewAPIError covers APIErrors built outside parseErrorResponse, such as
// the one `redmine api` derives from a raw response: messages get the same
// terminal sanitization as everywhere else (ESC shown as ^[) and the URL is
// scrubbed.
func TestNewAPIError(t *testing.T) {
	tests := []struct {
		name string
		err  *APIError
		want string
	}{
		{"control characters in messages", NewAPIError(422, "/issues.json", []string{"bad\x1b]0;pwned\x07", "x\u009b2J", "rtl\u202Eexe.pdf"}), `API error 422: bad^[]0;pwned^G; x\u009b2J; rtlexe.pdf`},
		{"credentials in URL", NewAPIError(404, "/issues.json?key=SECRET&limit=1", nil), "API error 404 for /issues.json?key=%2A%2A%2A&limit=1"},
		{"literal with credentials in URL", &APIError{StatusCode: 404, URL: "/issues.json?key=SECRET&limit=1"}, "API error 404 for /issues.json?key=%2A%2A%2A&limit=1"},
		{"plain URL", &APIError{StatusCode: 404, URL: "/issues/9.json"}, "API error 404 for /issues/9.json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
	if err := NewAPIError(404, "/x?key=SECRET", nil); strings.Contains(err.URL, "SECRET") {
		t.Errorf("URL = %q leaks credentials", err.URL)
	}
}
