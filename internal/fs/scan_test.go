package fs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanDirSortsDirsFirstAndFiltersHidden(t *testing.T) {
	dir := t.TempDir()
	must(t, os.Mkdir(filepath.Join(dir, "zdir"), 0o755))
	must(t, os.WriteFile(filepath.Join(dir, "afile.txt"), []byte("x"), 0o644))
	must(t, os.WriteFile(filepath.Join(dir, ".hidden"), []byte("x"), 0o644))

	entries, err := ScanDir(dir, ScanOptions{ShowHidden: false, SortDirsFirst: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Name != "zdir" || !entries[0].IsDir {
		t.Fatalf("expected zdir first, got %#v", entries[0])
	}
	if entries[1].Name != "afile.txt" {
		t.Fatalf("expected afile.txt second, got %#v", entries[1])
	}
}

func TestSafeDeleteMovesIntoNaviaTrash(t *testing.T) {
	dir := t.TempDir()
	dataHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataHome)
	path := filepath.Join(dir, "test.txt")
	must(t, os.WriteFile(path, []byte("hello"), 0o644))

	target, err := SafeDelete(path, dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("source still exists or stat failed unexpectedly: %v", err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("target missing: %v", err)
	}
	wantPrefix := filepath.Join(dataHome, "navia", "trash")
	if !IsSubpath(wantPrefix, target) {
		t.Fatalf("expected global trash target under %s, got %s", wantPrefix, target)
	}
}

func TestCopyAndMovePath(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.txt")
	copyDst := filepath.Join(dir, "b.txt")
	moveDst := filepath.Join(dir, "c.txt")
	must(t, os.WriteFile(src, []byte("hello"), 0o644))

	if err := CopyPath(src, copyDst); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(copyDst)
	if err != nil || string(data) != "hello" {
		t.Fatalf("bad copy: %q %v", string(data), err)
	}
	if err := MovePath(copyDst, moveDst); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(copyDst); !os.IsNotExist(err) {
		t.Fatalf("copy source still exists or stat failed unexpectedly: %v", err)
	}
	if _, err := os.Stat(moveDst); err != nil {
		t.Fatalf("move target missing: %v", err)
	}
}

func TestPreviewHonorsByteLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.txt")
	must(t, os.WriteFile(path, []byte("abcdefghijklmnopqrstuvwxyz"), 0o644))

	preview := BuildPreview(path, 5)
	if preview.Kind != PreviewText {
		t.Fatalf("expected text preview, got %s", preview.Kind)
	}
	if preview.Content == "abcdefghijklmnopqrstuvwxyz" {
		t.Fatal("preview ignored byte limit")
	}
}

func TestRecursiveSearchFilesAndText(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "assets", "sprites")
	must(t, os.MkdirAll(nested, 0o755))
	file := filepath.Join(nested, "hero_idle.txt")
	must(t, os.WriteFile(file, []byte("first\nneedle here\n"), 0o644))

	fileMatches, err := SearchFiles(dir, "hero", ScanOptions{ShowHidden: true, SortDirsFirst: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(fileMatches) == 0 || fileMatches[0].Entry.Path != file {
		t.Fatalf("expected recursive file match for %s, got %#v", file, fileMatches)
	}

	textMatches, err := SearchText(dir, "needle", 1024, ScanOptions{ShowHidden: true, SortDirsFirst: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(textMatches) == 0 || textMatches[0].Line != 2 {
		t.Fatalf("expected recursive text match on line 2, got %#v", textMatches)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
