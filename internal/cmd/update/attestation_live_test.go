//go:build releaselive

package update

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestReleaseAttestationLive runs the updater's real attestation verifier
// (GitHub attestations API and Sigstore TUF) against the archives of a
// published release, so the release workflow can prove that installed
// updaters accept it. Opt-in:
//
//	REDMINE_RELEASE_TAG=v2.15.0 REDMINE_RELEASE_ASSETS=dist-check \
//	  go test -tags releaselive ./internal/cmd/update -run TestReleaseAttestationLive
func TestReleaseAttestationLive(t *testing.T) {
	tag := os.Getenv("REDMINE_RELEASE_TAG")
	dir := os.Getenv("REDMINE_RELEASE_ASSETS")
	if tag == "" || dir == "" {
		t.Fatal("REDMINE_RELEASE_TAG and REDMINE_RELEASE_ASSETS must be set")
	}
	archives, err := filepath.Glob(filepath.Join(dir, RepoName+"-*"))
	if err != nil {
		t.Fatal(err)
	}
	var checked int
	for _, path := range archives {
		if ext := filepath.Ext(path); ext != ".gz" && ext != ".zip" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(data)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		err = newAttestationVerifier().Verify(ctx, digest[:], tag)
		cancel()
		if err != nil {
			t.Errorf("%s: updater would refuse %s: %v", filepath.Base(path), tag, err)
			continue
		}
		t.Logf("%s: attestation accepted for %s", filepath.Base(path), tag)
		checked++
	}
	if checked == 0 && !t.Failed() {
		t.Fatalf("no %s-* archives found in %s", RepoName, dir)
	}
}
