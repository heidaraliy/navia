package git

import (
	"strings"
	"testing"
)

func TestParsePatchReview(t *testing.T) {
	patch := strings.Join([]string{
		"diff --git a/old.txt b/new.txt",
		"similarity index 88%",
		"rename from old.txt",
		"rename to new.txt",
		"index 1111111..2222222 100644",
		"--- a/old.txt",
		"+++ b/new.txt",
		"@@ -1,2 +1,2 @@",
		" one",
		"-two",
		"+three",
		"diff --git a/added.txt b/added.txt",
		"new file mode 100644",
		"index 0000000..3333333",
		"--- /dev/null",
		"+++ b/added.txt",
		"@@ -0,0 +1,2 @@",
		"+alpha",
		"+beta",
		"diff --git a/deleted.txt b/deleted.txt",
		"deleted file mode 100644",
		"index 4444444..0000000",
		"--- a/deleted.txt",
		"+++ /dev/null",
		"@@ -1 +0,0 @@",
		"-gone",
		"",
	}, "\n")

	review, err := ParsePatchReview([]byte(patch))
	if err != nil {
		t.Fatal(err)
	}
	if len(review.Changes) != 3 {
		t.Fatalf("changes = %#v", review.Changes)
	}
	if got := review.Changes[0]; got.Kind != ChangeRenamed || got.OldPath != "old.txt" || got.Path != "new.txt" || got.IndexStatus != 'R' {
		t.Fatalf("rename change = %#v", got)
	}
	if got := review.Changes[1]; got.Kind != ChangeAdded || got.Path != "added.txt" || got.IndexStatus != 'A' {
		t.Fatalf("added change = %#v", got)
	}
	if got := review.Changes[2]; got.Kind != ChangeDeleted || got.Path != "deleted.txt" || got.IndexStatus != 'D' {
		t.Fatalf("deleted change = %#v", got)
	}
	if review.Summary.FilesAdded != 1 || review.Summary.FilesChanged != 1 || review.Summary.FilesRemoved != 1 {
		t.Fatalf("file summary = %#v", review.Summary)
	}
	if review.Summary.LinesAdded != 3 || review.Summary.LinesRemoved != 2 || review.Summary.LinesChanged != 5 {
		t.Fatalf("line summary = %#v", review.Summary)
	}
	if !strings.Contains(review.Patches["added.txt"], "+alpha") {
		t.Fatalf("missing per-file patch: %#v", review.Patches)
	}
}

func TestParsePatchReviewRejectsNonPatchText(t *testing.T) {
	if _, err := ParsePatchReview([]byte("hello\n")); err == nil {
		t.Fatal("ParsePatchReview accepted non-patch text")
	}
}
