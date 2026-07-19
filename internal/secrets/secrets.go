// Package secrets provides an optional, opt-in abstraction over the OS keyring
// for storing user credentials. All operations degrade gracefully when the
// keyring backend is unavailable (headless CI, no D-Bus, locked keychain) so
// the CLI never aborts or hangs because of it.
package secrets

import (
	"errors"
	"os"
	"strings"

	"github.com/zalando/go-keyring"
)

// service is the keyring service name used for all redmine-cli secrets.
const service = "redmine-cli"

// Field names for the two secrets that may live in the keyring.
const (
	FieldAPIKey   = "apikey"
	FieldPassword = "password"
)

// Store abstracts credential storage so the keyring backend can be swapped for
// an in-memory fake in tests.
type Store interface {
	// Get returns the stored secret. ok is false with a nil error when nothing
	// is stored; err is only non-nil for a real backend failure.
	Get(profile, field string) (secret string, ok bool, err error)
	Set(profile, field, secret string) error
	Delete(profile, field string) error
	// Available reports, best-effort, whether the keyring backend is usable on
	// this machine.
	Available() bool
}

// Default is the process-wide Store. Tests may reassign it to substitute a
// fake, or call keyring.MockInit to redirect the real backend in memory.
var Default Store = keyringStore{}

// keyringStore is the zalando/go-keyring backed implementation.
type keyringStore struct{}

func userKey(profile, field string) string {
	return profile + ":" + field
}

// disabled reports whether REDMINE_NO_KEYRING hard-disables all keyring access.
// This is the escape hatch that keeps a misconfigured runner from ever stalling
// on a keyring prompt.
func disabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("REDMINE_NO_KEYRING"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func (keyringStore) Get(profile, field string) (string, bool, error) {
	if disabled() {
		return "", false, nil
	}
	secret, err := keyring.Get(service, userKey(profile, field))
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	return secret, true, nil
}

func (keyringStore) Set(profile, field, secret string) error {
	if disabled() {
		return errors.New("system keyring disabled via REDMINE_NO_KEYRING")
	}
	return keyring.Set(service, userKey(profile, field), secret)
}

func (keyringStore) Delete(profile, field string) error {
	if disabled() {
		return nil
	}
	err := keyring.Delete(service, userKey(profile, field))
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}

func (keyringStore) Available() bool {
	if disabled() {
		return false
	}
	// Probe with a read of an unlikely key: a reachable backend answers with
	// ErrNotFound, while an unusable one returns a connection/platform error.
	_, err := keyring.Get(service, "__redmine_cli_probe__")
	return err == nil || errors.Is(err, keyring.ErrNotFound)
}
