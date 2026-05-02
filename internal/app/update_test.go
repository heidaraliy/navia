package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/heidaraliy/navia/internal/config"
	"github.com/heidaraliy/navia/internal/editor"
)

func TestBareCtrlWFromTreeDoesNotSwitchFocus(t *testing.T) {
	m := Model{
		editorTabs: []*editor.Buffer{editor.NewScratch("a.txt")},
		focus:      FocusTree,
	}
	updated, _ := m.updateNormal(tea.KeyMsg{Type: tea.KeyCtrlW})
	got := updated.(Model)
	if got.focus != FocusTree {
		t.Fatalf("focus = %v, want tree", got.focus)
	}
	if !got.windowPending {
		t.Fatal("window command should be pending")
	}
	updated, _ = got.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	got = updated.(Model)
	if got.focus != FocusEditor {
		t.Fatalf("focus after ctrl+w l = %v, want editor", got.focus)
	}
}

func TestPageKeysMoveTreeSelectionInChunks(t *testing.T) {
	m := Model{height: 24}
	for i := 0; i < 50; i++ {
		m.rows = append(m.rows, ResultRow{})
	}
	updated, _ := m.updateNormal(tea.KeyMsg{Type: tea.KeyPgDown})
	got := updated.(Model)
	if got.selectedIndex <= 1 {
		t.Fatalf("pgdown selectedIndex = %d, want chunk movement", got.selectedIndex)
	}
	updated, _ = got.updateNormal(tea.KeyMsg{Type: tea.KeyPgUp})
	got = updated.(Model)
	if got.selectedIndex != 0 {
		t.Fatalf("pgup selectedIndex = %d, want 0", got.selectedIndex)
	}
}

func TestHelpMenuPagesWithCtrlDAndCtrlU(t *testing.T) {
	m, err := New(t.TempDir(), config.Default())
	if err != nil {
		t.Fatal(err)
	}
	m.width = 80
	m.height = 16
	m.resizeHelp()

	updated, _ := m.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	got := updated.(Model)
	if got.mode != ModeHelp {
		t.Fatalf("mode = %v, want ModeHelp", got.mode)
	}
	if got.helpViewport.TotalLineCount() <= got.helpViewport.Height {
		t.Fatalf("help content lines = %d, height = %d; test needs scrollable help", got.helpViewport.TotalLineCount(), got.helpViewport.Height)
	}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	got = updated.(Model)
	if got.helpViewport.YOffset == 0 {
		t.Fatalf("help YOffset after ctrl+d = %d, want paged down", got.helpViewport.YOffset)
	}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	got = updated.(Model)
	if got.helpViewport.YOffset != 0 {
		t.Fatalf("help YOffset after ctrl+u = %d, want top", got.helpViewport.YOffset)
	}
}

