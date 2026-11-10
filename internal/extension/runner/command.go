package runner

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Command returns the program and arguments that run the extension
// executable at path with args. On Windows only .exe/.com/.bat/.cmd files
// run natively; anything else is a script extension and runs through sh
// from Git for Windows, as gh does.
func Command(path string, args []string) (string, []string, error) {
	return command(runtime.GOOS, path, args, exec.LookPath, fileExists)
}

func command(goos, path string, args []string, lookPath func(string) (string, error), exists func(string) bool) (string, []string, error) {
	if goos != "windows" || isNativeWindows(path) {
		return path, args, nil
	}
	sh, err := findSh(lookPath, exists)
	if err != nil {
		return "", nil, err
	}
	return sh, append([]string{path}, args...), nil
}

func isNativeWindows(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".exe", ".com", ".bat", ".cmd":
		return true
	}
	return false
}

// errNoSh explains how to get sh on Windows.
var errNoSh = errors.New("this extension is a script and needs sh, which was not found: install Git for Windows (https://git-scm.com/download/win) or add sh to PATH")

// findSh locates sh on Windows: on PATH, else relative to git.exe in the
// layouts of the Git for Windows installer and of a scoop shim.
func findSh(lookPath func(string) (string, error), exists func(string) bool) (string, error) {
	if sh, err := lookPath("sh"); err == nil {
		return sh, nil
	}
	git, err := lookPath("git")
	if err != nil {
		return "", errNoSh
	}
	gitDir := filepath.Dir(git)
	for _, candidate := range []string{
		// <Git>\cmd\git.exe and <Git>\bin\git.exe
		filepath.Join(gitDir, "..", "bin", "sh.exe"),
		// <Git>\mingw64\bin\git.exe
		filepath.Join(gitDir, "..", "..", "bin", "sh.exe"),
		// scoop: <scoop>\shims\git.exe
		filepath.Join(gitDir, "..", "apps", "git", "current", "bin", "sh.exe"),
	} {
		if exists(candidate) {
			return filepath.Clean(candidate), nil
		}
	}
	return "", errNoSh
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
