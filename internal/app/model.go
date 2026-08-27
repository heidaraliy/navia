package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/heidaraliy/navia/internal/config"
	navfs "github.com/heidaraliy/navia/internal/fs"
	"github.com/heidaraliy/navia/internal/gitview"
	"github.com/heidaraliy/navia/internal/syntax"
	"github.com/heidaraliy/navia/internal/textsafe"
)

const previewLimit = 1024 * 1024

type statusMsg struct {
	id      int
	changes []gitview.Change
	err     error
}
type summaryMsg struct {
	id     int
	counts gitview.Counts
	err    error
}
type diffMsg struct {
	id    int
	index int
	diff  gitview.FileDiff
	err   error
}
type editorMsg struct{ err error }
type searchMsg struct {
	id      int
	matches map[string]bool
	err     error
}

type Model struct {
	mode                        byte
	browseRoot                  string
	cfg                         config.Config
	navRows                     []navRow
	navSelected, navTop         int
	navPreview                  navfs.Preview
	navPreviewLines             []string
	navPreviewTop               int
	navPreviewID                int
	expanded                    map[string]bool
	navSearching                bool
	navSearchText               bool
	navQuery                    string
	navSearchID                 int
	navSearchLoading            bool
	historyOpen                 bool
	history                     []gitview.Commit
	historySelected             int
	historyOffset               int
	historyLoading              bool
	historyHasMore              bool
	historyID                   int
	diffRef                     string
	diffLabel                   string
	root                        string
	width, height               int
	leftWidth                   int
	dragging                    bool
	changes                     []gitview.Change
	selected, listTop, diffTop  int
	diff                        gitview.FileDiff
	diffLoading, summaryLoading bool
	requestID                   int
	statusRequestID             int
	summaryRequestID            int
	counts                      gitview.Counts
	err                         string
	status                      string
	sideBySide                  bool
	searching                   bool
	searchQuery                 string
	searchLoading               bool
	searchID                    int
	contentMatches              map[string]bool
	help                        bool
	fullscreen                  byte
	syntax                      syntax.Renderer
	editor                      string
}

func New(start string, cfg config.Config, diffMode bool) (Model, error) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return Model{}, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return Model{}, err
	}
	selectedPath := ""
	if !info.IsDir() {
		selectedPath, abs = abs, filepath.Dir(abs)
	}
	root, _ := gitview.Root(abs)
	if diffMode && root == "" {
		return Model{}, fmt.Errorf("diff mode requires a Git repository")
	}
	editor := cfg.Editor
	if editor == "" {
		editor = "nvim"
	}
	m := Model{mode: 'n', browseRoot: abs, root: root, cfg: cfg, expanded: map[string]bool{abs: true}, syntax: syntax.New(cfg.Theme), editor: editor, historyHasMore: true}
	if diffMode {
		m.mode, m.summaryLoading = 'd', true
	} else if err := m.rebuildNav(selectedPath); err != nil {
		return Model{}, err
	}
	return m, nil
}

func (m Model) Init() tea.Cmd {
	if m.mode == 'd' {
		return loadStatusRef(m.root, m.diffRef, 0)
	}
	return m.queueNavPreview()
}

func (m *Model) SetStatus(value string) { m.status = value }

