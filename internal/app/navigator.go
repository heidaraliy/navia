package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	navfs "github.com/heidaraliy/navia/internal/fs"
	"github.com/heidaraliy/navia/internal/gitview"
	"github.com/heidaraliy/navia/internal/textsafe"
)

type navRow struct {
	entry   navfs.FileEntry
	depth   int
	line    int
	snippet string
}

type navPreviewMsg struct {
	id      int
	path    string
	preview navfs.Preview
	lines   []string
}
type navSearchMsg struct {
	id   int
	rows []navRow
	err  error
}
type navEditorMsg struct{ err error }

func (m *Model) rebuildNav(selectedPath string) error {
	info, err := os.Stat(m.browseRoot)
	if err != nil {
		return err
	}
	root := navfs.NewEntry(m.browseRoot, info)
	root.Name = filepath.Base(m.browseRoot)
	m.navRows = []navRow{{entry: root}}
	if m.expanded[m.browseRoot] {
		m.appendNavChildren(m.browseRoot, 1)
	}
	if selectedPath != "" {
		for i := range m.navRows {
			if m.navRows[i].entry.Path == selectedPath {
				m.navSelected = i
				break
			}
		}
	}
	m.clampNavSelection()
	return nil
}

func (m *Model) appendNavChildren(dir string, depth int) {
	children, err := navfs.ScanDir(dir, m.navScanOptions())
	if err != nil {
		return
	}
	for _, child := range children {
		m.navRows = append(m.navRows, navRow{entry: child, depth: depth})
		if child.IsDir && m.expanded[child.Path] {
			m.appendNavChildren(child.Path, depth+1)
		}
	}
}

func (m Model) navScanOptions() navfs.ScanOptions {
	ignored := make(map[string]bool, len(m.cfg.IgnoreNames))
	for _, name := range m.cfg.IgnoreNames {
		ignored[strings.TrimSpace(name)] = true
	}
	return navfs.ScanOptions{ShowHidden: m.cfg.ShowHidden, SortDirsFirst: m.cfg.SortDirsFirst, IgnoreNames: ignored}
}

func (m Model) selectedNav() (navRow, bool) {
	if m.navSelected < 0 || m.navSelected >= len(m.navRows) {
		return navRow{}, false
	}
	return m.navRows[m.navSelected], true
}

func (m *Model) clampNavSelection() {
	if len(m.navRows) == 0 {
		m.navSelected, m.navTop = 0, 0
		return
	}
	if m.navSelected < 0 {
		m.navSelected = 0
	}
	if m.navSelected >= len(m.navRows) {
		m.navSelected = len(m.navRows) - 1
	}
	h := max(1, m.listHeight())
	if m.navSelected < m.navTop {
		m.navTop = m.navSelected
	}
	if m.navSelected >= m.navTop+h {
		m.navTop = m.navSelected - h + 1
	}
}

func (m *Model) moveNav(delta int) tea.Cmd {
	if len(m.navRows) == 0 {
		return nil
	}
	m.navSelected += delta
	m.clampNavSelection()
	return m.queueNavPreview()
}

func (m *Model) queueNavPreview() tea.Cmd {
	row, ok := m.selectedNav()
	m.navPreviewID++
	id := m.navPreviewID
	if !ok {
		m.navPreviewLines = []string{"No entries."}
		return nil
	}
	path, maxBytes, opts, renderer := row.entry.Path, m.cfg.PreviewMaxBytes, m.navScanOptions(), m.syntax
	m.navPreviewLines, m.navPreviewTop = []string{"Loading preview…"}, 0
	return func() tea.Msg {
		preview := navfs.BuildPreviewWithOptions(path, maxBytes, opts)
		content := textsafe.Multiline(preview.Content)
		lines := strings.Split(content, "\n")
		if preview.Kind == navfs.PreviewText {
			limit := min(len(lines), 400)
			lines = lines[:limit]
			for i, line := range lines {
				lines[i] = renderer.HighlightLine(path, line)
			}
		}
		return navPreviewMsg{id: id, path: path, preview: preview, lines: lines}
	}
}

