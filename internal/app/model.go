package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/heidaraliy/navia/internal/config"
	"github.com/heidaraliy/navia/internal/editor"
	navfs "github.com/heidaraliy/navia/internal/fs"
	"github.com/heidaraliy/navia/internal/git"
	"github.com/heidaraliy/navia/internal/syntax"
	"github.com/heidaraliy/navia/internal/ui"
)

type Mode int

const (
	ModeNormal Mode = iota
	ModeFilter
	ModeRename
	ModeNewFile
	ModeNewDir
	ModeGoToPath
	ModeConfirmDelete
	ModeHelp
	ModeDiff
	ModeDiffCommit
	ModeDiffConfirmRestore
	ModeDiffConfirmRemove
)

type SearchMode int

const (
	SearchFiles SearchMode = iota
	SearchText
)

type StartupSearch struct {
	Mode  SearchMode
	Query string
}

type StartupPatchReview struct {
	Label string
	Data  []byte
}

type FocusPane int

const (
	FocusTree FocusPane = iota
	FocusEditor
)

type ClipboardOperation int

const (
	ClipboardNone ClipboardOperation = iota
	ClipboardCopy
	ClipboardCut
)

type ClipboardState struct {
	Path     string
	Op       ClipboardOperation
	IsDir    bool
	BaseName string
}

type TreeRow struct {
	Entry    navfs.FileEntry
	Depth    int
	Expanded bool
}

type ResultRow struct {
	Entry   navfs.FileEntry
	Depth   int
	Line    int
	Snippet string
}

type editorJump struct {
	Path string
	Row  int
	Col  int
}

type previewLoadedMsg struct {
	id      int
	path    string
	preview navfs.Preview
	content string
}

type Model struct {
	cwd                  string
	entries              []navfs.FileEntry
	treeRows             []TreeRow
	treeRowByPath        map[string]TreeRow
	rows                 []ResultRow
	recursiveRows        []ResultRow
	recursiveRoot        string
	selectedIndex        int
	filter               string
	executedSearchQuery  string
	searchRequestID      int
	searchRunning        bool
	searchMode           SearchMode
	mode                 Mode
	helpReturnMode       Mode
	clipboard            ClipboardState
	preview              navfs.Preview
	previewViewport      viewport.Model
	previewRequestID     int
	diffViewport         viewport.Model
	diffChanges          []git.Change
	diffSummary          git.Summary
	diffPatchReview      *git.PatchReview
	diffPatchLabel       string
	diffSelectedIndex    int
	diffRefreshSignature string
	diffPreviewRequestID int
	pendingDiffAction    git.Change
	helpViewport         viewport.Model
	editorTabs           []*editor.Buffer
	activeTab            int
	jumpBack             []editorJump
	jumpForward          []editorJump
	treeHidden           bool
	focus                FocusPane
	windowPending        bool
	input                textinput.Model
	lastCommandHint      string
	statusMessage        string
	statusRevision       int
	treeRefreshSignature string
	cfg                  config.Config
	gitRoot              string
	width                int
	height               int
	styles               ui.Styles
	syntax               syntax.Renderer
	pendingDelete        navfs.FileEntry
	expandedDirs         map[string]bool
}

const (
	previewRenderMaxLines        = 400
	previewRenderOverscanScreens = 4
	previewRenderMaxLineRunes    = 2000
)

func New(start string, cfg config.Config) (Model, error) {
	cwd, err := navfs.ResolveDir(start)
	if err != nil {
		return Model{}, err
	}
	input := textinput.New()
	input.CharLimit = 512
	input.Prompt = "> "
	m := Model{
		cwd:             cwd,
		cfg:             cfg,
		styles:          ui.NewStyles(),
		syntax:          syntax.New(cfg.Theme),
		input:           input,
		previewViewport: viewport.New(40, 10),
		diffViewport:    viewport.New(67, 10),
		helpViewport:    viewport.New(80, 20),
		expandedDirs:    map[string]bool{cwd: true},
	}
	m.gitRoot = git.FindRoot(cwd)
	if err := m.refresh(); err != nil {
		return Model{}, err
	}
	return m, nil
}

func NewFromPath(start string, cfg config.Config) (Model, error) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return Model{}, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return Model{}, err
	}
	if info.IsDir() {
		return New(abs, cfg)
	}
	return NewWithFile(abs, cfg)
}

