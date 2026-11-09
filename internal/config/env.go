package config

import (
	"fmt"
	"os"
	"strings"
)

// Environment variables that mirror the --config, --profile and --header
// global flags. A flag given on the command line wins over its variable.
// Besides scripting, they let a process started by redmine-cli (such as an
// extension calling `redmine api`) reuse the connection settings of the
// invocation that started it.
const (
	// ConfigEnv names the config file, like --config.
	ConfigEnv = "REDMINE_CONFIG"
	// ProfileEnv selects the profile, like --profile.
	ProfileEnv = "REDMINE_PROFILE"
	// HeadersEnv carries extra HTTP headers, one "Name: value" per line,
	// like repeated --header flags. They replace profile headers of the same
	// name and are replaced by --header flags of the same name.
	HeadersEnv = "REDMINE_HEADERS"
)

// ParseHeadersEnv splits a REDMINE_HEADERS value into its "Name: value"
// entries. Entries are separated by newlines (CRLF is accepted too) and blank
// lines are skipped. Every entry is validated with ParseHeader, so a typo
// fails instead of being silently dropped. The entries are returned as given,
// in order, ready to be merged like --header values.
func ParseHeadersEnv(s string) ([]string, error) {
	var headers []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if _, _, err := ParseHeader(line); err != nil {
			return nil, fmt.Errorf("%s: %w", HeadersEnv, err)
		}
		headers = append(headers, line)
	}
	return headers, nil
}

// EnvHeaders returns the validated entries of REDMINE_HEADERS, or nil when
// the variable is unset or empty.
func EnvHeaders() ([]string, error) {
	return ParseHeadersEnv(os.Getenv(HeadersEnv))
}

// FormatHeadersEnv joins "Name: value" headers into a REDMINE_HEADERS value.
// Header values cannot contain line breaks (ParseHeader rejects them), so the
// result always parses back into the same entries.
func FormatHeadersEnv(headers []string) string {
	return strings.Join(headers, "\n")
}
