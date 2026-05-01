package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	if m.width == 0 {
		return "Navia is starting..."
	}
	leftW, rightW := m.paneWidths()
	top := m.renderTop()
	main := lipgloss.JoinHorizontal(lipgloss.Top, m.renderList(leftW), m.renderPreview(rightW))
	footer := m.renderFooter()
	body := lipgloss.JoinVertical(lipgloss.Left, top, main, footer)
	if m.mode == ModeHelp {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.renderHelp(), lipgloss.WithWhitespaceChars(" "))
	}
	if m.mode == ModeConfirmDelete || (m.mode != ModeNormal && m.mode != ModeFilter) {
		return body + "\n" + m.renderModal()
	}
	return body
}

func (m Model) renderTop() string {
	logo := lipgloss.JoinVertical(lipgloss.Left,
		m.styles.Brand.Render("                  ▀▀       "),
		m.styles.Brand.Render("████▄  ▀▀█▄ ██ ██ ██   ▀▀█▄"),
		m.styles.Brand.Render("██ ██ ▄█▀██ ██▄██ ██  ▄█▀██"),
		m.styles.Brand.Render("██ ██ ▀█▄██  ▀█▀  ██▄ ▀█▄██"),
	)
	context := m.renderHeaderContext()
	if m.width > 72 {
		gap := strings.Repeat(" ", max(2, m.width-lipgloss.Width(logo)-lipgloss.Width(context)-2))
		logo = lipgloss.JoinHorizontal(lipgloss.Top, logo, gap, context)
	}
	title := logo
	return m.styles.TopBar.Width(m.width).Height(m.topHeight()).Render(title)
}

func (m Model) renderHeaderContext() string {
	project := filepath.Base(m.cwd)
	path := m.cwd
	if m.gitRoot != "" {
		project = filepath.Base(m.gitRoot)
		path = gitPathLabel(m.gitRoot, m.cwd)
	}
	mode := "Normal"
	scope := "tree"
	query := ""
	if m.mode == ModeFilter || m.filter != "" {
		mode = "Search"
		scope = "recursive " + m.searchModeLabel()
		query = m.filter
		if query == "" {
			query = "type query"
		}
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		m.styles.Highlight.Render("Project  ")+project,
		m.styles.Dim.Render("Path     ")+truncate(path, 42),
		m.styles.Dim.Render("Mode     ")+mode+"  •  "+scope,
		m.styles.Dim.Render("Rows     ")+fmt.Sprintf("%d", len(m.rows))+"  "+m.styles.Dim.Render(query),
	)
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
	if len(m.rows) == 0 {
		content := m.styles.TreePane.Width(innerW).Height(innerH).Render(m.styles.Dim.Render("No entries."))
		return m.styles.Panel.Width(width - 2).Height(height).Render(content)
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
			line = m.styles.Dir.Render(line)
		}
		if i == m.selectedIndex {
			line = m.styles.Selected.Width(innerW).Render(line)
		}
		lines = append(lines, line)
	}
	content := m.styles.TreePane.Width(innerW).Height(innerH).Render(strings.Join(lines, "\n"))
	return m.styles.Panel.Width(width - 2).Height(height).Render(content)
}

func (m Model) renderPreview(width int) string {
	height := m.listHeight()
	innerW := max(8, width-4)
	innerH := max(4, height-2)
	title := m.preview.Title
	if title == "" {
		title = filepath.Base(m.selectedPathForStatus())
	}
	header := m.styles.Highlight.Render(title)
	content := m.previewViewport.View()
	body := m.styles.Pane.Width(innerW).Height(innerH).Render(header + "\n" + content)
	return m.styles.Panel.Width(width - 2).Height(height).Render(body)
}

func (m Model) renderFooter() string {
	status := m.statusMessage
	if status == "" {
		status = "q quit  ? help  enter/l expand  h collapse  / search  tab file/text  e editor"
	}
	cmd := m.lastCommandHint
	if cmd != "" {
		status += " | " + m.styles.Command.Render(cmd)
	}
	return m.styles.Footer.Width(m.width).Render(truncate(status, m.width-2))
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
	default:
		return ""
	}
}

func (m Model) renderHelp() string {
	help := `Navia

q / ctrl+c    quit
up/k          move up
down/j        move down
enter/l       expand/collapse directory, select file
backspace/h   collapse directory or jump to parent row
r             rename
y             copy
x             cut
p             paste
d             safe delete
n             new file
N             new directory
/             recursive search from current directory
tab           toggle file/text search while searching
e             open selected file in editor
g             go to path
?             close help
esc           cancel mode/filter

Navia shows shell equivalents after file operations so you can learn the command line gradually.`
	return m.styles.Modal.Width(min(72, m.width-8)).Render(help)
}

func (m Model) listHeight() int {
	height := m.height - m.topHeight() - 3
	if height < 8 {
		height = 8
	}
	return height
}

func (m Model) topHeight() int {
	return 4
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
