package app

import (
	"strings"
	"testing"
)

func TestTopHeightIsCompact(t *testing.T) {
	m := Model{}
	if got := m.topHeight(); got != 2 {
		t.Fatalf("topHeight = %d, want 2", got)
	}
}

func TestHelpIsGroupedByMode(t *testing.T) {
	help := helpContent()
	for _, want := range []string{"Global", "Tree", "Diff", "Search", "Editor Normal", "Editor Tabs", "Windows And History", "ctrl+w o"} {
		if !strings.Contains(help, want) {
			t.Fatalf("help missing %q:\n%s", want, help)
		}
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
