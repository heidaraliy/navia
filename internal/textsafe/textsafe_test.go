package textsafe

import (
	"strings"
	"testing"
)

func TestDisplayEscapesTerminalControls(t *testing.T) {
	got := Display("safe\x1b]52;c;boom\x07\nnext\tcol\x80bad")
	for _, bad := range []string{"\x1b", "\x07", "\n", "\t", "\x80"} {
		if strings.Contains(got, bad) {
			t.Fatalf("Display left raw control %q in %q", bad, got)
		}
	}
	for _, want := range []string{`safe`, `\x1b`, `\x07`, `\n`, `\t`, `\x80`} {
		if !strings.Contains(got, want) {
			t.Fatalf("Display missing %q in %q", want, got)
		}
	}
}

func TestDisplayLeavesPrintableTextUnchanged(t *testing.T) {
	const input = "Navia αβ"
	if got := Display(input); got != input {
		t.Fatalf("Display(%q) = %q", input, got)
	}
}

func TestContentPreservesTabsButEscapesTerminalControls(t *testing.T) {
	got := Content("a\tb\x1b[2J")
	if !strings.Contains(got, "\t") {
		t.Fatalf("Content did not preserve tab: %q", got)
	}
	if strings.Contains(got, "\x1b[2J") || !strings.Contains(got, `\x1b`) {
		t.Fatalf("Content did not visibly escape terminal controls: %q", got)
	}
}

func TestMultilinePreservesLayoutButEscapesTerminalControls(t *testing.T) {
	got := Multiline("Directory\n\n1 folders\n0 files\x1b[2J")
	if !strings.Contains(got, "Directory\n\n1 folders\n0 files") {
		t.Fatalf("Multiline did not preserve newlines: %q", got)
	}
	if strings.Contains(got, "\x1b[2J") || !strings.Contains(got, `\x1b`) {
		t.Fatalf("Multiline did not visibly escape terminal controls: %q", got)
	}
}
