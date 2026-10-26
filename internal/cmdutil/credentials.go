package cmdutil

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/aarondpn/redmine-cli/v2/internal/config"
)

// maxSecretFileSize bounds --api-key-file and --auth-token-file reads. An API
// key is 40 hex characters and a bearer token not much longer, so anything
// near this limit is the wrong file (or an endless stream).
const maxSecretFileSize = 64 << 10

// apiKeyOverride returns the API key supplied by --api-key or --api-key-file
// (cobra makes the two mutually exclusive). The file is read once and cached,
// so a stdin key ("-") survives a retried Config call.
func (f *Factory) apiKeyOverride() (string, error) {
	if f.APIKeyFile == "" {
		return f.APIKeyOverride, nil
	}
	if f.apiKeyFromFile != "" {
		return f.apiKeyFromFile, nil
	}
	var stdin io.Reader
	if f.IOStreams != nil {
		stdin = f.IOStreams.In
	}
	key, err := ReadSecretFile("--api-key-file", f.APIKeyFile, stdin)
	if err != nil {
		return "", err
	}
	f.apiKeyFromFile = key
	return key, nil
}

// ReadSecretFile reads the secret named by a file flag such as
// --api-key-file from path, or from stdin when path is "-", so the secret
// never appears in argv. Surrounding whitespace (such as the trailing newline
// of `echo` or an editor) is trimmed. An empty path, an empty secret and a
// file larger than 64 KiB are errors; flagName prefixes every message.
func ReadSecretFile(flagName, path string, stdin io.Reader) (string, error) {
	var r io.Reader
	switch path {
	case "":
		return "", fmt.Errorf("%s: file path is empty (use \"-\" to read from standard input)", flagName)
	case "-":
		if stdin == nil {
			return "", fmt.Errorf("%s: no standard input available", flagName)
		}
		r = stdin
	default:
		file, err := os.Open(path)
		if err != nil {
			return "", fmt.Errorf("%s: %w", flagName, err)
		}
		defer file.Close()
		r = file
	}
	data, err := io.ReadAll(io.LimitReader(r, maxSecretFileSize+1))
	if err != nil {
		return "", fmt.Errorf("%s: %w", flagName, err)
	}
	if len(data) > maxSecretFileSize {
		return "", fmt.Errorf("%s: %s is larger than %d bytes; it should contain only the secret", flagName, secretSource(path), maxSecretFileSize)
	}
	secret := strings.TrimSpace(string(data))
	if secret == "" {
		return "", fmt.Errorf("%s: %s is empty", flagName, secretSource(path))
	}
	return secret, nil
}

// secretSource names where ReadSecretFile read from, for error messages.
func secretSource(path string) string {
	if path == "-" {
		return "standard input"
	}
	return path
}

// storedCredentialOverrideMessage describes a profile's stored secret being
// sent to a server other than the one it was saved for.
func storedCredentialOverrideMessage(cfg *config.Config, profile, savedServer string) string {
	hint := "pass --api-key-file or REDMINE_API_KEY explicitly to silence this"
	if cfg.AuthMethod == "basic" {
		hint = "set REDMINE_PASSWORD explicitly to silence this"
	}
	return fmt.Sprintf("sending the stored credentials of profile %q (for %s) to %s; %s", profile, originOrRaw(savedServer), originOrRaw(cfg.Server), hint)
}

// profileHeadersInUse returns the sorted, canonical names of the profile's
// headers that the --header flags of this invocation do not replace.
func profileHeadersInUse(profileHeaders map[string]string, flags []string) []string {
	if len(profileHeaders) == 0 {
		return nil
	}
	explicit := make(map[string]bool, len(flags))
	for _, h := range flags {
		if name, _, err := config.ParseHeader(h); err == nil {
			explicit[name] = true
		}
	}
	var names []string
	for k := range profileHeaders {
		name := http.CanonicalHeaderKey(strings.TrimSpace(k))
		if !explicit[name] {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// profileHeadersOverrideMessage describes a profile's configured headers
// being sent to a server other than the one they were saved for. Only the
// header names are included; their values may be secrets.
func profileHeadersOverrideMessage(cfg *config.Config, profile, savedServer string, names []string) string {
	return fmt.Sprintf("sending the configured headers of profile %q (for %s) to %s: %s; pass the headers meant for that server with --header to silence this", profile, originOrRaw(savedServer), originOrRaw(cfg.Server), strings.Join(names, ", "))
}

// originOrRaw returns the normalized origin of server, or server itself when
// it has no parseable host.
func originOrRaw(server string) string {
	if origin := config.ServerOrigin(server); origin != "" {
		return origin
	}
	return server
}
