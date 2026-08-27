package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/heidaraliy/navia/internal/gitview"
)

type historyMsg struct {
	id      int
	append  bool
	commits []gitview.Commit
	err     error
}

func (m *Model) queueHistory(appendPage bool) tea.Cmd {
	m.historyID++
	id, root := m.historyID, m.root
	skip := 0
	if appendPage {
		skip = len(m.history)
	}
	m.historyLoading = true
	return func() tea.Msg {
		commits, err := gitview.Commits(root, skip, 50)
		return historyMsg{id: id, append: appendPage, commits: commits, err: err}
	}
}

func (m Model) updateHistory(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	count := len(m.history) + 1
	if m.historyHasMore {
		count++
	}
	switch msg.String() {
	case "esc", "q", "c":
		m.historyOpen = false
	case "up", "k":
		if m.historySelected > 0 {
			m.historySelected--
		}
	case "down", "j":
		if m.historySelected < count-1 {
			m.historySelected++
		}
	case "pgup", "K":
		m.historySelected = max(0, m.historySelected-10)
	case "pgdown", "J":
		m.historySelected = min(count-1, m.historySelected+10)
	case "enter":
		return m.chooseHistory(count)
	}
	return m, nil
}

func (m Model) chooseHistory(count int) (tea.Model, tea.Cmd) {
	if m.historyHasMore && m.historySelected == count-1 {
		return m, m.queueHistory(true)
	}
	m.diffRef, m.diffLabel = "", "Working Tree"
	if m.historySelected > 0 {
		commit := m.history[m.historySelected-1]
		m.diffRef, m.diffLabel = commit.Hash, commit.Short+"  "+commit.Subject
	}
	m.historyOpen = false
	m.selected, m.listTop, m.diffTop = 0, 0, 0
	m.summaryLoading = true
	return m, m.queueStatus()
}

func (m Model) renderHistory() string {
	rows := []string{lipgloss.NewStyle().Bold(true).Foreground(accent).Render("Compare"), ""}
	items := []string{"Working Tree"}
	for _, commit := range m.history {
		items = append(items, fmt.Sprintf("%s  %s  %s", commit.Short, commit.Subject, commit.Relative))
	}
	if m.historyHasMore {
		items = append(items, "Load more…")
	}
	start := max(0, m.historySelected-10)
	end := min(len(items), start+20)
	for i := start; i < end; i++ {
		line := truncateLeft(items[i], 74)
		if i == m.historySelected {
			line = selectedStyle.Width(76).Render(line)
		}
		rows = append(rows, line)
	}
	if m.historyLoading {
		rows = append(rows, "", dim.Render("Loading history…"))
	}
	rows = append(rows, "", dim.Render("Enter selects • Esc closes"))
	modal := lipgloss.NewStyle().Border(lipgloss.ThickBorder()).BorderForeground(accent).Padding(1, 3).Render(strings.Join(rows, "\n"))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal)
}

func (m Model) updateHistoryMouse(msg tea.MouseEvent) (tea.Model, tea.Cmd) {
	count := len(m.history) + 1
	if m.historyHasMore {
		count++
	}
	if msg.Button == tea.MouseButtonWheelUp {
		m.historySelected = max(0, m.historySelected-1)
		return m, nil
	}
	if msg.Button == tea.MouseButtonWheelDown {
		m.historySelected = min(count-1, m.historySelected+1)
		return m, nil
	}
	if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress {
		top := max(0, (m.height-min(26, count+7))/2) + 3
		index := max(0, m.historySelected-10) + msg.Y - top
		if index >= 0 && index < count {
			m.historySelected = index
			return m.chooseHistory(count)
		}
	}
	return m, nil
}
