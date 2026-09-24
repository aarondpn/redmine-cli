package config

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/http/httpguts"
)

// reservedHeaders are managed by the HTTP client or redmine-cli itself;
// configuring them is either ignored by Go or breaks response handling.
var reservedHeaders = map[string]bool{
	"Accept-Encoding":   true,
	"Connection":        true,
	"Content-Length":    true,
	"Content-Type":      true,
	"Host":              true,
	"Transfer-Encoding": true,
}

// ParseHeader splits a "Name: value" string into its canonical header name
// and value, rejecting anything that could not be sent as configured.
func ParseHeader(s string) (string, string, error) {
	name, value, ok := strings.Cut(s, ":")
	if !ok {
		return "", "", fmt.Errorf("invalid header %q: expected \"Name: value\"", s)
	}
	return validateHeader(name, value)
}

// validateHeader trims and validates a single header, returning its
// canonical name.
func validateHeader(name, value string) (string, string, error) {
	name = strings.TrimSpace(name)
	value = strings.TrimSpace(value)
	if !httpguts.ValidHeaderFieldName(name) {
		return "", "", fmt.Errorf("invalid header name %q", name)
	}
	name = http.CanonicalHeaderKey(name)
	if reservedHeaders[name] {
		return "", "", fmt.Errorf("header %q cannot be configured: it is managed by the HTTP client", name)
	}
	if value == "" {
		return "", "", fmt.Errorf("header %q has an empty value", name)
	}
	if !httpguts.ValidHeaderFieldValue(value) {
		return "", "", fmt.Errorf("header %q has an invalid value", name)
	}
	return name, value, nil
}

// MergeHeaders validates the configured base headers and applies the parsed
// "Name: value" overrides on top. The result uses canonical header names, so
// an override replaces a base entry regardless of how either is capitalized.
// The base map is never modified.
func MergeHeaders(base map[string]string, overrides []string) (map[string]string, error) {
	if len(base) == 0 && len(overrides) == 0 {
		return nil, nil
	}
	merged := make(map[string]string, len(base)+len(overrides))
	for k, v := range base {
		name, value, err := validateHeader(k, v)
		if err != nil {
			return nil, err
		}
		if _, dup := merged[name]; dup {
			return nil, fmt.Errorf("header %q is configured more than once", name)
		}
		merged[name] = value
	}
	for _, h := range overrides {
		name, value, err := ParseHeader(h)
		if err != nil {
			return nil, err
		}
		merged[name] = value
	}
	return merged, nil
}

// SameServerHost reports whether two server URLs point at the same host.
// Configured headers may carry secrets scoped to one server, so they are only
// carried over when a profile keeps its host.
func SameServerHost(a, b string) bool {
	ha, hb := serverHost(a), serverHost(b)
	return ha != "" && strings.EqualFold(ha, hb)
}

func serverHost(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		u, err = url.Parse("https://" + raw)
		if err != nil {
			return ""
		}
	}
	return u.Host
}
