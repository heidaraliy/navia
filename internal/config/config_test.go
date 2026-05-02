package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := Default()
	if !cfg.SafeDelete {
		t.Fatal("safe delete should default on")
	}
	if !cfg.SortDirsFirst {
		t.Fatal("dirs first should default on")
	}
	if cfg.PreviewMaxBytes <= 0 {
		t.Fatal("preview max bytes should be positive")
	}
	if cfg.EditorMaxBytes <= 0 {
		t.Fatal("editor max bytes should be positive")
	}
	if !cfg.EnableLSP {
		t.Fatal("lsp should be enabled by default")
	}
	if cfg.GoplsCommand == "" {
		t.Fatal("gopls command should have a default")
	}
	if cfg.Theme == "" {
		t.Fatal("theme should have a default")
	}
}

func TestSaveThemePersistsConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if err := SaveTheme("dim"); err != nil {
		t.Fatal(err)
	}
	cfg, warning := Load()
	if warning != "" {
		t.Fatalf("warning = %q", warning)
	}
	if cfg.Theme != "dim" {
		t.Fatalf("theme = %q", cfg.Theme)
	}
	if err := SaveTheme("amber"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "navia", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Count(text, "theme =") != 1 {
		t.Fatalf("theme line count in %q", text)
	}
	if !strings.Contains(text, `theme = "amber"`) {
		t.Fatalf("config = %q", text)
	}
}
