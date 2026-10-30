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
	// The stubs only need /bin/sh (absolute) and the `command -v` builtin,
	// so PATH can hold just the stub directory.
	tests := []struct {
		name    string
		setup   func(home, pathDir string) []string
		want    string
		wantErr bool
		// fallback cases reach the fixed system directories, where a real
		// install would shadow the temp-dir fixtures.
		fallback bool
	}{
		{
			name: "found on PATH",
			setup: func(home, pathDir string) []string {
				writeStub(t, pathDir, "path")
				return []string{"PATH=" + pathDir, "HOME=" + home}
			},
			want: "path mcp serve",
		},
		{
			name:     "found in HOME/.local/bin",
			fallback: true,
			setup: func(home, pathDir string) []string {
				writeStub(t, filepath.Join(home, ".local", "bin"), "local")
				return []string{"PATH=" + pathDir, "HOME=" + home}
			},
			want: "local mcp serve",
		},
		{
			name:     "found in HOME/go/bin",
			fallback: true,
			setup: func(home, pathDir string) []string {
				writeStub(t, filepath.Join(home, "go", "bin"), "gohome")
				return []string{"PATH=" + pathDir, "HOME=" + home}
			},
			want: "gohome mcp serve",
		},
		{
			name:     "found in GOBIN",
			fallback: true,
			setup: func(home, pathDir string) []string {
				gobin := filepath.Join(home, "custom-gobin")
				writeStub(t, gobin, "gobin")
				return []string{"PATH=" + pathDir, "HOME=" + home, "GOBIN=" + gobin}
			},
			want: "gobin mcp serve",
		},
		{
			name: "PATH wins over fallbacks",
			setup: func(home, pathDir string) []string {
				writeStub(t, pathDir, "path")
				writeStub(t, filepath.Join(home, ".local", "bin"), "local")
				return []string{"PATH=" + pathDir, "HOME=" + home}
			},
			want: "path mcp serve",
		},
		{
			name:     "non-executable file is skipped",
			fallback: true,
			setup: func(home, pathDir string) []string {
				dir := filepath.Join(home, ".local", "bin")
				writeStub(t, dir, "local")
				if err := os.Chmod(filepath.Join(dir, "redmine"), 0o644); err != nil {
					t.Fatal(err)
				}
				return []string{"PATH=" + pathDir, "HOME=" + home}
			},
			wantErr: true,
		},
		{
			name:     "not found",
			fallback: true,
			setup: func(home, pathDir string) []string {
				return []string{"PATH=" + pathDir, "HOME=" + home}
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.fallback {
				for _, p := range []string{"/opt/homebrew/bin/redmine", "/usr/local/bin/redmine"} {
					if _, err := os.Stat(p); err == nil {
						t.Skipf("%s exists and would shadow the fixtures", p)
					}
				}
			}
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
