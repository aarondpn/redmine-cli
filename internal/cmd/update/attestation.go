package update

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/fulcio/certificate"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/tuf"
	"github.com/sigstore/sigstore-go/pkg/util"
	"github.com/sigstore/sigstore-go/pkg/verify"
	"github.com/theupdateframework/go-tuf/v2/metadata/fetcher"
)

const (
	// attestationOIDCIssuer is the Fulcio certificate issuer for GitHub
	// Actions workload identities.
	attestationOIDCIssuer = "https://token.actions.githubusercontent.com"
	// attestationPredicateType is the predicate actions/attest emits for
	// build provenance.
	attestationPredicateType = "https://slsa.dev/provenance/v1"
	// attestationWorkflowSAN is the certificate SAN prefix every release
	// attestation must carry: the release workflow of this repository.
	attestationWorkflowSAN = "https://github.com/" + RepoOwner + "/" + RepoName + "/.github/workflows/release.yml@"
	// maxAttestationResponseBytes bounds the attestations API response. A
	// single Sigstore bundle is a few KiB; the release workflow only attests
	// each archive once, so anything near this size is not a real response.
	maxAttestationResponseBytes = 8 << 20
	// tufCacheValidityDays is how long the cached Sigstore trusted root is
	// used before TUF metadata is refreshed.
	tufCacheValidityDays = 1
	// attestationHTTPTimeout bounds each request to the attestations API and
	// the Sigstore TUF repository.
	attestationHTTPTimeout = 30 * time.Second
)

// attestationsURL is the GitHub attestations API endpoint; %s is the hex
// sha256 digest. It is a variable so tests can point it at a local server.
var attestationsURL = "https://api.github.com/repos/" + RepoOwner + "/" + RepoName + "/attestations/sha256:%s"

// errNoAttestation means GitHub has no attestation for the digest. This is
// treated as a verification failure, never as a transient error.
var errNoAttestation = errors.New("no attestation found for this archive")

// attestationUnavailableError wraps failures to reach the attestations API
// or the Sigstore TUF repository. Only these may be bypassed with
// --skip-attestation; invalid or missing attestations always refuse.
type attestationUnavailableError struct {
	err error
}

func (e *attestationUnavailableError) Error() string { return e.err.Error() }
func (e *attestationUnavailableError) Unwrap() error { return e.err }

// attestationVerifier checks that the archive with the given sha256 digest
// was attested by this repository's release workflow for tag.
type attestationVerifier interface {
	Verify(ctx context.Context, digest []byte, tag string) error
}

// newAttestationVerifier is a variable so tests can inject a fake verifier.
var newAttestationVerifier = func() attestationVerifier {
	return &sigstoreAttestationVerifier{
		httpClient:      &http.Client{Timeout: attestationHTTPTimeout},
		trustedMaterial: fetchSigstoreTrustedRoot,
	}
}

// sigstoreAttestationVerifier verifies GitHub artifact attestations against
// the Sigstore public-good instance.
type sigstoreAttestationVerifier struct {
	httpClient      *http.Client
	trustedMaterial func(ctx context.Context) (root.TrustedMaterial, error)
}

func (v *sigstoreAttestationVerifier) Verify(ctx context.Context, digest []byte, tag string) error {
	bundles, err := v.fetchBundles(ctx, digest)
	if err != nil {
		return err
	}

	trusted, err := v.trustedMaterial(ctx)
	if err != nil {
		return &attestationUnavailableError{err: fmt.Errorf("loading Sigstore trusted root: %w", err)}
	}

	return verifyAttestationBundles(trusted, bundles, digest, tag)
}

// fetchBundles downloads the Sigstore bundles GitHub stores for digest.
func (v *sigstoreAttestationVerifier) fetchBundles(ctx context.Context, digest []byte) ([]*bundle.Bundle, error) {
	// Filter server-side so unrelated attestations (e.g. SBOMs) cannot push
	// the provenance off the first page.
	reqURL := fmt.Sprintf(attestationsURL, hex.EncodeToString(digest)) +
		"?predicate_type=" + url.QueryEscape(attestationPredicateType)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	// A token only raises the rate limit; the attestations of a public
	// repository are readable anonymously.
	if token := githubToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return nil, &attestationUnavailableError{err: fmt.Errorf("querying GitHub attestations API: %w", err)}
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, errNoAttestation
	case resp.StatusCode != http.StatusOK:
		hint := ""
		switch resp.StatusCode {
		case http.StatusForbidden, http.StatusTooManyRequests:
			hint = " (rate limited? set GITHUB_TOKEN or GH_TOKEN)"
		case http.StatusUnauthorized:
			hint = " (GITHUB_TOKEN or GH_TOKEN rejected; unset it or use a valid token)"
		}
		return nil, &attestationUnavailableError{err: fmt.Errorf("GitHub attestations API returned status %d%s", resp.StatusCode, hint)}
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxAttestationResponseBytes+1))
	if err != nil {
		return nil, &attestationUnavailableError{err: fmt.Errorf("reading GitHub attestations API response: %w", err)}
	}
	if len(body) > maxAttestationResponseBytes {
		return nil, fmt.Errorf("GitHub attestations API response exceeds %d bytes", maxAttestationResponseBytes)
	}

	return parseAttestationResponse(body)
}

