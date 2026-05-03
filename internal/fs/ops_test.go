package fs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRenameAndCreateValidateNames(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "old.txt")
	must(t, os.WriteFile(src, []byte("x"), 0o644))

	target, err := Rename(src, "new.txt")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(target) != "new.txt" {
		t.Fatalf("target = %q", target)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("renamed file missing: %v", err)
	}

	if _, err := Rename(target, ""); err == nil {
		t.Fatal("Rename accepted empty name")
	}
	for _, name := range []string{"../escape.txt", "nested/file.txt", "/tmp/escape.txt", ".", ".."} {
		if _, err := Rename(target, name); err == nil {
			t.Fatalf("Rename accepted unsafe name %q", name)
		}
	}
	existing := filepath.Join(dir, "exists.txt")
	must(t, os.WriteFile(existing, []byte("x"), 0o644))
	if _, err := Rename(target, "exists.txt"); err == nil {
		t.Fatal("Rename overwrote existing path")
	}

	filePath, err := CreateFile(dir, "created.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filePath); err != nil {
		t.Fatalf("created file missing: %v", err)
	}
	if _, err := CreateFile(dir, ""); err == nil {
		t.Fatal("CreateFile accepted empty name")
	}
	for _, name := range []string{"../escape.txt", "nested/file.txt", "/tmp/escape.txt", ".", ".."} {
		if _, err := CreateFile(dir, name); err == nil {
			t.Fatalf("CreateFile accepted unsafe name %q", name)
		}
	}
	if _, err := CreateFile(dir, "created.txt"); err == nil {
		t.Fatal("CreateFile overwrote existing file")
	}

	dirPath, err := CreateDir(dir, "created-dir")
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(dirPath); err != nil || !info.IsDir() {
		t.Fatalf("created dir missing or not dir: %v %#v", err, info)
	}
	if _, err := CreateDir(dir, ""); err == nil {
		t.Fatal("CreateDir accepted empty name")
	}
	for _, name := range []string{"../escape", "nested/dir", "/tmp/escape", ".", ".."} {
		if _, err := CreateDir(dir, name); err == nil {
			t.Fatalf("CreateDir accepted unsafe name %q", name)
		}
	}
}

func TestCopyPathCopiesDirectoriesAndErrors(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	must(t, os.MkdirAll(filepath.Join(src, "nested"), 0o755))
	must(t, os.WriteFile(filepath.Join(src, "nested", "file.txt"), []byte("data"), 0o644))

	dst := filepath.Join(dir, "dst")
	if err := CopyPath(src, dst); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dst, "nested", "file.txt"))
	if err != nil || string(data) != "data" {
		t.Fatalf("bad copied file: %q %v", string(data), err)
	}
	if err := CopyPath(filepath.Join(dir, "missing"), filepath.Join(dir, "out")); err == nil {
		t.Fatal("CopyPath missing source succeeded")
	}
	if err := CopyPath(src, dst); err == nil {
		t.Fatal("CopyPath overwrote existing directory")
	}
}

func TestMovePathFallsBackWhenRenameCannotReplaceDirectory(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "file.txt")
	dst := filepath.Join(dir, "dst-dir")
	must(t, os.WriteFile(src, []byte("data"), 0o644))
	must(t, os.Mkdir(dst, 0o755))

	if err := MovePath(src, dst); err == nil {
		t.Fatal("MovePath file over existing directory succeeded")
	}
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("source should remain after failed fallback: %v", err)
	}
}

func TestMovePathFallsBackToCopyAndRemove(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	must(t, os.MkdirAll(src, 0o755))
	must(t, os.WriteFile(filepath.Join(src, "file.txt"), []byte("data"), 0o644))
	must(t, os.WriteFile(dst, []byte("blocks rename"), 0o644))

	if err := MovePath(src, dst); err == nil {
		t.Fatal("MovePath directory over file succeeded")
	}
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("source should remain after failed move: %v", err)
	}
}

func TestCopyAndMoveRejectDirectoryIntoItself(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	nested := filepath.Join(src, "nested")
	must(t, os.MkdirAll(nested, 0o755))
	must(t, os.WriteFile(filepath.Join(src, "file.txt"), []byte("data"), 0o644))

	if err := CopyPath(src, filepath.Join(nested, "copy")); err == nil {
		t.Fatal("CopyPath allowed directory copy into itself")
	}
	if err := MovePath(src, filepath.Join(nested, "moved")); err == nil {
		t.Fatal("MovePath allowed directory move into itself")
	}
	if _, err := os.Stat(filepath.Join(src, "file.txt")); err != nil {
		t.Fatalf("source should remain after rejected self operation: %v", err)
	}
}
