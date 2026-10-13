package cmdutil

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"
)

// AddTextFileFlag registers fileFlag as the file counterpart of the existing
// long-text flag inlineFlag (for example --description-file for
// --description). The value is a path, or "-" to read from standard input.
// The two flags are mutually exclusive.
//
// Call ReadTextFileFlag in RunE to resolve the value.
func AddTextFileFlag(cmd *cobra.Command, inlineFlag, fileFlag, shorthand string) {
	usage := fmt.Sprintf("Read --%s from file (use \"-\" to read from standard input)", inlineFlag)
	cmd.Flags().StringP(fileFlag, shorthand, "", usage)
	_ = cmd.MarkFlagFilename(fileFlag)
	cmd.MarkFlagsMutuallyExclusive(inlineFlag, fileFlag)
}

// ReadTextFileFlag resolves a long-text flag pair registered with
// AddTextFileFlag. When fileFlag was given, the file (or stdin for "-") is
// read and stored in *dst without trimming, so trailing newlines and
// whitespace survive (see ReadTextFile). It reports whether either flag was set, which is what
// callers should use instead of cmd.Flags().Changed(inlineFlag).
func ReadTextFileFlag(cmd *cobra.Command, stdin io.Reader, inlineFlag, fileFlag string, dst *string) (bool, error) {
	if !cmd.Flags().Changed(fileFlag) {
		return cmd.Flags().Changed(inlineFlag), nil
	}
	path, err := cmd.Flags().GetString(fileFlag)
	if err != nil {
		return false, err
	}
	content, err := ReadTextFile(path, stdin)
	if err != nil {
		return false, fmt.Errorf("--%s: %w", fileFlag, err)
	}
	*dst = content
	return true, nil
}

// RejectEmptyTextFile returns an error when fileFlag was given but its file
// (or stdin) was empty or held only whitespace. An empty file is almost
// always a mistake (a failed redirect, the wrong path): for notes and
// comments it would be a silent no-op, for descriptions it would silently
// wipe the existing text. Clearing a field stays possible through the inline
// flag (for example --description ""). hint, when not empty, is attached as
// the error's recovery hint. The check never alters content.
func RejectEmptyTextFile(cmd *cobra.Command, fileFlag, content, hint string) error {
	if cmd.Flags().Changed(fileFlag) && strings.TrimSpace(content) == "" {
		return WithHint(fmt.Errorf("--%s is empty", fileFlag), hint)
	}
	return nil
}

// ClearDescriptionHint is the RejectEmptyTextFile hint for --description-file.
const ClearDescriptionHint = `To clear the description, pass --description "" explicitly.`

// utf8BOM is the byte-order mark some editors (notably on Windows) prepend to
// UTF-8 files. It is not content and would show up as a stray character at
// the start of the Redmine text.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// ReadTextFile reads a UTF-8 text file, or standard input when path is "-".
// The content is returned as read, except that a leading UTF-8 byte-order
// mark is dropped. Line endings (including CRLF) and surrounding whitespace
// are preserved.
func ReadTextFile(path string, stdin io.Reader) (string, error) {
	var (
		data []byte
		err  error
	)
	switch path {
	case "":
		return "", fmt.Errorf("file path is empty (use \"-\" to read from standard input)")
	case "-":
		if stdin == nil {
			stdin = os.Stdin
		}
		data, err = io.ReadAll(stdin)
		if err != nil {
			return "", fmt.Errorf("reading standard input: %w", err)
		}
	default:
		data, err = os.ReadFile(path)
		if err != nil {
			return "", err
		}
	}
	if !utf8.Valid(data) {
		if path == "-" {
			return "", fmt.Errorf("standard input is not valid UTF-8 text")
		}
		return "", fmt.Errorf("%s is not valid UTF-8 text", path)
	}
	return string(bytes.TrimPrefix(data, utf8BOM)), nil
}

// RejectMultipleStdinFlags returns an error when more than one of the given
// file flags reads from standard input ("-"); stdin can only be consumed once.
func RejectMultipleStdinFlags(cmd *cobra.Command, fileFlags ...string) error {
	var fromStdin []string
	for _, name := range fileFlags {
		if !cmd.Flags().Changed(name) {
			continue
		}
		if v, _ := cmd.Flags().GetString(name); v == "-" {
			fromStdin = append(fromStdin, "--"+name)
		}
	}
	if len(fromStdin) > 1 {
		return fmt.Errorf("%s cannot all read from standard input; use a file path for all but one", strings.Join(fromStdin, ", "))
	}
	return nil
}
