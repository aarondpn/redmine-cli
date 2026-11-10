package runner

import (
	"errors"
	"path/filepath"
	"slices"
	"testing"
)

func TestCommandWindowsShLookup(t *testing.T) {
	gitRoot := filepath.Join("C:", "Program Files", "Git")
	tests := []struct {
		name     string
		goos     string
		path     string
		onPath   map[string]string
		files    []string
		wantName string
		wantArgs []string
		wantErr  bool
	}{
		{name: "unix runs directly", goos: "linux", path: "/x/redmine-cli-pr", wantName: "/x/redmine-cli-pr", wantArgs: []string{"a"}},
		{name: "windows exe runs directly", goos: "windows", path: `C:\x\redmine-cli-pr.exe`, wantName: `C:\x\redmine-cli-pr.exe`, wantArgs: []string{"a"}},
		{name: "windows cmd runs directly", goos: "windows", path: `C:\x\redmine-cli-pr.CMD`, wantName: `C:\x\redmine-cli-pr.CMD`, wantArgs: []string{"a"}},
		{
			name: "sh on PATH", goos: "windows", path: `C:\x\redmine-cli-pr`,
			onPath:   map[string]string{"sh": `C:\tools\sh.exe`},
			wantName: `C:\tools\sh.exe`, wantArgs: []string{`C:\x\redmine-cli-pr`, "a"},
		},
		{
			name: "sh next to git installer layout", goos: "windows", path: "script",
			onPath:   map[string]string{"git": filepath.Join(gitRoot, "cmd", "git.exe")},
			files:    []string{filepath.Join(gitRoot, "bin", "sh.exe")},
			wantName: filepath.Join(gitRoot, "bin", "sh.exe"), wantArgs: []string{"script", "a"},
		},
		{
			name: "sh for mingw64 git", goos: "windows", path: "script",
			onPath:   map[string]string{"git": filepath.Join(gitRoot, "mingw64", "bin", "git.exe")},
			files:    []string{filepath.Join(gitRoot, "bin", "sh.exe")},
			wantName: filepath.Join(gitRoot, "bin", "sh.exe"), wantArgs: []string{"script", "a"},
		},
		{
			name: "scoop shim", goos: "windows", path: "script",
			onPath:   map[string]string{"git": filepath.Join("C:", "scoop", "shims", "git.exe")},
			files:    []string{filepath.Join("C:", "scoop", "apps", "git", "current", "bin", "sh.exe")},
			wantName: filepath.Join("C:", "scoop", "apps", "git", "current", "bin", "sh.exe"), wantArgs: []string{"script", "a"},
		},
		{name: "no git, no sh", goos: "windows", path: "script", wantErr: true},
		{
			name: "git without sh", goos: "windows", path: "script",
			onPath: map[string]string{"git": filepath.Join(gitRoot, "cmd", "git.exe")}, wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookPath := func(name string) (string, error) {
				if p, ok := tt.onPath[name]; ok {
					return p, nil
				}
				return "", errors.New("not found")
			}
			exists := func(p string) bool { return slices.Contains(tt.files, filepath.Clean(p)) }
			name, args, err := command(tt.goos, tt.path, []string{"a"}, lookPath, exists)
			if tt.wantErr {
				if !errors.Is(err, errNoSh) {
					t.Fatalf("err = %v, want errNoSh", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if name != tt.wantName || !slices.Equal(args, tt.wantArgs) {
				t.Fatalf("got %q %q, want %q %q", name, args, tt.wantName, tt.wantArgs)
			}
		})
	}
}
