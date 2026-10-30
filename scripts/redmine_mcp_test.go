package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// writeStub installs an executable "redmine" in dir that prints its origin
// and arguments.
func writeStub(t *testing.T, dir, origin string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "#!/bin/sh\necho \"" + origin + " $*\"\n"
	if err := os.WriteFile(filepath.Join(dir, "redmine"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func runLauncher(t *testing.T, env []string) (string, string, error) {
	t.Helper()
	script, err := filepath.Abs("redmine-mcp.sh")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("/bin/sh", script, "mcp", "serve")
	cmd.Env = env
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	return strings.TrimSpace(stdout.String()), stderr.String(), err
}

func TestLauncher(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX sh launcher")
	}
	// The launcher only needs shell builtins, so PATH can hold just the stub
	// directory; REDMINE_MCP_SYSTEM_DIRS points the fixed system dirs at an
	// empty location so a real install cannot shadow the fixtures.
	tests := []struct {
		name    string
		setup   func(home, pathDir string) []string
		want    string
		wantErr bool
	}{
		{
			name: "found on PATH",
			setup: func(home, pathDir string) []string {
				writeStub(t, pathDir, "path")
				return []string{"PATH=" + pathDir, "HOME=" + home, "REDMINE_MCP_SYSTEM_DIRS=" + filepath.Join(home, "none")}
			},
			want: "path mcp serve",
		},
		{
			name: "found in HOME/.local/bin",
			setup: func(home, pathDir string) []string {
				writeStub(t, filepath.Join(home, ".local", "bin"), "local")
				return []string{"PATH=" + pathDir, "HOME=" + home, "REDMINE_MCP_SYSTEM_DIRS=" + filepath.Join(home, "none")}
			},
			want: "local mcp serve",
		},
		{
			name: "found in HOME/go/bin",
			setup: func(home, pathDir string) []string {
				writeStub(t, filepath.Join(home, "go", "bin"), "gohome")
				return []string{"PATH=" + pathDir, "HOME=" + home, "REDMINE_MCP_SYSTEM_DIRS=" + filepath.Join(home, "none")}
			},
			want: "gohome mcp serve",
		},
		{
			name: "found in GOBIN",
			setup: func(home, pathDir string) []string {
				gobin := filepath.Join(home, "custom-gobin")
				writeStub(t, gobin, "gobin")
				return []string{"PATH=" + pathDir, "HOME=" + home, "GOBIN=" + gobin, "REDMINE_MCP_SYSTEM_DIRS=" + filepath.Join(home, "none")}
			},
			want: "gobin mcp serve",
		},
		{
			name: "PATH wins over fallbacks",
			setup: func(home, pathDir string) []string {
				writeStub(t, pathDir, "path")
				writeStub(t, filepath.Join(home, ".local", "bin"), "local")
				return []string{"PATH=" + pathDir, "HOME=" + home, "REDMINE_MCP_SYSTEM_DIRS=" + filepath.Join(home, "none")}
			},
			want: "path mcp serve",
		},
		{
			name: "non-executable file is skipped",
			setup: func(home, pathDir string) []string {
				dir := filepath.Join(home, ".local", "bin")
				writeStub(t, dir, "local")
				if err := os.Chmod(filepath.Join(dir, "redmine"), 0o644); err != nil {
					t.Fatal(err)
				}
				return []string{"PATH=" + pathDir, "HOME=" + home, "REDMINE_MCP_SYSTEM_DIRS=" + filepath.Join(home, "none")}
			},
			wantErr: true,
		},
		{
			name: "not found",
			setup: func(home, pathDir string) []string {
				return []string{"PATH=" + pathDir, "HOME=" + home, "REDMINE_MCP_SYSTEM_DIRS=" + filepath.Join(home, "none")}
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			home := filepath.Join(root, "home")
			pathDir := filepath.Join(root, "pathbin")
			for _, d := range []string{home, pathDir} {
				if err := os.MkdirAll(d, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			stdout, stderr, err := runLauncher(t, tt.setup(home, pathDir))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected failure, got stdout %q", stdout)
				}
				for _, s := range []string{"brew install aarondpn/tap/redmine", "install.sh", "https://redmine-cli.dev/getting-started/installation/"} {
					if !strings.Contains(stderr, s) {
						t.Errorf("stderr missing %q: %s", s, stderr)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("launcher failed: %v\n%s", err, stderr)
			}
			if stdout != tt.want {
				t.Errorf("got %q, want %q", stdout, tt.want)
			}
		})
	}
}
