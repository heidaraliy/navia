package app

import (
	"os"
	"os/exec"
	"path/filepath"
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
