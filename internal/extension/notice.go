package extension

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/aarondpn/redmine-cli/v2/internal/ghrelease"
)

const (
	// NoticeCacheAge is how long an update check result is reused.
	NoticeCacheAge = 24 * time.Hour
	// maxNoticeClockSkew tolerates small clock differences when rejecting
	// cache entries from the future (which would pin the entry forever).
	maxNoticeClockSkew = 5 * time.Minute
)

// safeVersion matches what an update notice may print: a release tag or a
// commit ID. The cache file and the API are untrusted input.
var safeVersion = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z._+/-]{0,127}$`)

// Notice says that a newer version of an installed extension exists.
type Notice struct {
	Name    string
	Current string
	Latest  string
}

// noticeCache is the cached result of the last update check of one
// extension.
type noticeCache struct {
	CheckedAt time.Time `json:"checked_at"`
	// Current is the installed version the check ran against; a different
	// one (after an upgrade) invalidates the entry.
	Current string `json:"current"`
	Latest  string `json:"latest"`
}

// NoticeEligible reports whether ext gets update notices: it came from
// GitHub and is neither local nor pinned.
func NoticeEligible(ext *Extension) bool {
	return ext != nil && ext.Kind != KindLocal && !ext.Pinned() && ext.Install != nil && ext.Install.Owner != "" && installedVersion(ext) != ""
}

// installedVersion is what the update check compares: the release tag of a
// binary extension, the full commit of a git one.
func installedVersion(ext *Extension) string {
	if ext.Kind == KindBinary {
		return ext.Install.Tag
	}
	return ext.Revision
}

// CheckForUpdate reports whether a newer version of ext exists, reusing a
// check from the last NoticeCacheAge. Only successful checks are cached, so
// a network failure is retried on the next run. It returns (nil, nil) when
// ext is up to date or not eligible.
func (m *Manager) CheckForUpdate(ctx context.Context, ext *Extension) (*Notice, error) {
	if !NoticeEligible(ext) {
		return nil, nil
	}
	current := installedVersion(ext)
	now := m.now()
	if c := m.readNotice(ext.Name); c != nil && c.Current == current && now.Sub(c.CheckedAt) < NoticeCacheAge {
		return noticeFor(ext, c), nil
	}
	latest, err := m.latestVersion(ctx, ext)
	if err != nil {
		return nil, err
	}
	if !safeVersion.MatchString(latest) {
		return nil, errors.New("unexpected version format from GitHub")
	}
	c := &noticeCache{CheckedAt: now.UTC(), Current: current, Latest: latest}
	m.writeNotice(ext.Name, c)
	return noticeFor(ext, c), nil
}

func noticeFor(ext *Extension, c *noticeCache) *Notice {
	if c.Latest == c.Current {
		return nil
	}
	n := &Notice{Name: ext.Name, Current: c.Current, Latest: c.Latest}
	if ext.Kind == KindGit {
		n.Current, n.Latest = shortRevision(n.Current), shortRevision(n.Latest)
	}
	return n
}

// latestVersion asks GitHub for the latest release tag (binary) or the head
// commit of the default branch (git).
func (m *Manager) latestVersion(ctx context.Context, ext *Extension) (string, error) {
	gh := m.github()
	if ext.Kind == KindBinary {
		release, err := gh.FetchRelease(ctx, ext.Install.Owner, ext.Install.Repo, "")
		if err != nil {
			return "", err
		}
		return release.TagName, nil
	}
	if err := ghrelease.ValidateRepo(ext.Install.Owner, ext.Install.Repo); err != nil {
		return "", err
	}
	var commits []struct {
		SHA string `json:"sha"`
	}
	path := "/repos/" + ext.Install.Owner + "/" + ext.Install.Repo + "/commits?per_page=1"
	if err := gh.GetJSON(ctx, gh.APIURL(path), ghrelease.MaxMetadataBytes, &commits); err != nil {
		return "", err
	}
	if len(commits) == 0 {
		return "", errors.New("repository has no commits")
	}
	return commits[0].SHA, nil
}

func (m *Manager) noticePath(name string) string {
	return filepath.Join(m.metaDir(), name+".update-check.json")
}

// readNotice loads a cache entry; anything malformed counts as no entry.
func (m *Manager) readNotice(name string) *noticeCache {
	data, err := readLimited(m.noticePath(name), maxMetadataFile)
	if err != nil {
		return nil
	}
	var c noticeCache
	if json.Unmarshal(data, &c) != nil {
		return nil
	}
	if c.CheckedAt.IsZero() || c.CheckedAt.After(m.now().Add(maxNoticeClockSkew)) || c.Current == "" || !safeVersion.MatchString(c.Latest) {
		return nil
	}
	return &c
}

// writeNotice saves a cache entry; failures only cost a later re-check.
func (m *Manager) writeNotice(name string, c *noticeCache) {
	data, err := json.Marshal(c)
	if err != nil {
		return
	}
	_ = writeFileAtomic(m.noticePath(name), data, 0o600)
}

// clearNotice drops the cache entry after an install, upgrade or removal.
func (m *Manager) clearNotice(name string) {
	_ = os.Remove(m.noticePath(name))
}