func loadStatus(root string, id int) tea.Cmd {
	return func() tea.Msg { changes, err := gitview.Status(root); return statusMsg{id, changes, err} }
}
func loadStatusRef(root, ref string, id int) tea.Cmd {
	if ref == "" {
		return loadStatus(root, id)
	}
	return func() tea.Msg { changes, err := gitview.StatusCommit(root, ref); return statusMsg{id, changes, err} }
}
func loadSummary(root string, changes []gitview.Change, id int) tea.Cmd {
	return func() tea.Msg { counts, err := gitview.Aggregate(root, changes); return summaryMsg{id, counts, err} }
}
func loadSummaryRef(root, ref string, changes []gitview.Change, id int) tea.Cmd {
	if ref == "" {
		return loadSummary(root, changes, id)
	}
	return func() tea.Msg {
		counts, err := gitview.AggregateCommit(root, ref, changes)
		return summaryMsg{id, counts, err}
	}
}
func loadDiff(root string, change gitview.Change, id, index int) tea.Cmd {
	return func() tea.Msg {
		diff, err := gitview.Diff(root, change, previewLimit)
		return diffMsg{id, index, diff, err}
	}
}
func loadDiffRef(root, ref string, change gitview.Change, id, index int) tea.Cmd {
	if ref == "" {
		return loadDiff(root, change, id, index)
	}
	return func() tea.Msg {
		diff, err := gitview.DiffCommit(root, ref, change, previewLimit)
		return diffMsg{id, index, diff, err}
	}
}
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if next, cmd, handled := m.updateNavigatorMessage(msg); handled {
		return next, cmd
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.clampLayout()
	case statusMsg:
		if msg.id != m.statusRequestID {
			return m, nil
		}
		m.status = ""
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.err = ""
		selectedPath := m.selectedPath()
		m.changes = msg.changes
		m.selected = indexForPath(m.changes, selectedPath, m.selected)
		m.clampSelection()
		m.summaryLoading = true
		m.summaryRequestID++
		if len(m.changes) == 0 {
			m.diff = gitview.FileDiff{}
			m.diffLoading = false
			return m, loadSummaryRef(m.root, m.diffRef, m.changes, m.summaryRequestID)
		}
		return m, tea.Batch(m.queueDiff(), loadSummaryRef(m.root, m.diffRef, m.changes, m.summaryRequestID))
	case summaryMsg:
		if msg.id != m.summaryRequestID {
			return m, nil
		}
		m.summaryLoading = false
		if msg.err != nil {
			m.err = msg.err.Error()
		} else {
			m.counts = msg.counts
		}
	case diffMsg:
		if msg.id == m.requestID && msg.index == m.selected {
			m.diffLoading = false
			if msg.err != nil {
				m.err = msg.err.Error()
			} else {
				m.err = ""
				m.diff = msg.diff
				m.diffTop = 0
			}
		}
	case editorMsg:
		if msg.err != nil {
			m.err = "editor: " + msg.err.Error()
		} else {
			m.status = "Returned from editor; refreshing…"
		}
		return m, m.queueStatus()
	case searchMsg:
		if msg.id != m.searchID {
			return m, nil
		}
		m.searchLoading = false
		if msg.err != nil {
			m.err = "search: " + msg.err.Error()
		} else {
			m.err = ""
			m.contentMatches = msg.matches
			m.selected, m.listTop = 0, 0
			return m, m.queueDiff()
		}
	case tea.KeyMsg:
		return m.updateKey(msg)
	case tea.MouseMsg:
		return m.updateMouse(tea.MouseEvent(msg))
	}
	return m, nil
}

func (m Model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "ctrl+c" {
		return m, tea.Quit
	}
	if m.help {
		if key == "?" || key == "esc" || key == "q" || key == "enter" {
			m.help = false
		}
		return m, nil
	}
	if m.historyOpen {
		return m.updateHistory(msg)
	}
	if m.searching {
		switch key {
		case "esc":
			m.searching = false
			m.searchQuery = ""
			m.contentMatches = nil
			m.selected, m.listTop = 0, 0
			return m, m.queueDiff()
		case "enter":
			m.searching = false
			return m, m.queueContentSearch()
		case "backspace", "ctrl+h":
			runes := []rune(m.searchQuery)
			if len(runes) > 0 {
				m.searchQuery = string(runes[:len(runes)-1])
			}
			m.resetSearchSelection()
			return m, m.queueDiff()
		case "ctrl+u":
			m.searchQuery = ""
			m.resetSearchSelection()
			return m, m.queueDiff()
		}
		if msg.Type == tea.KeyRunes {
			m.searchQuery += string(msg.Runes)
			m.resetSearchSelection()
			return m, m.queueDiff()
		}
		return m, nil
	}
	switch key {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "up", "k":
		return m.moveSelection(-1)
	case "down", "j":
		return m.moveSelection(1)
	case "J", "shift+down":
		return m.moveSelection(max(1, m.listHeight()))
	case "K", "shift+up":
		return m.moveSelection(-max(1, m.listHeight()))
	case "ctrl+j", "ctrl+down":
		m.scrollDiff(max(1, m.diffHeight()-1))
	case "ctrl+k", "ctrl+up":
		m.scrollDiff(-max(1, m.diffHeight()-1))
	case "pgdown":
		m.scrollDiff(max(1, m.diffHeight()-1))
	case "pgup":
		m.scrollDiff(-max(1, m.diffHeight()-1))
	case "g":
		m.diffTop = 0
	case "G":
		m.diffTop = max(0, m.diffLineCount()-m.diffHeight())
	case "v":
		m.sideBySide = !m.sideBySide
		m.diffTop = 0
	case "r":
		return m, m.queueStatus()
	case "ctrl+o":
		return m, m.openEditor()
	case "enter":
		return m, m.openEditor()
	case "c":
		m.historyOpen = true
		m.historySelected = 0
		if len(m.history) == 0 {
			return m, m.queueHistory(false)
		}
		return m, nil
	case "esc":
		m.mode = 'n'
		m.fullscreen = 0
		return m, m.queueNavPreview()
	case "/":
		m.searching = true
		return m, nil
	case "?":
		m.help = true
		return m, nil
	case "F":
		if m.fullscreen == 'l' {
			m.fullscreen = 0
		} else {
			m.fullscreen = 'l'
		}
		m.clampSelection()
	case "f":
		if m.fullscreen == 'r' {
			m.fullscreen = 0
		} else {
			m.fullscreen = 'r'
		}
		m.scrollDiff(0)
	}
	return m, nil
}

