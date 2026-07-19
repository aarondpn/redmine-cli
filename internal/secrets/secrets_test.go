package secrets

import (
	"errors"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestGetNotFound(t *testing.T) {
	keyring.MockInit()

	secret, ok, err := Default.Get("work", FieldAPIKey)
	if err != nil {
		t.Fatalf("Get error = %v, want nil", err)
	}
	if ok {
		t.Fatalf("ok = true, want false for missing secret")
	}
	if secret != "" {
		t.Fatalf("secret = %q, want empty", secret)
	}
}

func TestSetGetRoundtrip(t *testing.T) {
	keyring.MockInit()

	if err := Default.Set("work", FieldAPIKey, "abc123"); err != nil {
		t.Fatalf("Set error = %v", err)
	}

	secret, ok, err := Default.Get("work", FieldAPIKey)
	if err != nil {
		t.Fatalf("Get error = %v", err)
	}
	if !ok {
		t.Fatalf("ok = false, want true")
	}
	if secret != "abc123" {
		t.Fatalf("secret = %q, want %q", secret, "abc123")
	}
}

func TestDelete(t *testing.T) {
	keyring.MockInit()

	if err := Default.Set("work", FieldPassword, "hunter2"); err != nil {
		t.Fatalf("Set error = %v", err)
	}
	if err := Default.Delete("work", FieldPassword); err != nil {
		t.Fatalf("Delete error = %v", err)
	}

	_, ok, err := Default.Get("work", FieldPassword)
	if err != nil {
		t.Fatalf("Get error = %v", err)
	}
	if ok {
		t.Fatalf("ok = true after delete, want false")
	}
}

func TestDeleteMissingIsNoError(t *testing.T) {
	keyring.MockInit()

	if err := Default.Delete("work", FieldAPIKey); err != nil {
		t.Fatalf("Delete missing entry error = %v, want nil", err)
	}
}

func TestNoKeyringDisables(t *testing.T) {
	keyring.MockInit()
	t.Setenv("REDMINE_NO_KEYRING", "1")

	if Default.Available() {
		t.Fatalf("Available() = true with REDMINE_NO_KEYRING set")
	}
	if err := Default.Set("work", FieldAPIKey, "abc123"); err == nil {
		t.Fatalf("Set error = nil with REDMINE_NO_KEYRING set, want error")
	}

	_, ok, err := Default.Get("work", FieldAPIKey)
	if err != nil {
		t.Fatalf("Get error = %v, want nil", err)
	}
	if ok {
		t.Fatalf("ok = true with REDMINE_NO_KEYRING set, want false")
	}
}

func TestGetSurfacesBackendError(t *testing.T) {
	backendErr := errors.New("dbus unavailable")
	keyring.MockInitWithError(backendErr)

	_, ok, err := Default.Get("work", FieldAPIKey)
	if err == nil {
		t.Fatalf("Get error = nil, want backend error")
	}
	if !errors.Is(err, backendErr) {
		t.Fatalf("Get error = %v, want %v", err, backendErr)
	}
	if ok {
		t.Fatalf("ok = true on backend error, want false")
	}
}

func TestAvailableFalseOnBackendError(t *testing.T) {
	keyring.MockInitWithError(errors.New("dbus unavailable"))

	if Default.Available() {
		t.Fatalf("Available() = true on backend error, want false")
	}
}

func TestAvailableTrueWithMock(t *testing.T) {
	keyring.MockInit()

	if !Default.Available() {
		t.Fatalf("Available() = false with working mock backend, want true")
	}
}
