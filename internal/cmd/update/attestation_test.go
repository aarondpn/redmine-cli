package update

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
)

// Recorded fixtures: the GitHub attestations API response for the v2.14.1
// redmine-cli-linux-amd64.tar.gz archive, and the Sigstore public-good
// trusted root (as printed by `gh attestation trusted-root`). Verification is
// fully offline: certificates are checked against the transparency log's
// integrated time, not the current time.
const (
	fixtureAttestations = "testdata/attestations-v2.14.1-linux-amd64.json"
	fixtureTrustedRoot  = "testdata/sigstore-trusted-root.json"
	fixtureTag          = "v2.14.1"
	fixtureDigestHex    = "7c9f1653c8f266284a0dbd37c69da3a5341a8fa39b14081e40bde5f28199f0b8"
)

func loadFixtureBundles(t *testing.T) []*bundle.Bundle {
	t.Helper()
	data, err := os.ReadFile(fixtureAttestations)
	if err != nil {
		t.Fatal(err)
	}
	bundles, err := parseAttestationResponse(data)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	return bundles
}

func loadFixtureTrustedRoot(t *testing.T) root.TrustedMaterial {
	t.Helper()
	tr, err := root.NewTrustedRootFromPath(fixtureTrustedRoot)
	if err != nil {
		t.Fatalf("load trusted root: %v", err)
	}
	return tr
}

func fixtureDigest(t *testing.T) []byte {
	t.Helper()
	d, err := hex.DecodeString(fixtureDigestHex)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestVerifyAttestationBundles_RecordedRelease(t *testing.T) {
	err := verifyAttestationBundles(loadFixtureTrustedRoot(t), loadFixtureBundles(t), fixtureDigest(t), fixtureTag)
	if err != nil {
		t.Fatalf("expected recorded v2.14.1 attestation to verify, got: %v", err)
	}
}

func TestVerifyAttestationBundles_Rejects(t *testing.T) {
	otherDigest := fixtureDigest(t)
	otherDigest[0] ^= 0xff

	tests := []struct {
		name   string
		digest []byte
		tag    string
	}{
		// The bundle is genuine, but for a different archive.
		{name: "digest mismatch", digest: otherDigest, tag: fixtureTag},
		// A genuine attestation from an older tag must not authorize a
		// release that claims to be a newer one (downgrade / replay).
		{name: "different tag", digest: fixtureDigest(t), tag: "v99.0.0"},
		// Regex metacharacters in the tag must not widen the SAN match.
		{name: "tag regex wildcard", digest: fixtureDigest(t), tag: "v2.14.."},
		{name: "tag prefix", digest: fixtureDigest(t), tag: "v2.14"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := verifyAttestationBundles(loadFixtureTrustedRoot(t), loadFixtureBundles(t), tt.digest, tt.tag)
			if err == nil {
				t.Fatal("expected verification to fail")
			}
		})
	}
}

func TestVerifyAttestationBundles_TamperedSignature(t *testing.T) {
	data, err := os.ReadFile(fixtureAttestations)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	env := raw["attestations"].([]any)[0].(map[string]any)["bundle"].(map[string]any)["dsseEnvelope"].(map[string]any)
	sig := env["signatures"].([]any)[0].(map[string]any)
	sigBytes, err := base64.StdEncoding.DecodeString(sig["sig"].(string))
	if err != nil {
		t.Fatal(err)
	}
	sigBytes[len(sigBytes)-1] ^= 0x01
	sig["sig"] = base64.StdEncoding.EncodeToString(sigBytes)
	tampered, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}

	bundles, err := parseAttestationResponse(tampered)
	if err != nil {
		t.Fatalf("parse tampered fixture: %v", err)
	}
	if err := verifyAttestationBundles(loadFixtureTrustedRoot(t), bundles, fixtureDigest(t), fixtureTag); err == nil {
		t.Fatal("expected tampered signature to fail verification")
	}
}

func TestVerifyAttestationBundles_UntrustedRoot(t *testing.T) {
	// An empty trusted root trusts no CA and no log, so even a genuine
	// bundle must be rejected.
	empty := &root.BaseTrustedMaterial{}
	if err := verifyAttestationBundles(empty, loadFixtureBundles(t), fixtureDigest(t), fixtureTag); err == nil {
		t.Fatal("expected verification against an empty trusted root to fail")
	}
}

