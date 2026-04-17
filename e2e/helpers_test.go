//go:build e2e

package e2e

import "os"

// actionEnvelope matches the JSON shape emitted by no-body mutators.
type actionEnvelope struct {
	Ok       bool   `json:"ok"`
	Action   string `json:"action"`
	Resource string `json:"resource"`
	ID       any    `json:"id"`
	Message  string `json:"message"`
}

// errorEnvelope matches the JSON shape written to stdout on failure when
// --output json is active (see output.ErrorEnvelope).
type errorEnvelope struct {
	Error struct {
		Message string   `json:"message"`
		Code    string   `json:"code"`
		Details []string `json:"details"`
	} `json:"error"`
}

// envelopeIntID coerces the envelope ID (which JSON decodes as float64) to an
// int for comparisons.
func envelopeIntID(v any) int {
	switch id := v.(type) {
	case float64:
		return int(id)
	case int:
		return id
	default:
		return 0
	}
}

func getenvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// The following accessors centralize env-var lookups so renames stay local to
// this file. Tests should prefer these over os.Getenv.
func e2eBaseURL() string  { return getenvDefault("REDMINE_E2E_BASE_URL", "http://127.0.0.1:3000") }
func e2eUsername() string { return getenvDefault("REDMINE_E2E_USERNAME", "admin") }
func e2eAPIKey() string   { return os.Getenv("REDMINE_E2E_API_KEY") }
func e2ePassword() string { return os.Getenv("REDMINE_E2E_PASSWORD") }
