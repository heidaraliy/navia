package app

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/heidaraliy/navia/internal/config"
	"github.com/heidaraliy/navia/internal/gitview"
	"github.com/heidaraliy/navia/internal/syntax"
)

func TestNavigatorViewsKeysAndMouseInteractions(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root+"/dir/file.go", "package demo\n")
	mustWrite(t, root+"/note.txt", "needle\n")
	m, err := New(root, config.Default(), false)
	if err != nil {
		t.Fatal(err)
	}
	m.width, m.height, m.leftWidth = 100, 30, 30
	m.SetStatus("ready")
	if !strings.Contains(m.Debug(), "files=") || headerLine("left", "right", 20) == "" {
		t.Fatal("debug helpers returned empty output")
	}
	for _, fullscreen := range []byte{0, 'l', 'r'} {
		candidate := m
		candidate.fullscreen = fullscreen
		if got := candidate.View(); !strings.Contains(got, "Files") {
			t.Fatalf("navigator view missing files in fullscreen %q", fullscreen)
		}
	}
	help := m
	help.help = true
	if !strings.Contains(help.View(), "Navia keybinds") {
		t.Fatal("navigator help did not render")
	}

	for _, key := range []string{"up", "down", "J", "K", "ctrl+j", "ctrl+k", "pgdown", "pgup", "enter", "right", "left", "/", "?", "F", "f"} {
		candidate := m
		if key == "right" || key == "left" || key == "enter" {
			candidate.navSelected = 1
		}
		updated, _ := candidate.updateNavKey(keyMessage(key))
		_ = updated.(Model)
	}
	for _, key := range []string{"esc", "tab", "enter", "backspace", "ctrl+u", " ", "x"} {
		candidate := m
		candidate.navSearching, candidate.navQuery = true, "needle"
		updated, _ := candidate.updateNavKey(keyMessage(key))
		_ = updated.(Model)
	}

	events := []tea.MouseEvent{
		{X: m.leftWidth, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress},
		{X: 40, Button: tea.MouseButtonLeft, Action: tea.MouseActionMotion},
		{X: 40, Button: tea.MouseButtonLeft, Action: tea.MouseActionRelease},
		{X: 2, Y: topHeight + 3, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress},
		{X: 2, Button: tea.MouseButtonWheelDown},
		{X: 80, Button: tea.MouseButtonWheelUp},
	}
	for _, event := range events {
		updated, _ := m.updateNavMouse(event)
		m = updated.(Model)
	}
	m.navPreviewLines = make([]string, 100)
	m.scrollNavPreview(50)
	m.scrollNavPreview(-100)
}

func TestNavigatorMessagesCollapseAndEditorPaths(t *testing.T) {
	root := gitFixture(t)
	mustWrite(t, root+"/dir/file.go", "package demo\n")
	m, err := New(root, config.Default(), false)
	if err != nil {
		t.Fatal(err)
	}
	m.width, m.height, m.leftWidth = 100, 30, 30
	m.navPreviewID, m.navSearchID, m.historyID = 2, 3, 4
	preview := m.navRows[0].entry.Path
	messages := []tea.Msg{
		historyMsg{id: 4, err: errors.New("history")},
		historyMsg{id: 4, commits: []gitview.Commit{{Hash: "a"}}},
		historyMsg{id: 4, append: true, commits: []gitview.Commit{{Hash: "b"}}},
		navPreviewMsg{id: 1},
		navPreviewMsg{id: 2, path: preview, lines: []string{"preview"}},
		navSearchMsg{id: 3, err: errors.New("search")},
		navSearchMsg{id: 3, rows: m.navRows},
		navEditorMsg{err: errors.New("editor")},
		navEditorMsg{},
		tea.WindowSizeMsg{Width: 80, Height: 20},
	}
	for _, msg := range messages {
		candidate := m
		updated, _, handled := candidate.updateNavigatorMessage(msg)
		if !handled {
			t.Fatalf("navigator message %T was not handled", msg)
		}
		_ = updated.(Model)
	}

	// Collapse an expanded directory, then use left again to select its parent.
	m.navSelected = 1
	updated, _ := m.expandNav()
	m = updated.(Model)
	updated, _ = m.collapseNav()
	m = updated.(Model)
	m.navSelected = 2
	updated, _ = m.collapseNav()
	m = updated.(Model)

	// Opening a regular file produces an external-editor command without
	// mutating it. Deleted files exercise the read-only snapshot path.
	m.navSelected = len(m.navRows) - 1
	_, _ = m.openNavSelection()
	diff := diffInteractionModel(t)
	deleted := diff
	deleted.changes = []gitview.Change{{Path: "a.txt", Kind: gitview.Deleted}}
	if cmd := deleted.openEditor(); cmd == nil {
		t.Fatal("deleted file did not produce editor command")
	}
	historical := diff
	historical.diffRef = "HEAD"
	if cmd := historical.openEditor(); cmd == nil {
		t.Fatal("historical file did not produce editor command")
	}
}

