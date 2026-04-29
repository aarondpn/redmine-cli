package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/aarondpn/redmine-cli/v2/internal/mcpgen"
)

func main() {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	root, err := findModuleRoot(wd)
	if err != nil {
		log.Fatal(err)
	}
	if err := mcpgen.Write(root); err != nil {
		log.Fatal(err)
	}
}

func findModuleRoot(start string) (string, error) {
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}
