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

func TestEOpensSelectedFileInNaviaEditor(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "alpha.txt")
	writeAppFile(t, file, "alpha\n")

	m, err := New(root, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	m.selectPath(file)

	updated, cmd := m.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	got := updated.(Model)
	if cmd != nil {
		t.Fatalf("e edit cmd = %v, want nil", cmd)
	}
	if got.activeBuffer() == nil || got.activeBuffer().Path != file {
		t.Fatalf("e edit active buffer = %#v, want %q", got.activeBuffer(), file)
	}
	if got.focus != FocusEditor {
		t.Fatalf("e edit focus = %v, want editor", got.focus)
	}
	if got.lastCommandHint != "" {
		t.Fatalf("e edit should not launch external editor hint, got %q", got.lastCommandHint)
	}
}

func TestEditorToTreeRestoresActiveFileSelectionAndPreview(t *testing.T) {
	root := t.TempDir()
	active := filepath.Join(root, "active.txt")
	other := filepath.Join(root, "other.txt")
	writeAppFile(t, active, "active preview body\n")
	writeAppFile(t, other, "other preview body\n")

	m, err := New(root, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	m.selectPath(active)
	updated, _ := m.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m = updated.(Model)
	m.selectPath(other)

	updated, _ = m.updateEditor(tea.KeyMsg{Type: tea.KeyCtrlW})
	m = updated.(Model)
	updated, cmd := m.updateEditor(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	got := updated.(Model)
	if got.focus != FocusTree {
		t.Fatalf("focus = %v, want tree", got.focus)
	}
	if entry, ok := got.selected(); !ok || entry.Path != active {
		t.Fatalf("selected = %#v, want %q", entry, active)
	}
	if cmd == nil {
		t.Fatal("expected preview command")
	}
	if msg, ok := cmd().(previewLoadedMsg); ok {
		got.applyPreviewLoaded(msg)
	}
	if got.preview.Path != active || !strings.Contains(got.preview.Content, "active preview body") {
		t.Fatalf("preview path/content = %q/%q, want active file preview", got.preview.Path, got.preview.Content)
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
	cmd := m.handleAutoRefresh()
	if cmd == nil {
		t.Fatal("expected auto refresh command")
	}
	msg, ok := cmd().(treeSignatureMsg)
	if !ok {
		t.Fatalf("auto refresh msg = %T, want treeSignatureMsg", cmd())
	}
	updated, _ := m.handleTreeSignature(msg)
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
	cmd := m.handleAutoRefresh()
	if cmd == nil {
		t.Fatal("expected diff refresh command")
	}
	msg, ok := cmd().(diffRefreshMsg)
	if !ok {
		t.Fatalf("auto refresh msg = %T, want diffRefreshMsg", cmd())
	}
	updated, _ := m.handleDiffRefresh(msg)
	got := updated.(Model)
	if !strings.Contains(got.diffViewport.View(), "three") {
		t.Fatalf("diff preview did not auto-refresh:\n%s", got.diffViewport.View())
	}
	if len(got.diffChanges) != 1 || got.diffChanges[0].Path != "tracked.txt" {
		t.Fatalf("diff selection changed unexpectedly: %#v", got.diffChanges)
	}
}

func TestNewRespectsShowHiddenConfig(t *testing.T) {
	root := t.TempDir()
	writeAppFile(t, filepath.Join(root, ".hidden"), "secret\n")
	writeAppFile(t, filepath.Join(root, "visible"), "ok\n")

	cfg := config.Default()
	cfg.ShowHidden = false
	m, err := New(root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := rowNamed(m.rows, ".hidden"); ok {
		t.Fatalf("hidden file should not be visible: %#v", m.rows)
	}
	if _, ok := rowNamed(m.rows, "visible"); !ok {
		t.Fatalf("visible file missing: %#v", m.rows)
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
	if m.mode != ModeNormal {
		t.Fatalf("mode = %v, want ModeNormal", m.mode)
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

func TestEscClosesSearchTabWithActionStatus(t *testing.T) {
	root := t.TempDir()
	match := filepath.Join(root, "combat-plan.md")
	other := filepath.Join(root, "notes.md")
	writeAppFile(t, match, "alpha\n")
	writeAppFile(t, other, "beta\n")

	m, err := NewWithSearch(root, config.Default(), StartupSearch{Mode: SearchFiles, Query: "combat"})
	if err != nil {
		t.Fatal(err)
	}
	if !m.hasSearchTab() {
		t.Fatal("expected active search state")
	}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(Model)
	if cmd == nil {
		t.Fatal("expected preview/status command after closing search")
	}
	if got.filter != "" || got.executedSearchQuery != "" || got.searchRunning {
		t.Fatalf("search state still open: filter=%q executed=%q running=%v", got.filter, got.executedSearchQuery, got.searchRunning)
	}
	if got.statusMessage != "Closed search tab." {
		t.Fatalf("status = %q", got.statusMessage)
	}
	if got.statusRevision == 0 {
		t.Fatal("status revision should advance for transient action feedback")
	}
	if len(got.rows) != len(got.treeRows) {
		t.Fatalf("rows = %d, want tree rows %d", len(got.rows), len(got.treeRows))
	}
}

func TestFilterEscClosesPendingSearchWithActionStatus(t *testing.T) {
	root := t.TempDir()
	writeAppFile(t, filepath.Join(root, "combat-plan.md"), "alpha\n")

	m, err := New(root, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	m.mode = ModeFilter
	m.filter = "combat"
	m.executedSearchQuery = ""
	m.applyFilter()

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(Model)
	if cmd == nil {
		t.Fatal("expected preview/status command after closing pending search")
	}
	if got.mode != ModeNormal || got.filter != "" || got.executedSearchQuery != "" {
		t.Fatalf("mode/search = %v/%q/%q", got.mode, got.filter, got.executedSearchQuery)
	}
	if got.statusMessage != "Closed search tab." {
		t.Fatalf("status = %q", got.statusMessage)
	}
}

func TestClosingLastEditorFromSearchClosesBothTabs(t *testing.T) {
	root := t.TempDir()
	match := filepath.Join(root, "combat-plan.md")
	writeAppFile(t, match, "alpha\n")

	m, err := NewWithSearch(root, config.Default(), StartupSearch{Mode: SearchFiles, Query: "combat"})
	if err != nil {
		t.Fatal(err)
	}
	_ = m.openEditorTab(match)

	updated, cmd := m.closeActiveTab(false)
	got := updated.(Model)
	if cmd == nil {
		t.Fatal("expected preview command after returning to tree")
	}
	if len(got.editorTabs) != 0 || got.focus != FocusTree || got.treeHidden {
		t.Fatalf("editor state = tabs %d focus %v hidden %v", len(got.editorTabs), got.focus, got.treeHidden)
	}
	if got.filter != "" || got.executedSearchQuery != "" || got.searchRunning {
		t.Fatalf("search state still open: filter=%q executed=%q running=%v", got.filter, got.executedSearchQuery, got.searchRunning)
	}
	if got.statusMessage != "Closed editor tab. Closed search tab." {
		t.Fatalf("status = %q", got.statusMessage)
	}
}

func TestStatusMessageClearsOnlyCurrentRevision(t *testing.T) {
	m := Model{statusMessage: "older", statusRevision: 4}
	updated, cmd := m.Update(statusMsg("newer"))
	got := updated.(Model)
	if cmd == nil {
		t.Fatal("expected status clear command")
	}
	if got.statusRevision != 5 || got.statusMessage != "newer" {
		t.Fatalf("status revision/message = %d/%q", got.statusRevision, got.statusMessage)
	}

	updated, _ = got.Update(statusClearMsg{revision: 4})
	stale := updated.(Model)
	if stale.statusMessage != "newer" {
		t.Fatalf("stale clear removed status: %q", stale.statusMessage)
	}

	updated, _ = got.Update(statusClearMsg{revision: got.statusRevision})
	cleared := updated.(Model)
	if cleared.statusMessage != "" {
		t.Fatalf("current clear left status = %q", cleared.statusMessage)
	}
}

func TestFilterEnterRunsSearchAsCommand(t *testing.T) {
	root := t.TempDir()
	match := filepath.Join(root, "combat-plan.md")
	writeAppFile(t, match, "alpha\n")

	m, err := New(root, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	m.mode = ModeFilter
	m.filter = "combat"
	updated, cmd := m.updateFilter(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)
	if cmd == nil {
		t.Fatal("expected async search command")
	}
	if !got.searchRunning || got.statusMessage != "Searching..." {
		t.Fatalf("search state running=%v status=%q", got.searchRunning, got.statusMessage)
	}
	msg, ok := cmd().(searchLoadedMsg)
	if !ok {
		t.Fatalf("search msg = %T, want searchLoadedMsg", cmd())
	}
	updated, _ = got.handleSearchLoaded(msg)
	got = updated.(Model)
	if got.searchRunning {
		t.Fatal("search should be complete")
	}
	if got.mode != ModeNormal {
		t.Fatalf("mode after search = %v, want ModeNormal", got.mode)
	}
	if len(got.rows) != 1 || got.rows[0].Entry.Path != match {
		t.Fatalf("rows = %#v, want %q", got.rows, match)
	}
}

func TestFilterModeLetsJKTypeBeforeSearchSubmission(t *testing.T) {
	m := Model{mode: ModeFilter}
	updated, _ := m.updateFilter(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got := updated.(Model)
	updated, _ = got.updateFilter(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	got = updated.(Model)
	if got.filter != "jk" {
		t.Fatalf("filter = %q, want typed jk", got.filter)
	}
}

func TestFilterModePastesFlattenedQueryText(t *testing.T) {
	root := t.TempDir()
	writeAppFile(t, filepath.Join(root, "alpha-beta.txt"), "needle\n")
	m, err := New(root, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	m.mode = ModeFilter
	m.filter = "alpha"
	m.executedSearchQuery = "alpha"

	updated, _ := m.updateFilter(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" beta\nneedle\tvalue"), Paste: true})
	got := updated.(Model)
	if got.filter != "alphabeta needle value" || got.executedSearchQuery != "" {
		t.Fatalf("pasted filter/executed = %q/%q", got.filter, got.executedSearchQuery)
	}
}

func TestSubmittedSearchResultsNavigateAndOpenSelection(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "a-combat.md")
	second := filepath.Join(root, "b-combat.md")
	writeAppFile(t, first, "alpha\n")
	writeAppFile(t, second, "beta\n")

	m, err := New(root, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	m.mode = ModeFilter
	m.filter = "combat"
	updated, cmd := m.updateFilter(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)
	if cmd == nil {
		t.Fatal("expected search command")
	}
	msg := cmd().(searchLoadedMsg)
	updated, _ = got.handleSearchLoaded(msg)
	got = updated.(Model)
	if got.mode != ModeNormal || got.selectedIndex != 0 {
		t.Fatalf("after load mode=%v selected=%d", got.mode, got.selectedIndex)
	}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyDown})
	got = updated.(Model)
	if got.selectedIndex != 1 || got.filter != "combat" || got.executedSearchQuery != "combat" {
		t.Fatalf("down changed selection/search incorrectly: selected=%d filter=%q executed=%q", got.selectedIndex, got.filter, got.executedSearchQuery)
	}
	got.mode = ModeFilter
	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	got = updated.(Model)
	if got.selectedIndex != 0 || got.filter != "combat" {
		t.Fatalf("submitted k should navigate without editing: selected=%d filter=%q", got.selectedIndex, got.filter)
	}
	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got = updated.(Model)
	if got.selectedIndex != 1 || got.filter != "combat" {
		t.Fatalf("submitted j should navigate without editing: selected=%d filter=%q", got.selectedIndex, got.filter)
	}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got = updated.(Model)
	if len(got.editorTabs) != 1 || got.editorTabs[0].Path != second {
		t.Fatalf("editor tabs = %#v, want %q", got.editorTabs, second)
	}
}

func TestStaleSearchResultIsIgnored(t *testing.T) {
	root := t.TempDir()
	writeAppFile(t, filepath.Join(root, "a.txt"), "alpha\n")
	m, err := New(root, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	m.mode = ModeFilter
	m.filter = "alpha"
	m.executedSearchQuery = "alpha"
	m.searchRequestID = 2
	m.rows = []ResultRow{{Entry: m.entries[0]}}

	updated, _ := m.handleSearchLoaded(searchLoadedMsg{id: 1, mode: SearchFiles, query: "alpha", root: root})
	got := updated.(Model)
	if got.searchRequestID != 2 || len(got.rows) != 1 {
		t.Fatalf("stale result changed model: id=%d rows=%#v", got.searchRequestID, got.rows)
	}
}

func TestStalePreviewResultIsIgnored(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a.txt")
	b := filepath.Join(root, "b.txt")
	writeAppFile(t, a, "a\n")
	writeAppFile(t, b, "b\n")
	m, err := New(root, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	m.selectPath(a)
	cmdA := m.queuePreview()
	m.selectPath(b)
	cmdB := m.queuePreview()
	if cmdA == nil || cmdB == nil {
		t.Fatal("expected preview commands")
	}
	if msg, ok := cmdA().(previewLoadedMsg); ok {
		m.applyPreviewLoaded(msg)
	}
	if m.preview.Path == a {
		t.Fatalf("stale preview applied for %s", a)
	}
	if msg, ok := cmdB().(previewLoadedMsg); ok {
		m.applyPreviewLoaded(msg)
	}
	if m.preview.Path != b {
		t.Fatalf("latest preview path = %q, want %q", m.preview.Path, b)
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