func (m Model) updateNavigatorMessage(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case historyMsg:
		if msg.id == m.historyID {
			m.historyLoading = false
			if msg.err != nil {
				m.err = msg.err.Error()
			} else if msg.append {
				m.history = append(m.history, msg.commits...)
			} else {
				m.history = msg.commits
			}
			m.historyHasMore = len(msg.commits) == 50
		}
		return m, nil, true
	case navPreviewMsg:
		if msg.id == m.navPreviewID {
			if row, ok := m.selectedNav(); ok && row.entry.Path == msg.path {
				m.navPreview, m.navPreviewLines, m.navPreviewTop = msg.preview, msg.lines, 0
			}
		}
		return m, nil, true
	case navSearchMsg:
		if msg.id == m.navSearchID {
			m.navSearchLoading = false
			if msg.err != nil {
				m.err = msg.err.Error()
			} else {
				m.navRows, m.navSelected, m.navTop = msg.rows, 0, 0
			}
			return m, m.queueNavPreview(), true
		}
		return m, nil, true
	case navEditorMsg:
		if msg.err != nil {
			m.err = "editor: " + msg.err.Error()
			return m, nil, true
		}
		selected := ""
		if row, ok := m.selectedNav(); ok {
			selected = row.entry.Path
		}
		if err := m.rebuildNav(selected); err != nil {
			m.err = err.Error()
		}
		m.status = "Returned from editor."
		return m, m.queueNavPreview(), true
	case tea.WindowSizeMsg:
		if m.mode == 'n' {
			m.width, m.height = msg.Width, msg.Height
			m.clampLayout()
			m.clampNavSelection()
			return m, nil, true
		}
	case tea.KeyMsg:
		if m.mode == 'n' {
			next, cmd := m.updateNavKey(msg)
			return next, cmd, true
		}
	case tea.MouseMsg:
		if m.mode == 'n' {
			next, cmd := m.updateNavMouse(tea.MouseEvent(msg))
			return next, cmd, true
		}
	}
	return m, nil, false
}

func (m Model) updateNavKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if m.help {
		if key == "?" || key == "esc" || key == "q" || key == "enter" {
			m.help = false
		}
		return m, nil
	}
	if m.navSearching {
		switch key {
		case "esc":
			m.navSearching, m.navQuery = false, ""
			_ = m.rebuildNav("")
			return m, m.queueNavPreview()
		case "tab":
			m.navSearchText = !m.navSearchText
			return m, nil
		case "enter":
			m.navSearching = false
			return m, m.queueNavSearch()
		case "backspace", "ctrl+h":
			r := []rune(m.navQuery)
			if len(r) > 0 {
				m.navQuery = string(r[:len(r)-1])
			}
			return m, nil
		case "ctrl+u":
			m.navQuery = ""
			return m, nil
		case " ":
			m.navQuery += " "
			return m, nil
		}
		if msg.Type == tea.KeyRunes {
			m.navQuery += string(msg.Runes)
		}
		return m, nil
	}
	switch key {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "up", "k":
		return m, m.moveNav(-1)
	case "down", "j":
		return m, m.moveNav(1)
	case "J", "shift+down":
		return m, m.moveNav(max(1, m.listHeight()))
	case "K", "shift+up":
		return m, m.moveNav(-max(1, m.listHeight()))
	case "ctrl+j", "ctrl+down", "pgdown":
		m.scrollNavPreview(max(1, m.diffHeight()-1))
	case "ctrl+k", "ctrl+up", "pgup":
		m.scrollNavPreview(-max(1, m.diffHeight()-1))
	case "enter":
		return m.openNavSelection()
	case "right", "l":
		return m.expandNav()
	case "left", "h", "backspace":
		return m.collapseNav()
	case "/":
		m.navSearching = true
	case "?":
		m.help = true
	case "D":
		start := m.browseRoot
		if row, ok := m.selectedNav(); ok {
			start = row.entry.Path
			if !row.entry.IsDir {
				start = filepath.Dir(start)
			}
		}
		m.root, _ = gitview.Root(start)
		if m.root == "" {
			m.err = "Diff mode requires a Git repository."
			return m, nil
		}
		m.mode, m.fullscreen, m.summaryLoading = 'd', 0, true
		return m, m.queueStatus()
	case "F":
		if m.fullscreen == 'l' {
			m.fullscreen = 0
		} else {
			m.fullscreen = 'l'
		}
	case "f":
		if m.fullscreen == 'r' {
			m.fullscreen = 0
		} else {
			m.fullscreen = 'r'
		}
	}
	return m, nil
}