func (m Model) updateMouse(msg tea.MouseEvent) (tea.Model, tea.Cmd) {
	if m.historyOpen {
		return m.updateHistoryMouse(msg)
	}
	if m.help {
		return m, nil
	}
	divider := m.leftWidth
	if m.fullscreen == 0 && msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress && abs(msg.X-divider) <= 1 {
		m.dragging = true
		return m, nil
	}
	if m.dragging && msg.Action == tea.MouseActionMotion {
		m.leftWidth = msg.X
		m.clampLayout()
		return m, nil
	}
	if msg.Action == tea.MouseActionRelease {
		m.dragging = false
		return m, nil
	}
	searchRow := topHeight + 1
	listVisible := m.fullscreen != 'r'
	if listVisible && msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress && (m.fullscreen == 'l' || msg.X < divider) && msg.Y == searchRow {
		m.searching = true
		return m, nil
	}
	firstFileRow := topHeight + 3
	if listVisible && msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress && (m.fullscreen == 'l' || msg.X < divider) && msg.Y >= firstFileRow && msg.Y < firstFileRow+m.listHeight() {
		index := m.listTop + msg.Y - firstFileRow
		if index >= 0 && index < len(m.visibleChanges()) {
			m.searching = false
			m.selected = index
			return m, m.queueDiff()
		}
	}
	if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown {
		delta := 3
		if msg.Button == tea.MouseButtonWheelUp {
			delta = -3
		}
		if m.fullscreen == 'l' || (m.fullscreen == 0 && msg.X < divider) {
			return m.moveSelection(delta)
		}
		m.scrollDiff(delta)
	}
	return m, nil
}

func (m Model) moveSelection(delta int) (tea.Model, tea.Cmd) {
	if len(m.visibleChanges()) == 0 {
		return m, nil
	}
	m.selected += delta
	m.clampSelection()
	return m, m.queueDiff()
}

func (m *Model) queueDiff() tea.Cmd {
	changes := m.visibleChanges()
	if len(changes) == 0 {
		m.diff = gitview.FileDiff{}
		m.diffLoading = false
		return nil
	}
	m.requestID++
	m.diffLoading = true
	m.diffTop = 0
	return loadDiffRef(m.root, m.diffRef, changes[m.selected], m.requestID, m.selected)
}

func (m *Model) queueContentSearch() tea.Cmd {
	if m.searchQuery == "" {
		return m.queueDiff()
	}
	m.searchID++
	m.searchLoading = true
	id, root, query := m.searchID, m.root, m.searchQuery
	changes := append([]gitview.Change(nil), m.changes...)
	return func() tea.Msg {
		var matches map[string]bool
		var err error
		if m.diffRef != "" {
			matches, err = gitview.SearchContentCommit(root, m.diffRef, query, changes)
		} else {
			matches, err = gitview.SearchContent(root, query, changes)
		}
		return searchMsg{id: id, matches: matches, err: err}
	}
}

func (m *Model) resetSearchSelection() {
	m.contentMatches = nil
	m.searchLoading = false
	m.searchID++
	m.selected, m.listTop = 0, 0
}

