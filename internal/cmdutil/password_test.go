package cmdutil

import (
	"bytes"
	"strings"
	"testing"
)

func passwordFactory(in any) (*Factory, *bytes.Buffer) {
	errOut := &bytes.Buffer{}
	f := &Factory{IOStreams: &IOStreams{Out: &bytes.Buffer{}, ErrOut: errOut}}
	switch v := in.(type) {
	case string:
		f.IOStreams.In = strings.NewReader(v)
	case *fakeTTY:
		f.IOStreams.In = v
	}
	return f, errOut
}

func TestReadNewPassword_FromPipe(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"s3cret\n", "s3cret"},
		{"s3cret\r\n", "s3cret"},
		{"s3cret", "s3cret"},
		{"  spaced pass \n", "  spaced pass "},
		{"two\nlines\n", "two\nlines"},
	}
	for _, tt := range tests {
		f, _ := passwordFactory(tt.in)
		got, err := ReadNewPassword(f, "password-stdin")
		if err != nil || got != tt.want {
			t.Errorf("ReadNewPassword(%q) = %q, %v; want %q", tt.in, got, err, tt.want)
		}
	}
}

func TestReadNewPassword_Errors(t *testing.T) {
	f, _ := passwordFactory("\n")
	if _, err := ReadNewPassword(f, "password-stdin"); err == nil || !strings.Contains(err.Error(), "--password-stdin: standard input is empty") {
		t.Errorf("empty: err = %v", err)
	}

	f, _ = passwordFactory(strings.Repeat("x", maxSecretFileSize+1))
	if _, err := ReadNewPassword(f, "password-stdin"); err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Errorf("oversized: err = %v", err)
	}

	f, _ = passwordFactory("pw\n")
	f.APIKeyFile = "-"
	if _, err := ReadNewPassword(f, "password-stdin"); err == nil || !strings.Contains(err.Error(), "cannot both read from standard input") {
		t.Errorf("stdin shared with --api-key-file -: err = %v", err)
	}
}

// fakeTTY is a stdin that reports a file descriptor; the terminal hooks are
// replaced so the hidden prompt can be driven without a real terminal.
type fakeTTY struct{ strings.Reader }

func (*fakeTTY) Fd() uintptr { return 42 }

func withFakeTerminal(t *testing.T, answers ...string) {
	t.Helper()
	oldIsTerminal, oldRead := stdinIsTerminal, readHidden
	t.Cleanup(func() { stdinIsTerminal, readHidden = oldIsTerminal, oldRead })
	stdinIsTerminal = func(fd int) bool { return fd == 42 }
	readHidden = func(int) ([]byte, error) {
		if len(answers) == 0 {
			t.Fatal("unexpected extra prompt")
		}
		a := answers[0]
		answers = answers[1:]
		return []byte(a), nil
	}
}

func TestReadNewPassword_TerminalPromptsTwice(t *testing.T) {
	withFakeTerminal(t, "s3cret pw", "s3cret pw")
	f, errOut := passwordFactory(&fakeTTY{})
	got, err := ReadNewPassword(f, "password-stdin")
	if err != nil || got != "s3cret pw" {
		t.Fatalf("got %q, %v", got, err)
	}
	if out := errOut.String(); !strings.Contains(out, "New password: ") || !strings.Contains(out, "Confirm new password: ") || strings.Contains(out, "s3cret") {
		t.Errorf("prompts = %q", out)
	}
}

func TestReadNewPassword_TerminalMismatch(t *testing.T) {
	withFakeTerminal(t, "one", "two")
	f, _ := passwordFactory(&fakeTTY{})
	if _, err := ReadNewPassword(f, "password-stdin"); err == nil || !strings.Contains(err.Error(), "do not match") {
		t.Fatalf("err = %v", err)
	}
}

func TestReadNewPassword_TerminalEmpty(t *testing.T) {
	withFakeTerminal(t, "")
	f, _ := passwordFactory(&fakeTTY{})
	if _, err := ReadNewPassword(f, "password-stdin"); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("err = %v", err)
	}
}
