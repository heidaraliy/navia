package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := Default()
	if !cfg.SortDirsFirst {
		t.Fatal("dirs first should default on")
	}
	if cfg.PreviewMaxBytes <= 0 {
		t.Fatal("preview max bytes should be positive")
	}
	if cfg.Theme == "" {
		t.Fatal("theme should have a default")
	}
	if len(cfg.IgnoreNames) == 0 {
		t.Fatal("ignore names should have defaults")
	}
}

func TestDefaultConfigHonorsEditorEnv(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "ed")
	if got := Default().Editor; got != "ed" {
		t.Fatalf("editor = %q want ed", got)
	}
	t.Setenv("VISUAL", "vim")
	if got := Default().Editor; got != "vim" {
		t.Fatalf("editor = %q want vim", got)
	}
}

func TestDefaultConfigFallsBackToNvim(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	if got := Default().Editor; got != "nvim" {
		t.Fatalf("editor = %q want nvim", got)
	}
}

func TestPathUsesXDGConfigHome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	want := filepath.Join(dir, "navia", "config.toml")
	if got := Path(); got != want {
		t.Fatalf("Path() = %q want %q", got, want)
	}
}

func TestLoadParsesConfigAndWarnings(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	path := filepath.Join(dir, "navia", "config.toml")
	mustWriteConfig(t, path, strings.Join([]string{
		"# comment",
		"show_hidden = true",
		`editor = "nano"`,
		"safe_delete = false",
		"sort_dirs_first = off",
		"preview_max_bytes = 12",
		"editor_max_bytes = 34",
		"enable_lsp = no",
		`gopls_command = "custom-gopls"`,
		`theme = "dim"`,
		`ignore_names = ".git, node_modules, dist"`,
		"ignored = value",
		"",
	}, "\n"))

	cfg, warning := Load()
	if warning != "" {
		t.Fatalf("warning = %q", warning)
	}
	if !cfg.ShowHidden || cfg.Editor != "nano" || cfg.SortDirsFirst ||
		cfg.PreviewMaxBytes != 12 || cfg.Theme != "dim" ||
		strings.Join(cfg.IgnoreNames, ",") != ".git,node_modules,dist" {
		t.Fatalf("cfg = %#v", cfg)
	}

	mustWriteConfig(t, path, "bad line\n")
	if _, warning := Load(); warning == "" {
		t.Fatal("invalid config did not return warning")
	}

	mustWriteConfig(t, path, strings.Repeat("x", 70*1024))
	if _, warning := Load(); warning == "" {
		t.Fatal("scanner error did not return warning")
	}
}

func TestLoadClampsPreviewMaxBytes(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	path := filepath.Join(dir, "navia", "config.toml")
	mustWriteConfig(t, path, "preview_max_bytes = 10737418240\n")

	cfg, warning := Load()
	if warning != "" {
		t.Fatalf("warning = %q", warning)
	}
	if cfg.PreviewMaxBytes != MaxPreviewBytes {
		t.Fatalf("PreviewMaxBytes = %d, want %d", cfg.PreviewMaxBytes, MaxPreviewBytes)
	}
}

func TestParseBoolFallback(t *testing.T) {
	if !parseBool("maybe", true) {
		t.Fatal("parseBool should preserve true fallback")
	}
	if parseBool("maybe", false) {
		t.Fatal("parseBool should preserve false fallback")
	}
}

func TestParseCSVSkipsEmptyParts(t *testing.T) {
	got := strings.Join(parseCSV("a, ,b,, c "), ",")
	if got != "a,b,c" {
		t.Fatalf("parseCSV = %q", got)
	}
}

func mustWriteConfig(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
