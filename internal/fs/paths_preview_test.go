package fs

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveDirAndSubpath(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file.txt")
	must(t, os.WriteFile(file, []byte("x"), 0o644))

	resolved, err := ResolveDir(file)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != filepath.Clean(dir) {
		t.Fatalf("resolved = %q want %q", resolved, filepath.Clean(dir))
	}
	if _, err := ResolveDir(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("ResolveDir missing path succeeded")
	}
	if !IsSubpath(dir, file) {
		t.Fatalf("%q should be under %q", file, dir)
	}
	if IsSubpath(dir, filepath.Dir(dir)) {
		t.Fatalf("%q should not be under %q", filepath.Dir(dir), dir)
	}
	if !IsSubpath(dir, dir) {
		t.Fatalf("%q should be under itself", dir)
	}
}

func TestUniquePathAndFormatSize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	must(t, os.WriteFile(path, []byte("x"), 0o644))
	must(t, os.WriteFile(filepath.Join(dir, "file_1.txt"), []byte("x"), 0o644))

	if got := UniquePath(filepath.Join(dir, "free.txt")); got != filepath.Join(dir, "free.txt") {
		t.Fatalf("UniquePath free = %q", got)
	}
	if got := UniquePath(path); got != filepath.Join(dir, "file_2.txt") {
		t.Fatalf("UniquePath occupied = %q", got)
	}
	if got := strconvItoa(0); got != "0" {
		t.Fatalf("strconvItoa(0) = %q", got)
	}

	tests := map[int64]string{12: "12 B", 1024: "1.0 KiB", 1024 * 1024: "1.0 MiB"}
	for size, want := range tests {
		if got := FormatSize(size); got != want {
			t.Fatalf("FormatSize(%d) = %q want %q", size, got, want)
		}
	}
}

func TestBuildPreviewVariants(t *testing.T) {
	dir := t.TempDir()
	if got := BuildPreview(filepath.Join(dir, "missing"), 10); got.Kind != PreviewError {
		t.Fatalf("missing preview kind = %s", got.Kind)
	}

	subdir := filepath.Join(dir, "sub")
	must(t, os.Mkdir(subdir, 0o755))
	must(t, os.Mkdir(filepath.Join(subdir, "nested"), 0o755))
	must(t, os.WriteFile(filepath.Join(subdir, "file.txt"), []byte("x"), 0o644))
	dirPreview := BuildPreview(subdir, 10)
	if dirPreview.Kind != PreviewDir || !strings.Contains(dirPreview.Content, "1 folders") || !strings.Contains(dirPreview.Content, "1 files") {
		t.Fatalf("dir preview = %#v", dirPreview)
	}

	binaryPath := filepath.Join(dir, "data.bin")
	must(t, os.WriteFile(binaryPath, []byte{1, 0, 2}, 0o644))
	if got := BuildPreview(binaryPath, 10); got.Kind != PreviewBinary {
		t.Fatalf("binary preview kind = %s", got.Kind)
	}

	textPath := filepath.Join(dir, "empty-limit.txt")
	must(t, os.WriteFile(textPath, []byte("hello"), 0o644))
	if got := BuildPreview(textPath, 0); got.Kind != PreviewText || got.Content != "hello" {
		t.Fatalf("text preview = %#v", got)
	}

	pngPath := filepath.Join(dir, "pixel.png")
	pngBytes, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAAAAAA6fptVAAAACklEQVR4nGNgAAAAAgAB4iG8MwAAAABJRU5ErkJggg==")
	if err != nil {
		t.Fatal(err)
	}
	must(t, os.WriteFile(pngPath, pngBytes, 0o644))
	if got := BuildPreview(pngPath, 10); got.Kind != PreviewImage || !strings.Contains(got.Content, "Dimensions: 1x1") {
		t.Fatalf("image preview = %#v", got)
	}

	badImage := filepath.Join(dir, "bad.png")
	must(t, os.WriteFile(badImage, []byte("not an image"), 0o644))
	if got := BuildPreview(badImage, 10); got.Kind != PreviewText {
		t.Fatalf("bad image should fall back to text, got %#v", got)
	}

	if !isImageExt(".JPG") || isImageExt(".txt") {
		t.Fatal("isImageExt case handling failed")
	}
}
