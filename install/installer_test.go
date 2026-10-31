package install

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	"gopkg.in/yaml.v3"
)

// TestInstallScriptMatchesGoReleaser renders the archive and checksum
// filenames declared in .goreleaser.yaml and asserts install.sh downloads
// exactly those names. Regression guard for #44/#45.
func TestInstallScriptMatchesGoReleaser(t *testing.T) {
	repoRoot := filepath.Join("..")

	cfg := loadGoReleaser(t, filepath.Join(repoRoot, ".goreleaser.yaml"))
	script := string(readFile(t, filepath.Join(repoRoot, "install.sh")))

	// Require an explicit project_name. GoReleaser's default derives from
	// the module path's last segment, which is "v2" under SIV and would
	// silently rename every release asset. Pinning here prevents that.
	projectName := cfg.ProjectName
	if projectName == "" {
		t.Fatal(".goreleaser.yaml: project_name must be set explicitly")
	}

	archiveTmpl := cfg.archiveNameTemplate()
	if archiveTmpl == "" {
		t.Fatal(".goreleaser.yaml: archives[0].name_template missing")
	}
	checksumTmpl := cfg.checksumNameTemplate()

	// Render with install.sh's runtime variables in template position.
	// GoReleaser's {{ .Os }}/{{ .Arch }} map to install.sh ${OS}/${ARCH};
	// {{ .Version }} maps to ${TAG#v}.
	data := map[string]string{
		"ProjectName": projectName,
		"Os":          "${OS}",
		"Arch":        "${ARCH}",
		"Version":     "${TAG#v}",
	}

	wantArchive := render(t, archiveTmpl, data) + ".tar.gz"
	wantChecksum := render(t, checksumTmpl, data)

	if got := extractShellVar(script, "ARCHIVE"); got != wantArchive {
		t.Errorf("install.sh ARCHIVE=%q, want %q (from goreleaser template %q)", got, wantArchive, archiveTmpl)
	}

	checksumURL := extractShellVar(script, "CHECKSUMS_URL")
	if !strings.HasSuffix(checksumURL, "/"+wantChecksum) {
		t.Errorf("install.sh CHECKSUMS_URL=%q does not end with %q (from goreleaser template %q)", checksumURL, wantChecksum, checksumTmpl)
	}
}

// TestPowerShellInstallScriptMatchesGoReleaser does the same for install.ps1,
// which downloads the Windows zip.
func TestPowerShellInstallScriptMatchesGoReleaser(t *testing.T) {
	repoRoot := filepath.Join("..")

	cfg := loadGoReleaser(t, filepath.Join(repoRoot, ".goreleaser.yaml"))
	script := string(readFile(t, filepath.Join(repoRoot, "install.ps1")))

	// install.ps1 only installs the Windows build; {{ .Arch }} and
	// {{ .Version }} map to its $Arch and $Version.
	data := map[string]string{
		"ProjectName": cfg.ProjectName,
		"Os":          "windows",
		"Arch":        "${Arch}",
		"Version":     "${Version}",
	}

	wantArchive := render(t, cfg.archiveNameTemplate(), data) + ".zip"
	wantChecksum := render(t, cfg.checksumNameTemplate(), data)

	if got := extractPowerShellVar(script, "Archive"); got != wantArchive {
		t.Errorf("install.ps1 $Archive = %q, want %q", got, wantArchive)
	}
	if got := extractPowerShellVar(script, "ChecksumsName"); got != wantChecksum {
		t.Errorf("install.ps1 $ChecksumsName = %q, want %q", got, wantChecksum)
	}
}

type goreleaserConfig struct {
	ProjectName string `yaml:"project_name"`
	Archives    []struct {
		NameTemplate string `yaml:"name_template"`
	} `yaml:"archives"`
	Checksum struct {
		NameTemplate string `yaml:"name_template"`
	} `yaml:"checksum"`
}

func (c goreleaserConfig) archiveNameTemplate() string {
	if len(c.Archives) == 0 {
		return ""
	}
	return c.Archives[0].NameTemplate
}

// checksumNameTemplate returns the configured template or GoReleaser's default.
func (c goreleaserConfig) checksumNameTemplate() string {
	if c.Checksum.NameTemplate != "" {
		return c.Checksum.NameTemplate
	}
	return "{{ .ProjectName }}_{{ .Version }}_checksums.txt"
}

func loadGoReleaser(t *testing.T, path string) goreleaserConfig {
	t.Helper()
	var cfg goreleaserConfig
	if err := yaml.Unmarshal(readFile(t, path), &cfg); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return cfg
}

func render(t *testing.T, tmpl string, data any) string {
	t.Helper()
	parsed, err := template.New("").Parse(tmpl)
	if err != nil {
		t.Fatalf("parse template %q: %v", tmpl, err)
	}
	var buf bytes.Buffer
	if err := parsed.Execute(&buf, data); err != nil {
		t.Fatalf("execute template %q: %v", tmpl, err)
	}
	return buf.String()
}

// extractShellVar returns the double-quoted value of the first top-level
// VAR="..." assignment in a shell script.
func extractShellVar(script, name string) string {
	prefix := name + `="`
	i := strings.Index(script, prefix)
	if i < 0 {
		return ""
	}
	rest := script[i+len(prefix):]
	end := strings.Index(rest, `"`)
	if end < 0 {
		return ""
	}
	return rest[:end]
}

// extractPowerShellVar returns the double-quoted value of the first
// $Name = "..." assignment in a PowerShell script.
func extractPowerShellVar(script, name string) string {
	return extractShellVar(strings.Replace(script, "$"+name+" = \"", name+"=\"", 1), name)
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return b
}
