package fs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func TestScanDirErrorsAndSortsWithoutDirsFirst(t *testing.T) {
	if _, err := ScanDir(filepath.Join(t.TempDir(), "missing"), ScanOptions{}); err == nil {
		t.Fatal("ScanDir missing directory succeeded")
	}
	entries := []FileEntry{
		{Name: "zdir", IsDir: true},
		{Name: "afile.txt", IsDir: false},
	}
	Sort(entries, false)
	if entries[0].Name != "afile.txt" {
		t.Fatalf("entries sorted dirs first despite disabled option: %#v", entries)
	}
}

func TestShouldSkipNameHonorsHiddenAndIgnoredNames(t *testing.T) {
	if ShouldSkipName("", ScanOptions{}) {
		t.Fatal("empty name should not be skipped")
	}
	if !ShouldSkipName(".hidden", ScanOptions{}) {
		t.Fatal("hidden name should be skipped by default")
	}
	if ShouldSkipName(".hidden", ScanOptions{ShowHidden: true}) {
		t.Fatal("hidden name should be visible when configured")
	}
	if !ShouldSkipName("node_modules", ScanOptions{ShowHidden: true, IgnoreNames: map[string]bool{"node_modules": true}}) {
		t.Fatal("ignored name should be skipped")
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

func TestGlobalTrashDirUsesHomeWhenXDGDataHomeIsUnset(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "")
	got, err := GlobalTrashDir()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(got, filepath.Join(".local", "share", "navia", "trash")) {
		t.Fatalf("GlobalTrashDir = %q", got)
	}
}

func TestSafeDeleteErrorsForMissingPath(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	if _, err := SafeDelete(filepath.Join(t.TempDir(), "missing"), ""); err == nil {
		t.Fatal("SafeDelete missing path succeeded")
	}
}

func TestSafeDeleteRejectsOutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	must(t, os.WriteFile(outside, []byte("do not move"), 0o644))

	if _, err := SafeDelete(outside, root); err == nil {
		t.Fatal("SafeDelete allowed outside-root path")
	}
	if data, err := os.ReadFile(outside); err != nil || string(data) != "do not move" {
		t.Fatalf("outside file changed after rejected delete: %q %v", string(data), err)
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
	if !preview.Truncated {
		t.Fatal("preview should report truncation")
	}
	if preview.Path != path || preview.Size == 0 || preview.ModTime.IsZero() {
		t.Fatalf("preview metadata missing: %#v", preview)
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

func TestRecursiveSearchSkipsHiddenWhenConfigured(t *testing.T) {
	dir := t.TempDir()
	hidden := filepath.Join(dir, ".hidden")
	must(t, os.MkdirAll(hidden, 0o755))
	must(t, os.WriteFile(filepath.Join(hidden, "needle.txt"), []byte("needle\n"), 0o644))
	must(t, os.WriteFile(filepath.Join(dir, "visible.txt"), []byte("needle\n"), 0o644))

	fileMatches, err := SearchFiles(dir, "needle", ScanOptions{ShowHidden: false, SortDirsFirst: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(fileMatches) != 0 {
		t.Fatalf("hidden file matches = %#v, want none", fileMatches)
	}

	textMatches, err := SearchText(dir, "needle", 1024, ScanOptions{ShowHidden: false, SortDirsFirst: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(textMatches) != 1 || textMatches[0].Entry.Name != "visible.txt" {
		t.Fatalf("text matches = %#v", textMatches)
	}

	if matches, err := SearchText(dir, "n", 1024, ScanOptions{}); err != nil || matches != nil {
		t.Fatalf("short query = %#v %v", matches, err)
	}
}

func TestRecursiveSearchSkipsIgnoredNames(t *testing.T) {
	dir := t.TempDir()
	ignored := filepath.Join(dir, "node_modules", "pkg")
	must(t, os.MkdirAll(ignored, 0o755))
	must(t, os.WriteFile(filepath.Join(ignored, "needle.txt"), []byte("needle\n"), 0o644))
	opts := ScanOptions{ShowHidden: true, SortDirsFirst: true, IgnoreNames: map[string]bool{"node_modules": true}}

	fileMatches, err := SearchFiles(dir, "needle", opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(fileMatches) != 0 {
		t.Fatalf("ignored file matches = %#v, want none", fileMatches)
	}

	textMatches, err := SearchText(dir, "needle", 1024, opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(textMatches) != 0 {
		t.Fatalf("ignored text matches = %#v, want none", textMatches)
	}
}

func TestSearchFilesCapsNonEmptyQueryResults(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < MaxSearchResults+5; i++ {
		must(t, os.WriteFile(filepath.Join(dir, "needle_"+strconvItoa(i)+".txt"), []byte("x"), 0o644))
	}
	matches, err := SearchFiles(dir, "needle", ScanOptions{ShowHidden: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != MaxSearchResults {
		t.Fatalf("matches = %d want %d", len(matches), MaxSearchResults)
	}
}

func TestSearchTextSkipsBinaryAndRespectsDefaultLimit(t *testing.T) {
	dir := t.TempDir()
	must(t, os.WriteFile(filepath.Join(dir, "binary.txt"), []byte{'n', 0, 'd'}, 0o644))
	must(t, os.WriteFile(filepath.Join(dir, "late.txt"), []byte(strings.Repeat("x", 300*1024)+"needle"), 0o644))
	matches, err := SearchText(dir, "needle", 0, ScanOptions{ShowHidden: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("matches beyond default limit or binary should be skipped: %#v", matches)
	}
}

func TestDirectoryPreviewIsCapped(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < MaxDirectoryPreviewEntries+5; i++ {
		must(t, os.WriteFile(filepath.Join(dir, fmt.Sprintf("file-%04d.txt", i)), []byte("x"), 0o644))
	}
	preview := BuildPreviewWithOptions(dir, 1024, ScanOptions{ShowHidden: true})
	if !preview.Truncated {
		t.Fatal("directory preview should report truncation")
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