func TestAsyncDiffLoadersAndContentSearch(t *testing.T) {
	m := diffInteractionModel(t)
	for _, cmd := range []tea.Cmd{
		loadStatus(m.root, 1),
		loadStatusRef(m.root, "HEAD", 2),
		loadSummary(m.root, m.changes, 3),
		loadSummaryRef(m.root, "HEAD", m.changes, 4),
		loadDiff(m.root, m.changes[0], 5, 0),
		loadDiffRef(m.root, "HEAD", gitview.Change{Path: "a.txt", Kind: gitview.Modified}, 6, 0),
	} {
		if cmd == nil || cmd() == nil {
			t.Fatal("loader returned no message")
		}
	}
	m.searchQuery = "two"
	if cmd := m.queueContentSearch(); cmd == nil || cmd() == nil {
		t.Fatal("working-tree content search returned no message")
	}
	m.diffRef, m.searchQuery = "HEAD", "one"
	if cmd := m.queueContentSearch(); cmd == nil || cmd() == nil {
		t.Fatal("commit content search returned no message")
	}
}

func diffInteractionModel(t *testing.T) Model {
	t.Helper()
	root := gitFixture(t)
	mustWrite(t, root+"/a.txt", "two\n")
	mustWrite(t, root+"/new.go", "package demo\n")
	m := Model{
		mode:           'd',
		root:           root,
		cfg:            config.Default(),
		width:          120,
		height:         35,
		leftWidth:      36,
		changes:        []gitview.Change{{Path: "a.txt", Kind: gitview.Modified}, {Path: "new.go", Kind: gitview.Untracked}},
		diffLabel:      "Working Tree",
		historyHasMore: true,
		syntax:         syntax.New("navia"),
		editor:         "true",
	}
	m.diff = gitview.FileDiff{
		Path: "a.txt",
		Lines: []gitview.DiffLine{
			{Kind: gitview.Hunk, Text: "@@ -1 +1 @@"},
			{Kind: gitview.Removed, Old: 1, Text: "one"},
			{Kind: gitview.Added, New: 1, Text: "two"},
		},
		Side: []gitview.SideLine{
			{Kind: gitview.Hunk, OldText: "@@ -1 +1 @@", NewText: "@@ -1 +1 @@"},
			{Kind: gitview.Removed, Old: 1, New: 1, OldText: "one", NewText: "two"},
		},
		Counts: gitview.Counts{LinesNew: 1, LinesModified: 1, LinesDeleted: 1},
		Size:   4,
	}
	m.counts = gitview.Counts{FilesNew: 1, FilesModified: 1, LinesNew: 1, LinesModified: 1, LinesDeleted: 1}
	return m
}

