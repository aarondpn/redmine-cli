package update

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"text/template"

	"gopkg.in/yaml.v3"
)

// TestExpectedAssetNamesMatchGoReleaser guards against drift between the
// names built by expectedAssetName / expectedChecksumsName and the actual
// assets GoReleaser produces. Regression for #44/#45.
func TestExpectedAssetNamesMatchGoReleaser(t *testing.T) {
	cfg := loadGoReleaserConfig(t)

	// Require an explicit project_name. GoReleaser's default derives from
	// the module path's last segment, which is "v2" under SIV and would
	// silently rename every release asset. Pinning here prevents that.
	projectName := cfg.ProjectName
	if projectName == "" {
		t.Fatal(".goreleaser.yaml: project_name must be set explicitly")
	}

	const sampleTag = "v1.2.3"
	data := map[string]string{
		"ProjectName": projectName,
		"Os":          runtime.GOOS,
		"Arch":        runtime.GOARCH,
		"Version":     "1.2.3",
	}

	// Archive
	archiveTmpl := cfg.archiveTemplate(runtime.GOOS)
	if archiveTmpl == "" {
		t.Fatal(".goreleaser.yaml: no archive name_template found")
	}
	wantArchive := renderTmpl(t, archiveTmpl, data) + archiveExt()

	if got := expectedAssetName(); got != wantArchive {
		t.Errorf("expectedAssetName() = %q, want %q (from goreleaser template %q)", got, wantArchive, archiveTmpl)
	}

	// Checksums
	checksumTmpl := cfg.Checksum.NameTemplate
	if checksumTmpl == "" {
		checksumTmpl = "{{ .ProjectName }}_{{ .Version }}_checksums.txt"
	}
	wantChecksum := renderTmpl(t, checksumTmpl, data)

	if got := expectedChecksumsName(sampleTag); got != wantChecksum {
		t.Errorf("expectedChecksumsName(%q) = %q, want %q (from goreleaser template %q)", sampleTag, got, wantChecksum, checksumTmpl)
	}
}

func archiveExt() string {
	if runtime.GOOS == "windows" {
		return ".zip"
	}
	return ".tar.gz"
}

type goreleaserArchive struct {
	NameTemplate    string `yaml:"name_template"`
	FormatOverrides []struct {
		Goos    string   `yaml:"goos"`
		Formats []string `yaml:"formats"`
	} `yaml:"format_overrides"`
}

type goreleaserConfig struct {
	ProjectName string              `yaml:"project_name"`
	Archives    []goreleaserArchive `yaml:"archives"`
	Checksum    struct {
		NameTemplate string `yaml:"name_template"`
	} `yaml:"checksum"`
}

func (c goreleaserConfig) archiveTemplate(_ string) string {
	if len(c.Archives) == 0 {
		return ""
	}
	return c.Archives[0].NameTemplate
}

func loadGoReleaserConfig(t *testing.T) goreleaserConfig {
	t.Helper()
	path := filepath.Join("..", "..", "..", ".goreleaser.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var cfg goreleaserConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return cfg
}

func renderTmpl(t *testing.T, tmpl string, data any) string {
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
