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

func TestCommandStrings(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"rename", RenameCommand("old.txt", "new.txt"), "Shell equivalent: mv old.txt new.txt"},
		{"copy file", CopyCommand("a.txt", "b.txt", false), "Shell equivalent: cp a.txt b.txt"},
		{"move", MoveCommand("a.txt", "dir/a.txt"), "Shell equivalent: mv a.txt dir/a.txt"},
		{"mkdir", MkdirCommand("new dir"), "Shell equivalent: mkdir 'new dir'"},
		{"touch", TouchCommand("new.txt"), "Shell equivalent: touch new.txt"},
		{"delete file", DeleteCommand("old.txt", false), "Navia safe-deleted this item. Dangerous shell equivalent would be: rm old.txt"},
		{"delete dir", DeleteCommand("old dir", true), "Navia safe-deleted this item. Dangerous shell equivalent would be: rm -rf 'old dir'"},
		{"open default", OpenCommand("file.txt", ""), "Shell equivalent: $EDITOR file.txt"},
		{"open custom", OpenCommand("file.txt", "vim"), "Shell equivalent: vim file.txt"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Fatalf("%s: got %q want %q", tt.name, tt.got, tt.want)
		}
	}
}

func TestQuoteVariants(t *testing.T) {
	tests := map[string]string{
		"":                "''",
		"./clean/../path": "path",
		"has$var":         "'has$var'",
		"brackets[1]":     "'brackets[1]'",
	}
	for input, want := range tests {
		if got := Quote(input); got != want {
			t.Fatalf("Quote(%q) = %q want %q", input, got, want)
		}
	}
}
