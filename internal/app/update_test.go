package app

import (
	"os"
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