func NewWithFile(path string, cfg config.Config) (Model, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return Model{}, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return Model{}, err
	}
	if info.IsDir() {
		return New(abs, cfg)
	}
	m, err := New(filepath.Dir(abs), cfg)
	if err != nil {
		return Model{}, err
	}
	m.StartFile(abs)
	return m, nil
}

func NewWithSearch(start string, cfg config.Config, search StartupSearch) (Model, error) {
	m, err := New(start, cfg)
	if err != nil {
		return Model{}, err
	}
	m.StartSearch(search)
	return m, nil
}

func NewWithDiff(start string, cfg config.Config) (Model, error) {
	m, err := New(start, cfg)
	if err != nil {
		return Model{}, err
	}
	m.StartDiff()
	return m, nil
}

func NewWithPatchReview(start string, cfg config.Config, patch StartupPatchReview) (Model, error) {
	m, err := New(start, cfg)
	if err != nil {
		return Model{}, err
	}
	if err := m.StartPatchReview(patch); err != nil {
		return Model{}, err
	}
	return m, nil
}

func (m Model) Init() tea.Cmd {
	return autoRefreshCmd()
}

func (m *Model) SetStatus(msg string) {
	m.statusMessage = msg
}

func (m *Model) StartSearch(search StartupSearch) {
	query := strings.TrimSpace(search.Query)
	if query == "" {
		return
	}
	m.searchMode = search.Mode
	m.filter = query
	m.executedSearchQuery = query
	m.mode = ModeNormal
	m.selectedIndex = 0
	m.applyFilter()
	m.clampSelection()
	m.refreshPreview()
}

func (m *Model) StartFile(path string) {
	if path == "" {
		return
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(m.cwd, path)
	}
	path = filepath.Clean(path)
	m.mode = ModeNormal
	m.filter = ""
	m.executedSearchQuery = ""
	m.searchRunning = false
	m.selectPath(path)
	m.refreshPreview()
	_ = m.openEditorTab(path)
	if buf := m.activeBuffer(); buf != nil {
		buf.Mode = editor.Normal
	}
}

func (m *Model) StartDiff() {
	m.enterDiffMode()
}

func (m *Model) StartPatchReview(patch StartupPatchReview) error {
	review, err := git.ParsePatchReview(patch.Data)
	if err != nil {
		return err
	}
	m.mode = ModeDiff
	m.focus = FocusTree
	m.diffPatchReview = &review
	m.diffPatchLabel = strings.TrimSpace(patch.Label)
	if m.diffPatchLabel == "" {
		m.diffPatchLabel = "Patch review"
	}
	m.diffChanges = review.Changes
	m.diffSummary = review.Summary
	m.diffSelectedIndex = 0
	m.statusMessage = "Patch review."
	m.clampDiffSelection()
	m.refreshDiffPreview()
	return nil
}

func (m *Model) refresh() error {
	if err := m.refreshTreeData(); err != nil {
		return err
	}
	m.refreshPreview()
	return nil
}

func (m *Model) refreshTreeData() error {
	entries, err := navfs.ScanDir(m.cwd, m.scanOptions())
	if err != nil {
		return err
	}
	m.entries = entries
	m.treeRows = m.buildTreeRows()
	m.rebuildTreeRowIndex()
	m.applyFilter()
	m.clampSelection()
	m.gitRoot = git.FindRoot(m.cwd)
	m.treeRefreshSignature = m.currentTreeSignature()
	return nil
}

func (m *Model) applyFilter() {
	if strings.TrimSpace(m.filter) == "" {
		m.rows = m.treeRowsToResultRows(m.treeRows)
		return
	}
	if m.filter != m.executedSearchQuery {
		m.rows = nil
		m.statusMessage = "Press Enter to run recursive search."
		return
	}
	if m.searchMode == SearchText {
		m.applyTextSearch()
	} else {
		m.applyFileSearch()
	}
}

func (m Model) hasSubmittedSearch() bool {
	query := strings.TrimSpace(m.filter)
	return query != "" && query == m.executedSearchQuery && !m.searchRunning
}

func (m *Model) applyFileSearch() {
	needle := strings.ToLower(strings.TrimSpace(m.filter))
	if needle == "" {
		m.rows = m.treeRowsToResultRows(m.treeRows)
		return
	}
	if len(needle) < 2 {
		m.rows = nil
		m.statusMessage = "Type at least 2 characters for recursive file search."
		return
	}
	m.ensureRecursiveRows()
	m.rows = m.rows[:0]
	for _, row := range m.recursiveRows {
		if navfs.MatchFileQuery(m.cwd, row.Entry.Path, needle) {
			m.rows = append(m.rows, row)
			if len(m.rows) >= navfs.MaxSearchResults {
				m.statusMessage = fmt.Sprintf("Showing first %d file matches.", navfs.MaxSearchResults)
				return
			}
		}
	}
	if len(m.rows) == 0 {
		m.statusMessage = "No file matches."
	}
}

