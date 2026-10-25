package cmdutil

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestFactoryApiClientCleartextServerWarning(t *testing.T) {
	tests := []struct {
		name     string
		server   string
		wantWarn bool
	}{
		{"remote http", "http://redmine.example.com", true},
		{"remote http with port", "http://10.0.0.5:3000", true},
		{"https", "https://redmine.example.com", false},
		{"loopback ipv4", "http://127.0.0.1:3000", false},
		{"loopback ipv4 range", "http://127.1.2.3", false},
		{"loopback ipv6", "http://[::1]:3000", false},
		{"localhost", "http://localhost:3000", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("REDMINE_STRICT", "")
			f := testFactoryWithConfig(t, "server: "+tt.server+"\napi_key: k\n")

			if _, err := f.ApiClient(); err != nil {
				t.Fatalf("ApiClient() error = %v, want nil outside strict mode", err)
			}
			errOut := f.IOStreams.ErrOut.(*bytes.Buffer).String()
			gotWarn := strings.Contains(errOut, "cleartext")
			if gotWarn != tt.wantWarn {
				t.Fatalf("warning printed = %v, want %v (stderr %q)", gotWarn, tt.wantWarn, errOut)
			}
			if gotWarn && !strings.Contains(errOut, "API key or password") {
				t.Errorf("warning %q should say the credentials are sent in cleartext", errOut)
			}
		})
	}
}

func TestFactoryApiClientCleartextServerStrict(t *testing.T) {
	t.Setenv("REDMINE_STRICT", "1")
	f := testFactoryWithConfig(t, "server: http://redmine.example.com\napi_key: k\n")

	_, err := f.ApiClient()
	var strictErr *StrictModeError
	if !errors.As(err, &strictErr) {
		t.Fatalf("ApiClient() error = %v, want *StrictModeError", err)
	}
	if !strings.Contains(err.Error(), "cleartext") {
		t.Errorf("error %q should mention cleartext credentials", err)
	}
}

func TestFactoryApiClientLoopbackHTTPAllowedInStrictMode(t *testing.T) {
	t.Setenv("REDMINE_STRICT", "1")
	f := testFactoryWithConfig(t, "server: http://127.0.0.1:3000\napi_key: k\n")

	if _, err := f.ApiClient(); err != nil {
		t.Fatalf("ApiClient() error = %v, loopback http must stay allowed in strict mode", err)
	}
}
