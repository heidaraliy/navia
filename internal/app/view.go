package app

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	termansi "github.com/charmbracelet/x/ansi"
	"github.com/heidaraliy/navia/internal/gitview"
	"github.com/heidaraliy/navia/internal/textsafe"
)

var (
	brand         = lipgloss.Color("73")
	accent        = lipgloss.Color("111")
	dim           = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	border        = lipgloss.NewStyle().Foreground(lipgloss.Color("24"))
	focusedBorder = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	selectedStyle = lipgloss.NewStyle().Background(lipgloss.Color("236")).Bold(true)
	modifiedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	newStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("84"))
	deletedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	otherStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("117"))
	hunkStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("75")).Background(lipgloss.Color("235"))
)

const (
	topHeight = 6
	driftLogo = "                  ▀▀       \n" +
		"████▄  ▀▀█▄ ██ ██ ██   ▀▀█▄\n" +
		"██ ██ ▄█▀██ ██▄██ ██  ▄█▀██\n" +
		"██ ██ ▀█▄██  ▀█▀  ██▄ ▀█▄██"
)

func (m Model) View() string {
	if m.width == 0 {
		return "Navia is opening…"
	}
	if m.mode == 'n' {
		return m.renderNavigator()
	}
	if m.historyOpen {
		return m.renderHistory()
	}
	if m.help {
		return m.renderHelp()
	}
	header := m.header()
	var body string
	switch m.fullscreen {
	case 'l':
		body = m.renderList(m.width)
	case 'r':
		body = m.renderDiff(m.width)
	default:
		body = lipgloss.JoinHorizontal(lipgloss.Top, m.renderList(m.leftWidth), m.renderDiff(m.width-m.leftWidth))
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, body)
}

func (m Model) header() string {
	logo := strings.Split(driftLogo, "\n")
	logoStyle := lipgloss.NewStyle().Foreground(brand)
	files := dim.Render("Files │ scanning repository…")
	lines := ""
	if m.mode == 'n' {
		files = dim.Render(fmt.Sprintf("Files │ %d visible", len(m.navRows)))
		lines = dim.Render("Root  │ " + truncateLeft(m.browseRoot, 44))
	} else if !m.summaryLoading {
		files = dim.Render(fmt.Sprintf("Files │ %d • ", len(m.changes))) + newStyle.Render(fmt.Sprintf("+%d", m.counts.FilesNew)) + dim.Render(" • ") + modifiedStyle.Render(fmt.Sprintf("~%d", m.counts.FilesModified)) + dim.Render(" • ") + deletedStyle.Render(fmt.Sprintf("-%d", m.counts.FilesDeleted))
		lines = dim.Render("Lines │ ") + newStyle.Render(fmt.Sprintf("+%s", comma(m.counts.LinesNew))) + dim.Render(" • ") + modifiedStyle.Render(fmt.Sprintf("~%s", comma(m.counts.LinesModified))) + dim.Render(" • ") + deletedStyle.Render(fmt.Sprintf("-%s", comma(m.counts.LinesDeleted)))
	}
	prompt := dim.Render("Press ? for keybinds.")
	if m.status != "" {
		prompt = dim.Render(m.status)
	}
	statsWidth := max(34, max(lipgloss.Width(files), max(lipgloss.Width(lines), lipgloss.Width(prompt)))+4)
	stats := []string{
		border.Render("╭" + strings.Repeat("─", statsWidth-2) + "╮"),
		border.Render("│") + " " + fit(files, statsWidth-3) + border.Render("│"),
		border.Render("│") + " " + fit(lines, statsWidth-3) + border.Render("│"),
		border.Render("│") + " " + fit(prompt, statsWidth-3) + border.Render("│"),
		border.Render("╰" + strings.Repeat("─", statsWidth-2) + "╯"),
	}
	rows := []string{
		fit(" "+logoStyle.Render(logo[0])+"   "+stats[0], m.width),
		fit(" "+logoStyle.Render(logo[1])+"   "+stats[1], m.width),
		fit(" "+logoStyle.Render(logo[2])+"   "+stats[2], m.width),
		fit(" "+logoStyle.Render(logo[3])+"   "+stats[3], m.width),
		fit(strings.Repeat(" ", lipgloss.Width(logo[0])+4)+stats[4], m.width),
		strings.Repeat(" ", m.width),
	}
	return strings.Join(rows, "\n")
}