func (m *Model) applyTextSearch() {
	opts := m.scanOptions()
	matches, err := navfs.SearchText(m.cwd, m.filter, m.cfg.PreviewMaxBytes, opts)
	if err != nil {
		m.statusMessage = err.Error()
	}
	m.rows = m.rows[:0]
	for _, match := range matches {
		m.rows = append(m.rows, ResultRow{Entry: match.Entry, Line: match.Line, Snippet: match.Snippet})
	}
	if len(m.rows) == 0 && strings.TrimSpace(m.filter) != "" {
		m.statusMessage = "No text matches."
	}
}

func (m *Model) ensureRecursiveRows() {
	if m.recursiveRoot == m.cwd && m.recursiveRows != nil {
		return
	}
	opts := m.scanOptions()
	matches, err := navfs.SearchFiles(m.cwd, "", opts)
	if err != nil {
		m.statusMessage = err.Error()
	}
	m.recursiveRows = m.recursiveRows[:0]
	for _, match := range matches {
		m.recursiveRows = append(m.recursiveRows, ResultRow{Entry: match.Entry})
	}
	m.recursiveRoot = m.cwd
}

func (m *Model) selected() (navfs.FileEntry, bool) {
	if len(m.rows) == 0 || m.selectedIndex < 0 || m.selectedIndex >= len(m.rows) {
		return navfs.FileEntry{}, false
	}
	return m.rows[m.selectedIndex].Entry, true
}

func (m *Model) selectedRow() (ResultRow, bool) {
	if len(m.rows) == 0 || m.selectedIndex < 0 || m.selectedIndex >= len(m.rows) {
		return ResultRow{}, false
	}
	return m.rows[m.selectedIndex], true
}

func (m *Model) clampSelection() {
	if len(m.rows) == 0 {
		m.selectedIndex = 0
		return
	}
	if m.selectedIndex < 0 {
		m.selectedIndex = 0
	}
	if m.selectedIndex >= len(m.rows) {
		m.selectedIndex = len(m.rows) - 1
	}
}

func (m *Model) selectPath(path string) {
	for i, row := range m.rows {
		if row.Entry.Path == path {
			m.selectedIndex = i
			return
		}
	}
	m.clampSelection()
}

func (m *Model) refreshPreview() {
	entry, ok := m.selected()
	if !ok {
		m.preview = navfs.Preview{Title: "empty", Content: "No entries."}
		m.previewViewport.SetContent(m.preview.Content)
		return
	}
	m.preview = navfs.BuildPreviewWithOptions(entry.Path, m.cfg.PreviewMaxBytes, m.scanOptions())
	if row, ok := m.selectedRow(); ok && row.Line > 0 {
		m.preview.Content = fmt.Sprintf("line %d: %s\n\n%s", row.Line, row.Snippet, m.preview.Content)
	}
	m.previewViewport.SetContent(m.renderPreviewContent())
	m.previewViewport.GotoTop()
}

func (m *Model) queuePreview() tea.Cmd {
	entry, ok := m.selected()
	if !ok {
		m.previewRequestID++
		m.preview = navfs.Preview{Title: "empty", Content: "No entries."}
		m.previewViewport.SetContent(m.preview.Content)
		return nil
	}
	row, _ := m.selectedRow()
	m.previewRequestID++
	id := m.previewRequestID
	path := entry.Path
	title := entry.Name
	m.preview = navfs.Preview{Title: title, Path: path, Content: "Loading preview..."}
	m.previewViewport.SetContent(m.preview.Content)
	m.previewViewport.GotoTop()
	return m.previewCmd(id, path, row)
}

func (m Model) previewCmd(id int, path string, row ResultRow) tea.Cmd {
	maxBytes := m.cfg.PreviewMaxBytes
	opts := m.scanOptions()
	return func() tea.Msg {
		start := perfNow()
		preview := navfs.BuildPreviewWithOptions(path, maxBytes, opts)
		if row.Line > 0 {
			preview.Content = fmt.Sprintf("line %d: %s\n\n%s", row.Line, row.Snippet, preview.Content)
		}
		content := m.renderPreviewContentFor(preview, path)
		perfLogDuration("preview.build", start, "path", path)
		return previewLoadedMsg{id: id, path: path, preview: preview, content: content}
	}
}