func TestDiffModeListsChangesAndEscReturnsToTree(t *testing.T) {
	root := initAppRepo(t)
	writeAppFile(t, filepath.Join(root, "tracked.txt"), "one\n")
	runAppGit(t, root, "add", "tracked.txt")
	runAppGit(t, root, "commit", "-m", "initial")
	writeAppFile(t, filepath.Join(root, "tracked.txt"), "one\ntwo\n")

	m, err := New(root, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := m.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	got := updated.(Model)
	if got.mode != ModeDiff {
		t.Fatalf("mode = %v, want ModeDiff", got.mode)
	}
	if len(got.diffChanges) != 1 || got.diffChanges[0].Path != "tracked.txt" {
		t.Fatalf("diffChanges = %#v, want tracked.txt", got.diffChanges)
	}

	updated, _ = got.updateDiff(tea.KeyMsg{Type: tea.KeyEsc})
	got = updated.(Model)
	if got.mode != ModeNormal {
		t.Fatalf("mode after esc = %v, want ModeNormal", got.mode)
	}
}

func TestDiffRestoreConfirmRemovesUntrackedFile(t *testing.T) {
	root := initAppRepo(t)
	path := filepath.Join(root, "scratch.txt")
	writeAppFile(t, path, "scratch\n")

	m, err := New(root, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	m.enterDiffMode()
	updated, _ := m.updateDiff(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	got := updated.(Model)
	if got.mode != ModeDiffConfirmRestore {
		t.Fatalf("mode = %v, want ModeDiffConfirmRestore", got.mode)
	}

	updated, _ = got.updateDiffConfirm(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	got = updated.(Model)
	if got.mode != ModeDiff {
		t.Fatalf("mode after confirm = %v, want ModeDiff", got.mode)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("scratch.txt still exists or unexpected stat error: %v", err)
	}
}

func TestShiftLDrillsIntoSelectedDirectoryRoot(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "repo")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	m, err := New(root, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	m.selectPath(sub)
	updated, _ := m.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})
	got := updated.(Model)
	if got.cwd != sub {
		t.Fatalf("cwd = %q, want %q", got.cwd, sub)
	}
	if !got.expandedDirs[sub] {
		t.Fatalf("new root should be expanded")
	}
	if got.filter != "" || got.executedSearchQuery != "" {
		t.Fatalf("search state not cleared: filter=%q executed=%q", got.filter, got.executedSearchQuery)
	}
}

func TestAutoRefreshTreeAddsNewVisibleFile(t *testing.T) {
	root := t.TempDir()
	m, err := New(root, config.Default())
	if err != nil {
		t.Fatal(err)
	}

	writeAppFile(t, filepath.Join(root, "live.txt"), "hello\n")
	updated, _ := m.Update(autoRefreshMsg{})
	got := updated.(Model)
	if _, ok := rowNamed(got.rows, "live.txt"); !ok {
		t.Fatalf("rows missing live.txt after auto-refresh: %#v", got.rows)
	}
}

func TestAutoRefreshDiffUpdatesSelectedPreview(t *testing.T) {
	root := initAppRepo(t)
	path := filepath.Join(root, "tracked.txt")
	writeAppFile(t, path, "one\n")
	runAppGit(t, root, "add", "tracked.txt")
	runAppGit(t, root, "commit", "-m", "initial")
	writeAppFile(t, path, "one\ntwo\n")

	m, err := New(root, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	m.enterDiffMode()
	writeAppFile(t, path, "one\ntwo\nthree\n")
	updated, _ := m.Update(autoRefreshMsg{})
	got := updated.(Model)
	if !strings.Contains(got.diffViewport.View(), "three") {
		t.Fatalf("diff preview did not auto-refresh:\n%s", got.diffViewport.View())
	}
	if len(got.diffChanges) != 1 || got.diffChanges[0].Path != "tracked.txt" {
		t.Fatalf("diff selection changed unexpectedly: %#v", got.diffChanges)
	}
}

func TestNewWithSearchStartsFileSearch(t *testing.T) {
	root := t.TempDir()
	match := filepath.Join(root, "notes", "combat-plan.md")
	writeAppFile(t, match, "alpha\n")
	writeAppFile(t, filepath.Join(root, "notes", "readme.md"), "beta\n")

	m, err := NewWithSearch(root, config.Default(), StartupSearch{Mode: SearchFiles, Query: "combat"})
	if err != nil {
		t.Fatal(err)
	}
	if m.mode != ModeFilter {
		t.Fatalf("mode = %v, want ModeFilter", m.mode)
	}
	if m.searchMode != SearchFiles || m.filter != "combat" || m.executedSearchQuery != "combat" {
		t.Fatalf("search state = mode %v filter %q executed %q", m.searchMode, m.filter, m.executedSearchQuery)
	}
	if len(m.rows) != 1 || m.rows[0].Entry.Path != match {
		t.Fatalf("rows = %#v, want only %q", m.rows, match)
	}
}

func TestNewWithSearchStartsTextSearch(t *testing.T) {
	root := t.TempDir()
	match := filepath.Join(root, "story.txt")
	writeAppFile(t, match, "alpha\nneedle in a line\n")
	writeAppFile(t, filepath.Join(root, "other.txt"), "alpha\n")

	m, err := NewWithSearch(root, config.Default(), StartupSearch{Mode: SearchText, Query: "needle"})
	if err != nil {
		t.Fatal(err)
	}
	if m.searchMode != SearchText || m.filter != "needle" || m.executedSearchQuery != "needle" {
		t.Fatalf("search state = mode %v filter %q executed %q", m.searchMode, m.filter, m.executedSearchQuery)
	}
	if len(m.rows) != 1 || m.rows[0].Entry.Path != match || m.rows[0].Line != 2 {
		t.Fatalf("rows = %#v, want text match in %q line 2", m.rows, match)
	}
	if !strings.Contains(m.preview.Content, "line 2: needle in a line") {
		t.Fatalf("preview = %q, want search line context", m.preview.Content)
	}
}

func TestEnterOpensFileSearchResultInEditor(t *testing.T) {
	root := t.TempDir()
	match := filepath.Join(root, "combat-plan.md")
	writeAppFile(t, match, "alpha\n")

	m, err := NewWithSearch(root, config.Default(), StartupSearch{Mode: SearchFiles, Query: "combat"})
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := m.updateFilter(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)
	if got.mode != ModeNormal {
		t.Fatalf("mode = %v, want ModeNormal", got.mode)
	}
	if len(got.editorTabs) != 1 || got.editorTabs[0].Path != match {
		t.Fatalf("editor tabs = %#v, want %q", got.editorTabs, match)
	}
	if got.focus != FocusEditor {
		t.Fatalf("focus = %v, want editor", got.focus)
	}
}

func initAppRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runAppGit(t, root, "init")
	runAppGit(t, root, "config", "user.name", "Navia Test")
	runAppGit(t, root, "config", "user.email", "navia@example.invalid")
	return root
}

func runAppGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func writeAppFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func rowNamed(rows []ResultRow, name string) (ResultRow, bool) {
	for _, row := range rows {
		if row.Entry.Name == name {
			return row, true
		}
	}
	return ResultRow{}, false
}