func TestParseAttestationResponse(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr error
	}{
		{name: "empty list", body: `{"attestations":[]}`, wantErr: errNoAttestation},
		{name: "null bundle", body: `{"attestations":[{"bundle":null}]}`, wantErr: errNoAttestation},
		{name: "malformed json", body: `{`},
		{name: "invalid bundle", body: `{"attestations":[{"bundle":{"mediaType":"bogus"}}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseAttestationResponse([]byte(tt.body))
			if err == nil {
				t.Fatal("expected error")
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
			var unavailable *attestationUnavailableError
			if errors.As(err, &unavailable) {
				t.Fatalf("parse errors must not be bypassable as unavailable: %v", err)
			}
		})
	}
}

func stubAttestationsURL(t *testing.T, url string) {
	t.Helper()
	orig := attestationsURL
	t.Cleanup(func() { attestationsURL = orig })
	attestationsURL = url + "/attestations/sha256:%s"
}

func fixtureTrustedRootFn(t *testing.T) func() (root.TrustedMaterial, error) {
	tr := loadFixtureTrustedRoot(t)
	return func() (root.TrustedMaterial, error) { return tr, nil }
}

func TestSigstoreAttestationVerifier_Verify(t *testing.T) {
	fixture, err := os.ReadFile(fixtureAttestations)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name            string
		handler         http.HandlerFunc
		trustedRootErr  error
		wantErr         bool
		wantErrIs       error
		wantUnavailable bool
		wantErrContains string
	}{
		{
			name: "valid attestation",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/attestations/sha256:"+fixtureDigestHex ||
					r.URL.Query().Get("predicate_type") != attestationPredicateType {
					http.NotFound(w, r)
					return
				}
				w.Write(fixture)
			},
		},
		{
			name:      "not found",
			handler:   func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) },
			wantErr:   true,
			wantErrIs: errNoAttestation,
		},
		{
			name:      "empty attestation list",
			handler:   func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"attestations":[]}`) },
			wantErr:   true,
			wantErrIs: errNoAttestation,
		},
		{
			name:            "server error",
			handler:         func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusBadGateway) },
			wantErr:         true,
			wantUnavailable: true,
		},
		{
			name:            "rate limited",
			handler:         func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusForbidden) },
			wantErr:         true,
			wantUnavailable: true,
			wantErrContains: "GITHUB_TOKEN",
		},
		{
			name:            "rejected token",
			handler:         func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusUnauthorized) },
			wantErr:         true,
			wantUnavailable: true,
			wantErrContains: "rejected",
		},
		{
			name: "oversized response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(`{"attestations":[`))
				w.Write([]byte(strings.Repeat(" ", maxAttestationResponseBytes)))
			},
			wantErr:         true,
			wantErrContains: "exceeds",
		},
		{
			name:            "trusted root unavailable",
			handler:         func(w http.ResponseWriter, r *http.Request) { w.Write(fixture) },
			trustedRootErr:  errors.New("tuf: connection refused"),
			wantErr:         true,
			wantUnavailable: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(tt.handler)
			defer ts.Close()
			stubAttestationsURL(t, ts.URL)

			v := &sigstoreAttestationVerifier{httpClient: ts.Client(), trustedMaterial: fixtureTrustedRootFn(t)}
			if tt.trustedRootErr != nil {
				v.trustedMaterial = func() (root.TrustedMaterial, error) { return nil, tt.trustedRootErr }
			}

			err := v.Verify(context.Background(), fixtureDigest(t), fixtureTag)
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error")
			}
			if tt.wantErrIs != nil && !errors.Is(err, tt.wantErrIs) {
				t.Errorf("expected %v, got %v", tt.wantErrIs, err)
			}
			var unavailable *attestationUnavailableError
			if got := errors.As(err, &unavailable); got != tt.wantUnavailable {
				t.Errorf("unavailable = %v, want %v (err: %v)", got, tt.wantUnavailable, err)
			}
			if tt.wantErrContains != "" && !strings.Contains(err.Error(), tt.wantErrContains) {
				t.Errorf("expected error to contain %q, got: %v", tt.wantErrContains, err)
			}
		})
	}
}

func TestSigstoreAttestationVerifier_NetworkError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := ts.URL
	ts.Close()
	stubAttestationsURL(t, url)

	v := &sigstoreAttestationVerifier{httpClient: http.DefaultClient, trustedMaterial: fixtureTrustedRootFn(t)}
	err := v.Verify(context.Background(), fixtureDigest(t), fixtureTag)
	var unavailable *attestationUnavailableError
	if !errors.As(err, &unavailable) {
		t.Fatalf("expected unavailable error for unreachable API, got: %v", err)
	}
}

func TestSigstoreAttestationVerifier_SendsToken(t *testing.T) {
	tests := []struct {
		name        string
		githubToken string
		ghToken     string
		want        string
	}{
		{name: "none", want: ""},
		{name: "GITHUB_TOKEN", githubToken: "gh-a", want: "Bearer gh-a"},
		{name: "GH_TOKEN", ghToken: "gh-b", want: "Bearer gh-b"},
		{name: "GITHUB_TOKEN wins", githubToken: "gh-a", ghToken: "gh-b", want: "Bearer gh-a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GITHUB_TOKEN", tt.githubToken)
			t.Setenv("GH_TOKEN", tt.ghToken)
			var got string
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = r.Header.Get("Authorization")
				http.NotFound(w, r)
			}))
			defer ts.Close()
			stubAttestationsURL(t, ts.URL)

			v := &sigstoreAttestationVerifier{httpClient: ts.Client(), trustedMaterial: fixtureTrustedRootFn(t)}
			_ = v.Verify(context.Background(), fixtureDigest(t), fixtureTag)
			if got != tt.want {
				t.Errorf("Authorization = %q, want %q", got, tt.want)
			}
		})
	}
}
