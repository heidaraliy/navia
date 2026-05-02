package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/heidaraliy/navia/internal/editor"
	navfs "github.com/heidaraliy/navia/internal/fs"
	"github.com/heidaraliy/navia/internal/git"
)

func (m Model) View() string {
	if m.width == 0 {
		return "Navia is starting..."
	}
	leftW, rightW := m.paneWidths()
	top := m.renderTop()
	main := m.renderMain(leftW, rightW)
	footer := m.renderFooter()
	body := lipgloss.JoinVertical(lipgloss.Left, top, main, footer)
	if m.mode == ModeHelp {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.renderHelp(), lipgloss.WithWhitespaceChars(" "))
	}
	if m.mode == ModeConfirmDelete || m.mode == ModeRename || m.mode == ModeNewFile || m.mode == ModeNewDir || m.mode == ModeGoToPath || m.mode == ModeDiffCommit || m.mode == ModeDiffConfirmRestore || m.mode == ModeDiffConfirmRemove {
		return body + "\n" + m.renderModal()
	}
	return body
}

func (m Model) renderMain(leftW, rightW int) string {
	if m.mode == ModeDiff || m.mode == ModeDiffCommit || m.mode == ModeDiffConfirmRestore || m.mode == ModeDiffConfirmRemove {
		return lipgloss.JoinHorizontal(lipgloss.Top, m.renderDiffList(leftW), m.renderDiffPane(rightW))
	}
	if m.treeHidden && m.activeBuffer() != nil {
		return m.renderRightPane(m.width)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, m.renderList(leftW), m.renderRightPane(rightW))
}

func (m Model) renderTop() string {
	label, meta := m.topModeLabel()
	row1 := label
	if meta != "" {
		if m.mode == ModeFilter || m.filter != "" {
			row1 += " " + m.styles.Highlight.Render(meta)
		} else {
			gap := max(1, m.width-lipgloss.Width(label)-lipgloss.Width(meta)-1)
			row1 += strings.Repeat(" ", gap) + m.styles.Dim.Render(meta)
		}
	}
	row2 := m.renderTabs(m.width)
	if row2 == "" {
		row2 = m.styles.Dim.Render(m.topContext())
	}
	return m.styles.TopBar.Width(m.width).Height(m.topHeight()).Render(clip(row1, m.width) + "\n" + clip(row2, m.width))
}

func (m Model) topModeLabel() (string, string) {
	mode := "TREE"
	fg := lipgloss.Color("230")
	bg := lipgloss.Color("39")
	if m.mode == ModeFilter || m.filter != "" {
		mode = "SEARCH"
		fg = lipgloss.Color("229")
		bg = lipgloss.Color("58")
	} else if m.mode == ModeHelp {
		mode = "HELP"
		bg = lipgloss.Color("147")
	} else if m.mode == ModeDiff || m.mode == ModeDiffCommit || m.mode == ModeDiffConfirmRestore || m.mode == ModeDiffConfirmRemove {
		mode = "DIFF"
		bg = lipgloss.Color("125")
	} else if m.focus == FocusEditor && m.activeBuffer() != nil {
		mode = "EDITOR"
		bg = lipgloss.Color("39")
	}
	if m.treeHidden && m.activeBuffer() != nil {
		mode += " ONLY"
	}
	label := lipgloss.NewStyle().Foreground(fg).Background(bg).Bold(true).Padding(0, 1).Render(mode)
	return label, m.topContext()
}

func (m Model) topContext() string {
	project := filepath.Base(m.cwd)
	path := m.cwd
	if m.gitRoot != "" {
		project = filepath.Base(m.gitRoot)
		path = gitPathLabel(m.gitRoot, m.cwd)
	}
	context := project + "  " + path
	if m.mode == ModeFilter || m.filter != "" {
		query := m.filter
		if query == "" {
			query = "type query"
		}
		context = "type:" + m.searchModeLabel() + "  query:" + query
	} else if m.mode == ModeDiff || m.mode == ModeDiffCommit || m.mode == ModeDiffConfirmRestore || m.mode == ModeDiffConfirmRemove {
		context = diffSummaryText(m.diffSummary)
	} else if buf := m.activeBuffer(); buf != nil {
		context = tabLabel(buf) + "  " + statusPath(buf.Path)
	}
	return context
}

