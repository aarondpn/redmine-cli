package api

import (
	"fmt"
	"net/http"
)

// ErrReadOnly is returned when a mutating request is attempted while the
// client is in read-only mode. The request is never sent.
type ErrReadOnly struct {
	Method string
	Path   string
}

func (e *ErrReadOnly) Error() string {
	return fmt.Sprintf(
		"read-only mode enabled: refusing %s %s (set via --read-only, REDMINE_READ_ONLY, or read_only in the profile; pass --read-only=false to allow writes)",
		e.Method, e.Path,
	)
}

// readOnlyTransport rejects any non-read HTTP method before delegating to base.
type readOnlyTransport struct {
	base http.RoundTripper
}

func (t *readOnlyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	switch req.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return t.base.RoundTrip(req)
	default:
		return nil, &ErrReadOnly{Method: req.Method, Path: req.URL.Path}
	}
}