func (m *Model) queueStatus() tea.Cmd {
	m.statusRequestID++
	return loadStatusRef(m.root, m.diffRef, m.statusRequestID)
}

func (m *Model) scrollDiff(delta int) {
	m.diffTop += delta
	maxTop := max(0, m.diffLineCount()-m.diffHeight())
	if m.diffTop < 0 {
		m.diffTop = 0
	}
	if m.diffTop > maxTop {
		m.diffTop = maxTop
	}
}

func (m Model) openEditor() tea.Cmd {
	changes := m.visibleChanges()
	if len(changes) == 0 {
		return nil
	}
	change := changes[m.selected]
	args := strings.Fields(m.editor)
	if len(args) == 0 {
		args = []string{"nvim"}
	}
	path := filepath.Join(m.root, filepath.FromSlash(change.Path))
	cleanup := ""
	if change.Kind == gitview.Deleted || m.diffRef != "" {
		ref := "HEAD"
		if m.diffRef != "" {
			ref = m.diffRef
			if change.Kind == gitview.Deleted {
				ref += "^"
			}
		}
		data, err := exec.Command("git", "-C", m.root, "show", ref+":"+change.Path).Output()
		if err != nil {
			return func() tea.Msg { return editorMsg{err: err} }
		}
		tmp, err := os.CreateTemp("", "drift-*-"+filepath.Base(change.Path))
		if err != nil {
			return func() tea.Msg { return editorMsg{err: err} }
		}
		path, cleanup = tmp.Name(), tmp.Name()
		if _, err = tmp.Write(data); err == nil {
			err = tmp.Close()
		} else {
			_ = tmp.Close()
		}
		if err != nil {
			_ = os.Remove(cleanup)
			return func() tea.Msg { return editorMsg{err: err} }
		}
		args = append(args, "-R")
	}
	cmd := exec.Command(args[0], append(args[1:], path)...)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if cleanup != "" {
			_ = os.Remove(cleanup)
		}
		return editorMsg{err: err}
	})
}

func (m *Model) clampLayout() {
	if m.leftWidth == 0 {
		m.leftWidth = m.width * 30 / 100
	}
	if m.width < 64 {
		m.leftWidth = max(22, m.width/3)
		return
	}
	if m.leftWidth < 24 {
		m.leftWidth = 24
	}
	if m.leftWidth > m.width-40 {
		m.leftWidth = m.width - 40
	}
}
func (m *Model) clampSelection() {
	changes := m.visibleChanges()
	if len(changes) == 0 {
		m.selected, m.listTop = 0, 0
		return
	}
	if m.selected < 0 {
		m.selected = 0
	}
	if m.selected >= len(changes) {
		m.selected = len(changes) - 1
	}
	h := max(1, m.listHeight())
	if m.selected < m.listTop {
		m.listTop = m.selected
	}
	if m.selected >= m.listTop+h {
		m.listTop = m.selected - h + 1
	}
}
func (m Model) selectedPath() string {
	changes := m.visibleChanges()
	if len(changes) == 0 || m.selected >= len(changes) {
		return ""
	}
	return changes[m.selected].Path
}
func (m Model) listHeight() int { return max(3, m.height-10) }
func (m Model) diffHeight() int { return max(3, m.listHeight()+2) }
func (m Model) diffLineCount() int {
	if m.sideBySide {
		return len(m.diff.Side)
	}
	return len(m.diff.Lines)
}
func indexForPath(changes []gitview.Change, path string, fallback int) int {
	for i, c := range changes {
		if c.Path == path {
			return i
		}
	}
	if fallback >= len(changes) {
		return max(0, len(changes)-1)
	}
	return max(0, fallback)
}

func (m Model) visibleChanges() []gitview.Change {
	query := strings.ToLower(m.searchQuery)
	if query == "" {
		return m.changes
	}
	filtered := make([]gitview.Change, 0, len(m.changes))
	for _, change := range m.changes {
		if strings.Contains(strings.ToLower(change.Path), query) || m.contentMatches[change.Path] {
			filtered = append(filtered, change)
		}
	}
	return filtered
}
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func (m Model) Debug() string {
	return fmt.Sprintf("root=%s files=%d selected=%d", m.root, len(m.changes), m.selected)
}
func cleanPath(path string) string { return textsafe.Content(strings.ReplaceAll(path, "\t", " ")) }
