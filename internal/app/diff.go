package app

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/heidaraliy/navia/internal/git"
)

func (m *Model) enterDiffMode() {
	if m.gitRoot == "" {
		m.statusMessage = "Diff mode requires a git repository."
		return
	}
	m.mode = ModeDiff
	m.focus = FocusTree
	m.statusMessage = "Diff mode."
	m.refreshDiff()
}

func (m *Model) refreshDiff() {
	if m.gitRoot == "" {
		m.diffChanges = nil
		m.diffSummary = git.Summary{}
		m.diffViewport.SetContent("Not inside a git repository.")
		return
	}
	changes, summary, err := git.Status(m.gitRoot)
	if err != nil {
		m.setError(err)
		return
	}
	m.diffChanges = changes
	m.diffSummary = summary
	m.clampDiffSelection()
	m.refreshDiffPreview()
}

func (m *Model) clampDiffSelection() {
	if len(m.diffChanges) == 0 {
		m.diffSelectedIndex = 0
		return
	}
	if m.diffSelectedIndex < 0 {
		m.diffSelectedIndex = 0
	}
	if m.diffSelectedIndex >= len(m.diffChanges) {
		m.diffSelectedIndex = len(m.diffChanges) - 1
	}
}

func (m *Model) selectedDiffChange() (git.Change, bool) {
	if len(m.diffChanges) == 0 || m.diffSelectedIndex < 0 || m.diffSelectedIndex >= len(m.diffChanges) {
		return git.Change{}, false
	}
	return m.diffChanges[m.diffSelectedIndex], true
}

func (m *Model) refreshDiffPreview() {
	change, ok := m.selectedDiffChange()
	if !ok {
		m.diffViewport.SetContent("No modified or untracked files.")
		m.diffViewport.GotoTop()
		return
	}
	diff, err := git.Diff(m.gitRoot, change, int(m.cfg.PreviewMaxBytes))
	if err != nil {
		m.diffViewport.SetContent(err.Error())
	} else {
		m.diffViewport.SetContent(formatUnifiedDiff(diff))
	}
	m.diffViewport.GotoTop()
}

func (m Model) updateDiff(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = ModeNormal
		m.statusMessage = "Tree mode."
	case "q", "ctrl+c":
		return m.guardedQuit()
	case "?":
		m.helpReturnMode = ModeDiff
		m.mode = ModeHelp
	case "up", "k":
		if m.diffSelectedIndex > 0 {
			m.diffSelectedIndex--
			m.refreshDiffPreview()
		}
	case "down", "j":
		if m.diffSelectedIndex < len(m.diffChanges)-1 {
			m.diffSelectedIndex++
			m.refreshDiffPreview()
		}
	case "ctrl+u", "pgup":
		m.diffViewport.HalfViewUp()
	case "ctrl+d", "pgdown":
		m.diffViewport.HalfViewDown()
	case "g":
		m.diffViewport.GotoTop()
	case "G":
		m.diffViewport.GotoBottom()
	case "r":
		m.refreshDiff()
		m.statusMessage = "Diff refreshed."
	case "s":
		m.stageSelectedDiff()
	case "u":
		m.unstageSelectedDiff()
	case "R":
		if change, ok := m.selectedDiffChange(); ok {
			m.pendingDiffAction = change
			m.mode = ModeDiffConfirmRestore
			m.statusMessage = "Restore file changes? Press y to discard, Esc to cancel."
		}
	case "D":
		if change, ok := m.selectedDiffChange(); ok {
			m.pendingDiffAction = change
			m.mode = ModeDiffConfirmRemove
			m.statusMessage = "Remove file? Press y to delete, Esc to cancel."
		}
	case "c":
		m.enterMode(ModeDiffCommit, "commit> ", "")
	case "p":
		if err := git.Push(m.gitRoot); err != nil {
			m.setError(err)
		} else {
			m.statusMessage = "Pushed current branch."
		}
	}
	return m, nil
}

func (m *Model) stageSelectedDiff() {
	change, ok := m.selectedDiffChange()
	if !ok {
		return
	}
	if err := git.Stage(m.gitRoot, change.Path); err != nil {
		m.setError(err)
		return
	}
	m.statusMessage = "Staged `" + change.Path + "`."
	m.refreshDiff()
}

func (m *Model) unstageSelectedDiff() {
	change, ok := m.selectedDiffChange()
	if !ok {
		return
	}
	if change.Kind == git.ChangeUntracked {
		m.statusMessage = "Untracked files are not staged."
		return
	}
	if err := git.Unstage(m.gitRoot, change.Path); err != nil {
		m.setError(err)
		return
	}
	m.statusMessage = "Unstaged `" + change.Path + "`."
	m.refreshDiff()
}

