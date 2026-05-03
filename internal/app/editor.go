package app

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/heidaraliy/navia/internal/config"
	"github.com/heidaraliy/navia/internal/editor"
	"github.com/heidaraliy/navia/internal/shellteach"
	"github.com/heidaraliy/navia/internal/syntax"
)

type editorStatusMsg string

func (m *Model) activeBuffer() *editor.Buffer {
	if len(m.editorTabs) == 0 || m.activeTab < 0 || m.activeTab >= len(m.editorTabs) {
		return nil
	}
	return m.editorTabs[m.activeTab]
}

func (m *Model) openSelectedInEditor() tea.Cmd {
	entry, ok := m.selected()
	if !ok || entry.IsDir {
		m.statusMessage = "Select a text file to edit."
		return nil
	}
	return m.openEditorTab(entry.Path)
}

func (m *Model) openEditorTab(path string) tea.Cmd {
	if path == "" {
		m.statusMessage = "No file path."
		return nil
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(m.cwd, path)
	}
	path = filepath.Clean(path)
	for i, tab := range m.editorTabs {
		if tab.Path == path {
			m.pushEditorJump()
			m.activeTab = i
			m.focus = FocusEditor
			m.statusMessage = "Switched to `" + tab.Name + "`."
			return m.lspDidOpenCmd(tab)
		}
	}
	buf, err := editor.Open(path, m.cfg.EditorMaxBytes)
	if err != nil {
		m.setEditorOpenError(err)
		return nil
	}
	m.editorTabs = append(m.editorTabs, buf)
	m.pushEditorJump()
	m.activeTab = len(m.editorTabs) - 1
	m.focus = FocusEditor
	m.statusMessage = "Editing `" + buf.Name + "`."
	return m.lspDidOpenCmd(buf)
}

func (m *Model) setEditorOpenError(err error) {
	switch {
	case errors.Is(err, editor.ErrDirectory):
		m.statusMessage = "Directories stay in the tree; select a file to edit."
	case errors.Is(err, editor.ErrBinary):
		m.statusMessage = "Binary file refused by Navia editor."
	case errors.Is(err, editor.ErrTooLarge):
		m.statusMessage = "File is too large for Navia editor."
	default:
		m.statusMessage = "Could not edit file: " + err.Error()
	}
}

func (m Model) updateEditor(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	buf := m.activeBuffer()
	if buf == nil {
		m.focus = FocusTree
		return m, nil
	}
	key := msg.String()
	if m.windowPending {
		m.windowPending = false
		switch key {
		case "h", "left":
			m.treeHidden = false
			m.focus = FocusTree
			m.statusMessage = "Tree focused."
			return m, m.restoreTreePreviewFromActiveBuffer()
		case "l", "right":
			m.treeHidden = false
			m.focus = FocusEditor
			m.statusMessage = "Editor focused."
		case "w":
			if m.focus == FocusEditor {
				m.treeHidden = false
				m.focus = FocusTree
				m.statusMessage = "Tree focused."
				return m, m.restoreTreePreviewFromActiveBuffer()
			} else {
				m.focus = FocusEditor
				m.statusMessage = "Editor focused."
			}
		case "o":
			m.treeHidden = !m.treeHidden
			m.focus = FocusEditor
			if m.treeHidden {
				m.statusMessage = "Tree hidden. ctrl+w o restores it."
			} else {
				m.statusMessage = "Tree restored."
			}
		default:
			m.statusMessage = "Unknown window command."
		}
		return m, nil
	}
	if key == "ctrl+w" {
		m.windowPending = true
		m.statusMessage = "Window command: h tree, l editor, w toggle."
		return m, nil
	}
	beforeRegister := buf.Register
	action := buf.HandleKey(key)
	if buf.Register != "" && buf.Register != beforeRegister {
		_ = clipboard.WriteAll(buf.Register)
	}
	return m.applyEditorAction(action)
}

