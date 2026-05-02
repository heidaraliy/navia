package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/heidaraliy/navia/internal/git"
)

const autoRefreshInterval = 900 * time.Millisecond

type autoRefreshMsg struct{}

func autoRefreshCmd() tea.Cmd {
	return tea.Tick(autoRefreshInterval, func(time.Time) tea.Msg {
		return autoRefreshMsg{}
	})
}

func (m *Model) handleAutoRefresh() {
	switch m.mode {
	case ModeDiff:
		m.autoRefreshDiff()
	case ModeNormal:
		m.autoRefreshTree()
	}
}

func (m *Model) autoRefreshTree() {
	next := m.currentTreeSignature()
	if next == "" || next == m.treeRefreshSignature {
		return
	}
	selectedPath := ""
	if entry, ok := m.selected(); ok {
		selectedPath = entry.Path
	}
	if err := m.refresh(); err != nil {
		m.setError(err)
		return
	}
	if selectedPath != "" {
		m.selectPath(selectedPath)
		m.refreshPreview()
	}
}

func (m *Model) currentTreeSignature() string {
	dirs := make([]string, 0, len(m.expandedDirs)+1)
	seen := map[string]bool{}
	addDir := func(dir string) {
		if dir == "" || seen[dir] {
			return
		}
		seen[dir] = true
		dirs = append(dirs, dir)
	}
	addDir(m.cwd)
	for dir, expanded := range m.expandedDirs {
		if expanded {
			addDir(dir)
		}
	}
	sort.Strings(dirs)

	var b strings.Builder
	for _, dir := range dirs {
		info, err := os.Lstat(dir)
		if err != nil {
			fmt.Fprintf(&b, "dir %s err %v\n", dir, err)
			continue
		}
		writeTreeSignatureEntry(&b, dir, info)
		children, err := os.ReadDir(dir)
		if err != nil {
			fmt.Fprintf(&b, "children %s err %v\n", dir, err)
			continue
		}
		for _, child := range children {
			info, err := child.Info()
			if err != nil {
				fmt.Fprintf(&b, "child %s err %v\n", filepath.Join(dir, child.Name()), err)
				continue
			}
			writeTreeSignatureEntry(&b, filepath.Join(dir, child.Name()), info)
		}
	}
	return b.String()
}

func writeTreeSignatureEntry(b *strings.Builder, path string, info os.FileInfo) {
	fmt.Fprintf(b, "%s %t %d %d %o\n", path, info.IsDir(), info.Size(), info.ModTime().UnixNano(), info.Mode().Perm())
}

func (m *Model) autoRefreshDiff() {
	if m.gitRoot == "" {
		return
	}
	changes, summary, err := git.Status(m.gitRoot)
	if err != nil {
		m.setError(err)
		return
	}
	selectedPath := ""
	if change, ok := m.selectedDiffChange(); ok {
		selectedPath = change.Path
	}
	selectedIndex := selectDiffIndex(changes, selectedPath, m.diffSelectedIndex)
	content := diffPreviewContent(m.gitRoot, changes, selectedIndex, int(m.cfg.PreviewMaxBytes))
	next := diffRefreshSignature(changes, summary, selectedIndex, content)
	if next == m.diffRefreshSignature {
		return
	}
	m.diffChanges = changes
	m.diffSummary = summary
	m.diffSelectedIndex = selectedIndex
	m.diffRefreshSignature = next
	m.diffViewport.SetContent(content)
}

func selectDiffIndex(changes []git.Change, path string, fallback int) int {
	if len(changes) == 0 {
		return 0
	}
	for i, change := range changes {
		if path != "" && change.Path == path {
			return i
		}
	}
	if fallback < 0 {
		return 0
	}
	if fallback >= len(changes) {
		return len(changes) - 1
	}
	return fallback
}

func diffRefreshSignature(changes []git.Change, summary git.Summary, selectedIndex int, content string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "summary %d %d %d %d %d %d\n", summary.FilesAdded, summary.FilesChanged, summary.FilesRemoved, summary.LinesAdded, summary.LinesChanged, summary.LinesRemoved)
	fmt.Fprintf(&b, "selected %d\n", selectedIndex)
	for _, change := range changes {
		fmt.Fprintf(&b, "%s %d %q %q\n", diffStatusText(change), change.Kind, change.OldPath, change.Path)
	}
	b.WriteString(content)
	return b.String()
}
