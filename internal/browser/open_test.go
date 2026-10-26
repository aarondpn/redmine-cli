package browser

import (
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestOpen_RejectsInvalidURL(t *testing.T) {
	// An empty PATH makes any launch attempt fail with a different error.
	t.Setenv("PATH", t.TempDir())
	for _, raw := range []string{"-aNoSuchApp", "file:///etc/passwd", "javascript:alert(1)//", "redmine.example.com"} {
		if err := Open(raw); err == nil || !strings.Contains(err.Error(), "refusing to open") {
			t.Errorf("Open(%q) = %v, want a refusal", raw, err)
		}
	}
}

func TestStartAndWait(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not installed")
	}

	if err := startAndWait(exec.Command("sh", "-c", "exit 0"), time.Minute); err != nil {
		t.Errorf("successful launcher: err = %v", err)
	}
	// A failing launcher (xdg-open without a browser exits with 3) is
	// reported.
	if err := startAndWait(exec.Command("sh", "-c", "exit 3"), time.Minute); err == nil {
		t.Error("failing launcher: want an error")
	}
	// A launcher that keeps running does not block the caller.
	start := time.Now()
	if err := startAndWait(exec.Command("sh", "-c", "sleep 5"), 50*time.Millisecond); err != nil {
		t.Errorf("long-running launcher: err = %v", err)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Errorf("startAndWait blocked for %v", elapsed)
	}
	if err := startAndWait(exec.Command("/nonexistent/launcher"), time.Minute); err == nil {
		t.Error("missing launcher: want an error")
	}
}
