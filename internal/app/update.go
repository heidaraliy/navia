package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	navfs "github.com/heidaraliy/navia/internal/fs"
	"github.com/heidaraliy/navia/internal/shellteach"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case statusMsg:
		return m.handleStatus(msg), nil
	case editorStatusMsg:
		m = m.handleEditorStatus(msg)
		return m, nil
	case definitionMsg:
		return m.handleDefinition(msg)
	case referencesMsg:
		return m.handleReferences(msg)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizePreview()
		m.resizeDiff()
		m.resizeHelp()
		return m, nil
	case tea.KeyMsg:
		if m.mode == ModeHelp {
			switch msg.String() {
			case "esc", "q", "?":
				m.mode = m.helpReturnMode
				if m.mode == ModeHelp {
					m.mode = ModeNormal
				}
			case "j", "down":
				m.helpViewport.LineDown(1)
			case "k", "up":
				m.helpViewport.LineUp(1)
			case "ctrl+d", "pgdown":
				m.helpViewport.HalfViewDown()
			case "ctrl+u", "pgup":
				m.helpViewport.HalfViewUp()
			case "G":
				m.helpViewport.GotoBottom()
			case "g":
				m.helpViewport.GotoTop()
			}
			return m, nil
		}
		if m.mode == ModeDiff {
			return m.updateDiff(msg)
		}
		if m.mode == ModeDiffConfirmRestore || m.mode == ModeDiffConfirmRemove {
			return m.updateDiffConfirm(msg)
		}
		if m.mode != ModeNormal && m.mode != ModeFilter && m.mode != ModeConfirmDelete {
			return m.updateInput(msg)
		}
		if m.mode == ModeFilter {
			return m.updateFilter(msg)
		}
		if m.mode == ModeConfirmDelete {
			return m.updateDeleteConfirm(msg)
		}
		if m.focus == FocusEditor && m.activeBuffer() != nil {
			return m.updateEditor(msg)
		}
		return m.updateNormal(msg)
	}
	return m, nil
}

func (m Model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.windowPending {
		m.windowPending = false
		switch msg.String() {
		case "h", "left":
			m.treeHidden = false
			m.focus = FocusTree
			m.statusMessage = "Tree focused."
		case "l", "right":
			if m.activeBuffer() != nil {
				m.focus = FocusEditor
				m.statusMessage = "Editor focused."
			}
		case "w":
			if m.activeBuffer() != nil {
				m.focus = FocusEditor
				m.statusMessage = "Editor focused."
			}
		case "o":
			if m.activeBuffer() != nil {
				m.treeHidden = true
				m.focus = FocusEditor
				m.statusMessage = "Tree hidden. ctrl+w o restores it."
			}
		default:
			m.statusMessage = "Unknown window command."
		}
		return m, nil
	}
	switch msg.String() {
	case "ctrl+c", "q":
		return m.guardedQuit()
	case "ctrl+w":
		if m.activeBuffer() != nil {
			m.windowPending = true
			m.statusMessage = "Window command: h tree, l editor, w toggle, o only."
		}
	case "up", "k":
		if m.selectedIndex > 0 {
			m.selectedIndex--
			m.refreshPreview()
		}
	case "down", "j":
		if m.selectedIndex < len(m.rows)-1 {
			m.selectedIndex++
			m.refreshPreview()
		}
	case "enter", "l":
		m.openSelected()
	case "L", "shift+enter":
		m.drillIntoSelectedRoot()
	case "backspace", "h":
		m.collapseOrParent()
	case "/":
		m.mode = ModeFilter
		m.statusMessage = "Filtering current directory."
	case "esc":
		m.filter = ""
		m.applyFilter()
		m.clampSelection()
		m.refreshPreview()
	case "r":
		if entry, ok := m.selected(); ok {
			m.enterMode(ModeRename, "rename> ", entry.Name)
		}
	case "n":
		m.enterMode(ModeNewFile, "new file> ", "")
	case "N":
		m.enterMode(ModeNewDir, "new dir> ", "")
	case "g":
		m.enterMode(ModeGoToPath, "go> ", m.cwd)
	case "y":
		if entry, ok := m.selected(); ok {
			m.clipboard = ClipboardState{Path: entry.Path, Op: ClipboardCopy, IsDir: entry.IsDir, BaseName: entry.Name}
			m.statusMessage = "Copied `" + entry.Name + "`. Move somewhere and press `p` to paste."
		}
	case "x":
		if entry, ok := m.selected(); ok {
			m.clipboard = ClipboardState{Path: entry.Path, Op: ClipboardCut, IsDir: entry.IsDir, BaseName: entry.Name}
			m.statusMessage = "Cut `" + entry.Name + "`. Move somewhere and press `p` to paste."
		}
	case "p":
		m.pasteClipboard()
	case "d":
		if entry, ok := m.selected(); ok {
			m.pendingDelete = entry
			m.mode = ModeConfirmDelete
			if entry.IsDir {
				m.statusMessage = "Delete folder safely? Press y to move it to .navia-trash, Esc to cancel."
			} else {
				m.statusMessage = "Delete file safely? Press y to move it to .navia-trash, Esc to cancel."
			}
		}
	case "e":
		return m, m.openEditorCmd()
	case "c":
		return m, m.openSelectedInEditor()
	case "?":
		m.helpReturnMode = ModeNormal
		m.mode = ModeHelp
	case "D":
		m.enterDiffMode()
	}
	return m, nil
}

