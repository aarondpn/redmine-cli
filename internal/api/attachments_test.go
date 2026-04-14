package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAttachmentUpload_Success(t *testing.T) {
	const wantBody = "hello world"
	const wantFilename = "notes with, comma.txt"
	var gotMethod, gotPath, gotCT, gotFilename, gotBody string
	var gotLen int64

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotCT = r.Header.Get("Content-Type")
		gotFilename = r.URL.Query().Get("filename")
		gotLen = r.ContentLength
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"upload":{"token":"abc123"}}`))
	}))
	defer ts.Close()

	c := newTestClient(ts)
	c.Attachments = &AttachmentService{client: c}

	tok, err := c.Attachments.Upload(context.Background(), wantFilename, strings.NewReader(wantBody), int64(len(wantBody)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok != "abc123" {
		t.Errorf("token = %q, want abc123", tok)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/uploads.json" {
		t.Errorf("path = %q, want /uploads.json", gotPath)
	}
	if gotCT != "application/octet-stream" {
		t.Errorf("content-type = %q, want application/octet-stream", gotCT)
	}
	if gotFilename != wantFilename {
		t.Errorf("filename query = %q, want %q", gotFilename, wantFilename)
	}
	if gotLen != int64(len(wantBody)) {
		t.Errorf("Content-Length = %d, want %d", gotLen, len(wantBody))
	}
	if gotBody != wantBody {
		t.Errorf("body = %q, want %q", gotBody, wantBody)
	}
}

func TestAttachmentUpload_SizeExceeded(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"errors":["This file cannot be uploaded because it exceeds the maximum allowed file size (1024000)"]}`))
	}))
	defer ts.Close()

	c := newTestClient(ts)
	c.Attachments = &AttachmentService{client: c}

	_, err := c.Attachments.Upload(context.Background(), "big.bin", strings.NewReader("x"), 1)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if !apiErr.IsValidationError() {
		t.Errorf("IsValidationError = false, want true (status %d)", apiErr.StatusCode)
	}
	if len(apiErr.Errors) == 0 || !strings.Contains(apiErr.Errors[0], "maximum allowed file size") {
		t.Errorf("errors = %v, want size-exceeded message", apiErr.Errors)
	}
}

func TestAttachmentUpload_MissingToken(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"upload":{}}`))
	}))
	defer ts.Close()

	c := newTestClient(ts)
	c.Attachments = &AttachmentService{client: c}

	_, err := c.Attachments.Upload(context.Background(), "x.bin", strings.NewReader("x"), 1)
	if err == nil || !strings.Contains(err.Error(), "missing token") {
		t.Fatalf("want missing-token error, got %v", err)
	}
}

func TestAttachmentUpload_EmptyFilenameOmitsQuery(t *testing.T) {
	var gotRawQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRawQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"upload":{"token":"t"}}`))
	}))
	defer ts.Close()

	c := newTestClient(ts)
	c.Attachments = &AttachmentService{client: c}

	if _, err := c.Attachments.Upload(context.Background(), "", strings.NewReader("x"), 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotRawQuery != "" {
		t.Errorf("raw query = %q, want empty", gotRawQuery)
	}
}

func TestAttachmentUpload_AuthTransportDoesNotOverrideContentType(t *testing.T) {
	// Regression test for the authTransport change: octet-stream must not be
	// replaced with application/json.
	var gotCT string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"upload":{"token":"t"}}`))
	}))
	defer ts.Close()

	transport := &authTransport{base: http.DefaultTransport, authMethod: "apikey", apiKey: "k"}
	c := newTestClient(ts)
	c.httpClient = &http.Client{Transport: transport}
	c.Attachments = &AttachmentService{client: c}

	if _, err := c.Attachments.Upload(context.Background(), "f.bin", strings.NewReader("x"), 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotCT != "application/octet-stream" {
		t.Errorf("content-type = %q, want application/octet-stream (authTransport must not override)", gotCT)
	}
}

func TestAuthTransport_DefaultsContentTypeToJSON(t *testing.T) {
	var gotCT string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	transport := &authTransport{base: http.DefaultTransport, authMethod: "apikey", apiKey: "k"}
	client := &http.Client{Transport: transport}
	req, _ := http.NewRequest(http.MethodGet, ts.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()
	if gotCT != "application/json" {
		t.Errorf("content-type = %q, want application/json", gotCT)
	}
}
