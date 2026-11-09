package config

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseHeadersEnv(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    []string
		wantErr string
	}{
		{name: "empty", in: "", want: nil},
		{name: "only blank lines", in: "\n \n\r\n", want: nil},
		{name: "single", in: "X-Team: core", want: []string{"X-Team: core"}},
		{name: "multiple in order", in: "X-A: 1\nX-B: 2", want: []string{"X-A: 1", "X-B: 2"}},
		{name: "crlf", in: "X-A: 1\r\nX-B: 2\r\n", want: []string{"X-A: 1", "X-B: 2"}},
		{name: "blank lines skipped", in: "\nX-A: 1\n\n\nX-B: 2\n", want: []string{"X-A: 1", "X-B: 2"}},
		{name: "value with colon", in: "X-Url: https://example.com", want: []string{"X-Url: https://example.com"}},
		{name: "missing colon", in: "X-A: 1\nbroken", wantErr: `REDMINE_HEADERS: invalid header "broken"`},
		{name: "empty value", in: "X-A:", wantErr: "REDMINE_HEADERS: header \"X-A\" has an empty value"},
		{name: "reserved header", in: "Host: example.com", wantErr: "REDMINE_HEADERS: header \"Host\" cannot be configured"},
		{name: "invalid name", in: "Bad Name: x", wantErr: "REDMINE_HEADERS: invalid header name"},
		{name: "lone carriage return in value", in: "X-A: a\rb", wantErr: "REDMINE_HEADERS: header \"X-A\" has an invalid value"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseHeadersEnv(tt.in)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("ParseHeadersEnv(%q) error = %v, want it to contain %q", tt.in, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseHeadersEnv(%q) unexpected error: %v", tt.in, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ParseHeadersEnv(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestEnvHeaders(t *testing.T) {
	t.Setenv(HeadersEnv, "")
	if got, err := EnvHeaders(); err != nil || got != nil {
		t.Fatalf("EnvHeaders() with unset variable = %q, %v; want nil, nil", got, err)
	}

	t.Setenv(HeadersEnv, "X-A: 1\nX-B: 2")
	got, err := EnvHeaders()
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"X-A: 1", "X-B: 2"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("EnvHeaders() = %q, want %q", got, want)
	}

	t.Setenv(HeadersEnv, "nonsense")
	if _, err := EnvHeaders(); err == nil {
		t.Fatal("EnvHeaders() expected an error for an invalid entry")
	}
}

func TestFormatHeadersEnvRoundTrips(t *testing.T) {
	tests := [][]string{
		nil,
		{"X-A: 1"},
		{"X-A: 1", "User-Agent: Mozilla/5.0 (X11; Linux x86_64)", "X-Url: https://example.com/a:b"},
	}
	for _, headers := range tests {
		got, err := ParseHeadersEnv(FormatHeadersEnv(headers))
		if err != nil {
			t.Fatalf("round trip of %q failed: %v", headers, err)
		}
		if !reflect.DeepEqual(got, headers) {
			t.Fatalf("round trip of %q = %q", headers, got)
		}
	}
}
