package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/internal/debug"
)

func TestLoadDefaultsOutputFormat(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("server: https://redmine.example.com\nauth_method: apikey\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath, debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}

	if cfg.OutputFormat != "table" {
		t.Fatalf("OutputFormat = %q, want %q", cfg.OutputFormat, "table")
	}
}

func TestSaveOmitsPageSize(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	cfg := &Config{
		Server:         "https://redmine.example.com",
		AuthMethod:     "apikey",
		APIKey:         "secret",
		DefaultProject: "demo",
		OutputFormat:   "json",
		NoColor:        true,
	}

	if err := Save(cfg, cfgPath); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	text := string(data)
	if strings.Contains(text, "page_size:") {
		t.Fatalf("saved config unexpectedly contains page_size:\n%s", text)
	}
	if !strings.Contains(text, "output_format: json") {
		t.Fatalf("saved config missing output_format:\n%s", text)
	}
}