func (m *Model) applyPreviewLoaded(msg previewLoadedMsg) {
	if msg.id != m.previewRequestID {
		return
	}
	if entry, ok := m.selected(); !ok || entry.Path != msg.path {
		return
	}
	m.preview = msg.preview
	m.previewViewport.SetContent(msg.content)
	m.previewViewport.GotoTop()
}

func (m *Model) buildTreeRows() []TreeRow {
	rootInfo, err := os.Stat(m.cwd)
	if err != nil {
		return nil
	}
	root := navfs.NewEntry(m.cwd, rootInfo)
	root.Name = filepath.Base(m.cwd)
	if root.Name == "" {
		root.Name = m.cwd
	}
	rows := []TreeRow{{Entry: root, Depth: 0, Expanded: m.expandedDirs[m.cwd]}}
	if m.expandedDirs[m.cwd] {
		rows = m.appendChildren(rows, m.cwd, 1)
	}
	return rows
}

func (m *Model) appendChildren(rows []TreeRow, dir string, depth int) []TreeRow {
	children, err := navfs.ScanDir(dir, m.scanOptions())
	if err != nil {
		return rows
	}
	for _, child := range children {
		expanded := child.IsDir && m.expandedDirs[child.Path]
		rows = append(rows, TreeRow{Entry: child, Depth: depth, Expanded: expanded})
		if expanded {
			rows = m.appendChildren(rows, child.Path, depth+1)
		}
	}
	return rows
}

func (m Model) scanOptions() navfs.ScanOptions {
	return navfs.ScanOptions{
		ShowHidden:    m.cfg.ShowHidden,
		SortDirsFirst: m.cfg.SortDirsFirst,
		IgnoreNames:   m.ignoreNameSet(),
	}
}

func (m Model) ignoreNameSet() map[string]bool {
	if len(m.cfg.IgnoreNames) == 0 {
		return nil
	}
	ignored := make(map[string]bool, len(m.cfg.IgnoreNames))
	for _, name := range m.cfg.IgnoreNames {
		name = strings.TrimSpace(name)
		if name != "" {
			ignored[name] = true
		}
	}
	return ignored
}

func (m Model) treeRowsToResultRows(rows []TreeRow) []ResultRow {
	result := make([]ResultRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, ResultRow{Entry: row.Entry, Depth: row.Depth})
	}
	return result
}

func (m Model) rowForPath(path string) (TreeRow, bool) {
	if m.treeRowByPath != nil {
		row, ok := m.treeRowByPath[path]
		return row, ok
	}
	for _, row := range m.treeRows {
		if row.Entry.Path == path {
			return row, true
		}
	}
	return TreeRow{}, false
}

func (m *Model) rebuildTreeRowIndex() {
	if len(m.treeRows) == 0 {
		m.treeRowByPath = nil
		return
	}
	rows := make(map[string]TreeRow, len(m.treeRows))
	for _, row := range m.treeRows {
		rows[row.Entry.Path] = row
	}
	m.treeRowByPath = rows
}

func (m *Model) enterMode(mode Mode, prompt, value string) {
	m.mode = mode
	m.input = textinput.New()
	m.input.Prompt = prompt
	m.input.CharLimit = 512
	m.input.SetValue(value)
	m.input.Focus()
	m.input.CursorEnd()
}

func (m *Model) exitMode() {
	m.mode = ModeNormal
	m.input.Blur()
}

func (m *Model) setError(err error) {
	if err != nil {
		m.statusMessage = err.Error()
	}
}

func (m Model) cwdLabel() string {
	if m.gitRoot != "" {
		return "[git] " + git.Rel(m.gitRoot, m.cwd)
	}
	return m.cwd
}

func (m Model) selectedPathForStatus() string {
	entry, ok := m.selected()
	if !ok {
		return ""
	}
	if m.gitRoot != "" {
		return git.Rel(m.gitRoot, entry.Path)
	}
	return entry.Path
}

func defaultNameForCopy(src, dir string) string {
	name := filepath.Base(src)
	dst := filepath.Join(dir, name)
	if _, err := os.Lstat(dst); os.IsNotExist(err) {
		return dst
	}
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for i := 1; ; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s_copy%d%s", base, i, ext))
		if _, err := os.Lstat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}