func (m Model) renderList(width int) string {
	paneWidth := max(10, width)
	w := paneWidth - 2
	h := m.listHeight()
	changes := m.visibleChanges()
	lines := make([]string, 0, h)
	if len(changes) == 0 {
		message := "  Clean working tree."
		if m.searchQuery != "" {
			message = "  No matching files."
		}
		lines = append(lines, dim.Render(message))
	}
	for i := m.listTop; i < len(changes) && len(lines) < h; i++ {
		change := changes[i]
		status := string(change.Kind)
		if change.Kind == gitview.Untracked {
			status = "A"
		}
		prefix := status + "  "
		path := cleanPath(change.Path)
		if change.OldPath != "" {
			path = cleanPath(change.OldPath) + " → " + path
		}
		line := prefix + truncateLeft(path, max(1, w-lipgloss.Width(prefix)))
		style := otherStyle
		switch change.Kind {
		case gitview.Modified:
			style = modifiedStyle
		case gitview.Untracked, gitview.Copied:
			style = newStyle
		case gitview.Deleted:
			style = deletedStyle
		}
		if i == m.selected {
			line = selectedStyle.Width(w).Render(line)
		} else {
			line = style.Render(line)
		}
		lines = append(lines, fit(line, w))
	}
	for len(lines) < h {
		lines = append(lines, strings.Repeat(" ", w))
	}
	title := fmt.Sprintf("Files Changed (%d)", len(m.changes))
	if m.diffLabel != "" {
		title = truncateLeft(m.diffLabel, max(12, w-4))
	}
	search := " Find a file…"
	if m.searchQuery != "" || m.searching {
		search = " / " + m.searchQuery
		if m.searching {
			search += "█"
		} else if m.searchLoading {
			search += "  searching contents…"
		}
	}
	frame := border
	heavy := false
	if m.searching {
		frame = focusedBorder
		heavy = true
	}
	rows := []string{
		paneTop(title, "", paneWidth, frame, heavy),
		paneRow(dim.Render(fit(search, w)), paneWidth, frame, heavy),
		paneRule(paneWidth, frame, heavy),
	}
	for _, line := range lines {
		rows = append(rows, paneRow(line, paneWidth, frame, heavy))
	}
	rows = append(rows, paneBottom(paneWidth, frame, heavy))
	return strings.Join(rows, "\n")
}

func (m Model) renderDiff(width int) string {
	paneWidth := max(14, width)
	w := paneWidth - 2
	h := m.diffHeight()
	changes := m.visibleChanges()
	title := " DIFF "
	if len(changes) > 0 {
		title = " " + truncateLeft(changes[m.selected].Path, max(8, w-32)) + " "
	}
	typeLabel := ""
	if m.diff.Binary {
		typeLabel = dim.Render("Binary file  ")
	}
	if m.diff.Size > 0 {
		typeLabel += dim.Render(formatSize(m.diff.Size) + "   │   ")
	}
	stats := typeLabel + newStyle.Render(fmt.Sprintf("+%d", m.diff.Counts.LinesNew)) + dim.Render(" • ") + modifiedStyle.Render(fmt.Sprintf("~%d", m.diff.Counts.LinesModified)) + dim.Render(" • ") + deletedStyle.Render(fmt.Sprintf("-%d", m.diff.Counts.LinesDeleted))
	var rows []string
	if m.diffLoading {
		rows = []string{dim.Render("Loading selected diff…")}
	} else if m.err != "" {
		rows = []string{deletedStyle.Render(m.err)}
	} else if m.diff.Binary {
		rows = m.renderBinary(w)
	} else if len(m.diff.Lines) == 0 {
		rows = []string{dim.Render("No textual diff for this file.")}
	} else if m.sideBySide {
		rows = m.renderSideBySide(w, h)
	} else {
		rows = m.renderUnified(w, h)
	}
	for len(rows) < h {
		rows = append(rows, strings.Repeat(" ", w))
	}
	framedRows := make([]string, 0, len(rows))
	for _, row := range rows {
		framedRows = append(framedRows, paneRow(row, paneWidth, border, false))
	}
	rows = append([]string{paneTop(strings.TrimSpace(title), stats, paneWidth, border, false)}, framedRows...)
	rows = append(rows, paneBottom(paneWidth, border, false))
	return strings.Join(rows, "\n")
}