func (m *Model) expandNav() (tea.Model, tea.Cmd) {
	row, ok := m.selectedNav()
	if !ok || !row.entry.IsDir || m.expanded[row.entry.Path] {
		return *m, nil
	}
	m.expanded[row.entry.Path] = true
	_ = m.rebuildNav(row.entry.Path)
	return *m, m.queueNavPreview()
}

func (m *Model) openNavSelection() (tea.Model, tea.Cmd) {
	row, ok := m.selectedNav()
	if !ok {
		return *m, nil
	}
	if row.entry.IsDir {
		m.expanded[row.entry.Path] = !m.expanded[row.entry.Path]
		_ = m.rebuildNav(row.entry.Path)
		return *m, m.queueNavPreview()
	}
	args := strings.Fields(m.editor)
	if len(args) == 0 {
		args = []string{"nvim"}
	}
	cmd := exec.Command(args[0], append(args[1:], row.entry.Path)...)
	return *m, tea.ExecProcess(cmd, func(err error) tea.Msg { return navEditorMsg{err} })
}

func (m *Model) collapseNav() (tea.Model, tea.Cmd) {
	row, ok := m.selectedNav()
	if !ok {
		return *m, nil
	}
	if row.entry.IsDir && m.expanded[row.entry.Path] {
		m.expanded[row.entry.Path] = false
		_ = m.rebuildNav(row.entry.Path)
		return *m, m.queueNavPreview()
	}
	parent := filepath.Dir(row.entry.Path)
	for i := range m.navRows {
		if m.navRows[i].entry.Path == parent {
			m.navSelected = i
			return *m, m.queueNavPreview()
		}
	}
	if row.entry.Path == m.browseRoot {
		parent = filepath.Dir(m.browseRoot)
		if parent != m.browseRoot {
			old := m.browseRoot
			m.browseRoot = parent
			m.expanded = map[string]bool{parent: true, old: true}
			_ = m.rebuildNav(old)
		}
	}
	return *m, m.queueNavPreview()
}

func (m *Model) queueNavSearch() tea.Cmd {
	query := strings.TrimSpace(m.navQuery)
	if query == "" {
		_ = m.rebuildNav("")
		return m.queueNavPreview()
	}
	m.navSearchID++
	id, root, opts, maxBytes, textMode := m.navSearchID, m.browseRoot, m.navScanOptions(), m.cfg.PreviewMaxBytes, m.navSearchText
	m.navSearchLoading = true
	return func() tea.Msg {
		var matches []navfs.SearchMatch
		var err error
		if textMode {
			matches, err = navfs.SearchText(root, query, maxBytes, opts)
		} else {
			matches, err = navfs.SearchFiles(root, query, opts)
		}
		rows := make([]navRow, 0, len(matches))
		for _, match := range matches {
			rows = append(rows, navRow{entry: match.Entry, line: match.Line, snippet: match.Snippet})
		}
		return navSearchMsg{id: id, rows: rows, err: err}
	}
}

func (m *Model) scrollNavPreview(delta int) {
	m.navPreviewTop += delta
	maxTop := max(0, len(m.navPreviewLines)-m.diffHeight())
	if m.navPreviewTop < 0 {
		m.navPreviewTop = 0
	}
	if m.navPreviewTop > maxTop {
		m.navPreviewTop = maxTop
	}
}

func (m Model) updateNavMouse(msg tea.MouseEvent) (tea.Model, tea.Cmd) {
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
	firstRow := topHeight + 3
	if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress && (m.fullscreen == 'l' || (m.fullscreen == 0 && msg.X < divider)) && msg.Y >= firstRow && msg.Y < firstRow+m.listHeight() {
		index := m.navTop + msg.Y - firstRow
		if index >= 0 && index < len(m.navRows) {
			m.navSelected = index
			return m, m.queueNavPreview()
		}
	}
	if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown {
		delta := 3
		if msg.Button == tea.MouseButtonWheelUp {
			delta = -3
		}
		if m.fullscreen == 'l' || (m.fullscreen == 0 && msg.X < divider) {
			return m, m.moveNav(delta)
		}
		m.scrollNavPreview(delta)
	}
	return m, nil
}

