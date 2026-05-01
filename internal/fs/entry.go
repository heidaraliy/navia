package fs

import (
	"path/filepath"
	"strings"
	"time"
)

type FileEntry struct {
	Name      string
	Path      string
	IsDir     bool
	Size      int64
	ModTime   time.Time
	IsHidden  bool
	Extension string
}

func NewEntry(path string, info interface {
	Name() string
	IsDir() bool
	Size() int64
	ModTime() time.Time
}) FileEntry {
	name := info.Name()
	return FileEntry{
		Name:      name,
		Path:      path,
		IsDir:     info.IsDir(),
		Size:      info.Size(),
		ModTime:   info.ModTime(),
		IsHidden:  strings.HasPrefix(name, "."),
		Extension: strings.ToLower(filepath.Ext(name)),
	}
}
