package runner

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestDecodeOutput(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		want   any
		wantOK bool
	}{
		{name: "object", in: `{"id": 1}`, want: map[string]any{"id": json.Number("1")}, wantOK: true},
		{name: "array with whitespace", in: "\n [1, 2]\n", want: []any{json.Number("1"), json.Number("2")}, wantOK: true},
		{name: "string scalar", in: `"done"`, want: "done", wantOK: true},
		{name: "large integer keeps digits", in: `12345678901234567890`, want: json.Number("12345678901234567890"), wantOK: true},
		{name: "null", in: `null`, want: nil, wantOK: true},
		{name: "empty", in: "  \n", wantOK: false},
		{name: "text", in: "hello world", wantOK: false},
		{name: "two documents", in: `{} {}`, wantOK: false},
		{name: "json then text", in: "{}\ntrailing", wantOK: false},
		{name: "truncated", in: `{"a":`, wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := DecodeOutput([]byte(tt.in))
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestErrorMessage(t *testing.T) {
	long := strings.Repeat("x", maxErrorMessage) + "END"
	tests := []struct {
		name string
		in   string
		want func(string) bool
	}{
		{name: "trimmed", in: "  oops\n", want: func(s string) bool { return s == "oops" }},
		{name: "escape sequences neutralized", in: "bad \x1b]52;c;evil\x07", want: func(s string) bool { return s == "bad ^[]52;c;evil^G" }},
		{name: "keeps the tail", in: "START" + long, want: func(s string) bool {
			return strings.HasPrefix(s, "...") && strings.HasSuffix(s, "END") && !strings.Contains(s, "START") && len(s) <= maxErrorMessage+3
		}},
		{name: "cuts on a rune boundary", in: "é" + strings.Repeat("é", maxErrorMessage), want: func(s string) bool {
			return !strings.ContainsRune(s, '�')
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ErrorMessage([]byte(tt.in)); !tt.want(got) {
				t.Fatalf("ErrorMessage = %q", got)
			}
		})
	}
}