// parseAttestationResponse decodes the attestations API response body into
// Sigstore bundles. Entries without an inline bundle are skipped.
func parseAttestationResponse(body []byte) ([]*bundle.Bundle, error) {
	var payload struct {
		Attestations []struct {
			Bundle json.RawMessage `json:"bundle"`
		} `json:"attestations"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decoding GitHub attestations API response: %w", err)
	}

	var bundles []*bundle.Bundle
	var parseErr error
	for _, a := range payload.Attestations {
		if len(a.Bundle) == 0 || string(a.Bundle) == "null" {
			continue
		}
		var b bundle.Bundle
		if err := b.UnmarshalJSON(a.Bundle); err != nil {
			parseErr = err
			continue
		}
		bundles = append(bundles, &b)
	}
	if len(bundles) == 0 {
		if parseErr != nil {
			return nil, fmt.Errorf("parsing attestation bundle: %w", parseErr)
		}
		return nil, errNoAttestation
	}
	return bundles, nil
}

// verifyAttestationBundles succeeds if at least one bundle is a valid SLSA
// provenance attestation for digest, signed by the release workflow running
// for tag, with a transparency log entry and SCT from the trusted root.
func verifyAttestationBundles(trusted root.TrustedMaterial, bundles []*bundle.Bundle, digest []byte, tag string) error {
	verifier, err := verify.NewVerifier(trusted,
		verify.WithSignedCertificateTimestamps(1),
		verify.WithTransparencyLog(1),
		verify.WithObserverTimestamps(1),
	)
	if err != nil {
		return fmt.Errorf("creating Sigstore verifier: %w", err)
	}

	identity, err := releaseCertificateIdentity(tag)
	if err != nil {
		return err
	}
	policy := verify.NewPolicy(
		verify.WithArtifactDigest("sha256", digest),
		verify.WithCertificateIdentity(identity),
	)

	var lastErr error
	for _, b := range bundles {
		result, err := verifier.Verify(b, policy)
		if err != nil {
			lastErr = err
			continue
		}
		if result.Statement == nil || result.Statement.GetPredicateType() != attestationPredicateType {
			lastErr = fmt.Errorf("attestation predicate type is not %s", attestationPredicateType)
			continue
		}
		return nil
	}
	if lastErr == nil {
		return errNoAttestation
	}
	return lastErr
}

// releaseCertificateIdentity pins the signing identity to the release
// workflow of this repository, running for exactly tag.
func releaseCertificateIdentity(tag string) (verify.CertificateIdentity, error) {
	san, err := verify.NewSANMatcher("", "^"+regexp.QuoteMeta(attestationWorkflowSAN+"refs/tags/"+tag)+"$")
	if err != nil {
		return verify.CertificateIdentity{}, err
	}
	issuer, err := verify.NewIssuerMatcher(attestationOIDCIssuer, "")
	if err != nil {
		return verify.CertificateIdentity{}, err
	}
	return verify.NewCertificateIdentity(san, issuer, certificate.Extensions{
		SourceRepositoryURI: "https://github.com/" + RepoOwner + "/" + RepoName,
		SourceRepositoryRef: "refs/tags/" + tag,
	})
}

// fetchSigstoreTrustedRoot obtains the Sigstore public-good trusted root via
// TUF, anchored at the root embedded in sigstore-go and cached under the
// user cache directory. Cancelling ctx aborts any TUF download in flight.
func fetchSigstoreTrustedRoot(ctx context.Context) (root.TrustedMaterial, error) {
	opts := tuf.DefaultOptions().WithCacheValidity(tufCacheValidityDays).WithFetcher(newTUFFetcher(ctx))
	if dir, err := os.UserCacheDir(); err == nil && dir != "" {
		opts = opts.WithCachePath(filepath.Join(dir, "redmine-cli", "sigstore-tuf"))
	} else {
		opts = opts.WithDisableLocalCache()
	}
	client, err := tuf.New(opts)
	if err != nil {
		return nil, err
	}
	return root.GetTrustedRoot(client)
}

// newTUFFetcher returns a TUF metadata fetcher whose requests time out and are
// cancelled with ctx. The default fetcher uses http.DefaultClient, which never
// times out (an unresponsive TUF CDN would hang the update), and neither
// go-tuf nor sigstore-go pass a context to its requests, so the client
// attaches ctx itself.
func newTUFFetcher(ctx context.Context) *fetcher.DefaultFetcher {
	f := fetcher.NewDefaultFetcher()
	f.SetHTTPUserAgent(util.ConstructUserAgent())
	f.SetHTTPClient(contextHTTPClient{ctx: ctx, client: &http.Client{Timeout: attestationHTTPTimeout}})
	return f
}

// contextHTTPClient sends every request with ctx.
type contextHTTPClient struct {
	ctx    context.Context
	client *http.Client
}

func (c contextHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return c.client.Do(req.WithContext(c.ctx))
}

func githubToken() string {
	if t := os.Getenv("GITHUB_TOKEN"); t != "" {
		return t
	}
	return os.Getenv("GH_TOKEN")
}
