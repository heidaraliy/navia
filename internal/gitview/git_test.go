package gitview

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDiffPresentationHidesRawPreambleAndCompactsHunk(t *testing.T) {
	lines, _ := parseDiff([]byte("diff --git a/a.go b/a.go\nindex abc..def 100644\n--- a/a.go\n+++ b/a.go\n@@ -10,2 +10,3 @@ func x\n-old\n+new\n"))
	if len(lines) != 3 {
		t.Fatalf("lines = %#v", lines)
	}
	if lines[0].Kind != Hunk || lines[0].Text != "── -10,2  +10,3 ──" {
		t.Fatalf("hunk = %#v", lines[0])
	}
}

func TestDiffPresentationCompactsFileMetadata(t *testing.T) {
	lines, _ := parseDiff([]byte("diff --git a/a b/a\nnew file mode 100644\n--- /dev/null\n+++ b/a\n@@ -0,0 +1 @@\n+x\n"))
	if len(lines) < 1 || lines[0].Text != "new file" {
		t.Fatalf("metadata = %#v", lines)
	}
}

func TestStatusDiffAndAggregate(t *testing.T) {
	root := fixture(t)
	write(t, root, "modified.go", "package demo\n\nfunc value() int { return 2 }\nfunc added() {}\n")
	if err := os.Remove(filepath.Join(root, "deleted.txt")); err != nil {
		t.Fatal(err)
	}
	write(t, root, "new file.go", "first\nsecond\n")

	changes, err := Status(root)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]Kind{"deleted.txt": Deleted, "modified.go": Modified, "new file.go": Untracked}
	if len(changes) != len(want) {
		t.Fatalf("changes = %#v", changes)
	}
	for _, change := range changes {
		if change.Kind != want[change.Path] {
			t.Errorf("%s kind = %c, want %c", change.Path, change.Kind, want[change.Path])
		}
	}

	counts, err := Aggregate(root, changes)
	if err != nil {
		t.Fatal(err)
	}
	if counts.FilesNew != 1 || counts.FilesModified != 1 || counts.FilesDeleted != 1 {
		t.Fatalf("file counts = %#v", counts)
	}
	if counts.LinesNew != 3 || counts.LinesModified != 1 || counts.LinesDeleted != 1 {
		t.Fatalf("line counts = %#v", counts)
	}

	modified := find(t, changes, "modified.go")
	diff, err := Diff(root, modified, 1024*1024)
	if err != nil {
		t.Fatal(err)
	}
	if diff.Counts.LinesModified != 1 || diff.Counts.LinesNew != 1 || len(diff.Side) >= len(diff.Lines) {
		t.Fatalf("diff counts/paired rows = %#v/%d/%d", diff.Counts, len(diff.Side), len(diff.Lines))
	}

	for query, path := range map[string]string{"return 2": "modified.go", "delete me": "deleted.txt", "second": "new file.go"} {
		matches, err := SearchContent(root, query, changes)
		if err != nil || !matches[path] {
			t.Errorf("search %q = %#v, %v; missing %q", query, matches, err, path)
		}
	}
}

func TestUnbornRepositoryAndBinary(t *testing.T) {
	root := t.TempDir()
	run(t, root, "init", "-q")
	write(t, root, "first.txt", "hello\n")
	writeBytes(t, root, "binary.bin", []byte{0, 1, 2})
	changes, err := Status(root)
	if err != nil {
		t.Fatal(err)
	}
	counts, err := Aggregate(root, changes)
	if err != nil || counts.FilesNew != 2 || counts.LinesNew != 1 {
		t.Fatalf("aggregate = %#v, %v", counts, err)
	}
	diff, err := Diff(root, find(t, changes, "binary.bin"), 1024)
	if err != nil || !diff.Binary {
		t.Fatalf("binary diff = %#v, %v", diff, err)
	}
}

func TestHunkCountsPairsWithinEachHunk(t *testing.T) {
	raw := []byte("@@ -1,2 +1,3 @@\n-old\n-old2\n+new\n+new2\n+extra\n@@ -10,2 +11,1 @@\n-gone\n-still gone\n+replacement\n")
	add, modified, deleted := hunkCounts(raw)
	if add != 1 || modified != 3 || deleted != 1 {
		t.Fatalf("counts = +%d ~%d -%d", add, modified, deleted)
	}
}

func TestCommitHistoryStatusDiffAndPagination(t *testing.T) {
	root := fixture(t)
	write(t, root, "modified.go", "package demo\n\nfunc value() int { return 3 }\nfunc newer() {}\n")
	run(t, root, "add", ".")
	run(t, root, "commit", "-qm", "second commit")
	commits, err := Commits(root, 0, 1)
	if err != nil || len(commits) != 1 || commits[0].Subject != "second commit" {
		t.Fatalf("commits = %#v, %v", commits, err)
	}
	more, err := Commits(root, 1, 1)
	if err != nil || len(more) != 1 || more[0].Subject != "day" {
		t.Fatalf("more = %#v, %v", more, err)
	}
	changes, err := StatusCommit(root, commits[0].Hash)
	if err != nil || len(changes) != 1 || changes[0].Path != "modified.go" {
		t.Fatalf("changes = %#v, %v", changes, err)
	}
	counts, err := AggregateCommit(root, commits[0].Hash, changes)
	if err != nil || counts.LinesModified != 1 || counts.LinesNew != 1 {
		t.Fatalf("counts = %#v, %v", counts, err)
	}
	diff, err := DiffCommit(root, commits[0].Hash, changes[0], 1024*1024)
	if err != nil || diff.Counts.LinesModified != 1 || diff.Counts.LinesNew != 1 {
		t.Fatalf("diff = %#v, %v", diff.Counts, err)
	}
	matches, err := SearchContentCommit(root, commits[0].Hash, "newer", changes)
	if err != nil || !matches["modified.go"] {
		t.Fatalf("matches = %#v, %v", matches, err)
	}
}

func TestRootCommitUsesEmptyTree(t *testing.T) {
	root := fixture(t)
	out, err := exec.Command("git", "-C", root, "rev-list", "--max-parents=0", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	hash := string(bytes.TrimSpace(out))
	changes, err := StatusCommit(root, hash)
	if err != nil || len(changes) != 2 {
		t.Fatalf("root changes = %#v, %v", changes, err)
	}
}

func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	run(t, root, "init", "-q")
	run(t, root, "config", "user.name", "Drift Test")
	run(t, root, "config", "user.email", "drift@example.com")
	write(t, root, "modified.go", "package demo\n\nfunc value() int { return 1 }\n")
	write(t, root, "deleted.txt", "delete me\n")
	run(t, root, "add", ".")
	run(t, root, "commit", "-qm", "day")
	return root
}

func run(t *testing.T, root string, args ...string) {
	t.Helper()
	if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}
func write(t *testing.T, root, path, value string) {
	t.Helper()
	writeBytes(t, root, path, []byte(value))
}
func writeBytes(t *testing.T, root, path string, value []byte) {
	t.Helper()
	full := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, value, 0o644); err != nil {
		t.Fatal(err)
	}
}
func find(t *testing.T, changes []Change, path string) Change {
	t.Helper()
	for _, change := range changes {
		if change.Path == path {
			return change
		}
	}
	t.Fatalf("missing %s", path)
	return Change{}
}
