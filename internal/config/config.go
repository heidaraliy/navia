package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	ShowHidden      bool
	Editor          string
	SortDirsFirst   bool
	PreviewMaxBytes int64
	Theme           string
	IgnoreNames     []string
}

const MaxPreviewBytes int64 = 4 * 1024 * 1024

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
		SortDirsFirst:   true,
		PreviewMaxBytes: 256 * 1024,
		Theme:           "navia",
		IgnoreNames:     []string{".git", "node_modules", ".next", "dist", "build", "target", ".cache"},
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
		case "sort_dirs_first":
			cfg.SortDirsFirst = parseBool(value, cfg.SortDirsFirst)
		case "preview_max_bytes":
			if n, err := strconv.ParseInt(value, 10, 64); err == nil && n > 0 {
				if n > MaxPreviewBytes {
					n = MaxPreviewBytes
				}
				cfg.PreviewMaxBytes = n
			}
		case "theme":
			if value != "" {
				cfg.Theme = value
			}
		case "ignore_names":
			cfg.IgnoreNames = parseCSV(value)
		}
	}
	if scanner.Err() != nil {
		return cfg, "Could not parse config completely; using parsed defaults."
	}
	return cfg, ""
}

func parseCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
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
