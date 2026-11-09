package cmdutil

import (
	"errors"
	"fmt"
	"io"
	"os"
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

// ReadNewPassword returns the new password named by a password file flag
// such as --password-file (flagName, without dashes, names it in errors).
// path is a file, or "-" for standard input. When "-" is given and stdin is a
// terminal, the password is prompted for twice on stderr with hidden input
// and must match. Otherwise the file or stdin is read to the end and one
// trailing line break is removed, so `printf %s "$PW"` and `echo "$PW"` give
// the same result; other whitespace is kept, it may be part of the password.
// An empty password, a file over 64 KiB and "-" together with
// --api-key-file - are errors.
func ReadNewPassword(f *Factory, flagName, path string) (string, error) {
	var r io.Reader
	source := path
	switch path {
	case "":
		return "", fmt.Errorf("--%s: file path is empty (use \"-\" to read from standard input)", flagName)
	case "-":
		if f.APIKeyFile == "-" {
			return "", fmt.Errorf("--%s - and --api-key-file - cannot both read from standard input", flagName)
		}
		in := f.IOStreams.In
		if in == nil {
			return "", fmt.Errorf("--%s: no standard input available", flagName)
		}
		if file, ok := in.(fdReader); ok && stdinIsTerminal(int(file.Fd())) {
			return promptNewPassword(int(file.Fd()), f.IOStreams.ErrOut, flagName)
		}
		r, source = in, "standard input"
	default:
		file, err := os.Open(path)
		if err != nil {
			return "", fmt.Errorf("--%s: %w", flagName, err)
		}
		defer file.Close()
		r = file
	}

	data, err := io.ReadAll(io.LimitReader(r, maxSecretFileSize+1))
	if err != nil {
		return "", fmt.Errorf("--%s: %w", flagName, err)
	}
	if len(data) > maxSecretFileSize {
		return "", fmt.Errorf("--%s: %s is larger than %d bytes; it should contain only the password", flagName, source, maxSecretFileSize)
	}
	password := strings.TrimSuffix(strings.TrimSuffix(string(data), "\n"), "\r")
	if password == "" {
		return "", fmt.Errorf("--%s: %s is empty", flagName, source)
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
