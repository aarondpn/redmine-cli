package extension

import (
	"context"
	"fmt"

	"github.com/aarondpn/redmine-cli/v2/internal/extension/spec"
)

// UpgradeStatus is the outcome of upgrading one extension.
type UpgradeStatus string

const (
	StatusUpgraded UpgradeStatus = "upgraded"
	StatusUpToDate UpgradeStatus = "up_to_date"
	// StatusAvailable: a dry run found a newer version.
	StatusAvailable UpgradeStatus = "available"
	StatusSkipped   UpgradeStatus = "skipped"
	StatusFailed    UpgradeStatus = "failed"
)

// UpgradeOptions controls Upgrade.
type UpgradeOptions struct {
	// DryRun only reports what would change.
	DryRun bool
	// Force reinstalls a binary extension even when it is up to date, and
	// resets a git extension to its upstream, discarding local changes.
	Force bool
}

// UpgradeResult reports the upgrade of one extension.
type UpgradeResult struct {
	Name   string        `json:"name"`
	Status UpgradeStatus `json:"status"`
	From   string        `json:"from,omitempty"`
	To     string        `json:"to,omitempty"`
	Note   string        `json:"note,omitempty"`
}

// Upgrade upgrades the installed extension name. Pinned and local
// extensions are skipped with a note. A binary extension is reinstalled from
// the latest release when its tag differs; a git extension is fast-forwarded
// to its upstream.
func (m *Manager) Upgrade(ctx context.Context, name string, opts UpgradeOptions) (*UpgradeResult, error) {
	ext, err := m.Get(name)
	if err != nil {
		return nil, err
	}
	return m.upgrade(ctx, ext, opts)
}

// UpgradeAll upgrades every installed extension. A failure is recorded in
// that extension's result (StatusFailed) and does not stop the others.
func (m *Manager) UpgradeAll(ctx context.Context, opts UpgradeOptions) ([]*UpgradeResult, error) {
	exts, err := m.List()
	if err != nil {
		return nil, err
	}
	results := make([]*UpgradeResult, 0, len(exts))
	for _, ext := range exts {
		res, err := m.upgrade(ctx, ext, opts)
		if err != nil {
			res = &UpgradeResult{Name: ext.Name, Status: StatusFailed, From: ext.Version(), Note: err.Error()}
		}
		results = append(results, res)
	}
	return results, nil
}

func (m *Manager) upgrade(ctx context.Context, ext *Extension, opts UpgradeOptions) (*UpgradeResult, error) {
	res := &UpgradeResult{Name: ext.Name, From: ext.Version()}
	switch {
	case ext.Kind == KindLocal:
		res.Status, res.Note = StatusSkipped, "local extension; edit it in "+ext.Dir
		return res, nil
	case ext.Pinned():
		res.Status, res.Note = StatusSkipped, "pinned to "+ext.Install.Tag+"; reinstall with 'redmine extension install --force' to change it"
		return res, nil
	case ext.Install == nil || ext.Install.Owner == "":
		return nil, fmt.Errorf("extension %q has no install metadata; reinstall it", ext.Name)
	case ext.Kind == KindBinary:
		return m.upgradeBinary(ctx, ext, opts, res)
	default:
		return m.upgradeGit(ctx, ext, opts, res)
	}
}

func (m *Manager) upgradeBinary(ctx context.Context, ext *Extension, opts UpgradeOptions, res *UpgradeResult) (*UpgradeResult, error) {
	src := Source{Owner: ext.Install.Owner, Repo: ext.Install.Repo, Name: ext.Name}
	release, err := m.github().FetchRelease(ctx, src.Owner, src.Repo, "")
	if err != nil {
		return nil, fmt.Errorf("fetching the latest release of %s: %w", src, err)
	}
	res.To = release.TagName
	if release.TagName == ext.Install.Tag && !opts.Force {
		res.Status = StatusUpToDate
		return res, nil
	}
	if opts.DryRun {
		res.Status = StatusAvailable
		return res, nil
	}
	assetName := BinaryAssetName(ext.Name, m.goos(), m.goarch())
	asset, ok := release.FindAsset(assetName)
	if !ok {
		return nil, fmt.Errorf("release %s of %s has no binary for %s/%s (expected asset %s)", release.TagName, src, m.goos(), m.goarch(), assetName)
	}
	if _, err := m.installBinary(ctx, src, release, asset, false); err != nil {
		return nil, err
	}
	res.Status = StatusUpgraded
	return res, nil
}

func (m *Manager) upgradeGit(ctx context.Context, ext *Extension, opts UpgradeOptions, res *UpgradeResult) (*UpgradeResult, error) {
	git := m.git()
	before, err := git.Revision(ctx, ext.Dir, "HEAD")
	if err != nil {
		return nil, err
	}
	res.From = shortRevision(before)
	if opts.DryRun || opts.Force {
		if err := git.Fetch(ctx, ext.Dir); err != nil {
			return nil, err
		}
	}
	if opts.DryRun {
		upstream, err := git.Revision(ctx, ext.Dir, "@{u}")
		if err != nil {
			return nil, err
		}
		res.To = shortRevision(upstream)
		res.Status = StatusAvailable
		if upstream == before {
			res.Status = StatusUpToDate
		}
		return res, nil
	}
	if opts.Force {
		err = git.ResetHard(ctx, ext.Dir, "@{u}")
	} else {
		err = git.Pull(ctx, ext.Dir)
	}
	if err != nil {
		return nil, err
	}
	after, err := git.Revision(ctx, ext.Dir, "HEAD")
	if err != nil {
		return nil, err
	}
	res.To = shortRevision(after)
	res.Status = StatusUpToDate
	if after != before {
		res.Status = StatusUpgraded
		m.clearNotice(ext.Name)
	}
	// A pull cannot be rolled back atomically like a binary install, so a
	// broken manifest is reported instead; the extension then reports it
	// when run.
	if _, err := m.loadManifest(ext.Dir, ext.Name); err != nil {
		res.Note = "the new " + spec.FileName + " is invalid: " + err.Error()
	}
	return res, nil
}
