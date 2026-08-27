package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/heidaraliy/navia/internal/config"
	"github.com/heidaraliy/navia/internal/gitview"
	"github.com/heidaraliy/navia/internal/syntax"
)

func TestNavigatorExpandsDirectoriesAndPreviewsFiles(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "dir", "file.go"), "package demo\n")
	m, err := New(root, config.Default(), false)
	if err != nil {
		t.Fatal(err)
	}
	m.width, m.height = 120, 40
	if m.mode != 'n' || len(m.navRows) != 2 {
		t.Fatalf("mode/rows=%c/%d", m.mode, len(m.navRows))
	}
	m.navSelected = 1
	updated, cmd := m.updateNavKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd == nil || len(m.navRows) != 3 {
		t.Fatalf("expanded rows/cmd=%d/%v", len(m.navRows), cmd)
	}
	m.navSelected = 2
	cmd = m.queueNavPreview()
	msg := cmd()
	updated, _, handled := m.updateNavigatorMessage(msg)
	m = updated.(Model)
	if !handled || m.navPreview.Kind != "text" || !strings.Contains(strings.Join(m.navPreviewLines, "\n"), "package") {
		t.Fatalf("preview=%#v", m.navPreview)
	}
}

func TestNavigatorSearchAndReadOnlyKeys(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "needle.txt"), "find this text\n")
	m, err := New(root, config.Default(), false)
	if err != nil {
		t.Fatal(err)
	}
	before := len(m.navRows)
	for _, key := range []string{"r", "n", "N", "y", "x", "p", "d", "e", "c", "ctrl+w"} {
		updated, cmd := m.updateNavKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
		m = updated.(Model)
		if cmd != nil || len(m.navRows) != before {
			t.Fatalf("legacy key %q changed navigator", key)
		}
	}
	m.navQuery = "needle"
	cmd := m.queueNavSearch()
	msg := cmd()
	updated, _, _ := m.updateNavigatorMessage(msg)
	m = updated.(Model)
	if len(m.navRows) != 1 || m.navRows[0].entry.Name != "needle.txt" {
		t.Fatalf("search rows=%#v", m.navRows)
	}
}

func TestArrowKeysExpandAndCollapseWithoutOpeningFiles(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "dir", "file.txt"), "x\n")
	m, err := New(root, config.Default(), false)
	if err != nil {
		t.Fatal(err)
	}
	m.navSelected = 1
	updated, cmd := m.updateNavKey(tea.KeyMsg{Type: tea.KeyRight})
	m = updated.(Model)
	if cmd == nil || !m.expanded[filepath.Join(root, "dir")] {
		t.Fatal("right did not expand directory")
	}
	m.navSelected = 2
	updated, cmd = m.updateNavKey(tea.KeyMsg{Type: tea.KeyRight})
	m = updated.(Model)
	if cmd != nil {
		t.Fatal("right opened a file")
	}
	m.navSelected = 1
	updated, cmd = m.updateNavKey(tea.KeyMsg{Type: tea.KeyLeft})
	m = updated.(Model)
	if cmd == nil || m.expanded[filepath.Join(root, "dir")] {
		t.Fatal("left did not collapse directory")
	}
	m.navSearching = true
	m.navQuery = "Physics"
	updated, _ = m.updateNavKey(tea.KeyMsg{Type: tea.KeySpace})
	m = updated.(Model)
	if m.navQuery != "Physics " {
		t.Fatalf("space query=%q", m.navQuery)
	}
}

func TestDiffRowsUseColoredGuttersWithoutFullRowBackgrounds(t *testing.T) {
	m := Model{
		diff: gitview.FileDiff{
			Path:  "deleted.go",
			Lines: []gitview.DiffLine{{Kind: gitview.Removed, Old: 1, Text: `package app`}},
			Side:  []gitview.SideLine{{Kind: gitview.Removed, Old: 1, OldText: `package app`}},
		},
		syntax: syntax.New("navia"),
	}
	for name, rendered := range map[string]string{
		"unified": strings.Join(m.renderUnified(100, 10), "\n"),
		"split":   strings.Join(m.renderSideBySide(100, 10), "\n"),
	} {
		if strings.Contains(rendered, "\x1b[48;") {
			t.Fatalf("%s diff applies a full-row background: %q", name, rendered)
		}
		if !strings.Contains(rendered, "-") {
			t.Fatalf("%s diff is missing its deletion gutter: %q", name, rendered)
		}
	}
}

func TestDiffRowsDoNotEmitLiteralTabs(t *testing.T) {
	m := Model{
		diff: gitview.FileDiff{
			Path:  "tabbed.go",
			Lines: []gitview.DiffLine{{Kind: gitview.Added, New: 1, Text: "\t\treturn true"}},
			Side:  []gitview.SideLine{{Kind: gitview.Added, New: 1, NewText: "\t\treturn true"}},
		},
		syntax: syntax.New("navia"),
	}
	for name, rendered := range map[string]string{
		"unified": strings.Join(m.renderUnified(40, 10), "\n"),
		"split":   strings.Join(m.renderSideBySide(40, 10), "\n"),
	} {
		if strings.ContainsRune(rendered, '\t') {
			t.Fatalf("%s diff emitted a literal tab: %q", name, rendered)
		}
		if width := lipgloss.Width(rendered); width != 40 {
			t.Fatalf("%s diff width=%d, want 40", name, width)
		}
	}
}

func TestDiffModeAndHistorySelection(t *testing.T) {
	root := gitFixture(t)
	mustWrite(t, filepath.Join(root, "a.txt"), "two\n")
	m, err := New(root, config.Default(), true)
	if err != nil {
		t.Fatal(err)
	}
	m.width, m.height = 120, 40
	msg := m.Init()()
	updated, cmd := m.Update(msg)
	m = updated.(Model)
	if len(m.changes) != 1 || cmd == nil {
		t.Fatalf("changes/cmd=%d/%v", len(m.changes), cmd)
	}
	m.history = []gitview.Commit{{Hash: "abc", Short: "abc", Subject: "one"}}
	m.historyOpen = true
	m.historySelected = 1
	updated, cmd = m.updateHistory(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.diffRef != "abc" || m.historyOpen || cmd == nil {
		t.Fatalf("history state=%q/%v/%v", m.diffRef, m.historyOpen, cmd)
	}
}

func TestViewsStayBoundedAtSmallAndLargeSizes(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 300; i++ {
		mustWrite(t, filepath.Join(root, "files", string(rune('a'+i%26)), "file.txt"), "x\n")
	}
	m, err := New(root, config.Default(), false)
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range [][2]int{{48, 12}, {160, 50}} {
		m.width, m.height = size[0], size[1]
		m.clampLayout()
		view := m.View()
		if strings.Count(view, "\n") > size[1]+4 {
			t.Fatalf("view lines=%d height=%d", strings.Count(view, "\n"), size[1])
		}
		for lineNumber, line := range strings.Split(view, "\n") {
			if width := lipgloss.Width(line); width >= size[0] {
				t.Fatalf("line %d width=%d terminal width=%d", lineNumber, width, size[0])
			}
		}
	}
}

func mustWrite(t *testing.T, path, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
		t.Fatal(err)
	}
}
func gitFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	runGit(t, root, "config", "user.name", "Navia Test")
	runGit(t, root, "config", "user.email", "navia@example.com")
	mustWrite(t, filepath.Join(root, "a.txt"), "one\n")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-qm", "initial")
	return root
}
func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}