func (m *Model) openSelected() {
	row, rowOK := m.selectedRow()
	entry, ok := m.selected()
	if !ok {
		return
	}
	if entry.IsDir {
		m.expandedDirs[entry.Path] = !m.expandedDirs[entry.Path]
		path := entry.Path
		m.setError(m.refresh())
		m.selectPath(path)
		return
	}
	if rowOK && row.Line > 0 {
		_ = m.openEditorTab(entry.Path)
		if buf := m.activeBuffer(); buf != nil {
			buf.Row = row.Line - 1
			buf.Col = 0
		}
		return
	}
	m.statusMessage = "Press `e` to open this file in your editor."
}

func (m *Model) collapseOrParent() {
	entry, ok := m.selected()
	if !ok {
		return
	}
	if entry.Path == m.cwd {
		m.goParent()
		return
	}
	if entry.IsDir && m.expandedDirs[entry.Path] {
		m.expandedDirs[entry.Path] = false
		path := entry.Path
		m.setError(m.refresh())
		m.selectPath(path)
		return
	}
	parent := filepath.Dir(entry.Path)
	m.selectPath(parent)
	m.refreshPreview()
}

func (m *Model) goParent() {
	parent := filepath.Dir(m.cwd)
	if parent == m.cwd {
		return
	}
	old := m.cwd
	m.cwd = parent
	m.filter = ""
	m.executedSearchQuery = ""
	m.recursiveRows = nil
	m.recursiveRoot = ""
	m.expandedDirs = map[string]bool{parent: true, old: true}
	if err := m.refresh(); err != nil {
		m.cwd = old
		m.expandedDirs = map[string]bool{old: true}
		m.setError(err)
	}
}

func (m *Model) drillIntoSelectedRoot() {
	entry, ok := m.selected()
	if !ok {
		return
	}
	if !entry.IsDir {
		m.statusMessage = "Select a directory to make it the root."
		return
	}
	if entry.Path == m.cwd {
		m.statusMessage = "Already at `" + entry.Name + "`."
		return
	}
	old := m.cwd
	m.cwd = entry.Path
	m.filter = ""
	m.executedSearchQuery = ""
	m.recursiveRows = nil
	m.recursiveRoot = ""
	m.selectedIndex = 0
	m.expandedDirs = map[string]bool{m.cwd: true}
	if err := m.refresh(); err != nil {
		m.cwd = old
		m.expandedDirs = map[string]bool{old: true}
		m.setError(err)
		return
	}
	m.statusMessage = "Rooted at `" + entry.Name + "`."
}

func (m Model) updateFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = ModeNormal
		m.filter = ""
	case "tab":
		if m.searchMode == SearchFiles {
			m.searchMode = SearchText
			m.statusMessage = "Text search."
		} else {
			m.searchMode = SearchFiles
			m.statusMessage = "File search."
		}
		m.executedSearchQuery = ""
	case "enter":
		if m.filter != m.executedSearchQuery {
			m.executedSearchQuery = m.filter
			m.applyFilter()
			m.clampSelection()
			m.refreshPreview()
			return m, nil
		}
		m.mode = ModeNormal
		m.openSelected()
	case "backspace":
		if len(m.filter) > 0 {
			m.filter = m.filter[:len(m.filter)-1]
			m.executedSearchQuery = ""
		}
	default:
		if s := msg.String(); len(s) == 1 {
			m.filter += s
			m.executedSearchQuery = ""
		}
	}
	m.selectedIndex = 0
	m.applyFilter()
	m.clampSelection()
	m.refreshPreview()
	return m, nil
}

