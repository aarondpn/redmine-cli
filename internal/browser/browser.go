// Package browser opens URLs in the user's default web browser.
package browser

import (
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"unicode"

	"github.com/aarondpn/redmine-cli/v2/internal/config"
)

// ValidateURL checks that raw is safe to hand to the platform URL launcher:
// an absolute http or https URL with a host, without whitespace or control
// characters, and not starting with '-' (which launchers such as macOS
// `open` would parse as an option).
func ValidateURL(raw string) (*url.URL, error) {
	if raw == "" {
		return nil, fmt.Errorf("empty URL")
	}
	if strings.HasPrefix(raw, "-") {
		return nil, fmt.Errorf("refusing to open %q: URL must not start with '-'", raw)
	}
	for _, r := range raw {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return nil, fmt.Errorf("refusing to open %q: URL contains whitespace or control characters", raw)
		}
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("refusing to open %q: %w", raw, err)
	}
	if scheme := strings.ToLower(u.Scheme); scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("refusing to open %q: only http and https URLs are allowed", raw)
	}
	if u.Hostname() == "" {
		return nil, fmt.Errorf("refusing to open %q: URL has no host", raw)
	}
	return u, nil
}

// SameHost reports whether u points at the configured server URL: the same
// origin (scheme, host and port, see config.OriginOf).
func SameHost(u *url.URL, server string) bool {
	s, err := url.Parse(server)
	if err != nil || s.Hostname() == "" {
		return false
	}
	return config.OriginOf(u) == config.OriginOf(s)
}

// Command validates raw with ValidateURL and returns the command that opens
// it in the default browser. The URL is passed as a single argument and
// never through a shell.
func Command(raw string) (*exec.Cmd, error) {
	if _, err := ValidateURL(raw); err != nil {
		return nil, err
	}
	name, args, err := launcher(runtime.GOOS, raw)
	if err != nil {
		return nil, err
	}
	return exec.Command(name, args...), nil
}

// launcher returns the program and arguments that open rawURL on goos.
func launcher(goos, rawURL string) (string, []string, error) {
	switch goos {
	case "darwin":
		return "open", []string{rawURL}, nil
	case "windows":
		// rundll32 hands the URL straight to the registered protocol
		// handler; `cmd /c start` would let cmd.exe interpret & | ^ etc.
		return "rundll32", []string{"url.dll,FileProtocolHandler", rawURL}, nil
	case "linux", "freebsd", "openbsd", "netbsd", "dragonfly":
		return "xdg-open", []string{rawURL}, nil
	default:
		return "", nil, fmt.Errorf("unsupported platform: %s", goos)
	}
}