func (m Model) applyEditorAction(action editor.Action) (tea.Model, tea.Cmd) {
	buf := m.activeBuffer()
	switch action.Kind {
	case editor.ActionNone:
		return m, nil
	case editor.ActionStatus:
		m.statusMessage = action.Message
	case editor.ActionSave:
		if buf == nil {
			return m, nil
		}
		if err := buf.Save(action.Force); err != nil {
			m.statusMessage = "Save failed: " + err.Error()
			return m, nil
		}
		m.statusMessage = "Saved `" + buf.Name + "`."
		m.lastCommandHint = "Shell equivalent: write file " + shellteach.Quote(buf.Path)
		m.setError(m.refresh())
		if action.Message == "close" {
			return m.closeActiveTab(false)
		}
		return m, m.lspDidSaveCmd(buf)
	case editor.ActionClose:
		return m.closeActiveTab(false)
	case editor.ActionCloseForce:
		return m.closeActiveTab(true)
	case editor.ActionQuitAll:
		if m.hasDirtyTabs() {
			m.statusMessage = "Unsaved tabs. Use :qa! to quit anyway."
			return m, nil
		}
		return m, tea.Quit
	case editor.ActionQuitAllForce:
		return m, tea.Quit
	case editor.ActionOpen:
		return m, m.openEditorTab(action.Path)
	case editor.ActionNextTab:
		m.pushEditorJump()
		m.nextTab()
	case editor.ActionPrevTab:
		m.pushEditorJump()
		m.prevTab()
	case editor.ActionListTabs:
		m.statusMessage = m.tabListStatus()
	case editor.ActionJumpBack:
		return m.jumpHistoryBack()
	case editor.ActionJumpForward:
		return m.jumpHistoryForward()
	case editor.ActionDefinition:
		return m, m.definitionCmd()
	case editor.ActionReferences:
		return m, m.referencesCmd()
	case editor.ActionExternal:
		if buf == nil {
			return m, nil
		}
		return m, m.openExternalForBufferCmd(buf)
	case editor.ActionTheme:
		if action.Message == "" {
			m.statusMessage = "Themes: " + strings.Join(syntax.Names(), ", ")
			return m, nil
		}
		if !syntax.Valid(action.Message) {
			m.statusMessage = "Unknown theme `" + action.Message + "`. Themes: " + strings.Join(syntax.Names(), ", ")
			return m, nil
		}
		m.syntax = syntax.New(action.Message)
		m.cfg.Theme = m.syntax.Name
		if err := config.SaveTheme(m.syntax.Name); err != nil {
			m.statusMessage = "Theme `" + m.syntax.Name + "` for this session; could not save config: " + err.Error()
		} else {
			m.statusMessage = "Theme `" + m.syntax.Name + "` saved."
		}
	}
	return m, nil
}

func (m Model) closeActiveTab(force bool) (tea.Model, tea.Cmd) {
	buf := m.activeBuffer()
	if buf == nil {
		return m, nil
	}
	if buf.Dirty && !force {
		m.statusMessage = "Unsaved changes in `" + buf.Name + "`. Use :w or :q!."
		return m, nil
	}
	m.editorTabs = append(m.editorTabs[:m.activeTab], m.editorTabs[m.activeTab+1:]...)
	if m.activeTab >= len(m.editorTabs) {
		m.activeTab = len(m.editorTabs) - 1
	}
	if len(m.editorTabs) == 0 {
		m.activeTab = 0
		m.focus = FocusTree
		m.treeHidden = false
		m.statusMessage = "Closed editor tab."
	} else {
		m.statusMessage = "Closed `" + buf.Name + "`."
	}
	return m, nil
}

func (m *Model) nextTab() {
	if len(m.editorTabs) == 0 {
		return
	}
	m.activeTab = (m.activeTab + 1) % len(m.editorTabs)
	m.statusMessage = "Tab `" + m.editorTabs[m.activeTab].Name + "`."
}

func (m *Model) prevTab() {
	if len(m.editorTabs) == 0 {
		return
	}
	m.activeTab--
	if m.activeTab < 0 {
		m.activeTab = len(m.editorTabs) - 1
	}
	m.statusMessage = "Tab `" + m.editorTabs[m.activeTab].Name + "`."
}

func (m *Model) pushEditorJump() {
	buf := m.activeBuffer()
	if buf == nil {
		return
	}
	jump := editorJump{Path: buf.Path, Row: buf.Row, Col: buf.Col}
	if len(m.jumpBack) > 0 && m.jumpBack[len(m.jumpBack)-1] == jump {
		return
	}
	m.jumpBack = append(m.jumpBack, jump)
	if len(m.jumpBack) > 100 {
		m.jumpBack = m.jumpBack[1:]
	}
	m.jumpForward = nil
}

