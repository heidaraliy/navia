package fs

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const MaxSearchResults = 500
const MaxIndexedFiles = 20000

type SearchMatch struct {
	Entry   FileEntry
	Line    int
	Snippet string
}

func SearchFiles(root, query string, opts ScanOptions) ([]SearchMatch, error) {
	query = strings.ToLower(strings.TrimSpace(query))
	var matches []SearchMatch
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path != root && ShouldSkipName(d.Name(), opts) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if query == "" || strings.Contains(strings.ToLower(d.Name()), query) {
			info, err := d.Info()
			if err != nil {
				return nil
			}
			matches = append(matches, SearchMatch{Entry: NewEntry(path, info)})
			if query == "" && len(matches) >= MaxIndexedFiles {
				return filepath.SkipAll
			}
		}
		return nil
	})
	sort.SliceStable(matches, func(i, j int) bool {
		aDepth := strings.Count(matches[i].Entry.Path, string(filepath.Separator))
		bDepth := strings.Count(matches[j].Entry.Path, string(filepath.Separator))
		if aDepth != bDepth {
			return aDepth < bDepth
		}
		return strings.ToLower(matches[i].Entry.Name) < strings.ToLower(matches[j].Entry.Name)
	})
	if query != "" && len(matches) > MaxSearchResults {
		matches = matches[:MaxSearchResults]
	}
	return matches, err
}

func SearchText(root, query string, maxBytes int64, opts ScanOptions) ([]SearchMatch, error) {
	query = strings.ToLower(strings.TrimSpace(query))
	if len(query) < 2 {
		return nil, nil
	}
	var matches []SearchMatch
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path != root && ShouldSkipName(d.Name(), opts) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		match, ok := searchFileText(path, query, maxBytes)
		if !ok {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		match.Entry = NewEntry(path, info)
		matches = append(matches, match)
		if len(matches) >= MaxSearchResults {
			return filepath.SkipAll
		}
		return nil
	})
	return matches, err
}

func searchFileText(path, query string, maxBytes int64) (SearchMatch, bool) {
	file, err := os.Open(path)
	if err != nil {
		return SearchMatch{}, false
	}
	defer file.Close()
	if maxBytes <= 0 {
		maxBytes = 256 * 1024
	}
	data, err := io.ReadAll(io.LimitReader(file, maxBytes))
	if err != nil || bytes.Contains(data, []byte{0}) {
		return SearchMatch{}, false
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	line := 0
	for scanner.Scan() {
		line++
		text := scanner.Text()
		if strings.Contains(strings.ToLower(text), query) {
			return SearchMatch{Line: line, Snippet: strings.TrimSpace(text)}, true
		}
	}
	return SearchMatch{}, false
}
