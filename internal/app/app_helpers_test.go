package app

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/heidaraliy/navia/internal/config"
	"github.com/heidaraliy/navia/internal/editor"
	navfs "github.com/heidaraliy/navia/internal/fs"
	"github.com/heidaraliy/navia/internal/git"
	"github.com/heidaraliy/navia/internal/ui"
)

func init() {
	if os.Getenv("NAVIA_FAKE_GOPLS") == "1" {
		runFakeLSPServer()
		os.Exit(0)
	}
}

func TestModelHelpersAndSelectionBounds(t *testing.T) {
	root := t.TempDir()
	writeAppFile(t, filepath.Join(root, "alpha.txt"), "alpha\n")
	if err := os.Mkdir(filepath.Join(root, "dir"), 0o755); err != nil {
		t.Fatal(err)
	}

	m, err := New(root, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	m.SetStatus("ready")
	if m.statusMessage != "ready" {
		t.Fatalf("status = %q, want ready", m.statusMessage)
	}
	if cmd := m.Init(); cmd == nil {
		t.Fatal("Init should return auto-refresh command")
	}
	if got := m.activeBuffer(); got != nil {
		t.Fatalf("activeBuffer = %#v, want nil", got)
	}
	m.editorTabs = []*editor.Buffer{editor.NewScratch(filepath.Join(root, "scratch.txt"))}
	m.activeTab = 3
	if got := m.activeBuffer(); got != nil {
		t.Fatalf("activeBuffer with out-of-range tab = %#v, want nil", got)
	}
	m.activeTab = 0
	if got := m.activeBuffer(); got == nil {
		t.Fatal("activeBuffer should return active editor tab")
	}

	m.selectedIndex = -10
	m.clampSelection()
	if m.selectedIndex != 0 {
		t.Fatalf("negative selection clamped to %d, want 0", m.selectedIndex)
	}
	m.selectedIndex = 100
	m.clampSelection()
	if m.selectedIndex != len(m.rows)-1 {
		t.Fatalf("high selection clamped to %d, want %d", m.selectedIndex, len(m.rows)-1)
	}
	m.rows = nil
	m.clampSelection()
	if m.selectedIndex != 0 {
		t.Fatalf("empty selection = %d, want 0", m.selectedIndex)
	}
	if _, ok := m.selected(); ok {
		t.Fatal("selected should be false with no rows")
	}
	if _, ok := m.selectedRow(); ok {
		t.Fatal("selectedRow should be false with no rows")
	}
	if _, ok := m.rowForPath(filepath.Join(root, "missing")); ok {
		t.Fatal("rowForPath should be false for missing path")
	}

	m.enterMode(ModeRename, "rename> ", "alpha.txt")
	if m.mode != ModeRename || m.input.Value() != "alpha.txt" {
		t.Fatalf("enterMode mode/value = %v/%q", m.mode, m.input.Value())
	}
	m.exitMode()
	if m.mode != ModeNormal || m.input.Focused() {
		t.Fatalf("exitMode left mode/focus = %v/%v", m.mode, m.input.Focused())
	}
	m.setError(errors.New("boom"))
	if m.statusMessage != "boom" {
		t.Fatalf("setError status = %q", m.statusMessage)
	}
	m.setError(nil)
	if m.statusMessage != "boom" {
		t.Fatalf("setError(nil) changed status to %q", m.statusMessage)
	}

	m.gitRoot = root
	if got := m.cwdLabel(); got != "[git] ." {
		t.Fatalf("cwdLabel = %q", got)
	}
	m.rows = m.treeRowsToResultRows(m.treeRows)
	m.selectPath(filepath.Join(root, "alpha.txt"))
	if got := m.selectedPathForStatus(); got != "alpha.txt" {
		t.Fatalf("selectedPathForStatus = %q", got)
	}
}

func TestFilteringAndSearchHelpers(t *testing.T) {
	root := t.TempDir()
	writeAppFile(t, filepath.Join(root, "alpha.txt"), "alpha\nneedle\n")
	writeAppFile(t, filepath.Join(root, "nested", "beta.txt"), "beta\n")

	m, err := New(root, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	beforeRows := len(m.rows)
	m.StartSearch(StartupSearch{Mode: SearchFiles, Query: "   "})
	if len(m.rows) != beforeRows || m.mode != ModeNormal {
		t.Fatalf("blank StartSearch changed rows/mode: %d/%v", len(m.rows), m.mode)
	}

	m.filter = "a"
	m.executedSearchQuery = "a"
	m.searchMode = SearchFiles
	m.applyFilter()
	if len(m.rows) != 0 || !strings.Contains(m.statusMessage, "at least 2") {
		t.Fatalf("short file search rows/status = %d/%q", len(m.rows), m.statusMessage)
	}

	m.filter = "alpha"
	m.executedSearchQuery = ""
	m.applyFilter()
	if m.rows != nil || !strings.Contains(m.statusMessage, "Press Enter") {
		t.Fatalf("pending recursive search rows/status = %#v/%q", m.rows, m.statusMessage)
	}

	m.executedSearchQuery = "alpha"
	m.applyFilter()
	if len(m.rows) != 1 || m.rows[0].Entry.Name != "alpha.txt" {
		t.Fatalf("file search rows = %#v", m.rows)
	}
	oldRows := m.recursiveRows
	m.ensureRecursiveRows()
	if len(m.recursiveRows) != len(oldRows) {
		t.Fatalf("cached recursive rows changed: %d vs %d", len(m.recursiveRows), len(oldRows))
	}

	m.searchMode = SearchText
	m.filter = "does-not-exist"
	m.executedSearchQuery = m.filter
	m.applyFilter()
	if len(m.rows) != 0 || m.statusMessage != "No text matches." {
		t.Fatalf("text miss rows/status = %d/%q", len(m.rows), m.statusMessage)
	}
}

func TestInputModesApplyFilesystemOperations(t *testing.T) {
	root := t.TempDir()
	writeAppFile(t, filepath.Join(root, "old.txt"), "old\n")

	m, err := New(root, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	m.selectPath(filepath.Join(root, "old.txt"))
	m.enterMode(ModeRename, "rename> ", "")
	m.input.SetValue("renamed.txt")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)
	if !exists(filepath.Join(root, "renamed.txt")) {
		t.Fatal("renamed.txt was not created by rename mode")
	}
	if got.mode != ModeNormal || !strings.Contains(got.statusMessage, "Renamed") {
		t.Fatalf("rename mode/status = %v/%q", got.mode, got.statusMessage)
	}

	got.enterMode(ModeNewFile, "new file> ", "")
	got.input.SetValue("fresh.txt")
	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got = updated.(Model)
	if !exists(filepath.Join(root, "fresh.txt")) {
		t.Fatal("fresh.txt was not created")
	}

	got.enterMode(ModeNewDir, "new dir> ", "")
	got.input.SetValue("folder")
	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got = updated.(Model)
	if !exists(filepath.Join(root, "folder")) {
		t.Fatal("folder was not created")
	}

	got.enterMode(ModeGoToPath, "go> ", "")
	got.input.SetValue(filepath.Join(root, "folder"))
	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got = updated.(Model)
	if got.cwd != filepath.Join(root, "folder") {
		t.Fatalf("cwd = %q, want folder", got.cwd)
	}

	got.enterMode(ModeNewFile, "new file> ", "")
	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got = updated.(Model)
	if got.mode != ModeNormal {
		t.Fatalf("esc input mode = %v, want normal", got.mode)
	}
}

func TestNormalModeClipboardDeleteParentAndOpenCommands(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "xdg-data"))
	writeAppFile(t, filepath.Join(root, "copy.txt"), "copy\n")
	writeAppFile(t, filepath.Join(root, "cut.txt"), "cut\n")
	writeAppFile(t, filepath.Join(root, "delete.txt"), "delete\n")
	if err := os.Mkdir(filepath.Join(root, "child"), 0o755); err != nil {
		t.Fatal(err)
	}

	m, err := New(root, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	m.selectPath(filepath.Join(root, "copy.txt"))
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	got := updated.(Model)
	got.selectPath(root)
	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	got = updated.(Model)
	if !exists(filepath.Join(root, "copy_copy1.txt")) {
		t.Fatal("copy paste did not create copy_copy1.txt")
	}

	got.selectPath(filepath.Join(root, "cut.txt"))
	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	got = updated.(Model)
	got.selectPath(filepath.Join(root, "child"))
	got.drillIntoSelectedRoot()
	got.pasteClipboard()
	if exists(filepath.Join(root, "cut.txt")) || !exists(filepath.Join(root, "child", "cut.txt")) {
		t.Fatal("cut paste did not move cut.txt into child")
	}
	got.pasteClipboard()
	if got.statusMessage != "Clipboard is empty." {
		t.Fatalf("empty clipboard status = %q", got.statusMessage)
	}

	got.goParent()
	if got.cwd != root {
		t.Fatalf("goParent cwd = %q, want root", got.cwd)
	}
	got.selectPath(filepath.Join(root, "child"))
	got.collapseOrParent()
	got.collapseOrParent()
	if entry, ok := got.selected(); !ok || entry.Path != root {
		t.Fatalf("collapseOrParent selected = %#v, want root %q", entry, root)
	}

	got.selectPath(filepath.Join(root, "delete.txt"))
	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	got = updated.(Model)
	if got.mode != ModeConfirmDelete {
		t.Fatalf("delete mode = %v", got.mode)
	}
	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	got = updated.(Model)
	if got.mode != ModeNormal || got.statusMessage != "Delete cancelled." {
		t.Fatalf("delete cancel mode/status = %v/%q", got.mode, got.statusMessage)
	}
	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	got = updated.(Model)
	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	got = updated.(Model)
	if exists(filepath.Join(root, "delete.txt")) || !strings.Contains(got.statusMessage, "Safe-deleted") {
		t.Fatalf("safe delete exists/status = %v/%q", exists(filepath.Join(root, "delete.txt")), got.statusMessage)
	}

}

func TestUpdateDispatchAndHelpKeys(t *testing.T) {
	m := Model{mode: ModeHelp, helpReturnMode: ModeDiff, helpViewport: viewportForTest(20, 4), styles: ui.NewStyles()}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got := updated.(Model)
	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	got = updated.(Model)
	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	got = updated.(Model)
	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	got = updated.(Model)
	if got.mode != ModeDiff {
		t.Fatalf("help returned to mode %v, want diff", got.mode)
	}

	got.helpReturnMode = ModeHelp
	got.mode = ModeHelp
	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got = updated.(Model)
	if got.mode != ModeNormal {
		t.Fatalf("help self-return mode = %v, want normal", got.mode)
	}

	updated, _ = got.Update(statusMsg("plain status"))
	got = updated.(Model)
	if got.statusMessage != "plain status" {
		t.Fatalf("status dispatch = %q", got.statusMessage)
	}
	updated, _ = got.Update(tea.WindowSizeMsg{Width: 96, Height: 31})
	got = updated.(Model)
	if got.width != 96 || got.height != 31 || got.previewViewport.Width == 0 || got.diffViewport.Width == 0 || got.helpViewport.Width == 0 {
		t.Fatalf("resize dispatch failed: %#v", got)
	}
	updated, _ = got.Update(struct{}{})
	got = updated.(Model)
	if got.width != 96 || got.height != 31 {
		t.Fatalf("unknown message changed dimensions: %dx%d", got.width, got.height)
	}
}

func TestDiffHelpersAndGitActions(t *testing.T) {
	root := initAppRepo(t)
	writeAppFile(t, filepath.Join(root, "tracked.txt"), "one\n")
	runAppGit(t, root, "add", "tracked.txt")
	runAppGit(t, root, "commit", "-m", "initial")
	writeAppFile(t, filepath.Join(root, "tracked.txt"), "one\ntwo\n")
	writeAppFile(t, filepath.Join(root, "untracked.txt"), "new\n")

	m, err := New(root, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	noGit := m
	noGit.gitRoot = ""
	noGit.enterDiffMode()
	if noGit.mode == ModeDiff || !strings.Contains(noGit.statusMessage, "requires a git") {
		t.Fatalf("no-git diff mode/status = %v/%q", noGit.mode, noGit.statusMessage)
	}
	noGit.refreshDiff()
	if got := noGit.diffViewport.View(); !strings.Contains(got, "Not inside") {
		t.Fatalf("no-git diff viewport = %q", got)
	}

	m.enterDiffMode()
	if len(m.diffChanges) < 2 {
		t.Fatalf("diffChanges = %#v, want tracked and untracked", m.diffChanges)
	}
	m.diffSelectedIndex = -1
	m.clampDiffSelection()
	if m.diffSelectedIndex != 0 {
		t.Fatalf("negative diff index = %d", m.diffSelectedIndex)
	}
	m.diffSelectedIndex = 99
	m.clampDiffSelection()
	if m.diffSelectedIndex != len(m.diffChanges)-1 {
		t.Fatalf("high diff index = %d", m.diffSelectedIndex)
	}
	if got := diffPreviewContent(root, nil, 0, 1024); got != "No modified or untracked files." {
		t.Fatalf("empty diff preview = %q", got)
	}

	m.diffSelectedIndex = indexOfDiffPath(m.diffChanges, "untracked.txt")
	m.unstageSelectedDiff()
	if m.statusMessage != "Untracked files are not staged." {
		t.Fatalf("unstage untracked status = %q", m.statusMessage)
	}
	m.stageSelectedDiff()
	if !strings.Contains(m.statusMessage, "Staged") {
		t.Fatalf("stage status = %q", m.statusMessage)
	}
	m.diffSelectedIndex = indexOfDiffPath(m.diffChanges, "tracked.txt")
	m.unstageSelectedDiff()
	if !strings.Contains(m.statusMessage, "Unstaged") {
		t.Fatalf("unstage status = %q", m.statusMessage)
	}

	updated, _ := m.updateDiff(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	got := updated.(Model)
	if got.mode != ModeHelp || got.helpReturnMode != ModeDiff {
		t.Fatalf("diff help mode/return = %v/%v", got.mode, got.helpReturnMode)
	}
	updated, _ = m.updateDiff(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	got = updated.(Model)
	if got.mode != ModeDiffConfirmRemove {
		t.Fatalf("diff remove mode = %v", got.mode)
	}
	updated, _ = got.updateDiffConfirm(tea.KeyMsg{Type: tea.KeyEsc})
	got = updated.(Model)
	if got.mode != ModeDiff || got.statusMessage != "Diff action cancelled." {
		t.Fatalf("diff confirm cancel = %v/%q", got.mode, got.statusMessage)
	}
	m.applyDiffCommit("")
	if !strings.Contains(m.statusMessage, "commit message") {
		t.Fatalf("empty commit status = %q", m.statusMessage)
	}

	if got := selectDiffIndex([]git.Change{{Path: "a"}, {Path: "b"}}, "b", 0); got != 1 {
		t.Fatalf("selectDiffIndex by path = %d", got)
	}
	if got := selectDiffIndex([]git.Change{{Path: "a"}}, "", -5); got != 0 {
		t.Fatalf("selectDiffIndex low fallback = %d", got)
	}
	if got := selectDiffIndex([]git.Change{{Path: "a"}}, "", 5); got != 0 {
		t.Fatalf("selectDiffIndex high fallback = %d", got)
	}
}

func TestDiffLabelsAndFormattingEdges(t *testing.T) {
	cases := []struct {
		change git.Change
		status string
		kind   string
	}{
		{git.Change{Kind: git.ChangeUntracked}, "??", "untracked"},
		{git.Change{Kind: git.ChangeAdded, IndexStatus: 'A'}, "A-", "added"},
		{git.Change{Kind: git.ChangeDeleted, WorktreeStatus: 'D'}, "-D", "removed"},
		{git.Change{Kind: git.ChangeRenamed}, "--", "renamed"},
		{git.Change{Kind: git.ChangeModified, IndexStatus: 'M', WorktreeStatus: 'M'}, "MM", "modified"},
	}
	for _, tc := range cases {
		if got := diffStatusText(tc.change); got != tc.status {
			t.Fatalf("diffStatusText(%#v) = %q, want %q", tc.change, got, tc.status)
		}
		if got := diffKindLabel(tc.change); got != tc.kind {
			t.Fatalf("diffKindLabel(%#v) = %q, want %q", tc.change, got, tc.kind)
		}
	}
	summary := git.Summary{FilesAdded: 1, FilesChanged: 2, FilesRemoved: 3, LinesAdded: 4, LinesChanged: 5, LinesRemoved: 6}
	if got := diffSummaryText(summary); !strings.Contains(got, "Files +1 ~2 -3") || !strings.Contains(got, "Lines +4 ~5 -6") {
		t.Fatalf("diffSummaryText = %q", got)
	}
	if oldLine, newLine := parseHunkStart("not a hunk"); oldLine != 0 || newLine != 0 {
		t.Fatalf("bad hunk parsed as %d/%d", oldLine, newLine)
	}
	formatted := formatUnifiedDiff("rename from old.txt\nrename to new.txt\n\\ No newline at end of file\ncontext")
	for _, want := range []string{"rename from old.txt", "rename to new.txt", "\\ No newline"} {
		if !strings.Contains(formatted, want) {
			t.Fatalf("formatted edge diff missing %q:\n%s", want, formatted)
		}
	}
}

func TestEditorActionsTabsJumpsAndReload(t *testing.T) {
	root := t.TempDir()
	pathA := filepath.Join(root, "a.txt")
	pathB := filepath.Join(root, "b.txt")
	writeAppFile(t, pathA, "a\n")
	writeAppFile(t, pathB, "b\n")

	m, err := New(root, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	m.setEditorOpenError(editor.ErrDirectory)
	if !strings.Contains(m.statusMessage, "Directories") {
		t.Fatalf("directory edit error status = %q", m.statusMessage)
	}
	m.setEditorOpenError(editor.ErrBinary)
	if !strings.Contains(m.statusMessage, "Binary") {
		t.Fatalf("binary edit error status = %q", m.statusMessage)
	}
	m.setEditorOpenError(editor.ErrTooLarge)
	if !strings.Contains(m.statusMessage, "too large") {
		t.Fatalf("large edit error status = %q", m.statusMessage)
	}
	m.setEditorOpenError(errors.New("custom"))
	if !strings.Contains(m.statusMessage, "custom") {
		t.Fatalf("custom edit error status = %q", m.statusMessage)
	}

	if cmd := m.openEditorTab(""); cmd != nil || m.statusMessage != "No file path." {
		t.Fatalf("empty open tab cmd/status = %v/%q", cmd, m.statusMessage)
	}
	if cmd := m.openEditorTab(pathA); cmd != nil {
		t.Fatalf("lspDidOpenCmd should be nil, got %v", cmd)
	}
	if len(m.editorTabs) != 1 || m.focus != FocusEditor {
		t.Fatalf("editor tabs/focus = %d/%v", len(m.editorTabs), m.focus)
	}
	if cmd := m.openEditorTab("a.txt"); cmd != nil || len(m.editorTabs) != 1 || !strings.Contains(m.statusMessage, "Switched") {
		t.Fatalf("reopen tab cmd/count/status = %v/%d/%q", cmd, len(m.editorTabs), m.statusMessage)
	}
	_ = m.openEditorTab(pathB)
	m.nextTab()
	if m.activeTab != 0 {
		t.Fatalf("nextTab active = %d, want 0", m.activeTab)
	}
	m.prevTab()
	if m.activeTab != 1 {
		t.Fatalf("prevTab active = %d, want 1", m.activeTab)
	}
	if got := m.tabListStatus(); !strings.Contains(got, "[2:b.txt]") {
		t.Fatalf("tabListStatus = %q", got)
	}

	m.activeBuffer().Dirty = true
	if !m.hasDirtyTabs() {
		t.Fatal("hasDirtyTabs should be true")
	}
	updated, cmd := m.guardedQuit()
	got := updated.(Model)
	if cmd != nil || !strings.Contains(got.statusMessage, "Unsaved") {
		t.Fatalf("guardedQuit dirty cmd/status = %v/%q", cmd, got.statusMessage)
	}
	updated, _ = got.closeActiveTab(false)
	got = updated.(Model)
	if !strings.Contains(got.statusMessage, "Unsaved changes") {
		t.Fatalf("close dirty status = %q", got.statusMessage)
	}
	updated, _ = got.closeActiveTab(true)
	got = updated.(Model)
	if len(got.editorTabs) != 1 || got.activeTab != 0 {
		t.Fatalf("force close tabs/active = %d/%d", len(got.editorTabs), got.activeTab)
	}

	got.activeBuffer().Row = 0
	got.activeBuffer().Col = 0
	got.jumpBack = []editorJump{{Path: pathA, Row: 0, Col: 0}, {Path: pathB, Row: 0, Col: 0}}
	updated, _ = got.jumpHistoryBack()
	got = updated.(Model)
	if got.activeBuffer().Path != pathB || len(got.jumpForward) != 1 {
		t.Fatalf("jump back active/forward = %q/%#v", got.activeBuffer().Path, got.jumpForward)
	}
	updated, _ = got.jumpHistoryForward()
	got = updated.(Model)
	if got.activeBuffer().Path != pathA {
		t.Fatalf("jump forward active = %q", got.activeBuffer().Path)
	}
	empty := Model{}
	updated, _ = empty.jumpHistoryBack()
	if got := updated.(Model); got.statusMessage != "No older jump." {
		t.Fatalf("empty jump back status = %q", got.statusMessage)
	}

	time.Sleep(time.Millisecond)
	writeAppFile(t, pathA, "changed\n")
	got.reloadActiveIfChanged()
	if got.activeBuffer().Lines[0] != "changed" {
		t.Fatalf("reloadActiveIfChanged line = %q", got.activeBuffer().Lines[0])
	}
}

func TestApplyEditorActionVariants(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "xdg"))
	path := filepath.Join(root, "edit.txt")
	writeAppFile(t, path, "one\n")
	m, err := New(root, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	_ = m.openEditorTab(path)

	updated, _ := m.applyEditorAction(editor.Action{Kind: editor.ActionStatus, Message: "hello"})
	got := updated.(Model)
	if got.statusMessage != "hello" {
		t.Fatalf("status action = %q", got.statusMessage)
	}
	got.activeBuffer().Lines = []string{"saved"}
	got.activeBuffer().Dirty = true
	updated, cmd := got.applyEditorAction(editor.Action{Kind: editor.ActionSave})
	got = updated.(Model)
	if cmd != nil || !strings.Contains(got.statusMessage, "Saved") || got.activeBuffer().Dirty {
		t.Fatalf("save action cmd/status/dirty = %v/%q/%v", cmd, got.statusMessage, got.activeBuffer().Dirty)
	}
	updated, _ = got.applyEditorAction(editor.Action{Kind: editor.ActionListTabs})
	got = updated.(Model)
	if !strings.Contains(got.statusMessage, "edit.txt") {
		t.Fatalf("list tabs status = %q", got.statusMessage)
	}
	updated, _ = got.applyEditorAction(editor.Action{Kind: editor.ActionTheme})
	got = updated.(Model)
	if !strings.Contains(got.statusMessage, "Themes:") {
		t.Fatalf("theme list status = %q", got.statusMessage)
	}
	updated, _ = got.applyEditorAction(editor.Action{Kind: editor.ActionTheme, Message: "missing-theme"})
	got = updated.(Model)
	if !strings.Contains(got.statusMessage, "Unknown theme") {
		t.Fatalf("bad theme status = %q", got.statusMessage)
	}
	updated, _ = got.applyEditorAction(editor.Action{Kind: editor.ActionTheme, Message: "navia"})
	got = updated.(Model)
	if got.cfg.Theme != "navia" || !strings.Contains(got.statusMessage, "saved") {
		t.Fatalf("theme save cfg/status = %q/%q", got.cfg.Theme, got.statusMessage)
	}

	other := filepath.Join(root, "other.txt")
	writeAppFile(t, other, "other\n")
	updated, _ = got.applyEditorAction(editor.Action{Kind: editor.ActionOpen, Path: other})
	got = updated.(Model)
	if got.activeBuffer().Path != other {
		t.Fatalf("open action active = %q", got.activeBuffer().Path)
	}
	updated, cmd = got.applyEditorAction(editor.Action{Kind: editor.ActionDefinition})
	if cmd != nil {
		t.Fatalf("non-go definition cmd = %v, want nil", cmd)
	}
	got = updated.(Model)
	updated, cmd = got.applyEditorAction(editor.Action{Kind: editor.ActionReferences})
	if cmd != nil {
		t.Fatalf("non-go references cmd = %v, want nil", cmd)
	}

	updated, cmd = got.applyEditorAction(editor.Action{Kind: editor.ActionExternal})
	got = updated.(Model)
	if cmd == nil || got.lastCommandHint == "" {
		t.Fatalf("external action cmd/hint = %v/%q", cmd, got.lastCommandHint)
	}
	updated, cmd = got.applyEditorAction(editor.Action{Kind: editor.ActionQuitAll})
	got = updated.(Model)
	if cmd == nil {
		t.Fatal("quit all without dirty tabs should return tea.Quit")
	}
	updated, cmd = got.applyEditorAction(editor.Action{Kind: editor.ActionQuitAllForce})
	if cmd == nil {
		t.Fatal("force quit should return tea.Quit")
	}
}

func TestUpdateEditorWindowAndKeyActions(t *testing.T) {
	m := Model{focus: FocusEditor, editorTabs: []*editor.Buffer{editor.NewScratch("scratch.txt")}, activeTab: 0}
	updated, _ := m.updateEditor(tea.KeyMsg{Type: tea.KeyCtrlW})
	got := updated.(Model)
	if !got.windowPending {
		t.Fatal("ctrl+w should set editor window pending")
	}
	updated, _ = got.updateEditor(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	got = updated.(Model)
	if !got.treeHidden || got.focus != FocusEditor {
		t.Fatalf("ctrl+w o treeHidden/focus = %v/%v", got.treeHidden, got.focus)
	}
	updated, _ = got.updateEditor(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	got = updated.(Model)
	if got.activeBuffer().Mode != editor.Normal {
		t.Fatalf("unknown editor key changed mode to %v", got.activeBuffer().Mode)
	}

	got.activeTab = 99
	updated, _ = got.updateEditor(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	got = updated.(Model)
	if got.focus != FocusTree {
		t.Fatalf("nil active editor focus = %v, want tree", got.focus)
	}
}

func TestLSPCommandsAndHandlers(t *testing.T) {
	root := initAppRepo(t)
	goFile := filepath.Join(root, "main.go")
	refFile := filepath.Join(root, "ref.go")
	writeAppFile(t, goFile, "package main\nfunc main() {}\n")
	writeAppFile(t, refFile, "package main\nfunc ref() {}\n")
	runAppGit(t, root, "add", "main.go", "ref.go")
	runAppGit(t, root, "commit", "-m", "go files")

	m, err := New(root, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	if got := m.lspRoot(); got != root {
		t.Fatalf("lspRoot = %q, want git root", got)
	}
	m.gitRoot = ""
	if got := m.lspRoot(); got != root {
		t.Fatalf("lspRoot without cached git root = %q", got)
	}
	m.cwd = t.TempDir()
	if got := m.lspRoot(); got != m.cwd {
		t.Fatalf("lspRoot outside git = %q, want cwd", got)
	}

	m.cwd = root
	m.gitRoot = root
	_ = m.openEditorTab(goFile)
	m.cfg.EnableLSP = false
	if cmd := m.definitionCmd(); cmd != nil {
		t.Fatalf("disabled definition cmd = %v, want nil", cmd)
	}
	if cmd := m.referencesCmd(); cmd != nil {
		t.Fatalf("disabled references cmd = %v, want nil", cmd)
	}

	t.Setenv("NAVIA_FAKE_GOPLS", "1")
	t.Setenv("NAVIA_FAKE_LSP_DEFINITION", goFile)
	t.Setenv("NAVIA_FAKE_LSP_REFERENCE", refFile)
	m.cfg.EnableLSP = true
	m.cfg.GoplsCommand = os.Args[0]
	cmd := m.definitionCmd()
	if cmd == nil {
		t.Fatal("definitionCmd returned nil for Go buffer with LSP enabled")
	}
	msg := cmd()
	def, ok := msg.(definitionMsg)
	if !ok || def.Err != nil || len(def.Locations) != 1 {
		t.Fatalf("definition msg = %#v", msg)
	}
	updated, _ := m.handleDefinition(def)
	got := updated.(Model)
	if got.activeBuffer().Path != goFile || got.activeBuffer().Row != 1 || got.activeBuffer().Col != 2 {
		t.Fatalf("definition handler buffer/pos = %q %d:%d", got.activeBuffer().Path, got.activeBuffer().Row, got.activeBuffer().Col)
	}
	updated, _ = got.handleDefinition(definitionMsg{Err: errors.New("bad")})
	if got := updated.(Model); !strings.Contains(got.statusMessage, "gd failed") {
		t.Fatalf("definition error status = %q", got.statusMessage)
	}
	updated, _ = got.handleDefinition(definitionMsg{})
	if got := updated.(Model); got.statusMessage != "No definition found." {
		t.Fatalf("definition empty status = %q", got.statusMessage)
	}

	cmd = m.referencesCmd()
	if cmd == nil {
		t.Fatal("referencesCmd returned nil for Go buffer with LSP enabled")
	}
	msg = cmd()
	refs, ok := msg.(referencesMsg)
	if !ok || refs.Err != nil || len(refs.Locations) != 2 {
		t.Fatalf("references msg = %#v", msg)
	}
	updated, _ = m.handleReferences(refs)
	got = updated.(Model)
	if len(got.rows) != 1 || got.rows[0].Entry.Path != refFile || got.rows[0].Line != 3 {
		t.Fatalf("references handler rows = %#v", got.rows)
	}
	updated, _ = got.handleReferences(referencesMsg{Err: errors.New("bad")})
	if got := updated.(Model); !strings.Contains(got.statusMessage, "gr failed") {
		t.Fatalf("references error status = %q", got.statusMessage)
	}
	updated, _ = got.handleReferences(referencesMsg{})
	if got := updated.(Model); got.statusMessage != "No references found." {
		t.Fatalf("references empty status = %q", got.statusMessage)
	}
}

func TestViewRenderingHelpers(t *testing.T) {
	root := t.TempDir()
	writeAppFile(t, filepath.Join(root, "alpha.txt"), "alpha\n")
	m, err := New(root, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	m.styles = ui.NewStyles()
	m.width = 80
	m.height = 24
	if got := m.View(); !strings.Contains(got, "TREE") {
		t.Fatalf("normal view missing tree tag:\n%s", got)
	}
	starting := m
	starting.width = 0
	if got := starting.View(); got != "Navia is starting..." {
		t.Fatalf("starting view = %q", got)
	}
	if got := m.renderMain(26, 54); !strings.Contains(got, "alpha.txt") {
		t.Fatalf("renderMain missing alpha.txt:\n%s", got)
	}
	if got := m.renderTop(); !strings.Contains(got, "Rows:") {
		t.Fatalf("renderTop missing row meta:\n%s", got)
	}
	if got := m.topRightMeta(); !strings.Contains(got, "Rows:") {
		t.Fatalf("topRightMeta = %q", got)
	}
	if got := m.topContext(); !strings.Contains(got, filepath.Base(root)) {
		t.Fatalf("topContext = %q", got)
	}
	if got := gitPathLabel(root, filepath.Join(root, "sub", "file.go")); got != filepath.Join("sub", "file.go") {
		t.Fatalf("gitPathLabel = %q", got)
	}
	if got := gitPathLabel(root, root); got != "." {
		t.Fatalf("gitPathLabel root = %q", got)
	}
	if got := m.renderList(32); !strings.Contains(got, "alpha.txt") {
		t.Fatalf("renderList missing file:\n%s", got)
	}
	m.rows = nil
	if got := m.renderList(32); !strings.Contains(got, "No entries") {
		t.Fatalf("empty renderList = %q", got)
	}
	m.rows = m.treeRowsToResultRows(m.treeRows)
	m.selectedIndex = 0
	if got := m.renderPreview(48); !strings.Contains(got, "A microIDE") {
		t.Fatalf("idle preview missing brand:\n%s", got)
	}
	if !m.shouldRenderIdleBrand() {
		t.Fatal("should render idle brand at root with no editor tabs")
	}
	if got := m.renderIdleBrand(40, 12); !strings.Contains(got, "Navia") && !strings.Contains(got, "github.com") {
		t.Fatalf("idle brand = %q", got)
	}
	if got := m.panelStyle(FocusTree); got.GetBorderStyle().Top == "" {
		t.Fatalf("panelStyle returned empty style: %#v", got)
	}
	if got := m.renderFooter(); !strings.Contains(got, "q quit") {
		t.Fatalf("footer = %q", got)
	}
	m.statusMessage = "custom"
	m.lastCommandHint = "Shell equivalent: true"
	if got := m.renderFooter(); !strings.Contains(got, "custom") || !strings.Contains(got, "true") {
		t.Fatalf("custom footer = %q", got)
	}
}

func TestViewRenderingDiffEditorModalHelpAndUtilities(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "edit.go")
	writeAppFile(t, path, "package main\n")
	m, err := New(root, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	m.styles = ui.NewStyles()
	m.width = 88
	m.height = 26
	m.diffChanges = []git.Change{
		{Path: "added.go", Kind: git.ChangeAdded, IndexStatus: 'A'},
		{Path: "gone.go", Kind: git.ChangeDeleted, WorktreeStatus: 'D'},
		{Path: "new.go", Kind: git.ChangeUntracked},
		{Path: "newname.go", OldPath: "oldname.go", Kind: git.ChangeRenamed, IndexStatus: 'R'},
	}
	m.diffSummary = git.Summary{FilesAdded: 1, FilesChanged: 1}
	m.diffViewport.SetContent("          | diff --git a/a b/a\n        1 | +add\n   1      | -remove\n          | @@ -1 +1 @@")
	m.mode = ModeDiff
	if got := m.renderMain(30, 58); !strings.Contains(got, "added.go") || !strings.Contains(got, "+add") {
		t.Fatalf("diff main = %q", got)
	}
	if got := m.renderDiffList(30); !strings.Contains(got, "renamed") {
		t.Fatalf("diff list = %q", got)
	}
	if got := m.renderDiffPane(58); !strings.Contains(got, "added.go") {
		t.Fatalf("diff pane = %q", got)
	}
	if got := m.renderDiffContent(); !strings.Contains(got, "+add") || !strings.Contains(got, "-remove") {
		t.Fatalf("diff content = %q", got)
	}
	if got := m.topRightMeta(); got != "Files:4" {
		t.Fatalf("diff topRightMeta = %q", got)
	}
	if got := m.topContext(); !strings.Contains(got, "Files +1") {
		t.Fatalf("diff topContext = %q", got)
	}
	if got := m.renderFooter(); !strings.Contains(got, "s stage") {
		t.Fatalf("diff footer = %q", got)
	}
	m.diffChanges = nil
	if got := m.renderDiffList(30); !strings.Contains(got, "No modified") {
		t.Fatalf("empty diff list = %q", got)
	}

	_ = m.openEditorTab(path)
	m.focus = FocusEditor
	m.mode = ModeNormal
	m.width = 44
	if got := m.renderTabs(12); !strings.Contains(got, "edit.go") {
		t.Fatalf("renderTabs narrow = %q", got)
	}
	if start, end := m.visibleTabRange(0); start != m.activeTab || end != m.activeTab {
		t.Fatalf("visibleTabRange zero = %d/%d", start, end)
	}
	if got := m.tabWidth(-1); got != 0 {
		t.Fatalf("tabWidth(-1) = %d", got)
	}
	if got := m.tabText(0); !strings.Contains(got, "edit.go") {
		t.Fatalf("tabText = %q", got)
	}
	if got := m.renderTab(0); !strings.Contains(got, "edit.go") {
		t.Fatalf("renderTab = %q", got)
	}
	if got := m.renderRightPane(44); !strings.Contains(got, "edit.go") {
		t.Fatalf("editor right pane = %q", got)
	}
	if got := m.topRightMeta(); !strings.Contains(got, "Lines:") {
		t.Fatalf("editor topRightMeta = %q", got)
	}
	if got := m.topContext(); !strings.Contains(got, "edit.go") {
		t.Fatalf("editor topContext = %q", got)
	}
	m.statusMessage = ""
	if got := m.renderFooter(); !strings.Contains(got, ":w save") {
		t.Fatalf("editor footer = %q", got)
	}
	m.focus = FocusTree
	if got := m.renderEditor(44, m.activeBuffer()); !strings.Contains(got, "edit.go") {
		t.Fatalf("blurred editor = %q", got)
	}

	m.pendingDelete = navfs.FileEntry{Name: "old.txt"}
	for _, tc := range []struct {
		mode Mode
		want string
	}{
		{ModeConfirmDelete, "Safe delete"},
		{ModeRename, "Rename"},
		{ModeNewFile, "Create new file"},
		{ModeNewDir, "Create new directory"},
		{ModeGoToPath, "Go to path"},
		{ModeDiffCommit, "Commit changes"},
		{ModeDiffConfirmRestore, "Restore"},
		{ModeDiffConfirmRemove, "Remove"},
	} {
		m.mode = tc.mode
		m.pendingDiffAction = git.Change{Path: "change.txt"}
		if got := m.renderModal(); !strings.Contains(got, tc.want) {
			t.Fatalf("renderModal(%v) = %q, want %q", tc.mode, got, tc.want)
		}
		if got := m.View(); !strings.Contains(got, tc.want) {
			t.Fatalf("modal View(%v) missing %q", tc.mode, tc.want)
		}
	}
	m.mode = ModeNormal
	if got := m.renderModal(); got != "" {
		t.Fatalf("normal modal = %q", got)
	}
	m.mode = ModeHelp
	m.width = 120
	m.height = 80
	m.helpViewport = viewport.New(84, 70)
	if got := m.renderHelp(); !strings.Contains(got, "Global") || !strings.Contains(got, "Editor Commands") {
		t.Fatalf("renderHelp = %q", got)
	}
	if got := m.View(); !strings.Contains(got, "Global") {
		t.Fatalf("help View = %q", got)
	}

	if got := commandCueColor("INSERT"); got != "114" {
		t.Fatalf("insert cue color = %q", got)
	}
	if got := commandCueColor("VISUAL-LINE"); got != "183" {
		t.Fatalf("visual cue color = %q", got)
	}
	if got := editorModeColor("SEARCH"); got != "58" {
		t.Fatalf("search mode color = %q", got)
	}
	if got := searchModeColor(SearchText); got != "90" {
		t.Fatalf("search text color = %q", got)
	}
	if got := (Model{searchMode: SearchText}).searchModeLabel(); got != "text" {
		t.Fatalf("searchModeLabel = %q", got)
	}
	if got := truncate("abcdef", 4); got != "abc…" {
		t.Fatalf("truncate = %q", got)
	}
	if got := truncate("abcdef", 1); got != "a" {
		t.Fatalf("truncate width 1 = %q", got)
	}
	if got := truncate("abcdef", 0); got != "" {
		t.Fatalf("truncate width 0 = %q", got)
	}
	if got := clip("abcdef", 3); got != "abc" {
		t.Fatalf("clip = %q", got)
	}
	if got := clip("abcdef", 0); got != "" {
		t.Fatalf("clip width 0 = %q", got)
	}
	if got := clipStyled("abcdef", 0); got != "" {
		t.Fatalf("clipStyled width 0 = %q", got)
	}
	if got := center("abcdef", 3); got != "ab…" {
		t.Fatalf("center truncate = %q", got)
	}
	if got := min(2, 3); got != 2 {
		t.Fatalf("min = %d", got)
	}
}

func TestPreviewRenderBoundsAndNotices(t *testing.T) {
	m := Model{previewViewport: viewportForTest(0, 0), preview: navfs.Preview{Kind: navfs.PreviewText, Content: "short"}, styles: ui.NewStyles()}
	if got := m.previewRenderLineLimit(); got != 96 {
		t.Fatalf("default preview line limit = %d", got)
	}
	if got := m.previewRenderLineWidth(); got != 320 {
		t.Fatalf("default preview line width = %d", got)
	}
	m.previewViewport.Width = 1000
	m.previewViewport.Height = 1000
	if got := m.previewRenderLineLimit(); got != previewRenderMaxLines {
		t.Fatalf("max preview line limit = %d", got)
	}
	if got := m.previewRenderLineWidth(); got != previewRenderMaxLineRunes {
		t.Fatalf("max preview line width = %d", got)
	}
	if got, clipped := clipPreviewLine("abc", 0); got != "abc" || clipped {
		t.Fatalf("clipPreviewLine short = %q/%v", got, clipped)
	}
	if got := previewRenderNotice(false, true, 10); !strings.Contains(got, "clipped long lines") {
		t.Fatalf("long-line notice = %q", got)
	}
}

func TestEditorLabelsAndPaths(t *testing.T) {
	buf := editor.NewScratch("/tmp/example.txt")
	for _, tc := range []struct {
		mode editor.Mode
		want string
	}{
		{editor.Insert, "INSERT"},
		{editor.Visual, "VISUAL"},
		{editor.VisualLine, "VISUAL-LINE"},
		{editor.Command, "EXEC"},
		{editor.Search, "SEARCH"},
		{editor.Normal, "NORMAL"},
	} {
		buf.Mode = tc.mode
		if got := editorModeLabel(buf); got != tc.want {
			t.Fatalf("editorModeLabel(%v) = %q", tc.mode, got)
		}
	}
	if got := editorModeLabel(nil); got != "NORMAL" {
		t.Fatalf("nil editorModeLabel = %q", got)
	}
	buf.Dirty = true
	if got := tabLabel(buf); got != "example.txt*" {
		t.Fatalf("dirty tabLabel = %q", got)
	}
	if got := statusPath(filepath.Join(os.Getenv("HOME"), "navia-test.txt")); !strings.HasPrefix(got, "~") {
		t.Fatalf("home statusPath = %q", got)
	}
	if got := statusPath("/definitely/not/home/navia-test.txt"); got != "/definitely/not/home/navia-test.txt" {
		t.Fatalf("plain statusPath = %q", got)
	}
}

func TestOpenSelectedInEditorAndTextSearchResult(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "alpha.txt")
	writeAppFile(t, file, "first\nsecond\n")
	if err := os.Mkdir(filepath.Join(root, "dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	m, err := New(root, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	m.selectPath(filepath.Join(root, "dir"))
	if cmd := m.openSelectedInEditor(); cmd != nil || m.statusMessage != "Select a text file to edit." {
		t.Fatalf("dir openSelectedInEditor cmd/status = %v/%q", cmd, m.statusMessage)
	}
	m.rows = []ResultRow{{Entry: navfs.FileEntry{Name: "alpha.txt", Path: file}, Line: 2, Snippet: "second"}}
	m.selectedIndex = 0
	m.openSelected()
	if m.activeBuffer() == nil || m.activeBuffer().Path != file || m.activeBuffer().Row != 1 {
		t.Fatalf("text search open buffer = %#v", m.activeBuffer())
	}
}

func TestDefaultNameForCopyUsesNextAvailableSuffix(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "note.txt")
	writeAppFile(t, src, "one\n")
	if got := defaultNameForCopy(src, root); got != filepath.Join(root, "note_copy1.txt") {
		t.Fatalf("defaultNameForCopy existing source = %q", got)
	}
	writeAppFile(t, filepath.Join(root, "note_copy1.txt"), "copy\n")
	if got := defaultNameForCopy(src, root); got != filepath.Join(root, "note_copy2.txt") {
		t.Fatalf("defaultNameForCopy second suffix = %q", got)
	}
}

func viewportForTest(width, height int) viewport.Model {
	v := viewport.New(width, height)
	v.SetContent("one\ntwo\nthree\nfour\nfive")
	return v
}

func indexOfDiffPath(changes []git.Change, path string) int {
	for i, change := range changes {
		if change.Path == path {
			return i
		}
	}
	return 0
}

func runFakeLSPServer() {
	reader := bufio.NewReader(os.Stdin)
	for {
		body, err := readFakeLSPMessage(reader)
		if err != nil {
			return
		}
		var request struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
		}
		if err := json.Unmarshal(body, &request); err != nil || request.ID == 0 {
			continue
		}
		var result any = map[string]any{}
		switch request.Method {
		case "textDocument/definition":
			result = fakeLocation(os.Getenv("NAVIA_FAKE_LSP_DEFINITION"), 1, 2)
		case "textDocument/references":
			result = []any{
				fakeLocation(os.Getenv("NAVIA_FAKE_LSP_REFERENCE"), 2, 4),
				fakeLocation(filepath.Join(os.TempDir(), "missing-navia-reference.go"), 4, 1),
			}
		}
		writeFakeLSPResponse(request.ID, result)
	}
}

func readFakeLSPMessage(reader *bufio.Reader) ([]byte, error) {
	length := -1
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		key, value, ok := strings.Cut(line, ":")
		if ok && strings.EqualFold(strings.TrimSpace(key), "Content-Length") {
			n, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return nil, err
			}
			length = n
		}
	}
	if length < 0 {
		return nil, fmt.Errorf("missing content length")
	}
	body := make([]byte, length)
	_, err := io.ReadFull(reader, body)
	return body, err
}

func writeFakeLSPResponse(id int, result any) {
	response, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
	fmt.Fprintf(os.Stdout, "Content-Length: %d\r\n\r\n%s", len(response), response)
}

func fakeLocation(path string, line, character int) map[string]any {
	return map[string]any{
		"uri": lspFileURIForTest(path),
		"range": map[string]any{
			"start": map[string]any{
				"line":      line,
				"character": character,
			},
		},
	}
}

func lspFileURIForTest(path string) string {
	return "file://" + filepath.ToSlash(path)
}
