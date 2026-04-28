package prompts

import (
	"embed"
	"io/fs"
)

//go:embed *.md
var files embed.FS

// FS returns the embedded prompt templates.
func FS() fs.FS {
	return files
}
