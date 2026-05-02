package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	ShowHidden      bool
	Editor          string
	SafeDelete      bool
	SortDirsFirst   bool
	PreviewMaxBytes int64
	EditorMaxBytes  int64
	EnableLSP       bool
	GoplsCommand    string
	Theme           string
}

func SaveTheme(theme string) error {
	path := Path()
	if path == "" {
		return fmt.Errorf("could not resolve config path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var lines []string
	if data, err := os.ReadFile(path); err == nil {
		lines = strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	}
	found := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "theme") {
			if key, _, ok := strings.Cut(trimmed, "="); ok && strings.TrimSpace(key) == "theme" {
				lines[i] = `theme = "` + theme + `"`
				found = true
			}
		}
	}
	if !found {
		lines = append(lines, `theme = "`+theme+`"`)
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

func Default() Config {
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "nvim"
	}
	return Config{
		ShowHidden:      false,
		Editor:          editor,
		SafeDelete:      true,
		SortDirsFirst:   true,
		PreviewMaxBytes: 256 * 1024,
		EditorMaxBytes:  1024 * 1024,
		EnableLSP:       true,
		GoplsCommand:    "gopls",
		Theme:           "navia",
	}
}

func Path() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "navia", "config.toml")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".config", "navia", "config.toml")
	}
	return ""
}

func Load() (Config, string) {
	cfg := Default()
	path := Path()
	if path == "" {
		return cfg, ""
	}
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, ""
		}
		return cfg, "Could not read config; using defaults."
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return cfg, "Config has an invalid line; using parsed defaults."
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"`)
		switch key {
		case "show_hidden":
			cfg.ShowHidden = parseBool(value, cfg.ShowHidden)
		case "editor":
			if value != "" {
				cfg.Editor = value
			}
		case "safe_delete":
			cfg.SafeDelete = parseBool(value, cfg.SafeDelete)
		case "sort_dirs_first":
			cfg.SortDirsFirst = parseBool(value, cfg.SortDirsFirst)
		case "preview_max_bytes":
			if n, err := strconv.ParseInt(value, 10, 64); err == nil && n > 0 {
				cfg.PreviewMaxBytes = n
			}
		case "editor_max_bytes":
			if n, err := strconv.ParseInt(value, 10, 64); err == nil && n > 0 {
				cfg.EditorMaxBytes = n
			}
		case "enable_lsp":
			cfg.EnableLSP = parseBool(value, cfg.EnableLSP)
		case "gopls_command":
			if value != "" {
				cfg.GoplsCommand = value
			}
		case "theme":
			if value != "" {
				cfg.Theme = value
			}
		}
	}
	if scanner.Err() != nil {
		return cfg, "Could not parse config completely; using parsed defaults."
	}
	return cfg, ""
}

func parseBool(value string, fallback bool) bool {
	switch strings.ToLower(value) {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	default:
		return fallback
	}
}
