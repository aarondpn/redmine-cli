package cmdutil

import (
	"bytes"
	"testing"
)

// A plain http:// server is a legitimate setup (internal instances), so
// ordinary commands connect without a warning; only auth login points out
// the cleartext risk.
func TestFactoryApiClientCleartextServerNoWarning(t *testing.T) {
	for _, server := range []string{
		"http://redmine.example.com",
		"http://10.0.0.5:3000",
		"https://redmine.example.com",
		"http://127.0.0.1:3000",
		"http://localhost:3000",
	} {
		t.Run(server, func(t *testing.T) {
			f := testFactoryWithConfig(t, "server: "+server+"\napi_key: k\n")

			if _, err := f.ApiClient(); err != nil {
				t.Fatalf("ApiClient() error = %v, want nil", err)
			}
			if errOut := f.IOStreams.ErrOut.(*bytes.Buffer).String(); errOut != "" {
				t.Fatalf("ApiClient() printed %q, want no warning", errOut)
			}
		})
	}
}
