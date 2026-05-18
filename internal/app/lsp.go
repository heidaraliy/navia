package app

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/heidaraliy/navia/internal/editor"
	navfs "github.com/heidaraliy/navia/internal/fs"
	"github.com/heidaraliy/navia/internal/git"
	"github.com/heidaraliy/navia/internal/lsp"
)

type definitionMsg struct {
	Locations []lsp.Location
	Rows      []ResultRow
	Symbol    string
	Fallback  bool
	Err       error
}

type referencesMsg struct {
	Locations []lsp.Location
	Rows      []ResultRow
	Symbol    string
	Fallback  bool
	Err       error
}

func (m Model) lspRoot() string {
	root := m.gitRoot
	if root == "" {
		root = git.FindRoot(m.cwd)
	}
	if root == "" {
		root = m.cwd
	}
	return root
}

func (m *Model) lspDidOpenCmd(buf *editor.Buffer) tea.Cmd {
	return nil
}

func (m *Model) lspDidSaveCmd(buf *editor.Buffer) tea.Cmd {
	return nil
}

func (m Model) definitionCmd() tea.Cmd {
	buf := m.activeBuffer()
	if buf == nil {
		return nil
	}
	symbol := buf.SymbolAtCursor()
	if symbol == "" {
		m.statusMessage = "No symbol under cursor."
		return nil
	}
	if filepath.Ext(buf.Path) != ".go" || !m.cfg.EnableLSP {
		return m.symbolDefinitionCmd(symbol)
	}
	return func() tea.Msg {
		client, err := lsp.Start(m.cfg.GoplsCommand, m.lspRoot())
		if err != nil {
			return definitionMsg{Err: err}
		}
		defer client.Close()
		locations, err := client.Definition(buf.Path, buf.CursorLine(), buf.CursorCol())
		return definitionMsg{Locations: locations, Err: err}
	}
}

func (m Model) referencesCmd() tea.Cmd {
	buf := m.activeBuffer()
	if buf == nil {
		return nil
	}
	symbol := buf.SymbolAtCursor()
	if symbol == "" {
		m.statusMessage = "No symbol under cursor."
		return nil
	}
	if filepath.Ext(buf.Path) != ".go" || !m.cfg.EnableLSP {
		return m.symbolReferencesCmd(symbol)
	}
	return func() tea.Msg {
		client, err := lsp.Start(m.cfg.GoplsCommand, m.lspRoot())
		if err != nil {
			return referencesMsg{Err: err}
		}
		defer client.Close()
		locations, err := client.References(buf.Path, buf.CursorLine(), buf.CursorCol())
		return referencesMsg{Locations: locations, Err: err}
	}
}

func (m Model) symbolDefinitionCmd(symbol string) tea.Cmd {
	root := m.lspRoot()
	opts := m.scanOptions()
	maxBytes := m.cfg.PreviewMaxBytes
	return func() tea.Msg {
		matches, err := navfs.SearchSymbolDefinitions(root, symbol, maxBytes, opts)
		return definitionMsg{
			Locations: locationsFromMatches(matches),
			Rows:      rowsFromMatches(matches),
			Symbol:    symbol,
			Fallback:  true,
			Err:       err,
		}
	}
}

func (m Model) symbolReferencesCmd(symbol string) tea.Cmd {
	root := m.lspRoot()
	opts := m.scanOptions()
	maxBytes := m.cfg.PreviewMaxBytes
	return func() tea.Msg {
		matches, err := navfs.SearchSymbolReferences(root, symbol, maxBytes, opts)
		return referencesMsg{
			Locations: locationsFromMatches(matches),
			Rows:      rowsFromMatches(matches),
			Symbol:    symbol,
			Fallback:  true,
			Err:       err,
		}
	}
}

func (m Model) handleDefinition(msg definitionMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.statusMessage = "gd failed: " + msg.Err.Error()
		return m, nil
	}
	if len(msg.Locations) == 0 {
		m.statusMessage = "No definition found."
		return m, nil
	}
	if msg.Fallback && len(msg.Rows) > 1 && !hasStrongDefinitionWinner(msg.Rows) {
		m.rows = msg.Rows
		m.selectedIndex = 0
		m.focus = FocusTree
		m.statusMessage = fmt.Sprintf("%d definition candidates for `%s`. Enter opens selected result.", len(m.rows), msg.Symbol)
		return m, m.queuePreview()
	}
	loc := msg.Locations[0]
	cmd := m.openEditorTab(loc.Path)
	if buf := m.activeBuffer(); buf != nil {
		buf.Row = loc.Line - 1
		buf.Col = loc.Character - 1
	}
	m.statusMessage = fmt.Sprintf("Definition: %s:%d", filepath.Base(loc.Path), loc.Line)
	return m, cmd
}

func (m Model) handleReferences(msg referencesMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.statusMessage = "gr failed: " + msg.Err.Error()
		return m, nil
	}
	if len(msg.Locations) == 0 {
		m.statusMessage = "No references found."
		return m, nil
	}
	if msg.Fallback {
		m.rows = msg.Rows
		m.selectedIndex = 0
		m.focus = FocusTree
		m.statusMessage = fmt.Sprintf("%d references for `%s`. Enter opens selected result.", len(m.rows), msg.Symbol)
		return m, m.queuePreview()
	}
	m.rows = m.rows[:0]
	for _, loc := range msg.Locations {
		info, err := os.Stat(loc.Path)
		if err != nil {
			continue
		}
		m.rows = append(m.rows, ResultRow{
			Entry:   navfs.NewEntry(loc.Path, info),
			Line:    loc.Line,
			Snippet: fmt.Sprintf("column %d", loc.Character),
		})
	}
	m.selectedIndex = 0
	m.focus = FocusTree
	m.statusMessage = fmt.Sprintf("%d references. Enter opens selected result.", len(m.rows))
	return m, nil
}

func locationsFromMatches(matches []navfs.SearchMatch) []lsp.Location {
	locations := make([]lsp.Location, 0, len(matches))
	for _, match := range matches {
		locations = append(locations, lsp.Location{
			Path:      match.Entry.Path,
			Line:      match.Line,
			Character: max(1, match.Column),
		})
	}
	return locations
}

func rowsFromMatches(matches []navfs.SearchMatch) []ResultRow {
	rows := make([]ResultRow, 0, len(matches))
	for _, match := range matches {
		rows = append(rows, ResultRow{
			Entry:   match.Entry,
			Line:    match.Line,
			Column:  match.Column,
			Score:   match.Score,
			Snippet: match.Snippet,
		})
	}
	return rows
}

func hasStrongDefinitionWinner(rows []ResultRow) bool {
	if len(rows) < 2 {
		return true
	}
	return rows[0].Score > rows[1].Score
}
