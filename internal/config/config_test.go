package config

import "testing"

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
}
