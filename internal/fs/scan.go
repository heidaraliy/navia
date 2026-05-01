package fs

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ScanOptions struct {
	ShowHidden    bool
	SortDirsFirst bool
}

func ScanDir(dir string, opts ScanOptions) ([]FileEntry, error) {
	items, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	entries := make([]FileEntry, 0, len(items))
	for _, item := range items {
		if !opts.ShowHidden && strings.HasPrefix(item.Name(), ".") {
			continue
		}
		info, err := item.Info()
		if err != nil {
			continue
		}
		entries = append(entries, NewEntry(filepath.Join(dir, item.Name()), info))
	}
	Sort(entries, opts.SortDirsFirst)
	return entries, nil
}

func Sort(entries []FileEntry, dirsFirst bool) {
	sort.SliceStable(entries, func(i, j int) bool {
		if dirsFirst && entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
}
