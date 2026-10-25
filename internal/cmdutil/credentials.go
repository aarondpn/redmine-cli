package cmdutil

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/aarondpn/redmine-cli/v2/internal/config"
)

// maxAPIKeyFileSize bounds --api-key-file reads; a Redmine API key is 40 hex
// characters, so anything near this limit is the wrong file.
const maxAPIKeyFileSize = 64 << 10

// apiKeyOverride returns the API key supplied by --api-key or --api-key-file.
// The file is read once and cached, so a stdin key ("-") survives a retried
// Config call.
func (f *Factory) apiKeyOverride() (string, error) {
	if f.APIKeyFile == "" {
		return f.APIKeyOverride, nil
	}
	if f.APIKeyOverride != "" {
		return "", errors.New("--api-key and --api-key-file cannot be used together")
	}
	if f.apiKeyFromFile != "" {
		return f.apiKeyFromFile, nil
	}
	key, err := readAPIKeyFile(f.APIKeyFile, f.IOStreams)
	if err != nil {
		return "", err
	}
	f.apiKeyFromFile = key
	return key, nil
}

// readAPIKeyFile reads an API key from path ("-" reads stdin) and trims the
// surrounding whitespace, including the trailing newline editors and
// `echo` add.
func readAPIKeyFile(path string, ios *IOStreams) (string, error) {
	var r io.Reader
	if path == "-" {
		if ios == nil || ios.In == nil {
			return "", errors.New("--api-key-file -: no stdin available")
		}
		r = ios.In
	} else {
		file, err := os.Open(path)
		if err != nil {
			return "", fmt.Errorf("reading --api-key-file: %w", err)
		}
		defer file.Close()
		r = file
	}
	data, err := io.ReadAll(io.LimitReader(r, maxAPIKeyFileSize+1))
	if err != nil {
		return "", fmt.Errorf("reading --api-key-file: %w", err)
	}
	if len(data) > maxAPIKeyFileSize {
		return "", fmt.Errorf("--api-key-file %s is larger than %d bytes; it should contain only the API key", path, maxAPIKeyFileSize)
	}
	key := strings.TrimSpace(string(data))
	if key == "" {
		return "", fmt.Errorf("--api-key-file %s is empty", path)
	}
	return key, nil
}

// storedCredentialOverrideMessage describes a profile's stored secret being
// sent to a server other than the one it was saved for.
func storedCredentialOverrideMessage(cfg *config.Config, profile, savedServer string) string {
	hint := "pass --api-key/REDMINE_API_KEY explicitly to silence this"
	if cfg.AuthMethod == "basic" {
		hint = "set REDMINE_PASSWORD explicitly to silence this"
	}
	return fmt.Sprintf("sending the stored credentials of profile %q (for %s) to %s; %s", profile, originOrRaw(savedServer), originOrRaw(cfg.Server), hint)
}

// originOrRaw returns the normalized origin of server, or server itself when
// it has no parseable host.
func originOrRaw(server string) string {
	if origin := config.ServerOrigin(server); origin != "" {
		return origin
	}
	return server
}
