package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/aarondpn/redmine-cli/internal/cmd/update"
)

func TestWaitForStartupUpdate_FastResult_NoHint(t *testing.T) {
	updateDone := make(chan *update.CheckResult, 1)
	updateDone <- &update.CheckResult{
		NewVersion: "2.0.0",
		ReleaseURL: "https://github.com/aarondpn/redmine-cli/releases/tag/v2.0.0",
	}

	var buf bytes.Buffer
	waitForStartupUpdate(&buf, "1.0.0", updateDone, nil, 10*time.Millisecond, 50*time.Millisecond)

	out := buf.String()
	if strings.Contains(out, "Checking for updates...") {
		t.Fatalf("unexpected progress hint in output: %q", out)
	}
	if !strings.Contains(out, "v1.0.0") || !strings.Contains(out, "v2.0.0") {
		t.Fatalf("expected update notice in output, got %q", out)
	}
}

func TestWaitForStartupUpdate_SlowResult_PrintsHintAndNotice(t *testing.T) {
	updateDone := make(chan *update.CheckResult, 1)
	go func() {
		time.Sleep(20 * time.Millisecond)
		updateDone <- &update.CheckResult{
			NewVersion: "2.0.0",
			ReleaseURL: "https://github.com/aarondpn/redmine-cli/releases/tag/v2.0.0",
		}
	}()

	var buf bytes.Buffer
	waitForStartupUpdate(&buf, "1.0.0", updateDone, nil, 10*time.Millisecond, 100*time.Millisecond)

	out := buf.String()
	if !strings.Contains(out, "Checking for updates...") {
		t.Fatalf("expected progress hint in output, got %q", out)
	}
	if !strings.Contains(out, "v1.0.0") || !strings.Contains(out, "v2.0.0") {
		t.Fatalf("expected update notice in output, got %q", out)
	}
}

func TestWaitForStartupUpdate_SlowNilResult_PrintsHintOnly(t *testing.T) {
	updateDone := make(chan *update.CheckResult, 1)
	go func() {
		time.Sleep(20 * time.Millisecond)
		updateDone <- nil
	}()

	var buf bytes.Buffer
	waitForStartupUpdate(&buf, "1.0.0", updateDone, nil, 10*time.Millisecond, 100*time.Millisecond)

	out := buf.String()
	if !strings.Contains(out, "Checking for updates...") {
		t.Fatalf("expected progress hint in output, got %q", out)
	}
	if strings.Contains(out, "A new version of redmine is available") {
		t.Fatalf("did not expect update notice in output, got %q", out)
	}
}

func TestWaitForStartupUpdate_TimeoutCancelsCheck(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	updateDone := make(chan *update.CheckResult, 1)
	canceled := make(chan struct{})

	go func() {
		<-ctx.Done()
		close(canceled)
	}()

	var buf bytes.Buffer
	waitForStartupUpdate(&buf, "1.0.0", updateDone, cancel, 10*time.Millisecond, 40*time.Millisecond)

	select {
	case <-canceled:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected update check context to be canceled")
	}

	out := buf.String()
	if !strings.Contains(out, "Checking for updates...") {
		t.Fatalf("expected progress hint in output, got %q", out)
	}
	if strings.Contains(out, "A new version of redmine is available") {
		t.Fatalf("did not expect update notice in output, got %q", out)
	}
}
