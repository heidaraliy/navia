package fs

import (
	"fmt"
	"image"
	"math"
	"os"
)

const maxPreviewImagePixels = 40_000_000

// RenderImage renders an image with upper-half blocks. Each terminal cell
// carries two vertical pixels, so this works through tmux without relying on a
// terminal-specific image protocol.
func RenderImage(path string, maxColumns, maxRows int) ([]string, error) {
	if maxColumns < 1 || maxRows < 1 {
		return nil, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	config, _, err := image.DecodeConfig(file)
	if err != nil {
		return nil, err
	}
	if config.Width < 1 || config.Height < 1 || int64(config.Width)*int64(config.Height) > maxPreviewImagePixels {
		return nil, fmt.Errorf("image dimensions are too large for preview")
	}
	if _, err := file.Seek(0, 0); err != nil {
		return nil, err
	}
	img, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	scale := math.Min(float64(maxColumns)/float64(width), float64(maxRows*2)/float64(height))
	if scale > 4 {
		scale = 4
	}
	renderWidth := max(1, int(math.Round(float64(width)*scale)))
	renderHeight := max(1, int(math.Round(float64(height)*scale)))
	indent := (maxColumns - renderWidth) / 2
	lines := make([]string, 0, (renderHeight+1)/2)
	for y := 0; y < renderHeight; y += 2 {
		line := make([]byte, 0, indent+renderWidth*32)
		for range indent {
			line = append(line, ' ')
		}
		for x := 0; x < renderWidth; x++ {
			upper := sampleImage(img, bounds, x, y, renderWidth, renderHeight)
			lower := [3]uint8{40, 44, 52}
			if y+1 < renderHeight {
				lower = sampleImage(img, bounds, x, y+1, renderWidth, renderHeight)
			}
			line = fmt.Appendf(line, "\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm▀", upper[0], upper[1], upper[2], lower[0], lower[1], lower[2])
		}
		line = append(line, "\x1b[0m"...)
		lines = append(lines, string(line))
	}
	return lines, nil
}

func sampleImage(img image.Image, bounds image.Rectangle, x, y, width, height int) [3]uint8 {
	sourceX := bounds.Min.X + min(bounds.Dx()-1, x*bounds.Dx()/width)
	sourceY := bounds.Min.Y + min(bounds.Dy()-1, y*bounds.Dy()/height)
	r, g, b, a := img.At(sourceX, sourceY).RGBA()
	alpha := uint32(a >> 8)
	return [3]uint8{
		uint8(((r>>8)*alpha + 40*(255-alpha)) / 255),
		uint8(((g>>8)*alpha + 44*(255-alpha)) / 255),
		uint8(((b>>8)*alpha + 52*(255-alpha)) / 255),
	}
}