func (m Model) renderBinary(width int) []string {
	title := lipgloss.NewStyle().Bold(true).Foreground(accent).Render("◇  Binary file changed")
	description := dim.Render("   This file is binary and cannot be meaningfully diffed.")
	return []string{"", "", fit("   "+title, width), "", fit(description, width)}
}

func (m Model) renderHelp() string {
	bindings := [][2]string{
		{"Search files", "/"},
		{"Select file", "j/k or ↑/↓"},
		{"Page file list", "J/K or ⇧↑/↓"},
		{"Page file diff", "Ctrl-j/k, Ctrl-↑/↓, PgUp/Dn"},
		{"Fullscreen file list", "F"},
		{"Fullscreen file diff", "f"},
		{"Toggle diff layout", "v"},
		{"Open in Neovim", "Ctrl-o"},
		{"Refresh", "r"},
		{"Quit", "q"},
	}
	const labelWidth = 24
	bindings = append(bindings, [2]string{"Commit history", "c"}, [2]string{"Return to files", "Esc"})
	rows := []string{lipgloss.NewStyle().Bold(true).Foreground(accent).Render("Navia keybinds"), ""}
	for _, binding := range bindings {
		label := fit(binding[0], labelWidth)
		rows = append(rows, dim.Render(label)+lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229")).Render(binding[1]))
	}
	rows = append(rows, "", dim.Render("Press ?, Esc, Enter, or q to close."))
	modal := lipgloss.NewStyle().
		Border(lipgloss.ThickBorder()).
		BorderForeground(lipgloss.Color("39")).
		Padding(1, 3).
		Render(strings.Join(rows, "\n"))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal)
}

func (m Model) renderUnified(width, height int) []string {
	end := min(len(m.diff.Lines), m.diffTop+height)
	rows := make([]string, 0, end-m.diffTop)
	for _, line := range m.diff.Lines[m.diffTop:end] {
		marker := " "
		if line.Kind == gitview.Added {
			marker = newStyle.Render("+")
		}
		if line.Kind == gitview.Removed {
			marker = deletedStyle.Render("-")
		}
		gutter := fmt.Sprintf("%s %4s %4s │ ", marker, number(line.Old), number(line.New))
		text := line.Text
		if line.Kind == gitview.Hunk || line.Kind == gitview.Header {
			text = textsafe.Content(text)
		}
		if line.Kind == gitview.Added || line.Kind == gitview.Removed || line.Kind == gitview.Context {
			text = m.syntax.HighlightLine(m.diff.Path, text)
		}
		row := fit(gutter+text, width)
		switch line.Kind {
		case gitview.Hunk:
			row = hunkStyle.Width(width).Render(row)
		case gitview.Header:
			row = dim.Width(width).Render(row)
		}
		rows = append(rows, row)
	}
	return rows
}

func (m Model) renderSideBySide(width, height int) []string {
	leftW := max(8, (width-1)/2)
	rightW := max(8, width-leftW-1)
	end := min(len(m.diff.Side), m.diffTop+height)
	rows := make([]string, 0, end-m.diffTop)
	for _, line := range m.diff.Side[m.diffTop:end] {
		old, new := line.OldText, line.NewText
		if line.Kind == gitview.Hunk || line.Kind == gitview.Header {
			old, new = textsafe.Content(old), textsafe.Content(new)
		}
		if old != "" && (line.Kind == gitview.Context || line.Kind == gitview.Removed) {
			old = m.syntax.HighlightLine(m.diff.Path, old)
		}
		if new != "" && (line.Kind == gitview.Context || line.Kind == gitview.Added || line.Kind == gitview.Removed) {
			new = m.syntax.HighlightLine(m.diff.Path, new)
		}
		oldMarker, newMarker := " ", " "
		if line.OldText != "" && line.Kind == gitview.Removed {
			oldMarker = deletedStyle.Render("-")
		}
		if line.NewText != "" && (line.Kind == gitview.Added || line.Kind == gitview.Removed) {
			newMarker = newStyle.Render("+")
		}
		left := fit(fmt.Sprintf("%s %4s │ %s", oldMarker, number(line.Old), old), leftW)
		right := fit(fmt.Sprintf("%s %4s │ %s", newMarker, number(line.New), new), rightW)
		rows = append(rows, left+dim.Render("│")+right)
	}
	return rows
}