func TestDiffViewsAndKeyInteractions(t *testing.T) {
	base := diffInteractionModel(t)
	views := []Model{base}
	split := base
	split.sideBySide = true
	views = append(views, split)
	binary := base
	binary.diff.Binary = true
	views = append(views, binary)
	loading := base
	loading.diffLoading = true
	views = append(views, loading)
	empty := base
	empty.changes, empty.diff = nil, gitview.FileDiff{}
	views = append(views, empty)
	failed := base
	failed.err = "broken"
	views = append(views, failed)
	for _, fullscreen := range []byte{0, 'l', 'r'} {
		for _, candidate := range views {
			candidate.fullscreen = fullscreen
			if got := candidate.View(); got == "" || !strings.Contains(got, "Navia") && candidate.help {
				t.Fatalf("empty diff view for fullscreen %q", fullscreen)
			}
		}
	}
	help := base
	help.help = true
	if !strings.Contains(help.View(), "Navia keybinds") {
		t.Fatal("diff help did not render")
	}
	history := base
	history.historyOpen = true
	history.history = []gitview.Commit{{Hash: "abc", Short: "abc", Subject: "subject", Relative: "now"}}
	if !strings.Contains(history.View(), "Compare") {
		t.Fatal("history did not render")
	}

	for _, key := range []string{"up", "down", "J", "K", "ctrl+j", "ctrl+k", "pgdown", "pgup", "g", "G", "v", "r", "enter", "ctrl+o", "c", "/", "?", "F", "f", "esc"} {
		m := base
		updated, _ := m.updateKey(keyMessage(key))
		_ = updated.(Model)
	}
	for _, key := range []string{"esc", "enter", "backspace", "ctrl+u", "x"} {
		m := base
		m.searching, m.searchQuery = true, "query"
		var msg tea.KeyMsg
		if len(key) == 1 {
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
		} else {
			msg = keyMessage(key)
		}
		updated, _ := m.updateKey(msg)
		_ = updated.(Model)
	}
	for _, key := range []string{"?", "esc", "q", "enter"} {
		m := base
		m.help = true
		updated, _ := m.updateKey(keyMessage(key))
		if updated.(Model).help {
			t.Fatalf("help remained open for %q", key)
		}
	}
}

func TestDiffMessagesMouseAndHistoryInteractions(t *testing.T) {
	base := diffInteractionModel(t)
	base.statusRequestID, base.summaryRequestID, base.requestID, base.searchID = 2, 3, 4, 5
	messages := []tea.Msg{
		tea.WindowSizeMsg{Width: 90, Height: 24},
		statusMsg{id: 1},
		statusMsg{id: 2, err: errors.New("status")},
		statusMsg{id: 2, changes: base.changes},
		summaryMsg{id: 2},
		summaryMsg{id: 3, err: errors.New("summary")},
		summaryMsg{id: 3, counts: base.counts},
		diffMsg{id: 4, index: 0, err: errors.New("diff")},
		diffMsg{id: 4, index: 0, diff: base.diff},
		editorMsg{err: errors.New("editor")},
		editorMsg{},
		searchMsg{id: 4},
		searchMsg{id: 5, err: errors.New("search")},
		searchMsg{id: 5, matches: map[string]bool{"a.txt": true}},
	}
	for _, msg := range messages {
		m := base
		m.statusRequestID, m.summaryRequestID, m.requestID, m.searchID = 2, 3, 4, 5
		updated, _ := m.Update(msg)
		_ = updated.(Model)
	}

	mouse := []tea.MouseEvent{
		{X: base.leftWidth, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress},
		{X: 44, Button: tea.MouseButtonLeft, Action: tea.MouseActionMotion},
		{X: 44, Button: tea.MouseButtonLeft, Action: tea.MouseActionRelease},
		{X: 3, Y: topHeight + 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress},
		{X: 3, Y: topHeight + 3, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress},
		{X: 3, Button: tea.MouseButtonWheelDown},
		{X: 80, Button: tea.MouseButtonWheelUp},
	}
	m := base
	for _, event := range mouse {
		updated, _ := m.updateMouse(event)
		m = updated.(Model)
	}

	m = base
	m.history = []gitview.Commit{{Hash: "abc", Short: "abc", Subject: "subject"}}
	for _, key := range []string{"up", "down", "K", "J", "enter", "esc"} {
		updated, _ := m.updateHistory(keyMessage(key))
		m = updated.(Model)
	}
	for _, event := range []tea.MouseEvent{{Button: tea.MouseButtonWheelUp}, {Button: tea.MouseButtonWheelDown}, {X: 1, Y: 12, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}} {
		updated, _ := m.updateHistoryMouse(event)
		m = updated.(Model)
	}
}

func keyMessage(key string) tea.KeyMsg {
	switch key {
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "pgup":
		return tea.KeyMsg{Type: tea.KeyPgUp}
	case "pgdown":
		return tea.KeyMsg{Type: tea.KeyPgDown}
	case "ctrl+j":
		return tea.KeyMsg{Type: tea.KeyCtrlJ}
	case "ctrl+k":
		return tea.KeyMsg{Type: tea.KeyCtrlK}
	case "ctrl+o":
		return tea.KeyMsg{Type: tea.KeyCtrlO}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
	case "ctrl+u":
		return tea.KeyMsg{Type: tea.KeyCtrlU}
	case " ":
		return tea.KeyMsg{Type: tea.KeySpace}
	case "q", "?":
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
}
