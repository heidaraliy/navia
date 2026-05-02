package fs

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type PreviewKind string

const (
	PreviewText   PreviewKind = "text"
	PreviewBinary PreviewKind = "binary"
	PreviewImage  PreviewKind = "image"
	PreviewDir    PreviewKind = "dir"
	PreviewError  PreviewKind = "error"
)

type Preview struct {
	Kind      PreviewKind
	Title     string
	Content   string
	Path      string
	Size      int64
	ModTime   time.Time
	Truncated bool
}

const MaxDirectoryPreviewEntries = 1000

func BuildPreview(path string, maxBytes int64) Preview {
	return BuildPreviewWithOptions(path, maxBytes, ScanOptions{ShowHidden: true})
}

func BuildPreviewWithOptions(path string, maxBytes int64, opts ScanOptions) Preview {
	info, err := os.Stat(path)
	if err != nil {
		return Preview{Kind: PreviewError, Title: filepath.Base(path), Path: path, Content: err.Error()}
	}
	if info.IsDir() {
		dir, err := os.Open(path)
		if err != nil {
			return Preview{Kind: PreviewError, Title: info.Name(), Path: path, Size: info.Size(), ModTime: info.ModTime(), Content: err.Error()}
		}
		defer dir.Close()
		items, err := dir.ReadDir(MaxDirectoryPreviewEntries + 1)
		if err != nil && err != io.EOF {
			return Preview{Kind: PreviewError, Title: info.Name(), Path: path, Size: info.Size(), ModTime: info.ModTime(), Content: err.Error()}
		}
		dirs, files := 0, 0
		visible := 0
		for _, item := range items {
			if ShouldSkipName(item.Name(), opts) {
				continue
			}
			visible++
			if visible > MaxDirectoryPreviewEntries {
				break
			}
			if item.IsDir() {
				dirs++
			} else {
				files++
			}
		}
		truncated := visible > MaxDirectoryPreviewEntries
		content := fmt.Sprintf("Directory\n\n%d folders\n%d files", dirs, files)
		if truncated {
			content += fmt.Sprintf("\n\n[preview limited to first %d entries]", MaxDirectoryPreviewEntries)
		}
		return Preview{Kind: PreviewDir, Title: info.Name(), Path: path, Size: info.Size(), ModTime: info.ModTime(), Content: content, Truncated: truncated}
	}
	if isImageExt(filepath.Ext(path)) {
		if preview, ok := imagePreview(path, info.Size(), info.ModTime()); ok {
			return preview
		}
	}
	return filePreview(path, info.Size(), info.ModTime(), maxBytes)
}

func filePreview(path string, size int64, modTime time.Time, maxBytes int64) Preview {
	file, err := os.Open(path)
	if err != nil {
		return Preview{Kind: PreviewError, Title: filepath.Base(path), Path: path, Size: size, ModTime: modTime, Content: err.Error()}
	}
	defer file.Close()

	if maxBytes <= 0 {
		maxBytes = 256 * 1024
	}
	data, err := io.ReadAll(io.LimitReader(file, maxBytes))
	if err != nil {
		return Preview{Kind: PreviewError, Title: filepath.Base(path), Path: path, Size: size, ModTime: modTime, Content: err.Error()}
	}
	if bytes.Contains(data, []byte{0}) {
		return Preview{
			Kind:    PreviewBinary,
			Title:   filepath.Base(path),
			Path:    path,
			Size:    size,
			ModTime: modTime,
			Content: fmt.Sprintf("Binary file\n\nSize: %s\nExtension: %s",
				FormatSize(size), filepath.Ext(path)),
		}
	}
	text := string(data)
	truncated := size > int64(len(data))
	if truncated {
		text += "\n\n[preview truncated]"
	}
	return Preview{Kind: PreviewText, Title: filepath.Base(path), Path: path, Size: size, ModTime: modTime, Content: text, Truncated: truncated}
}

func imagePreview(path string, size int64, modTime time.Time) (Preview, bool) {
	file, err := os.Open(path)
	if err != nil {
		return Preview{}, false
	}
	defer file.Close()
	cfg, format, err := image.DecodeConfig(file)
	if err != nil {
		return Preview{}, false
	}
	content := fmt.Sprintf("Image\n\nFormat: %s\nDimensions: %dx%d\nSize: %s\nExtension: %s",
		format, cfg.Width, cfg.Height, FormatSize(size), filepath.Ext(path))
	return Preview{Kind: PreviewImage, Title: filepath.Base(path), Path: path, Size: size, ModTime: modTime, Content: content}, true
}

func isImageExt(ext string) bool {
	switch strings.ToLower(ext) {
	case ".png", ".jpg", ".jpeg", ".gif":
		return true
	default:
		return false
	}
}

func FormatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(size)/float64(div), "KMGTPE"[exp])
}
