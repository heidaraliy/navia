package syntax

import (
	"strings"
	"testing"
)

func TestThemesAndHighlightFallback(t *testing.T) {
	for _, name := range []string{"navia", "dim", "mono", "amber"} {
		if !Valid(name) {
			t.Fatalf("theme %q should be valid", name)
		}
	}
	if Valid("missing") {
		t.Fatal("missing theme should be invalid")
	}
	r := New("navia")
	out := r.HighlightLine("main.go", "package main")
	if !strings.Contains(out, "\x1b[") {
		t.Fatalf("expected ansi highlighting, got %q", out)
	}
	if got := r.HighlightLine("unknown.nope", "plain"); got != "plain" {
		t.Fatalf("fallback = %q", got)
	}
}

func TestHighlightLineWithSearchHighlightsPlainFallback(t *testing.T) {
	r := New("navia")
	out := r.HighlightLineWithSearch("unknown.nope", "alpha beta", "BET")
	if !strings.Contains(out, "\x1b[48;2;229;231;146m") {
		t.Fatalf("expected search highlight, got %q", out)
	}
}
