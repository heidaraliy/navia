package syntax

import (
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
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

func TestHighlightEscapesUntrustedTerminalControls(t *testing.T) {
	r := New("navia")
	for _, out := range []string{
		r.HighlightLine("unknown.nope", "alpha\x1b]52;c;boom\x07"),
		r.HighlightLine("notes.md", "# title\x1b[2J"),
		r.HighlightLine("main.go", "package main\x1b[2J"),
	} {
		if strings.Contains(out, "\x1b]") || strings.Contains(out, "\x1b[2J") || strings.Contains(out, "\x07") {
			t.Fatalf("highlight left unsafe terminal controls in %q", out)
		}
		if !strings.Contains(out, `\x1b`) {
			t.Fatalf("highlight did not render escaped control visibly: %q", out)
		}
	}
}

func TestMarkdownRichHighlighting(t *testing.T) {
	r := New("navia")
	heading := r.HighlightLine("notes.md", "# Roadmap")
	if !strings.Contains(heading, markdownH1Style) || !strings.Contains(heading, "# Roadmap") {
		t.Fatalf("heading highlight = %q", heading)
	}

	task := r.HighlightLine("notes.md", "- [ ] write docs")
	if !strings.Contains(task, markdownMarkerStyle) || !strings.Contains(task, markdownOpenStyle) || !strings.Contains(task, "[ ]") {
		t.Fatalf("task highlight = %q", task)
	}

	done := r.HighlightLine("notes.md", "1. [x] shipped")
	if !strings.Contains(done, markdownDoneStyle) || !strings.Contains(done, "[x]") {
		t.Fatalf("done task highlight = %q", done)
	}

	inline := r.HighlightLine("notes.md", "Use `navia` from [README](README.md).")
	if !strings.Contains(inline, markdownCodeStyle) || !strings.Contains(inline, markdownLinkStyle) {
		t.Fatalf("inline highlight = %q", inline)
	}

	search := r.HighlightLineWithSearch("notes.md", "## Todo", "todo")
	if !strings.Contains(search, markdownH2Style) || !strings.Contains(search, "\x1b[48;2;229;231;146m") {
		t.Fatalf("markdown search highlight = %q", search)
	}
}

func TestNamesNewAndStyledSearch(t *testing.T) {
	names := Names()
	if strings.Join(names, ",") != "amber,dim,mono,navia" {
		t.Fatalf("Names() = %#v", names)
	}
	if got := New("").Name; got != "navia" {
		t.Fatalf("New empty name = %q", got)
	}
	if got := New("missing").Name; got != "navia" {
		t.Fatalf("New invalid name = %q", got)
	}
	r := New("mono")
	if out := r.HighlightLineWithSearch("main.go", "func main() {}", "main"); !strings.Contains(out, "\x1b[48;2;229;231;146m") {
		t.Fatalf("styled search missing highlight: %q", out)
	}
}

func TestRenderTokenValueAndStyleValueBranches(t *testing.T) {
	plain := chroma.StyleEntry{}
	if got := styleValue(plain, "x"); got != "x" {
		t.Fatalf("plain style = %q", got)
	}
	if got := styleValue(plain, ""); got != "" {
		t.Fatalf("empty style = %q", got)
	}
	entry := chroma.StyleEntry{Colour: chroma.MustParseColour("#010203"), Bold: chroma.Yes}
	styled := styleValue(entry, "x")
	if !strings.Contains(styled, "\x1b[38;2;1;2;3m") || !strings.Contains(styled, "\x1b[1m") || !strings.HasSuffix(styled, "\x1b[0m") {
		t.Fatalf("styled value = %q", styled)
	}
	if got := renderTokenValue(entry, "alpha", ""); !strings.Contains(got, "alpha") {
		t.Fatalf("no-query token = %q", got)
	}
	if got := renderTokenValue(entry, "alpha", "z"); !strings.Contains(got, "alpha") {
		t.Fatalf("no-match token = %q", got)
	}
	if got := renderTokenValue(entry, "alpha beta beta", "bet"); strings.Count(got, "\x1b[48;2;229;231;146m") != 2 {
		t.Fatalf("search token = %q", got)
	}
}
