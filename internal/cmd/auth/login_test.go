package auth

import (
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/config"
	"github.com/aarondpn/redmine-cli/v2/internal/secrets"
)

type stubStore struct{ available bool }

func (s stubStore) Get(profile, field string) (string, bool, error) { return "", false, nil }
func (s stubStore) Set(profile, field, secret string) error         { return nil }
func (s stubStore) Delete(profile, field string) error              { return nil }
func (s stubStore) Available() bool                                 { return s.available }

func TestResolveKeyringChoiceNonTTY(t *testing.T) {
	orig := secrets.Default
	t.Cleanup(func() { secrets.Default = orig })

	f := &cmdutil.Factory{IOStreams: &cmdutil.IOStreams{IsTTY: false}}
	keyringProfile := &config.Config{CredentialStore: config.CredentialStoreKeyring}

	cases := []struct {
		name      string
		available bool
		existing  *config.Config
		want      bool
		wantErr   bool
	}{
		{name: "new profile stays plaintext", available: true},
		{name: "plaintext profile stays plaintext", available: true, existing: &config.Config{}},
		{name: "keyring profile keeps keyring", available: true, existing: keyringProfile, want: true},
		{name: "keyring profile with unusable backend errors", available: false, existing: keyringProfile, wantErr: true},
		{name: "new profile with unusable backend stays plaintext", available: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			secrets.Default = stubStore{available: tc.available}
			got, err := resolveKeyringChoice(f, "work", tc.existing, false, false)
			if tc.wantErr {
				if err == nil {
					t.Fatal("resolveKeyringChoice error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("resolveKeyringChoice = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestResolveKeyringChoiceExplicitFlag(t *testing.T) {
	orig := secrets.Default
	t.Cleanup(func() { secrets.Default = orig })
	secrets.Default = stubStore{available: false}

	f := &cmdutil.Factory{IOStreams: &cmdutil.IOStreams{IsTTY: true}}

	if _, err := resolveKeyringChoice(f, "work", nil, true, true); err == nil {
		t.Fatal("resolveKeyringChoice error = nil, want error for --keyring with unusable backend")
	}

	keyringProfile := &config.Config{CredentialStore: config.CredentialStoreKeyring}
	got, err := resolveKeyringChoice(f, "work", keyringProfile, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if got {
		t.Fatal("resolveKeyringChoice = true, want false: --keyring=false must win over existing keyring profile")
	}
}