func (m Model) renderNavigator() string {
	if m.help {
		return m.renderNavHelp()
	}
	header := m.header()
	var body string
	switch m.fullscreen {
	case 'l':
		body = m.renderNavList(m.width)
	case 'r':
		body = m.renderNavPreview(m.width)
	default:
		body = lipgloss.JoinHorizontal(lipgloss.Top, m.renderNavList(m.leftWidth), m.renderNavPreview(m.width-m.leftWidth))
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, body)
}

func (m Model) renderNavList(width int) string {
	paneWidth, w, h := max(10, width), max(8, width-2), m.listHeight()
	lines := make([]string, 0, h)
	for i := m.navTop; i < len(m.navRows) && len(lines) < h; i++ {
		row := m.navRows[i]
		icon := "  "
		if row.entry.IsDir {
			if m.expanded[row.entry.Path] {
				icon = "▾ "
			} else {
				icon = "▸ "
			}
		}
		label := strings.Repeat("  ", row.depth) + icon + row.entry.Name
		if row.line > 0 {
			label = fmt.Sprintf("%s:%d  %s", row.entry.Name, row.line, row.snippet)
		}
		line := fit(label, w)
		if row.entry.IsDir {
			line = lipgloss.NewStyle().Foreground(accent).Render(line)
		}
		if i == m.navSelected {
			line = selectedStyle.Width(w).Render(line)
		}
		lines = append(lines, line)
	}
	for len(lines) < h {
		lines = append(lines, strings.Repeat(" ", w))
	}
	title := fmt.Sprintf("Files (%d)", len(m.navRows))
	search := " Find a file…"
	if m.navSearching || m.navQuery != "" {
		kind := "files"
		if m.navSearchText {
			kind = "text"
		}
		search = " / [" + kind + "] " + m.navQuery
		if m.navSearching {
			search += "█"
		} else if m.navSearchLoading {
			search += " searching…"
		}
	}
	rows := []string{paneTop(title, "", paneWidth, border, false), paneRow(dim.Render(fit(search, w)), paneWidth, border, false), paneRule(paneWidth, border, false)}
	for _, line := range lines {
		rows = append(rows, paneRow(line, paneWidth, border, false))
	}
	rows = append(rows, paneBottom(paneWidth, border, false))
	return strings.Join(rows, "\n")
}

func (m Model) renderNavPreview(width int) string {
	paneWidth, w, h := max(14, width), max(12, width-2), m.diffHeight()
	title := "Preview"
	if row, ok := m.selectedNav(); ok {
		title = truncateLeft(row.entry.Path, max(8, w-20))
	}
	stats := ""
	if m.navPreview.Size > 0 {
		stats = dim.Render(navfs.FormatSize(m.navPreview.Size))
	}
	end := min(len(m.navPreviewLines), m.navPreviewTop+h)
	lines := append([]string(nil), m.navPreviewLines[m.navPreviewTop:end]...)
	for i := range lines {
		lines[i] = fit(lines[i], w)
	}
	for len(lines) < h {
		lines = append(lines, strings.Repeat(" ", w))
	}
	rows := []string{paneTop(title, stats, paneWidth, border, false)}
	for _, line := range lines {
		rows = append(rows, paneRow(line, paneWidth, border, false))
	}
	rows = append(rows, paneBottom(paneWidth, border, false))
	return strings.Join(rows, "\n")
}

func (m Model) renderNavHelp() string {
	bindings := [][2]string{{"Search files or text", "/ then Tab"}, {"Select file", "j/k or ↑/↓"}, {"Page file list", "J/K or ⇧↑/↓"}, {"Page preview", "Ctrl-j/k or PgUp/Dn"}, {"Expand directory", "l or →"}, {"Collapse / parent", "h or ←"}, {"Open file in editor", "Enter"}, {"Open Git diff", "D"}, {"Fullscreen panes", "F / f"}, {"Quit", "q"}}
	rows := []string{lipgloss.NewStyle().Bold(true).Foreground(accent).Render("Navia keybinds"), ""}
	for _, b := range bindings {
		rows = append(rows, dim.Render(fit(b[0], 27))+b[1])
	}
	rows = append(rows, "", dim.Render("Press ?, Esc, Enter, or q to close."))
	modal := lipgloss.NewStyle().Border(lipgloss.ThickBorder()).BorderForeground(accent).Padding(1, 3).Render(strings.Join(rows, "\n"))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal)
}
