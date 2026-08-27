package fs

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	termansi "github.com/charmbracelet/x/ansi"
)

func TestRenderImageUsesHalfBlocksWithinBounds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "colors.png")
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	img.Set(1, 0, color.RGBA{G: 255, A: 255})
	img.Set(0, 1, color.RGBA{B: 255, A: 255})
	img.Set(1, 1, color.RGBA{R: 255, G: 255, A: 255})
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, img); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	lines, err := RenderImage(path, 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 || termansi.StringWidth(lines[0]) != 2 {
		t.Fatalf("rendered image = %#v", lines)
	}
	for _, want := range []string{"\x1b[38;2;255;0;0m", "\x1b[48;2;0;0;255m", "▀"} {
		if !strings.Contains(lines[0], want) {
			t.Fatalf("render missing %q: %q", want, lines[0])
		}
	}
}
