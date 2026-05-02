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
	if len(cfg.IgnoreNames) == 0 {
		t.Fatal("ignore names should have defaults")
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

func TestLoadParsesIgnoreNames(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	path := filepath.Join(dir, "navia", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("ignore_names = \".git, node_modules, dist\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, warning := Load()
	if warning != "" {
		t.Fatalf("warning = %q", warning)
	}
	got := strings.Join(cfg.IgnoreNames, ",")
	if got != ".git,node_modules,dist" {
		t.Fatalf("IgnoreNames = %q", got)
	}
}

func TestLoadClampsPreviewMaxBytes(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	path := filepath.Join(dir, "navia", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("preview_max_bytes = 10737418240\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, warning := Load()
	if warning != "" {
		t.Fatalf("warning = %q", warning)
	}
	if cfg.PreviewMaxBytes != MaxPreviewBytes {
		t.Fatalf("PreviewMaxBytes = %d, want %d", cfg.PreviewMaxBytes, MaxPreviewBytes)
	}
}
