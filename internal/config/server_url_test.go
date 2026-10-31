package config

import (
	"strings"
	"testing"
)

func TestNormalizeServerURL(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "https://redmine.example.com", want: "https://redmine.example.com"},
		{in: "  https://redmine.example.com/  ", want: "https://redmine.example.com"},
		{in: "redmine.example.com", want: "https://redmine.example.com"},
		{in: "localhost:3000", want: "https://localhost:3000"},
		{in: "http://localhost:3000/", want: "http://localhost:3000"},
		{in: "HTTPS://Redmine.Example.com", want: "https://Redmine.Example.com"},
		{in: "https://host/projects/foo/issues", want: "https://host"},
		{in: "https://host/issues/5?tab=notes#note-2", want: "https://host"},
		{in: "https://host/my/account", want: "https://host"},
		{in: "https://host/my/page", want: "https://host"},
		{in: "https://host/login?back_url=%2F", want: "https://host"},
		{in: "https://host/time_entries/report", want: "https://host"},
		{in: "https://host/projects/foo/wiki/Start", want: "https://host"},
		{in: "https://host/redmine", want: "https://host/redmine"},
		{in: "https://host/redmine/", want: "https://host/redmine"},
		{in: "https://host/redmine/issues/5", want: "https://host/redmine"},
		{in: "https://host/tools/redmine/projects/foo", want: "https://host/tools/redmine"},
		{in: "host/redmine/my/page", want: "https://host/redmine"},
		{in: "https://host//redmine//", want: "https://host/redmine"},
		{in: "https://host?x=1", want: "https://host"},
		{in: "", wantErr: true},
		{in: "   ", wantErr: true},
		{in: "ftp://host", wantErr: true},
		{in: "https://", wantErr: true},
		{in: "http://bob:hunter2@host/issues", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := NormalizeServerURL(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("NormalizeServerURL(%q) = %q, want error", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("NormalizeServerURL(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestNormalizeServerURL_ErrorsHideCredentials(t *testing.T) {
	for _, in := range []string{
		"http://bob:sekret@h/%zz",
		"ftp://bob:sekret@h/",
		"bob:sekret@h/%zz",
		"https://bob:sekret@host/issues",
	} {
		t.Run(in, func(t *testing.T) {
			_, err := NormalizeServerURL(in)
			if err == nil {
				t.Fatalf("NormalizeServerURL(%q): want error", in)
			}
			if strings.Contains(err.Error(), "sekret") {
				t.Fatalf("error leaks the password: %v", err)
			}
		})
	}
}

func TestEnsureScheme(t *testing.T) {
	cases := map[string]string{
		"redmine.example.com":   "https://redmine.example.com",
		"localhost:3000":        "https://localhost:3000",
		"http://host":           "http://host",
		"https://host/redmine/": "https://host/redmine/",
		"":                      "",
	}
	for in, want := range cases {
		if got := EnsureScheme(in); got != want {
			t.Errorf("EnsureScheme(%q) = %q, want %q", in, got, want)
		}
	}
}
