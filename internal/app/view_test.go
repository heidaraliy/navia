package app

import (
	"strings"
	"testing"

	"github.com/heidaraliy/navia/internal/editor"
)

func TestTopHeightIsCompact(t *testing.T) {
	m := Model{}
	if got := m.topHeight(); got != 2 {
		t.Fatalf("topHeight = %d, want 2", got)
	}
}

func TestHelpIsGroupedByMode(t *testing.T) {
	help := helpContent()
	for _, want := range []string{"Global", "Tree", "Diff", "Search", "Editor Normal", "Editor Tabs", "Windows And History", "ctrl+w o", "L / shift+enter"} {
		if !strings.Contains(help, want) {
			t.Fatalf("help missing %q:\n%s", want, help)
		}
	}
}

func TestSearchTopLeftShowsTypeTagAndPlaceholder(t *testing.T) {
	m := Model{mode: ModeFilter, searchMode: SearchFiles}
	got := m.topLeft()
	for _, want := range []string{"SEARCH", "FILES", "enter query..."} {
		if !strings.Contains(got, want) {
			t.Fatalf("topLeft missing %q: %q", want, got)
		}
	}
	m.filter = "combat"
	got = m.topLeft()
	if !strings.Contains(got, "combat") || strings.Contains(got, "enter query...") {
		t.Fatalf("topLeft query state = %q", got)
	}
}

func TestDiffTopLeftShowsDiffTag(t *testing.T) {
	m := Model{mode: ModeDiff}
	got := m.topLeft()
	if !strings.Contains(got, "DIFF") {
		t.Fatalf("topLeft missing DIFF: %q", got)
	}
}

func TestEditorTopLeftShowsBufferModeAndCommand(t *testing.T) {
	buf := editor.NewScratch("a.go")
	buf.Mode = editor.Command
	buf.HandleKey("w")
	m := Model{focus: FocusEditor, editorTabs: []*editor.Buffer{buf}}
	got := m.topLeft()
	for _, want := range []string{"EDITOR", "EXEC", ":w"} {
		if !strings.Contains(got, want) {
			t.Fatalf("topLeft missing %q: %q", want, got)
		}
	}
}

func TestEditorTopLeftShowsNormalPendingCommand(t *testing.T) {
	buf := editor.NewScratch("a.go")
	buf.HandleKey("d")
	buf.HandleKey("a")
	m := Model{focus: FocusEditor, editorTabs: []*editor.Buffer{buf}}
	got := m.topLeft()
	for _, want := range []string{"EDITOR", "NORMAL", "da"} {
		if !strings.Contains(got, want) {
			t.Fatalf("topLeft missing %q: %q", want, got)
		}
	}
}

func TestCommandCueUsesModeColorWithoutBackground(t *testing.T) {
	m := Model{}
	normal := m.commandCue("NORMAL")
	exec := m.commandCue("EXEC")
	if !normal.GetUnderline() || !exec.GetUnderline() {
		t.Fatal("command cue should underline command text")
	}
	if normal.GetForeground() == exec.GetForeground() {
		t.Fatalf("normal and exec command cues should differ")
	}
	rendered := exec.Render(":w")
	if strings.Contains(rendered, "[48;") {
		t.Fatalf("command cue should not tint top bar background: %q", rendered)
	}
}

func TestClipStyledDoesNotBreakANSIEscapes(t *testing.T) {
	input := "\x1b[38;2;255;0;255mabcdef\x1b[0m"
	got := clipStyled(input, 3)
	if strings.Contains(got, "\x1b[38;") && !strings.Contains(got, "\x1b[0m") {
		t.Fatalf("styled clip did not preserve reset: %q", got)
	}
	if strings.Contains(got, "[38;2") && !strings.Contains(got, "\x1b[38;2") {
		t.Fatalf("styled clip broke escape sequence: %q", got)
	}
}

func TestFormatUnifiedDiffAddsLineNumberGutters(t *testing.T) {
	diff := strings.Join([]string{
		"diff --git a/a.txt b/a.txt",
		"--- a/a.txt",
		"+++ b/a.txt",
		"@@ -2,2 +2,3 @@",
		" keep",
		"-old",
		"+new",
		"+extra",
	}, "\n")
	got := formatUnifiedDiff(diff)
	for _, want := range []string{
		"   2    2 │  keep",
		"   3      │ -old",
		"        3 │ +new",
		"        4 │ +extra",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("formatted diff missing %q:\n%s", want, got)
		}
	}
}

func TestDiffLineStyleDetectsFormattedLines(t *testing.T) {
	cases := map[string]string{
		"        3 │ +new":        "add",
		"   3      │ -old":        "remove",
		"          │ @@ -1 +1 @@": "hunk",
		"          │ --- a/a.txt": "header",
	}
	for line, want := range cases {
		if got := diffLineStyle(line); got != want {
			t.Fatalf("diffLineStyle(%q) = %q, want %q", line, got, want)
		}
	}
}