func (m Model) updateDiffConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "n":
		m.mode = ModeDiff
		m.statusMessage = "Diff action cancelled."
	case "y":
		change := m.pendingDiffAction
		var err error
		if m.mode == ModeDiffConfirmRestore {
			err = git.Restore(m.gitRoot, change)
		} else {
			err = git.Remove(m.gitRoot, change)
		}
		if err != nil {
			m.setError(err)
		} else {
			m.statusMessage = "Updated `" + change.Path + "`."
			m.refreshDiff()
		}
		m.mode = ModeDiff
	}
	return m, nil
}

func (m *Model) applyDiffCommit(message string) {
	if err := git.Commit(m.gitRoot, message); err != nil {
		m.setError(err)
		return
	}
	m.statusMessage = "Committed changes."
	m.refreshDiff()
}

func diffStatusText(change git.Change) string {
	if change.Kind == git.ChangeUntracked {
		return "??"
	}
	return string([]byte{statusByte(change.IndexStatus), statusByte(change.WorktreeStatus)})
}

func statusByte(b byte) byte {
	if b == 0 || b == ' ' {
		return '-'
	}
	return b
}

func diffKindLabel(change git.Change) string {
	switch change.Kind {
	case git.ChangeAdded:
		return "added"
	case git.ChangeDeleted:
		return "removed"
	case git.ChangeUntracked:
		return "untracked"
	case git.ChangeRenamed:
		return "renamed"
	default:
		return "modified"
	}
}

func diffSummaryText(s git.Summary) string {
	return fmt.Sprintf("Files +%d ~%d -%d  Lines +%d ~%d -%d", s.FilesAdded, s.FilesChanged, s.FilesRemoved, s.LinesAdded, s.LinesChanged, s.LinesRemoved)
}

func diffLineStyle(line string) string {
	switch {
	case strings.Contains(line, " │ diff --git") || strings.Contains(line, " │ --- ") || strings.Contains(line, " │ +++ "):
		return "header"
	case strings.Contains(line, " │ +"):
		return "add"
	case strings.Contains(line, " │ -"):
		return "remove"
	case strings.Contains(line, " │ @@"):
		return "hunk"
	default:
		return ""
	}
}

var diffHunkPattern = regexp.MustCompile(`@@ -([0-9]+)(?:,[0-9]+)? \+([0-9]+)(?:,[0-9]+)? @@`)

func formatUnifiedDiff(diff string) string {
	lines := strings.Split(diff, "\n")
	oldLine, newLine := 0, 0
	formatted := make([]string, 0, len(lines))
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "@@"):
			oldLine, newLine = parseHunkStart(line)
			formatted = append(formatted, diffGutter("", "", line))
		case strings.HasPrefix(line, "diff --git "), strings.HasPrefix(line, "index "), strings.HasPrefix(line, "new file mode "), strings.HasPrefix(line, "deleted file mode "), strings.HasPrefix(line, "similarity index "), strings.HasPrefix(line, "rename from "), strings.HasPrefix(line, "rename to "), strings.HasPrefix(line, "--- "), strings.HasPrefix(line, "+++ "):
			formatted = append(formatted, diffGutter("", "", line))
		case strings.HasPrefix(line, "+"):
			formatted = append(formatted, diffGutter("", strconv.Itoa(newLine), line))
			newLine++
		case strings.HasPrefix(line, "-"):
			formatted = append(formatted, diffGutter(strconv.Itoa(oldLine), "", line))
			oldLine++
		case strings.HasPrefix(line, "\\"):
			formatted = append(formatted, diffGutter("", "", line))
		default:
			oldText, newText := "", ""
			if oldLine > 0 {
				oldText = strconv.Itoa(oldLine)
				oldLine++
			}
			if newLine > 0 {
				newText = strconv.Itoa(newLine)
				newLine++
			}
			formatted = append(formatted, diffGutter(oldText, newText, line))
		}
	}
	return strings.Join(formatted, "\n")
}

func parseHunkStart(line string) (int, int) {
	matches := diffHunkPattern.FindStringSubmatch(line)
	if len(matches) != 3 {
		return 0, 0
	}
	oldLine, _ := strconv.Atoi(matches[1])
	newLine, _ := strconv.Atoi(matches[2])
	return oldLine, newLine
}

func diffGutter(oldLine, newLine, content string) string {
	return fmt.Sprintf("%4s %4s │ %s", oldLine, newLine, content)
}
