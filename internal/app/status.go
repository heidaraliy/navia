package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const statusMessageTTL = 3500 * time.Millisecond

type statusClearMsg struct {
	revision int
}

func statusClearCmd(revision int) tea.Cmd {
	return tea.Tick(statusMessageTTL, func(time.Time) tea.Msg {
		return statusClearMsg{revision: revision}
	})
}

func (m Model) withStatusTimeout(previous string, cmd tea.Cmd) (tea.Model, tea.Cmd) {
	if m.statusMessage == "" || m.statusMessage == previous {
		return m, cmd
	}
	m.statusRevision++
	clearCmd := statusClearCmd(m.statusRevision)
	if cmd == nil {
		return m, clearCmd
	}
	return m, tea.Batch(cmd, clearCmd)
}

func (m Model) handleStatusClear(msg statusClearMsg) Model {
	if msg.revision == m.statusRevision {
		m.statusMessage = ""
	}
	return m
}
