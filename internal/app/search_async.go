package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	navfs "github.com/heidaraliy/navia/internal/fs"
)

type searchLoadedMsg struct {
	id     int
	mode   SearchMode
	query  string
	root   string
	rows   []ResultRow
	status string
	err    error
}

func (m *Model) startSearch() tea.Cmd {
	query := strings.TrimSpace(m.filter)
	m.searchRequestID++
	id := m.searchRequestID
	m.searchRunning = true
	m.rows = nil
	m.selectedIndex = 0
	m.statusMessage = "Searching..."
	m.previewRequestID++
	m.preview = navfs.Preview{Title: "search", Content: "Searching..."}
	m.previewViewport.SetContent(m.preview.Content)
	if query == "" {
		m.searchRunning = false
		m.applyFilter()
		return m.queuePreview()
	}
	mode := m.searchMode
	root := m.cwd
	opts := m.scanOptions()
	maxBytes := m.cfg.PreviewMaxBytes
	return func() tea.Msg {
		start := perfNow()
		var rows []ResultRow
		var status string
		var err error
		switch mode {
		case SearchText:
			var matches []navfs.SearchMatch
			matches, err = navfs.SearchText(root, query, maxBytes, opts)
			for _, match := range matches {
				rows = append(rows, ResultRow{Entry: match.Entry, Line: match.Line, Snippet: match.Snippet})
			}
			if len(rows) == 0 && err == nil {
				status = "No text matches."
			}
		default:
			var matches []navfs.SearchMatch
			matches, err = navfs.SearchFiles(root, query, opts)
			for _, match := range matches {
				rows = append(rows, ResultRow{Entry: match.Entry})
			}
			if len(rows) >= navfs.MaxSearchResults {
				status = fmt.Sprintf("Showing first %d file matches.", navfs.MaxSearchResults)
			} else if len(rows) == 0 && err == nil {
				status = "No file matches."
			}
		}
		perfLogDuration("search.run", start, "root", root, "query", query)
		return searchLoadedMsg{id: id, mode: mode, query: query, root: root, rows: rows, status: status, err: err}
	}
}

func (m Model) handleSearchLoaded(msg searchLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.id != m.searchRequestID || msg.root != m.cwd || msg.query != m.executedSearchQuery || msg.mode != m.searchMode {
		return m, nil
	}
	m.searchRunning = false
	if msg.err != nil {
		m.statusMessage = msg.err.Error()
		m.rows = nil
		m.selectedIndex = 0
		return m, nil
	}
	m.rows = msg.rows
	m.selectedIndex = 0
	m.clampSelection()
	m.statusMessage = msg.status
	return m, m.queuePreview()
}