func (m Model) jumpHistoryBack() (tea.Model, tea.Cmd) {
	current := m.currentEditorJump()
	if current.Path == "" || len(m.jumpBack) == 0 {
		m.statusMessage = "No older jump."
		return m, nil
	}
	target := m.jumpBack[len(m.jumpBack)-1]
	m.jumpBack = m.jumpBack[:len(m.jumpBack)-1]
	m.jumpForward = append(m.jumpForward, current)
	return m.restoreEditorJump(target)
}

func (m Model) jumpHistoryForward() (tea.Model, tea.Cmd) {
	current := m.currentEditorJump()
	if current.Path == "" || len(m.jumpForward) == 0 {
		m.statusMessage = "No newer jump."
		return m, nil
	}
	target := m.jumpForward[len(m.jumpForward)-1]
	m.jumpForward = m.jumpForward[:len(m.jumpForward)-1]
	m.jumpBack = append(m.jumpBack, current)
	return m.restoreEditorJump(target)
}

func (m Model) currentEditorJump() editorJump {
	buf := m.activeBuffer()
	if buf == nil {
		return editorJump{}
	}
	return editorJump{Path: buf.Path, Row: buf.Row, Col: buf.Col}
}

func (m Model) restoreEditorJump(jump editorJump) (tea.Model, tea.Cmd) {
	found := false
	for i, tab := range m.editorTabs {
		if tab.Path == jump.Path {
			m.activeTab = i
			found = true
			break
		}
	}
	if !found {
		buf, err := editor.Open(jump.Path, m.cfg.EditorMaxBytes)
		if err != nil {
			m.setEditorOpenError(err)
			return m, nil
		}
		m.editorTabs = append(m.editorTabs, buf)
		m.activeTab = len(m.editorTabs) - 1
	}
	if buf := m.activeBuffer(); buf != nil {
		buf.Row = jump.Row
		buf.Col = jump.Col
		m.statusMessage = "Jumped to `" + buf.Name + "`."
	}
	m.focus = FocusEditor
	return m, nil
}

func (m Model) tabListStatus() string {
	if len(m.editorTabs) == 0 {
		return "No editor tabs."
	}
	parts := make([]string, 0, len(m.editorTabs))
	for i, tab := range m.editorTabs {
		label := strconv.Itoa(i+1) + ":" + tabLabel(tab)
		if i == m.activeTab {
			label = "[" + label + "]"
		}
		parts = append(parts, label)
	}
	return strings.Join(parts, "  ")
}

func (m Model) hasDirtyTabs() bool {
	for _, tab := range m.editorTabs {
		if tab.Dirty {
			return true
		}
	}
	return false
}

func (m Model) guardedQuit() (tea.Model, tea.Cmd) {
	if m.hasDirtyTabs() {
		m.statusMessage = "Unsaved tabs. Use :qa! from the editor or save first."
		return m, nil
	}
	return m, tea.Quit
}

func (m Model) openExternalForBufferCmd(buf *editor.Buffer) tea.Cmd {
	editorName := m.cfg.Editor
	m.lastCommandHint = shellteach.OpenCommand(buf.Path, editorName)
	return tea.ExecProcess(exec.Command(editorName, buf.Path), func(err error) tea.Msg {
		if err != nil {
			return editorStatusMsg("Could not open file: " + err.Error())
		}
		return editorStatusMsg("Returned from external editor.")
	})
}

func (m *Model) reloadActiveIfChanged() {
	buf := m.activeBuffer()
	if buf == nil {
		return
	}
	info, err := os.Stat(buf.Path)
	if err != nil || !info.ModTime().After(buf.ModifiedAt) {
		return
	}
	reloaded, err := editor.Open(buf.Path, m.cfg.EditorMaxBytes)
	if err != nil {
		m.statusMessage = "Could not reload external edits: " + err.Error()
		return
	}
	m.editorTabs[m.activeTab] = reloaded
}

func editorModeLabel(buf *editor.Buffer) string {
	if buf == nil {
		return "NORMAL"
	}
	switch buf.Mode {
	case editor.Insert:
		return "INSERT"
	case editor.Visual:
		return "VISUAL"
	case editor.VisualLine:
		return "VISUAL-LINE"
	case editor.Command:
		return "EXEC"
	case editor.Search:
		return "SEARCH"
	default:
		return "NORMAL"
	}
}

func tabLabel(tab *editor.Buffer) string {
	name := tab.Name
	if tab.Dirty {
		name += "*"
	}
	return name
}

func statusPath(path string) string {
	if home, err := os.UserHomeDir(); err == nil {
		if rel, ok := strings.CutPrefix(path, home); ok {
			return "~" + rel
		}
	}
	return path
}
