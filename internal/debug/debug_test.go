package debug

import (
	"bytes"
	"testing"
)

func TestLoggerPrintf_SanitizesServerText(t *testing.T) {
	var buf bytes.Buffer
	New(&buf).Printf("Resolved project %q to %s (%d)", "x", "Evil\x1b]52;c;ZXZpbA==\x07\u202Egnp.exe\rFAKE", 7)
	want := "[debug] Resolved project \"x\" to Evil^[]52;c;ZXZpbA==^Ggnp.exe^MFAKE (7)\n"
	if got := buf.String(); got != want {
		t.Errorf("log line = %q, want %q", got, want)
	}
}

func TestLoggerPrintf_KeepsPlainLines(t *testing.T) {
	var buf bytes.Buffer
	New(&buf).Printf("HTTP %s %s -> %d (100%%)", "GET", "https://h/x?a=1", 200)
	if got, want := buf.String(), "[debug] HTTP GET https://h/x?a=1 -> 200 (100%)\n"; got != want {
		t.Errorf("log line = %q, want %q", got, want)
	}
}

func TestScrubURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"no secrets", "https://redmine.example.com/issues.json?offset=0&limit=25", "https://redmine.example.com/issues.json?offset=0&limit=25"},
		{"redmine key", "https://redmine.example.com/issues.json?key=SECRET&limit=5", "https://redmine.example.com/issues.json?key=%2A%2A%2A&limit=5"},
		{"case-insensitive keys", "https://h/x?API_KEY=s1&ApiKey=s2&Token=s3", "https://h/x?API_KEY=%2A%2A%2A&ApiKey=%2A%2A%2A&Token=%2A%2A%2A"},
		{"token style", "https://h/x?access_token=s&password=p&secret=q&signature=z", "https://h/x?access_token=%2A%2A%2A&password=%2A%2A%2A&secret=%2A%2A%2A&signature=%2A%2A%2A"},
		{"presigned s3", "https://bucket.s3.amazonaws.com/f?X-Amz-Algorithm=AWS4&X-Amz-Credential=AKIA&X-Amz-Security-Token=T&X-Amz-Signature=abc", "https://bucket.s3.amazonaws.com/f?X-Amz-Algorithm=AWS4&X-Amz-Credential=%2A%2A%2A&X-Amz-Security-Token=%2A%2A%2A&X-Amz-Signature=%2A%2A%2A"},
		{"repeated key", "https://h/x?key=a&key=b", "https://h/x?key=%2A%2A%2A&key=%2A%2A%2A"},
		{"userinfo with password", "https://alice:hunter2@redmine.example.com/issues.json", "https://%2A%2A%2A@redmine.example.com/issues.json"},
		{"userinfo keeps query order", "https://alice:pw@h/x?offset=0&limit=25", "https://%2A%2A%2A@h/x?offset=0&limit=25"},
		{"userinfo without password", "https://alice@redmine.example.com/", "https://%2A%2A%2A@redmine.example.com/"},
		{"relative path", "/issues/1.json", "/issues/1.json"},
		{"keeps other params as sent", "https://h/x?z=1&key=s&a=%41", "https://h/x?z=1&key=%2A%2A%2A&a=%41"},
		{"key without value", "https://h/x?key", "https://h/x?key=%2A%2A%2A"},
		{"escaped key name", "https://h/x?ke%79=s", "https://h/x?ke%79=%2A%2A%2A"},
		{"semicolon separator", "https://h/x?key=s;limit=5", "https://h/x?key=%2A%2A%2A;limit=5"},
		{"semicolon inside value", "https://h/x?a=1&key=s;x=y", "https://h/x?a=1&key=%2A%2A%2A;x=y"},
		{"invalid escape in value", "https://h/x?api_key=s%zz&a=1", "https://h/x?api_key=%2A%2A%2A&a=1"},
		{"unparseable with userinfo", "https://alice:pw@h:bad/x?key=s", "https://%2A%2A%2A@h:bad/x?key=%2A%2A%2A"},
		{"unparseable without secrets", "http://[::1]:%zz/x?a=1", "http://[::1]:%zz/x?a=1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ScrubURL(tt.in); got != tt.want {
				t.Errorf("ScrubURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
