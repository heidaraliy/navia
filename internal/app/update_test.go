package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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
