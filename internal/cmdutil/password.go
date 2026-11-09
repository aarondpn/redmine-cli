package cmdutil

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"golang.org/x/term"
)

// Hooks for ReadNewPassword, replaced in tests: a test cannot attach a
// terminal to the factory's stdin.
var (
	stdinIsTerminal = func(fd int) bool { return term.IsTerminal(fd) }
	readHidden      = func(fd int) ([]byte, error) { return term.ReadPassword(fd) }
)

// fdReader is an input stream backed by a file descriptor, such as os.Stdin.
type fdReader interface {
	io.Reader
	Fd() uintptr
}

// ReadNewPassword returns the new password for a --password-stdin flag
// (flagName names it in errors). When stdin is a terminal the password is
// prompted for twice on stderr with hidden input and must match; otherwise
// stdin is read to the end and one trailing line break is removed, so
// `printf %s "$PW" |` and `echo "$PW" |` give the same result. Other
// whitespace is kept: it may be part of the password. An empty password is an
// error, as is stdin already used for --api-key-file -.
func ReadNewPassword(f *Factory, flagName string) (string, error) {
	if f.APIKeyFile == "-" {
		return "", fmt.Errorf("--%s and --api-key-file - cannot both read from standard input", flagName)
	}
	in := f.IOStreams.In
	if in == nil {
		return "", fmt.Errorf("--%s: no standard input available", flagName)
	}
	if file, ok := in.(fdReader); ok && stdinIsTerminal(int(file.Fd())) {
		return promptNewPassword(int(file.Fd()), f.IOStreams.ErrOut, flagName)
	}

	data, err := io.ReadAll(io.LimitReader(in, maxSecretFileSize+1))
	if err != nil {
		return "", fmt.Errorf("--%s: %w", flagName, err)
	}
	if len(data) > maxSecretFileSize {
		return "", fmt.Errorf("--%s: standard input is larger than %d bytes; it should contain only the password", flagName, maxSecretFileSize)
	}
	password := strings.TrimSuffix(strings.TrimSuffix(string(data), "\n"), "\r")
	if password == "" {
		return "", fmt.Errorf("--%s: standard input is empty", flagName)
	}
	return password, nil
}

func promptNewPassword(fd int, errOut io.Writer, flagName string) (string, error) {
	if errOut == nil {
		errOut = io.Discard
	}
	read := func(prompt string) (string, error) {
		fmt.Fprint(errOut, prompt)
		b, err := readHidden(fd)
		fmt.Fprintln(errOut)
		if err != nil {
			return "", fmt.Errorf("--%s: %w", flagName, err)
		}
		return string(b), nil
	}
	password, err := read("New password: ")
	if err != nil {
		return "", err
	}
	if password == "" {
		return "", fmt.Errorf("--%s: the password is empty", flagName)
	}
	confirmation, err := read("Confirm new password: ")
	if err != nil {
		return "", err
	}
	if confirmation != password {
		return "", errors.New("the passwords do not match")
	}
	return password, nil
}
