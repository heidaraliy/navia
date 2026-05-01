package shellteach

import "testing"

func TestQuoteEscapesShellPaths(t *testing.T) {
	got := Quote("assets/player idle's.png")
	want := "'assets/player idle'\\''s.png'"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestCopyCommandDirectory(t *testing.T) {
	got := CopyCommand("assets/items", "assets/items backup", true)
	want := "Shell equivalent: cp -r assets/items 'assets/items backup'"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
