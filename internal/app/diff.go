package app

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/heidaraliy/navia/internal/git"
)

type diffPreviewLoadedMsg struct {
	id            int
	selectedIndex int
	content       string
	signature     string
}

func (m *Model) enterDiffMode() {
	m.diffPatchReview = nil
	m.diffPatchLabel = ""
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
	if m.diffPatchReview != nil {
		m.diffChanges = m.diffPatchReview.Changes
		m.diffSummary = m.diffPatchReview.Summary
		m.clampDiffSelection()
		m.refreshDiffPreview()
		return
	}
	if m.gitRoot == "" {
		m.diffChanges = nil
		m.diffSummary = git.Summary{}
		m.diffViewport.SetContent("Not inside a git repository.")
		m.diffRefreshSignature = ""
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
	content := m.diffPreviewContent(m.diffChanges, m.diffSelectedIndex, int(m.cfg.PreviewMaxBytes))
	m.diffViewport.SetContent(content)
	m.diffRefreshSignature = diffRefreshSignature(m.diffChanges, m.diffSummary, m.diffSelectedIndex, content)
	m.diffViewport.GotoTop()
}

func (m *Model) queueDiffPreview() tea.Cmd {
	m.diffPreviewRequestID++
	id := m.diffPreviewRequestID
	selected := m.diffSelectedIndex
	m.diffViewport.SetContent("Loading diff...")
	m.diffViewport.GotoTop()
	return m.diffPreviewCmd(id, selected)
}

func (m Model) diffPreviewCmd(id int, selected int) tea.Cmd {
	root := m.gitRoot
	changes := append([]git.Change(nil), m.diffChanges...)
	summary := m.diffSummary
	review := m.diffPatchReview
	maxBytes := int(m.cfg.PreviewMaxBytes)
	return func() tea.Msg {
		start := perfNow()
		content := diffPreviewContent(root, changes, selected, maxBytes, review)
		signature := diffRefreshSignature(changes, summary, selected, content)
		perfLogDuration("diff.preview", start, "root", root)
		return diffPreviewLoadedMsg{id: id, selectedIndex: selected, content: content, signature: signature}
	}
}

func (m *Model) applyDiffPreviewLoaded(msg diffPreviewLoadedMsg) {
	if m.mode != ModeDiff || msg.id != m.diffPreviewRequestID || msg.selectedIndex != m.diffSelectedIndex {
		return
	}
	m.diffViewport.SetContent(msg.content)
	m.diffRefreshSignature = msg.signature
	m.diffViewport.GotoTop()
}

func (m Model) diffPreviewContent(changes []git.Change, selectedIndex int, maxBytes int) string {
	return diffPreviewContent(m.gitRoot, changes, selectedIndex, maxBytes, m.diffPatchReview)
}

func diffPreviewContent(gitRoot string, changes []git.Change, selectedIndex int, maxBytes int, review *git.PatchReview) string {
	if len(changes) == 0 || selectedIndex < 0 || selectedIndex >= len(changes) {
		return "No modified or untracked files."
	}
	if review != nil {
		return patchPreviewContent(review, changes[selectedIndex], maxBytes)
	}
	diff, err := git.Diff(gitRoot, changes[selectedIndex], maxBytes)
	if err != nil {
		return err.Error()
	}
	return formatUnifiedDiff(diff)
}

func patchPreviewContent(review *git.PatchReview, change git.Change, maxBytes int) string {
	diff := review.Patches[change.Path]
	if diff == "" {
		return "No patch for " + change.Path + "."
	}
	if maxBytes > 0 && len(diff) > maxBytes {
		diff = diff[:maxBytes] + "\n... truncated ...\n"
	}
	return formatUnifiedDiff(diff)
}

func (m Model) updateDiff(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = ModeNormal
		m.statusMessage = "Tree mode."
	case "q", "ctrl+c":
		return m.guardedQuit()
	case "?":
		m.enterHelpMode(ModeDiff)
	case "up", "k":
		if m.diffSelectedIndex > 0 {
			m.diffSelectedIndex--
			return m, m.queueDiffPreview()
		}
	case "down", "j":
		if m.diffSelectedIndex < len(m.diffChanges)-1 {
			m.diffSelectedIndex++
			return m, m.queueDiffPreview()
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
		if m.diffPatchReview != nil {
			m.statusMessage = "Patch review is already loaded."
			return m, nil
		}
		m.refreshDiff()
		m.statusMessage = "Diff refreshed."
	case "s":
		if m.diffPatchReview != nil {
			m.statusMessage = "Patch review is read-only."
			return m, nil
		}
		m.stageSelectedDiff()
	case "u":
		if m.diffPatchReview != nil {
			m.statusMessage = "Patch review is read-only."
			return m, nil
		}
		m.unstageSelectedDiff()
	case "R":
		if m.diffPatchReview != nil {
			m.statusMessage = "Patch review is read-only."
			return m, nil
		}
		if change, ok := m.selectedDiffChange(); ok {
			m.pendingDiffAction = change
			m.mode = ModeDiffConfirmRestore
			m.statusMessage = "Restore file changes? Press y to discard, Esc to cancel."
		}
	case "D":
		if m.diffPatchReview != nil {
			m.statusMessage = "Patch review is read-only."
			return m, nil
		}
		if change, ok := m.selectedDiffChange(); ok {
			m.pendingDiffAction = change
			m.mode = ModeDiffConfirmRemove
			m.statusMessage = "Remove file? Press y to delete, Esc to cancel."
		}
	case "c":
		if m.diffPatchReview != nil {
			m.statusMessage = "Patch review is read-only."
			return m, nil
		}
		m.enterMode(ModeDiffCommit, "commit> ", "")
	case "p":
		if m.diffPatchReview != nil {
			m.statusMessage = "Patch review is read-only."
			return m, nil
		}
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
	content = displayText(content)
	return fmt.Sprintf("%4s %4s │ %s", oldLine, newLine, content)
}
