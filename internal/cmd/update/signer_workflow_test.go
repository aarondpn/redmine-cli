package update

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReleaseWorkflowMatchesPinnedSigner guards the identity every installed
// updater pins: attestations must come from the workflow file named in
// attestationWorkflowSAN, running for a tag push in this repository. Renaming
// or moving that workflow, or releasing from anything but a v* tag push, makes
// every released updater refuse new releases (short of --skip-attestation).
func TestReleaseWorkflowMatchesPinnedSigner(t *testing.T) {
	const repoPrefix = "https://github.com/" + RepoOwner + "/" + RepoName + "/"
	if !strings.HasPrefix(attestationWorkflowSAN, repoPrefix) || !strings.HasSuffix(attestationWorkflowSAN, "@") {
		t.Fatalf("attestationWorkflowSAN = %q, want %s<workflow path>@", attestationWorkflowSAN, repoPrefix)
	}
	workflowPath := strings.TrimSuffix(strings.TrimPrefix(attestationWorkflowSAN, repoPrefix), "@")

	data, err := os.ReadFile(filepath.Join("..", "..", "..", filepath.FromSlash(workflowPath)))
	if err != nil {
		t.Fatalf("pinned release workflow %s: %v", workflowPath, err)
	}
	workflow := string(data)
	for _, want := range []string{
		"push:\n    tags:\n      - \"v*\"",
		"id-token: write",
		"attestations: write",
		"uses: actions/attest@",
		"subject-checksums:",
	} {
		if !strings.Contains(workflow, want) {
			t.Errorf("%s no longer contains %q; installed updaters pin this workflow as the release signer", workflowPath, want)
		}
	}

	gomod, err := os.ReadFile(filepath.Join("..", "..", "..", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if module := "module github.com/" + RepoOwner + "/" + RepoName; !strings.HasPrefix(string(gomod), module) {
		t.Errorf("go.mod does not start with %q; the pinned signer repository would no longer match", module)
	}
}
