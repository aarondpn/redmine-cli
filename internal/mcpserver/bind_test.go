package mcpserver

import "testing"

// The loopback check is api.IsLoopbackHost, shared with the cleartext-server
// warning, so host names are compared case-insensitively and a trailing dot
// or a *.localhost name is recognized here too.
func TestNormalizeBindAddr_LoopbackHostNames(t *testing.T) {
	cases := []struct {
		in           string
		wantLoopback bool
	}{
		{"LOCALHOST:8080", true},
		{"localhost.:8080", true},
		{"mcp.localhost:8080", true},
		{"127.0.0.2:8080", true},
		{"localhost.example.com:8080", false},
		{"redmine-mcp:8080", false},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			gotAddr, gotLoopback := NormalizeBindAddr(tc.in)
			if gotAddr != tc.in {
				t.Errorf("addr = %q, want it unchanged", gotAddr)
			}
			if gotLoopback != tc.wantLoopback {
				t.Errorf("isLoopback = %v, want %v", gotLoopback, tc.wantLoopback)
			}
		})
	}
}
