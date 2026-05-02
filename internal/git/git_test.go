package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindRoot(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "repo")
	child := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := FindRoot(child); got != root {
		t.Fatalf("got %q want %q", got, root)
	}
}

func TestStatusSummarizesTrackedAndUntrackedChanges(t *testing.T) {
	root := initRepo(t)
	writeFile(t, filepath.Join(root, "tracked.txt"), "one\n")
	runGit(t, root, "add", "tracked.txt")
	runGit(t, root, "commit", "-m", "initial")

	writeFile(t, filepath.Join(root, "tracked.txt"), "one\ntwo\n")
	writeFile(t, filepath.Join(root, "new.txt"), "alpha\nbeta\n")
	if err := os.Remove(filepath.Join(root, "tracked.txt")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "added.txt"), "first\n")

	changes, summary, err := Status(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 3 {
		t.Fatalf("changes = %#v, want 3", changes)
	}
	if summary.FilesAdded != 2 || summary.FilesRemoved != 1 || summary.FilesChanged != 0 {
		t.Fatalf("file summary = %#v, want added=2 removed=1 changed=0", summary)
	}
	if summary.LinesAdded != 3 || summary.LinesRemoved != 1 || summary.LinesChanged != 4 {
		t.Fatalf("line summary = %#v, want +3 -1 total=4", summary)
	}
}

func TestUntrackedDiff(t *testing.T) {
	root := initRepo(t)
	writeFile(t, filepath.Join(root, "new.txt"), "alpha\nbeta\n")
	diff, err := Diff(root, Change{Path: "new.txt", Kind: ChangeUntracked}, 1024)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"new file mode", "+++ b/new.txt", "+alpha", "+beta"} {
		if !contains(diff, want) {
			t.Fatalf("diff missing %q:\n%s", want, diff)
		}
	}
}

func TestDiffWorksBeforeFirstCommit(t *testing.T) {
	root := initRepo(t)
	writeFile(t, filepath.Join(root, "tracked.txt"), "one\n")
	runGit(t, root, "add", "tracked.txt")
	changes, summary, err := Status(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0].Kind != ChangeAdded {
		t.Fatalf("changes = %#v, want one added file", changes)
	}
	if summary.LinesAdded != 1 {
		t.Fatalf("summary = %#v, want one added line", summary)
	}
	diff, err := Diff(root, changes[0], 1024)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(diff, "+one") {
		t.Fatalf("diff missing added line:\n%s", diff)
	}
}

func TestValidateRelPathRejectsEscapes(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{"../outside", "/tmp/outside"} {
		if err := validateRelPath(root, path); err == nil {
			t.Fatalf("validateRelPath(%q) succeeded, want error", path)
		}
	}
}

func initRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.name", "Navia Test")
	runGit(t, root, "config", "user.email", "navia@example.invalid")
	return root
}

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
