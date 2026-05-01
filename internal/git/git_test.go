package git

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindRoot(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "repo")
	child := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := FindRoot(child); got != root {
		t.Fatalf("got %q want %q", got, root)
	}
}