func truncateLeft(value string, width int) string {
	if termansi.StringWidth(value) <= width {
		return value
	}
	if width <= 1 {
		return "…"
	}
	runes := []rune(value)
	for len(runes) > 0 && termansi.StringWidth("…"+string(runes)) > width {
		runes = runes[1:]
	}
	return "…" + string(runes)
}
func fit(value string, width int) string {
	value = termansi.Truncate(value, width, "")
	return value + strings.Repeat(" ", max(0, width-termansi.StringWidth(value)))
}
func number(n int) string {
	if n == 0 {
		return ""
	}
	return fmt.Sprint(n)
}

func headerLine(left, right string, width int) string {
	if right == "" {
		return fit(left, width)
	}
	rightWidth := termansi.StringWidth(right)
	left = termansi.Truncate(left, max(0, width-rightWidth-1), "")
	gap := max(1, width-termansi.StringWidth(left)-rightWidth)
	return fit(left+strings.Repeat(" ", gap)+right, width)
}

func paneTop(title, right string, width int, frame lipgloss.Style, heavy bool) string {
	leftCorner, line, rightCorner := "╭", "─", "╮"
	if heavy {
		leftCorner, line, rightCorner = "┏", "━", "┓"
	}
	inner := max(0, width-2)
	left := line + " " + lipgloss.NewStyle().Bold(true).Foreground(accent).Render(title) + " "
	rightText := ""
	if right != "" {
		rightText = " " + right + " "
	}
	reserved := termansi.StringWidth(rightText)
	if right != "" {
		reserved++
	}
	left = termansi.Truncate(left, max(0, inner-reserved), "")
	fill := strings.Repeat(line, max(0, inner-termansi.StringWidth(left)-reserved))
	ending := ""
	if right != "" {
		ending = rightText + frame.Render(line)
	}
	return frame.Render(leftCorner) + frame.Render(left) + frame.Render(fill) + ending + frame.Render(rightCorner)
}

func paneRow(content string, width int, frame lipgloss.Style, heavy bool) string {
	vertical := "│"
	if heavy {
		vertical = "┃"
	}
	return frame.Render(vertical) + fit(content, max(0, width-2)) + frame.Render(vertical)
}

func paneRule(width int, frame lipgloss.Style, heavy bool) string {
	left, line, right := "├", "─", "┤"
	if heavy {
		left, line, right = "┣", "━", "┫"
	}
	return frame.Render(left + strings.Repeat(line, max(0, width-2)) + right)
}

func paneBottom(width int, frame lipgloss.Style, heavy bool) string {
	left, line, right := "╰", "─", "╯"
	if heavy {
		left, line, right = "┗", "━", "┛"
	}
	return frame.Render(left + strings.Repeat(line, max(0, width-2)) + right)
}

func comma(value int) string {
	result := strconv.Itoa(value)
	for i := len(result) - 3; i > 0; i -= 3 {
		result = result[:i] + "," + result[i:]
	}
	return result
}

func formatSize(bytes int64) string {
	const (
		kilobyte = 1024
		megabyte = 1024 * kilobyte
		gigabyte = 1024 * megabyte
	)
	switch {
	case bytes >= gigabyte:
		return fmt.Sprintf("%.1f GB", float64(bytes)/gigabyte)
	case bytes >= megabyte:
		return fmt.Sprintf("%.1f MB", float64(bytes)/megabyte)
	case bytes >= kilobyte:
		return fmt.Sprintf("%.1f KB", float64(bytes)/kilobyte)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