func (m Model) renderTabs(width int) string {
	if width <= 0 || len(m.editorTabs) == 0 {
		return ""
	}
	indicatorW := 0
	if m.activeTab > 0 {
		indicatorW += 2
	}
	if m.activeTab < len(m.editorTabs)-1 {
		indicatorW += 2
	}
	available := max(0, width-indicatorW)
	start, end := m.visibleTabRange(available)
	parts := make([]string, 0, end-start+2)
	if start > 0 {
		parts = append(parts, m.styles.Dim.Render("‹ "))
	}
	for i := start; i < end; i++ {
		parts = append(parts, m.renderTab(i))
	}
	if end < len(m.editorTabs) {
		parts = append(parts, m.styles.Dim.Render(" ›"))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func (m Model) visibleTabRange(width int) (int, int) {
	if width <= 0 {
		return m.activeTab, m.activeTab
	}
	start, end := m.activeTab, m.activeTab+1
	used := m.tabWidth(m.activeTab)
	for {
		added := false
		if start > 0 && used+m.tabWidth(start-1) <= width {
			start--
			used += m.tabWidth(start)
			added = true
		}
		if end < len(m.editorTabs) && used+m.tabWidth(end) <= width {
			used += m.tabWidth(end)
			end++
			added = true
		}
		if !added {
			break
		}
	}
	return start, end
}

func (m Model) tabWidth(index int) int {
	if index < 0 || index >= len(m.editorTabs) {
		return 0
	}
	return lipgloss.Width(m.tabText(index)) + 5
}

func (m Model) tabText(index int) string {
	return fmt.Sprintf("%d:%s", index+1, tabLabel(m.editorTabs[index]))
}

func (m Model) renderTab(index int) string {
	active := index == m.activeTab
	focused := active && m.focus == FocusEditor
	style := lipgloss.NewStyle().MarginRight(1)
	text := "╭ " + m.tabText(index) + " ╮"
	switch {
	case focused:
		style = style.Foreground(lipgloss.Color("230")).Background(lipgloss.Color("24")).Bold(true)
	case active:
		style = style.Foreground(lipgloss.Color("81")).Bold(true)
	default:
		style = style.Foreground(lipgloss.Color("67"))
	}
	return style.Render(text)
}

func gitPathLabel(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." {
		return "."
	}
	return rel
}

func (m Model) searchModeLabel() string {
	if m.searchMode == SearchText {
		return "text"
	}
	return "files"
}

func (m Model) renderList(width int) string {
	height := m.listHeight()
	innerW := max(8, width-4)
	innerH := max(4, height-2)
	panel := m.panelStyle(FocusTree)
	if len(m.rows) == 0 {
		content := m.styles.TreePane.Width(innerW).Height(innerH).Render(m.styles.Dim.Render("No entries."))
		return panel.Width(width - 2).Height(height).Render(content)
	}
	start := m.selectedIndex - innerH/2
	if start < 0 {
		start = 0
	}
	if start+innerH > len(m.rows) {
		start = len(m.rows) - innerH
	}
	if start < 0 {
		start = 0
	}
	lines := make([]string, 0, innerH)
	treeFocused := m.focus == FocusTree
	for i := start; i < len(m.rows) && len(lines) < innerH; i++ {
		result := m.rows[i]
		entry := result.Entry
		row, ok := m.rowForPath(entry.Path)
		if !ok {
			row = TreeRow{Entry: entry, Depth: result.Depth}
		}
		name := entry.Name
		if entry.IsDir {
			name += "/"
		}
		indent := strings.Repeat(" ", row.Depth)
		icon := "  "
		if entry.IsDir {
			if row.Expanded {
				icon = "▾ "
			} else {
				icon = "▸ "
			}
		}
		label := indent + icon + name
		if result.Line > 0 {
			label = fmt.Sprintf("%s:%d %s", name, result.Line, result.Snippet)
		}
		line := truncate(label, innerW)
		if entry.IsDir {
			if treeFocused {
				line = m.styles.Dir.Render(line)
			} else {
				line = m.styles.DirDim.Render(line)
			}
		} else if !treeFocused {
			line = m.styles.TreeDim.Render(line)
		}
		if i == m.selectedIndex {
			if treeFocused {
				line = m.styles.Selected.Width(innerW).Render(line)
			} else {
				line = m.styles.SelectedBlurred.Width(innerW).Render(line)
			}
		}
		lines = append(lines, line)
	}
	content := m.styles.TreePane.Width(innerW).Height(innerH).Render(strings.Join(lines, "\n"))
	return panel.Width(width - 2).Height(height).Render(content)
}

func (m Model) renderDiffList(width int) string {
	height := m.listHeight()
	innerW := max(8, width-4)
	innerH := max(4, height-2)
	panel := m.panelStyle(FocusTree)
	if len(m.diffChanges) == 0 {
		content := m.styles.TreePane.Width(innerW).Height(innerH).Render(m.styles.Dim.Render("No modified or untracked files."))
		return panel.Width(width - 2).Height(height).Render(content)
	}
	start := m.diffSelectedIndex - innerH/2
	if start < 0 {
		start = 0
	}
	if start+innerH > len(m.diffChanges) {
		start = len(m.diffChanges) - innerH
	}
	if start < 0 {
		start = 0
	}
	lines := make([]string, 0, innerH)
	for i := start; i < len(m.diffChanges) && len(lines) < innerH; i++ {
		change := m.diffChanges[i]
		label := fmt.Sprintf("%s %-9s %s", diffStatusText(change), diffKindLabel(change), change.Path)
		if change.OldPath != "" {
			label = fmt.Sprintf("%s %-9s %s -> %s", diffStatusText(change), diffKindLabel(change), change.OldPath, change.Path)
		}
		line := truncate(label, innerW)
		switch change.Kind {
		case git.ChangeAdded, git.ChangeUntracked:
			line = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render(line)
		case git.ChangeDeleted:
			line = lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Render(line)
		default:
			line = m.styles.TreePane.Render(line)
		}
		if i == m.diffSelectedIndex {
			line = m.styles.Selected.Width(innerW).Render(line)
		}
		lines = append(lines, line)
	}
	content := m.styles.TreePane.Width(innerW).Height(innerH).Render(strings.Join(lines, "\n"))
	return panel.Width(width - 2).Height(height).Render(content)
}

func (m Model) renderDiffPane(width int) string {
	height := m.listHeight()
	innerW := max(8, width-4)
	innerH := max(4, height-2)
	title := "Diff"
	if change, ok := m.selectedDiffChange(); ok {
		title = change.Path
	}
	header := m.styles.Highlight.Render(truncate(title, innerW))
	content := m.renderDiffContent()
	body := m.styles.Pane.Width(innerW).Height(innerH).Render(header + "\n" + content)
	return m.panelStyle(FocusEditor).Width(width - 2).Height(height).Render(body)
}

func (m Model) renderDiffContent() string {
	content := m.diffViewport.View()
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		switch diffLineStyle(line) {
		case "add":
			lines[i] = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render(line)
		case "remove":
			lines[i] = lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Render(line)
		case "hunk":
			lines[i] = m.styles.Highlight.Render(line)
		case "header":
			lines[i] = m.styles.Dim.Render(line)
		default:
			lines[i] = line
		}
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderRightPane(width int) string {
	if buf := m.activeBuffer(); buf != nil {
		return m.renderEditor(width, buf)
	}
	return m.renderPreview(width)
}

func (m Model) renderPreview(width int) string {
	height := m.listHeight()
	innerW := max(8, width-4)
	innerH := max(4, height-2)
	panel := m.panelStyle(FocusEditor)
	if m.shouldRenderIdleBrand() {
		body := m.styles.Pane.Width(innerW).Height(innerH).Render(m.renderIdleBrand(innerW, innerH))
		return panel.Width(width - 2).Height(height).Render(body)
	}
	title := m.preview.Title
	if title == "" {
		title = filepath.Base(m.selectedPathForStatus())
	}
	header := m.styles.Highlight.Render(title)
	content := m.previewViewport.View()
	if m.preview.Kind == navfs.PreviewText {
		content = m.highlightPreviewContent(content)
	}
	body := m.styles.Pane.Width(innerW).Height(innerH).Render(header + "\n" + content)
	return panel.Width(width - 2).Height(height).Render(body)
}

func (m Model) highlightPreviewContent(content string) string {
	entry, ok := m.selected()
	if !ok {
		return content
	}
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = m.syntax.HighlightLine(entry.Path, line)
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderEditor(width int, buf *editor.Buffer) string {
	height := m.listHeight()
	innerW := max(8, width-4)
	innerH := max(4, height-2)
	active := m.activeBuffer()
	title := ""
	editorFocused := m.focus == FocusEditor
	if active != nil {
		path := statusPath(active.Path)
		maxPath := max(0, innerW-lipgloss.Width(active.Name)-1)
		if editorFocused {
			title = m.styles.Highlight.Render(active.Name) + " " + m.styles.Dim.Render(clip(path, maxPath))
		} else {
			title = m.styles.TreeDim.Render(active.Name + " " + clip(path, maxPath))
		}
	}
	lines := buf.VisibleHighlighted(innerW, innerH-1, func(path, line string) string {
		return m.syntax.HighlightLineWithSearch(path, line, buf.SearchQuery())
	})
	if !editorFocused {
		for i, line := range lines {
			lines[i] = m.styles.TreeDim.Render(line)
		}
	}
	content := title + "\n" + strings.Join(lines, "\n")
	body := lipgloss.NewStyle().Width(innerW).Height(innerH).Render(content)
	return m.panelStyle(FocusEditor).Width(width - 2).Height(height).Render(body)
}

func (m Model) panelStyle(pane FocusPane) lipgloss.Style {
	if m.focus == pane {
		return m.styles.Focused
	}
	return m.styles.Blurred
}

func (m Model) shouldRenderIdleBrand() bool {
	entry, ok := m.selected()
	return ok && entry.Path == m.cwd && len(m.editorTabs) == 0 && m.filter == ""
}

func (m Model) renderIdleBrand(width, height int) string {
	logo := []string{
		"                  ▀▀       ",
		"████▄  ▀▀█▄ ██ ██ ██   ▀▀█▄",
		"██ ██ ▄█▀██ ██▄██ ██  ▄█▀██",
		"██ ██ ▀█▄██  ▀█▀  ██▄ ▀█▄██",
	}
	blockW := 29
	lines := make([]string, 0, len(logo))
	for _, line := range logo {
		lines = append(lines, m.styles.Brand.Render(center(line, blockW)))
	}
	lines = append(lines, "")
	lines = append(lines, m.styles.Highlight.Render(center("A microIDE in your terminal.", blockW)))
	lines = append(lines, m.styles.Dim.Render(center("github.com/heidaraliy/navia", blockW)))
	content := strings.Join(lines, "\n")
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
}

func (m Model) renderFooter() string {
	status := m.statusMessage
	right := ""
	if buf := m.activeBuffer(); buf != nil {
		right = fmt.Sprintf("%s  Lines:%d  %d:%d", buf.Mode.String(), len(buf.Lines), buf.CursorLine(), buf.CursorCol())
		if cmd := buf.CommandLine(); cmd != "" {
			status = cmd
		}
	} else if m.mode == ModeDiff || m.mode == ModeDiffCommit || m.mode == ModeDiffConfirmRestore || m.mode == ModeDiffConfirmRemove {
		right = fmt.Sprintf("Files:%d", len(m.diffChanges))
	} else {
		right = fmt.Sprintf("Rows:%d", len(m.rows))
	}
	if status == "" {
		if m.mode == ModeDiff || m.mode == ModeDiffCommit || m.mode == ModeDiffConfirmRestore || m.mode == ModeDiffConfirmRemove {
			status = "Esc tree  s stage  u unstage  R restore  D rm  c commit  p push  r refresh"
		} else if m.activeBuffer() != nil && m.focus == FocusEditor {
			status = ":w save  :q close  :bn/:bp tabs  :bl list  ctrl+o/i jumps  gd/gr"
		} else {
			status = "q quit  ? help  D diff  enter/l expand  h collapse  / search  c edit  e external"
		}
	}
	cmd := m.lastCommandHint
	if cmd != "" {
		status += " | " + m.styles.Command.Render(cmd)
	}
	statusW := max(0, m.width-lipgloss.Width(right)-3)
	status = clip(status, statusW)
	gap := max(1, m.width-lipgloss.Width(status)-lipgloss.Width(right)-2)
	line := status + strings.Repeat(" ", gap) + m.styles.Dim.Render(right)
	return m.styles.Footer.Width(m.width).Render(line)
}

func (m Model) renderModal() string {
	switch m.mode {
	case ModeConfirmDelete:
		name := m.pendingDelete.Name
		return m.styles.Modal.Render("Safe delete `" + name + "`?\n\nPress y to move it to .navia-trash.\nPress Esc to cancel.")
	case ModeRename:
		return m.styles.Modal.Render("Rename\n\n" + m.input.View())
	case ModeNewFile:
		return m.styles.Modal.Render("Create new file\n\n" + m.input.View())
	case ModeNewDir:
		return m.styles.Modal.Render("Create new directory\n\n" + m.input.View())
	case ModeGoToPath:
		return m.styles.Modal.Render("Go to path\n\n" + m.input.View())
	case ModeDiffCommit:
		return m.styles.Modal.Render("Commit changes\n\n" + m.input.View())
	case ModeDiffConfirmRestore:
		return m.styles.Modal.Render("Restore `" + m.pendingDiffAction.Path + "`?\n\nThis discards working tree changes or removes an untracked file.\nPress y to continue.\nPress Esc to cancel.")
	case ModeDiffConfirmRemove:
		return m.styles.Modal.Render("Remove `" + m.pendingDiffAction.Path + "`?\n\nTracked files use git rm. Untracked files are deleted from disk.\nPress y to continue.\nPress Esc to cancel.")
	default:
		return ""
	}
}

func (m Model) renderHelp() string {
	content := helpContent()
	help := m.helpViewport
	if help.Width == 0 {
		help.Width = min(84, m.width-8)
		help.Height = max(8, m.height-6)
	}
	help.SetContent(content)
	return m.styles.Modal.Width(min(90, m.width-6)).Height(min(m.height-4, max(10, help.Height+2))).Render(help.View())
}

func helpContent() string {
	sections := []string{
		helpSection("Global", [][2]string{
			{"q / ctrl+c", "quit"},
			{"?", "toggle help"},
			{"esc", "cancel current mode"},
		}),
		helpSection("Tree", [][2]string{
			{"up/k, down/j", "move selection"},
			{"enter/l", "expand directory or open search result"},
			{"backspace/h", "collapse or jump to parent"},
			{"/", "recursive search"},
			{"g", "go to path"},
			{"c", "open selected file in Navia editor"},
			{"e", "open selected file externally"},
			{"D", "open diff mode"},
		}),
		helpSection("Diff", [][2]string{
			{"up/k, down/j", "move changed file selection"},
			{"ctrl+u / ctrl+d", "scroll diff"},
			{"s / u", "stage / unstage selected file"},
			{"R / D", "restore / remove selected file"},
			{"c / p", "commit / push current branch"},
			{"r / esc", "refresh / return to tree"},
		}),
		helpSection("File Operations", [][2]string{
			{"r", "rename"},
			{"n / N", "new file / directory"},
			{"y / x / p", "copy / cut / paste"},
			{"d", "safe delete"},
		}),
		helpSection("Search", [][2]string{
			{"tab", "toggle file/text search"},
			{"enter", "run search or open result"},
			{"esc", "clear search"},
		}),
		helpSection("Editor Normal", [][2]string{
			{"i/a/I/A/o/O", "enter insert"},
			{"h/j/k/l, w/b/e", "move cursor"},
			{"gg/G, :number", "jump"},
			{"u / ctrl+r", "undo / redo"},
			{"gd / gr", "definition / references"},
		}),
		helpSection("Editor Insert/Visual", [][2]string{
			{"esc", "return to normal"},
			{"v / V", "visual / visual line"},
			{"y / d / p", "yank / delete / paste"},
		}),
		helpSection("Editor Tabs", [][2]string{
			{":bn / :bp", "next / previous tab"},
			{":bl", "list tabs"},
			{":bd / :bd!", "close tab"},
			{"gt / gT", "next / previous tab"},
		}),
		helpSection("Windows And History", [][2]string{
			{"ctrl+w h/l", "focus tree / editor"},
			{"ctrl+w w", "focus tree"},
			{"ctrl+w o", "toggle editor-only view"},
			{"ctrl+o / ctrl+i", "jump back / forward"},
		}),
		helpSection("Editor Commands", [][2]string{
			{":w / :w!", "save / force save"},
			{":q / :q!", "close / force close"},
			{":wq", "save and close"},
			{":qa / :qa!", "quit all / force quit"},
			{":e path", "open file"},
		}),
	}
	return "Navia\n\n" + strings.Join(sections, "\n\n")
}

func helpSection(title string, rows [][2]string) string {
	lines := []string{title}
	for _, row := range rows {
		lines = append(lines, fmt.Sprintf("  %-16s %s", row[0], row[1]))
	}
	return strings.Join(lines, "\n")
}

func (m Model) listHeight() int {
	height := m.height - m.topHeight() - 3
	if height < 8 {
		height = 8
	}
	return height
}

func (m Model) topHeight() int {
	return 2
}

func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	if width <= 1 {
		return string(runes[:width])
	}
	return string(runes[:width-1]) + "…"
}

func clip(s string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	return string(runes[:width])
}

func center(s string, width int) string {
	if lipgloss.Width(s) >= width {
		return truncate(s, width)
	}
	left := (width - lipgloss.Width(s)) / 2
	return strings.Repeat(" ", left) + s
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
