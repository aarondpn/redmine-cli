package update

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// managedInstall describes a binary owned by a Windows package manager.
// Replacing it in place would leave the package manager's recorded version
// out of sync, so `redmine update` points at the manager's own command.
type managedInstall struct {
	// Manager is the package manager's display name.
	Manager string
	// Command upgrades the installation through the package manager.
	Command string
}

// wingetPackageID is the winget package identifier from .goreleaser.yaml.
const wingetPackageID = "aarondpn.redmine-cli"

// detectManagedInstall is a variable so tests can simulate Scoop and winget
// installations.
var detectManagedInstall = defaultDetectManagedInstall

func defaultDetectManagedInstall() (managedInstall, bool) {
	execPath, err := resolveExecPath()
	if err != nil {
		return managedInstall{}, false
	}
	return managedInstallFor(execPath, os.Getenv)
}

// managedInstallFor reports whether execPath lies inside a Scoop or winget
// installation of redmine. Paths are compared case-insensitively with
// forward slashes, as Windows paths are.
//
// Scoop installs apps under <root>/apps/<app>/<version>, with <root> taken
// from SCOOP or SCOOP_GLOBAL and defaulting to a directory named "scoop".
// winget extracts portable packages under .../WinGet/Packages/<id>_<source>,
// for both user (%LOCALAPPDATA%\Microsoft) and machine (%ProgramFiles%)
// scope; its Links shim is a symlink that resolveExecPath already resolved.
func managedInstallFor(execPath string, getenv func(string) string) (managedInstall, bool) {
	p := normalizeManagedPath(execPath)

	scoopRoots := []string{"/scoop/"}
	for _, env := range []string{"SCOOP", "SCOOP_GLOBAL"} {
		if root := getenv(env); root != "" {
			scoopRoots = append(scoopRoots, strings.TrimSuffix(normalizeManagedPath(root), "/")+"/")
		}
	}
	for _, root := range scoopRoots {
		if strings.Contains(p, root+"apps/"+binaryName+"/") {
			return managedInstall{Manager: "Scoop", Command: "scoop update " + binaryName}, true
		}
	}

	if strings.Contains(p, "/winget/packages/"+strings.ToLower(wingetPackageID)+"_") {
		return managedInstall{Manager: "winget", Command: "winget upgrade " + wingetPackageID}, true
	}
	return managedInstall{}, false
}

func normalizeManagedPath(p string) string {
	return strings.ToLower(strings.ReplaceAll(filepath.ToSlash(p), `\`, "/"))
}

// managedInstallError refuses a self-update of a package-managed binary.
func managedInstallError(m managedInstall, latestTag string) error {
	return fmt.Errorf("redmine %s is available, but this binary was installed with %s; update it with:\n  %s",
		latestTag, m.Manager, m.Command)
}
