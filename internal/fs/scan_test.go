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

func TestFileSearchMatchesPartialTokensAndExtensions(t *testing.T) {
	root := t.TempDir()
	must(t, os.MkdirAll(filepath.Join(root, "cpp", "core"), 0o755))
	must(t, os.WriteFile(filepath.Join(root, "cpp", "core", "PhysicsSystem.cpp"), []byte("x"), 0o644))
	must(t, os.WriteFile(filepath.Join(root, "cpp", "core", "PhysicsSystem.h"), []byte("x"), 0o644))
	for _, query := range []string{"Physics .cpp", "phys sys cpp", "phsys .cpp"} {
		matches, err := SearchFiles(root, query, ScanOptions{})
		if err != nil || len(matches) == 0 || matches[0].Entry.Name != "PhysicsSystem.cpp" {
			t.Fatalf("query %q = %#v, %v", query, matches, err)
		}
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
	must(t, os.WriteFile(file, []byte("first\nneedle here\nneedle again\n"), 0o644))

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
	if len(textMatches) != 2 || textMatches[0].Line != 2 || textMatches[1].Line != 3 {
		t.Fatalf("expected recursive text matches on lines 2 and 3, got %#v", textMatches)
	}
}

func TestSymbolSearchDefinitionsAndReferences(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"main.go":     "package main\nfunc StartServer() {}\nfunc (m Model) Render() {}\n",
		"ui.ts":       "export function StartServer() {}\nconst Render = () => null\n",
		"system.cpp":  "class StartServer {};\nvoid World::Render() {\n}\n",
		"script.lua":  "local function StartServer()\nend\nthing = function()\nend\n",
		"readme.txt":  "StartServer mention\nNotStartServer nope\n",
		"ignored.bin": "StartServer\x00hidden\n",
	}
	for name, content := range files {
		must(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
	}

	defs, err := SearchSymbolDefinitions(dir, "StartServer", 2048, ScanOptions{ShowHidden: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(defs) != 4 {
		t.Fatalf("definition matches = %#v", defs)
	}
	if defs[0].Score < defs[len(defs)-1].Score || defs[0].Column == 0 {
		t.Fatalf("definition ranking/column = %#v", defs)
	}

	refs, err := SearchSymbolReferences(dir, "StartServer", 2048, ScanOptions{ShowHidden: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, ref := range refs {
		if strings.Contains(ref.Entry.Name, "ignored") || strings.Contains(ref.Snippet, "NotStartServer") {
			t.Fatalf("bad reference match = %#v", ref)
		}
	}
	if len(refs) != 5 {
		t.Fatalf("reference matches = %#v", refs)
	}
}

func TestRecursiveSearchFilesMatchesPathTokens(t *testing.T) {
	dir := t.TempDir()
	wanted := filepath.Join(dir, "assets", "technoviolet", "item.json")
	other := filepath.Join(dir, "assets", "neutral", "item.json")
	must(t, os.MkdirAll(filepath.Dir(wanted), 0o755))
	must(t, os.MkdirAll(filepath.Dir(other), 0o755))
	must(t, os.WriteFile(wanted, []byte("{}"), 0o644))
	must(t, os.WriteFile(other, []byte("{}"), 0o644))

	matches, err := SearchFiles(dir, "item.json techno", ScanOptions{ShowHidden: true, SortDirsFirst: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 || matches[0].Entry.Path != wanted {
		t.Fatalf("path-token matches = %#v, want only %s", matches, wanted)
	}

	matches, err = SearchFiles(dir, "ITEM.JSON", ScanOptions{ShowHidden: true, SortDirsFirst: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 2 {
		t.Fatalf("case-insensitive item matches = %#v, want both item.json files", matches)
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