func (m Model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.exitMode()
		return m, nil
	case "enter":
		value := m.input.Value()
		mode := m.mode
		m.applyInput(value)
		if mode == ModeDiffCommit {
			m.mode = ModeDiff
			m.input.Blur()
		} else {
			m.exitMode()
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *Model) applyInput(value string) {
	switch m.mode {
	case ModeRename:
		entry, ok := m.selected()
		if !ok {
			return
		}
		target, err := navfs.Rename(entry.Path, value)
		if err != nil {
			m.setError(err)
			return
		}
		m.statusMessage = "Renamed `" + entry.Name + "` to `" + filepath.Base(target) + "`."
		m.lastCommandHint = shellteach.RenameCommand(entry.Path, target)
	case ModeNewFile:
		path, err := navfs.CreateFile(m.cwd, value)
		if err != nil {
			m.setError(err)
			return
		}
		m.statusMessage = "Created file `" + filepath.Base(path) + "`."
		m.lastCommandHint = shellteach.TouchCommand(path)
	case ModeNewDir:
		path, err := navfs.CreateDir(m.cwd, value)
		if err != nil {
			m.setError(err)
			return
		}
		m.statusMessage = "Created directory `" + filepath.Base(path) + "`."
		m.lastCommandHint = shellteach.MkdirCommand(path)
	case ModeGoToPath:
		dir, err := navfs.ResolveDir(value)
		if err != nil {
			m.setError(err)
			return
		}
		m.cwd = dir
		m.selectedIndex = 0
		m.expandedDirs = map[string]bool{dir: true}
	case ModeDiffCommit:
		m.applyDiffCommit(value)
	}
	m.setError(m.refresh())
}

func (m Model) updateDeleteConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "n":
		m.mode = ModeNormal
		m.statusMessage = "Delete cancelled."
	case "y":
		entry := m.pendingDelete
		target, err := navfs.SafeDelete(entry.Path, m.cwd)
		if err != nil {
			m.setError(err)
		} else {
			m.statusMessage = "Safe-deleted `" + entry.Name + "` into `" + target + "`."
			m.lastCommandHint = shellteach.DeleteCommand(entry.Path, entry.IsDir)
			m.setError(m.refresh())
		}
		m.mode = ModeNormal
	}
	return m, nil
}

func (m *Model) pasteClipboard() {
	if m.clipboard.Op == ClipboardNone || m.clipboard.Path == "" {
		m.statusMessage = "Clipboard is empty."
		return
	}
	dst := defaultNameForCopy(m.clipboard.Path, m.cwd)
	var err error
	if m.clipboard.Op == ClipboardCopy {
		err = navfs.CopyPath(m.clipboard.Path, dst)
		m.lastCommandHint = shellteach.CopyCommand(m.clipboard.Path, dst, m.clipboard.IsDir)
	} else {
		err = navfs.MovePath(m.clipboard.Path, dst)
		m.lastCommandHint = shellteach.MoveCommand(m.clipboard.Path, dst)
		m.clipboard = ClipboardState{}
	}
	if err != nil {
		m.setError(err)
		return
	}
	m.statusMessage = "Pasted `" + filepath.Base(dst) + "`."
	m.setError(m.refresh())
}

func (m Model) openEditorCmd() tea.Cmd {
	entry, ok := m.selected()
	if !ok || entry.IsDir {
		m.statusMessage = "Select a file to open."
		return nil
	}
	editor := m.cfg.Editor
	m.lastCommandHint = shellteach.OpenCommand(entry.Path, editor)
	return tea.ExecProcess(exec.Command(editor, entry.Path), func(err error) tea.Msg {
		if err != nil {
			return statusMsg("Could not open file: " + err.Error())
		}
		return statusMsg("Returned from editor.")
	})
}

type statusMsg string

func (m Model) handleStatus(msg statusMsg) Model {
	m.statusMessage = string(msg)
	return m
}

func (m Model) handleEditorStatus(msg editorStatusMsg) Model {
	m.statusMessage = string(msg)
	m.reloadActiveIfChanged()
	m.setError(m.refresh())
	return m
}

func (m *Model) resizePreview() {
	left, right := m.paneWidths()
	_ = left
	if m.treeHidden && m.activeBuffer() != nil {
		right = m.width
	}
	height := m.height - m.topHeight() - 2
	if height < 4 {
		height = 4
	}
	m.previewViewport.Width = right - 6
	if m.previewViewport.Width < 8 {
		m.previewViewport.Width = 8
	}
	m.previewViewport.Height = height - 4
	if m.previewViewport.Height < 4 {
		m.previewViewport.Height = 4
	}
}

func (m *Model) resizeDiff() {
	_, right := m.paneWidths()
	height := m.height - m.topHeight() - 2
	if height < 4 {
		height = 4
	}
	m.diffViewport.Width = right - 6
	if m.diffViewport.Width < 8 {
		m.diffViewport.Width = 8
	}
	m.diffViewport.Height = height - 5
	if m.diffViewport.Height < 4 {
		m.diffViewport.Height = 4
	}
}

func (m *Model) resizeHelp() {
	m.helpViewport.Width = min(84, m.width-10)
	if m.helpViewport.Width < 30 {
		m.helpViewport.Width = 30
	}
	m.helpViewport.Height = m.height - 8
	if m.helpViewport.Height < 8 {
		m.helpViewport.Height = 8
	}
}

func (m Model) paneWidths() (int, int) {
	if m.width <= 0 {
		return 33, 67
	}
	left := m.width / 3
	if left < 30 {
		left = 30
	}
	if left > m.width-24 {
		left = m.width - 24
	}
	return left, m.width - left
}

func (m Model) debugString() string {
	return fmt.Sprintf("%s %d entries", m.cwd, len(m.entries))
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}
