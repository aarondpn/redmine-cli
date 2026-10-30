//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// realPNG encodes a 16x16 PNG that image tooling (ImageMagick) accepts.
func realPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for x := 0; x < 16; x++ {
		for y := 0; y < 16; y++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 16), G: uint8(y * 16), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// createIssueWithFile creates an issue carrying one attachment with the given
// name and content and returns the attachment's metadata.
func createIssueWithFile(t *testing.T, r *cliRunner, projectIdentifier, filename string, payload []byte) attachmentEntry {
	t.Helper()
	path := filepath.Join(t.TempDir(), filename)
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatalf("write attach file: %v", err)
	}
	var created struct {
		ID int `json:"id"`
	}
	r.runJSON(t, &created, "issues", "create",
		"--project", projectIdentifier,
		"--tracker", firstTrackerName(t, r),
		"--subject", "E2E attachment "+filename,
		"--attach", path)
	if created.ID == 0 {
		t.Fatal("issues create returned no ID")
	}
	return findIssueAttachmentID(t, r, created.ID, filename)
}

func TestAttachments_UpdateAndDelete(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	issueID, filename, payload := createIssueWithAttachment(t, r, proj.Identifier)
	att := findIssueAttachmentID(t, r, issueID, filename)
	id := strconv.Itoa(att.ID)

	newName := "renamed-" + filename
	var env actionEnvelope
	r.runJSON(t, &env, "attachments", "update", id, "--filename", newName, "--description", "e2e description")
	if !env.Ok || env.Action != "updated" || env.Resource != "attachment" || envelopeIntID(env.ID) != att.ID {
		t.Fatalf("update envelope = %+v", env)
	}

	var meta attachmentEntry
	r.runJSON(t, &meta, "attachments", "get", id)
	if meta.Filename != newName || meta.Description != "e2e description" || meta.Filesize != int64(len(payload)) {
		t.Fatalf("attachments get after update = %+v", meta)
	}

	// An empty --description clears it; other fields stay.
	r.runJSON(t, &env, "attachments", "update", id, "--description", "")
	r.runJSON(t, &meta, "attachments", "get", id)
	if meta.Description != "" || meta.Filename != newName {
		t.Fatalf("after clearing description = %+v", meta)
	}

	// At least one field flag is required.
	r.runExpectJSONError(t, "attachments", "update", id)

	// Declined confirmation keeps the attachment.
	r.runJSON(t, &env, "attachments", "delete", id)
	if env.Ok {
		t.Fatalf("cancelled delete envelope = %+v", env)
	}
	r.runJSON(t, &meta, "attachments", "get", id)

	r.runJSON(t, &env, "attachments", "delete", id, "--force")
	if !env.Ok || env.Action != "deleted" || env.Resource != "attachment" || envelopeIntID(env.ID) != att.ID {
		t.Fatalf("delete envelope = %+v", env)
	}
	assertErrorCode(t, r.runExpectJSONError(t, "attachments", "get", id), "not_found")
}

func TestAttachments_Thumbnail(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	att := createIssueWithFile(t, r, proj.Identifier, "thumb-"+strconv.FormatInt(time.Now().UnixNano(), 36)+".png", realPNG(t))
	id := strconv.Itoa(att.ID)

	dest := filepath.Join(t.TempDir(), "thumb.png")
	stdout, stderr, err := r.runRaw("attachments", "thumbnail", id, "--path", dest, "--size", "100")
	if err != nil {
		if strings.Contains(string(stderr), `"code":"not_found"`) {
			t.Skip("thumbnails unavailable on this server (ImageMagick convert missing)")
		}
		t.Fatalf("thumbnail failed: %v\nstderr:\n%s", err, stderr)
	}
	var res struct {
		ID    int    `json:"id"`
		Path  string `json:"path"`
		Bytes int64  `json:"bytes"`
		Size  int    `json:"size"`
	}
	r.runJSON(t, &res, "attachments", "thumbnail", id, "--path", dest, "--size", "100", "--clobber")
	if res.ID != att.ID || res.Path != dest || res.Size != 100 || res.Bytes == 0 {
		t.Fatalf("thumbnail result = %+v (first run stdout: %s)", res, stdout)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if int64(len(got)) != res.Bytes || !bytes.HasPrefix(got, []byte("\x89PNG")) {
		t.Fatalf("thumbnail file is not a PNG of %d bytes (%d bytes, prefix %q)", res.Bytes, len(got), got[:min(8, len(got))])
	}

	// An existing file is protected without --clobber.
	r.runExpectJSONError(t, "attachments", "thumbnail", id, "--path", dest)

	// --path - streams the image bytes.
	out, _, err := r.runRaw("attachments", "thumbnail", id, "--path", "-")
	if err != nil || !bytes.HasPrefix(out, []byte("\x89PNG")) {
		t.Fatalf("thumbnail to stdout: err=%v prefix=%q", err, out[:min(8, len(out))])
	}
}

func TestAttachments_ThumbnailNotImage(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	att := createIssueWithFile(t, r, proj.Identifier, "notes-"+strconv.FormatInt(time.Now().UnixNano(), 36)+".txt", []byte("plain text\n"))
	dest := filepath.Join(t.TempDir(), "thumb.png")
	assertErrorCode(t, r.runExpectJSONError(t, "attachments", "thumbnail", strconv.Itoa(att.ID), "--path", dest), "not_found")
	if _, err := os.Stat(dest); err == nil {
		t.Fatal("no file may be left behind when the thumbnail request fails")
	}
}

func TestMCPStdio_AttachmentWriteTools(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	issueID, filename, _ := createIssueWithAttachment(t, r, proj.Identifier)
	att := findIssueAttachmentID(t, r, issueID, filename)
	id := strconv.Itoa(att.ID)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	session := r.startMCPStdio(t, ctx, "--enable-writes")

	out, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "update_attachment",
		Arguments: map[string]any{"id": att.ID, "filename": "mcp-" + filename, "description": "via mcp"},
	})
	if err != nil || out.IsError {
		t.Fatalf("update_attachment: err=%v result=%s", err, structuredJSON(out))
	}
	var meta attachmentEntry
	r.runJSON(t, &meta, "attachments", "get", id)
	if meta.Filename != "mcp-"+filename || meta.Description != "via mcp" {
		t.Fatalf("after update_attachment = %+v", meta)
	}

	out, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "update_attachment", Arguments: map[string]any{"id": att.ID}})
	if err != nil || !out.IsError {
		t.Fatalf("update_attachment without fields should be a tool error: err=%v result=%s", err, structuredJSON(out))
	}

	out, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "delete_attachment", Arguments: map[string]any{"id": att.ID}})
	if err != nil || out.IsError {
		t.Fatalf("delete_attachment: err=%v result=%s", err, structuredJSON(out))
	}
	assertErrorCode(t, r.runExpectJSONError(t, "attachments", "get", id), "not_found")
}
