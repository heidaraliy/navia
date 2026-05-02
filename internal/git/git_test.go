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
	for _, path := range []string{"../outside", "/tmp/outside", ""} {
		if err := validateRelPath(root, path); err == nil {
			t.Fatalf("validateRelPath(%q) succeeded, want error", path)
		}
	}
	if err := validateRelPath("", "file.txt"); err == nil {
		t.Fatal("validateRelPath empty root succeeded")
	}
}

func TestRelAndPorcelainParsing(t *testing.T) {
	root := t.TempDir()
	if got := Rel("", "/tmp/file"); got != "/tmp/file" {
		t.Fatalf("Rel empty root = %q", got)
	}
	if got := Rel(root, root); got != "." {
		t.Fatalf("Rel root = %q", got)
	}
	if got := Rel(root, filepath.Join(root, "a.txt")); got != "a.txt" {
		t.Fatalf("Rel child = %q", got)
	}

	out := []byte("R  new.txt\x00old.txt\x00C  copy.txt\x00source.txt\x00?? new.txt\x00")
	changes, err := parsePorcelainStatus(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 3 || changes[0].OldPath != "old.txt" || changes[1].OldPath != "source.txt" || changes[2].Kind != ChangeUntracked {
		t.Fatalf("changes = %#v", changes)
	}
	for _, malformed := range [][]byte{[]byte("x\x00"), []byte("R  new.txt")} {
		if _, err := parsePorcelainStatus(malformed); err == nil {
			t.Fatalf("parsePorcelainStatus(%q) returned nil", malformed)
		}
	}
}

func TestGitMutatingOperations(t *testing.T) {
	origin := filepath.Join(t.TempDir(), "origin.git")
	runGit(t, "", "init", "--bare", origin)
	root := initRepo(t)
	runGit(t, root, "remote", "add", "origin", origin)

	writeFile(t, filepath.Join(root, "tracked.txt"), "one\n")
	if err := Stage(root, "tracked.txt"); err != nil {
		t.Fatal(err)
	}
	if err := Commit(root, "initial"); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "tracked.txt"), "one\ntwo\n")
	if err := Stage(root, "tracked.txt"); err != nil {
		t.Fatal(err)
	}
	if err := Unstage(root, "tracked.txt"); err != nil {
		t.Fatal(err)
	}
	if err := Restore(root, Change{Path: "tracked.txt", WorktreeStatus: 'M'}); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(root, "tracked.txt")); err != nil || string(data) != "one\n" {
		t.Fatalf("restore tracked data = %q %v", string(data), err)
	}

	writeFile(t, filepath.Join(root, "remove.txt"), "remove\n")
	if err := Remove(root, Change{Path: "remove.txt", Kind: ChangeUntracked}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "remove.txt")); !os.IsNotExist(err) {
		t.Fatalf("remove untracked stat = %v", err)
	}

	writeFile(t, filepath.Join(root, "tracked.txt"), "changed\n")
	if err := Remove(root, Change{Path: "tracked.txt"}); err != nil {
		t.Fatal(err)
	}
	if err := Commit(root, "remove tracked"); err != nil {
		t.Fatal(err)
	}
	if err := Push(root); err != nil {
		t.Fatal(err)
	}
	if err := Commit(root, ""); err == nil {
		t.Fatal("empty commit message succeeded")
	}
}

func TestGitErrorBranchesAndDiffVariants(t *testing.T) {
	root := initRepo(t)
	if err := Stage(root, "../outside"); err == nil {
		t.Fatal("Stage escape succeeded")
	}
	if err := Unstage(root, "../outside"); err == nil {
		t.Fatal("Unstage escape succeeded")
	}
	if err := Restore(root, Change{Path: "../outside"}); err == nil {
		t.Fatal("Restore escape succeeded")
	}
	if err := Remove(root, Change{Path: "../outside"}); err == nil {
		t.Fatal("Remove escape succeeded")
	}
	if _, err := Diff(root, Change{Path: "../outside"}, 10); err == nil {
		t.Fatal("Diff escape succeeded")
	}
	if _, err := Diff(root, Change{Path: "missing.txt", Kind: ChangeUntracked}, 10); err == nil {
		t.Fatal("Diff missing untracked succeeded")
	}
	writeFile(t, filepath.Join(root, "binary.bin"), string([]byte{0xff, 0x00}))
	if diff, err := Diff(root, Change{Path: "binary.bin", Kind: ChangeUntracked}, 10); err != nil || !strings.Contains(diff, "Binary untracked file") {
		t.Fatalf("binary diff = %q %v", diff, err)
	}
	mustMkdir(t, filepath.Join(root, "newdir"))
	if diff, err := Diff(root, Change{Path: "newdir", Kind: ChangeUntracked}, 10); err != nil || !strings.Contains(diff, "Untracked directory") {
		t.Fatalf("dir diff = %q %v", diff, err)
	}
	writeFile(t, filepath.Join(root, "long.txt"), "abcdef")
	if diff, err := Diff(root, Change{Path: "long.txt", Kind: ChangeUntracked}, 3); err != nil || !strings.Contains(diff, "truncated") {
		t.Fatalf("truncated diff = %q %v", diff, err)
	}
	if _, err := gitOutput(root, "not-a-real-command"); err == nil {
		t.Fatal("gitOutput bad command succeeded")
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
	gitArgs := args
	if root != "" {
		gitArgs = append([]string{"-C", root}, args...)
	}
	cmd := exec.Command("git", gitArgs...)
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

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}
