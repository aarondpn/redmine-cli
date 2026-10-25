package browser

import (
	"reflect"
	"testing"
)

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"https", "https://redmine.example.com/issues/1", false},
		{"http", "http://redmine.example.com/issues/1", false},
		{"uppercase scheme", "HTTPS://redmine.example.com/issues/1", false},
		{"port and query", "https://redmine.example.com:8443/search?q=a&b=c#frag", false},
		{"other host", "https://evil.example/", false},
		{"empty", "", true},
		{"leading dash option", "-aNoSuchApp", true},
		{"leading dash url", "-https://redmine.example.com", true},
		{"file", "file:///etc/passwd", true},
		{"javascript", "javascript:alert(1)", true},
		{"custom scheme", "vscode://file/etc/passwd", true},
		{"data", "data:text/html,<script>alert(1)</script>", true},
		{"scheme relative", "//evil.example/path", true},
		{"relative path", "/issues/1", true},
		{"no host", "https:///issues/1", true},
		{"opaque http", "http:evil.example", true},
		{"embedded space", "https://redmine.example.com/a b", true},
		{"embedded newline", "https://redmine.example.com/\n-a", true},
		{"control character", "https://redmine.example.com/\x1b]52;c;eA==\x07", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}

func TestSameHost(t *testing.T) {
	tests := []struct {
		name   string
		url    string
		server string
		want   bool
	}{
		{"same host", "https://redmine.example.com/issues/1", "https://redmine.example.com", true},
		{"case insensitive", "https://Redmine.Example.com/issues/1", "https://redmine.example.com/", true},
		{"server with path", "https://redmine.example.com/redmine/issues/1", "https://redmine.example.com/redmine", true},
		{"explicit default port", "https://redmine.example.com:443/x", "https://redmine.example.com", true},
		{"other host", "https://evil.example/issues/1", "https://redmine.example.com", false},
		{"subdomain lookalike", "https://redmine.example.com.evil.example/", "https://redmine.example.com", false},
		{"userinfo trick", "https://redmine.example.com@evil.example/", "https://redmine.example.com", false},
		{"other port", "https://redmine.example.com:8443/", "https://redmine.example.com", false},
		{"scheme downgrade", "http://redmine.example.com/", "https://redmine.example.com", false},
		{"no server configured", "https://redmine.example.com/", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := ValidateURL(tt.url)
			if err != nil {
				t.Fatalf("ValidateURL(%q): %v", tt.url, err)
			}
			if got := SameHost(u, tt.server); got != tt.want {
				t.Errorf("SameHost(%q, %q) = %v, want %v", tt.url, tt.server, got, tt.want)
			}
		})
	}
}

func TestLauncher(t *testing.T) {
	const u = "https://redmine.example.com/issues/1?a=1&b=2|x^y"
	tests := []struct {
		goos     string
		wantName string
		wantArgs []string
		wantErr  bool
	}{
		{goos: "darwin", wantName: "open", wantArgs: []string{u}},
		{goos: "linux", wantName: "xdg-open", wantArgs: []string{u}},
		{goos: "freebsd", wantName: "xdg-open", wantArgs: []string{u}},
		// No cmd.exe: the URL reaches the protocol handler without shell
		// parsing of & | ^.
		{goos: "windows", wantName: "rundll32", wantArgs: []string{"url.dll,FileProtocolHandler", u}},
		{goos: "plan9", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.goos, func(t *testing.T) {
			name, args, err := launcher(tt.goos, u)
			if (err != nil) != tt.wantErr {
				t.Fatalf("launcher(%q) error = %v, wantErr %v", tt.goos, err, tt.wantErr)
			}
			if name != tt.wantName || !reflect.DeepEqual(args, tt.wantArgs) {
				t.Errorf("launcher(%q) = %q %q, want %q %q", tt.goos, name, args, tt.wantName, tt.wantArgs)
			}
		})
	}
}

func TestCommand_RejectsInvalidURL(t *testing.T) {
	for _, u := range []string{"-aNoSuchApp", "file:///etc/passwd", "javascript:alert(1)"} {
		if cmd, err := Command(u); err == nil {
			t.Errorf("Command(%q) = %v, want error", u, cmd.Args)
		}
	}
}
