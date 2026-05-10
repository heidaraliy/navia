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
	query = strings.TrimSpace(query)
	tokens := searchTokens(query)
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
		if len(tokens) == 0 || matchFileTokens(root, path, tokens) {
			info, err := d.Info()
			if err != nil {
				return nil
			}
			matches = append(matches, SearchMatch{Entry: NewEntry(path, info)})
			if len(tokens) == 0 && len(matches) >= MaxIndexedFiles {
				return filepath.SkipAll
			}
		}
		return nil
	})
	sort.SliceStable(matches, func(i, j int) bool {
		aDepth := strings.Count(searchRelativePath(root, matches[i].Entry.Path), "/")
		bDepth := strings.Count(searchRelativePath(root, matches[j].Entry.Path), "/")
		if aDepth != bDepth {
			return aDepth < bDepth
		}
		aPath := strings.ToLower(searchRelativePath(root, matches[i].Entry.Path))
		bPath := strings.ToLower(searchRelativePath(root, matches[j].Entry.Path))
		return aPath < bPath
	})
	if len(tokens) != 0 && len(matches) > MaxSearchResults {
		matches = matches[:MaxSearchResults]
	}
	return matches, err
}

func MatchFileQuery(root, path, query string) bool {
	tokens := searchTokens(query)
	if len(tokens) == 0 {
		return true
	}
	return matchFileTokens(root, path, tokens)
}

func matchFileTokens(root, path string, tokens []string) bool {
	name := strings.ToLower(filepath.Base(path))
	rel := strings.ToLower(searchRelativePath(root, path))
	for _, token := range tokens {
		if !strings.Contains(name, token) && !strings.Contains(rel, token) {
			return false
		}
	}
	return true
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
		fileMatches := searchFileText(path, query, maxBytes, MaxSearchResults-len(matches))
		if len(fileMatches) == 0 {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		entry := NewEntry(path, info)
		for _, match := range fileMatches {
			match.Entry = entry
			matches = append(matches, match)
			if len(matches) >= MaxSearchResults {
				return filepath.SkipAll
			}
		}
		if len(matches) >= MaxSearchResults {
			return filepath.SkipAll
		}
		return nil
	})
	return matches, err
}

func searchFileText(path, query string, maxBytes int64, maxMatches int) []SearchMatch {
	if maxMatches <= 0 {
		return nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()
	if maxBytes <= 0 {
		maxBytes = 256 * 1024
	}
	data, err := io.ReadAll(io.LimitReader(file, maxBytes))
	if err != nil || bytes.Contains(data, []byte{0}) {
		return nil
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	line := 0
	var matches []SearchMatch
	for scanner.Scan() {
		line++
		text := scanner.Text()
		if strings.Contains(strings.ToLower(text), query) {
			matches = append(matches, SearchMatch{Line: line, Snippet: strings.TrimSpace(text)})
			if len(matches) >= maxMatches {
				break
			}
		}
	}
	return matches
}

func searchTokens(query string) []string {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(query)))
	tokens := fields[:0]
	for _, field := range fields {
		if field != "" {
			tokens = append(tokens, field)
		}
	}
	return tokens
}

func searchRelativePath(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil && rel != "." {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(path)
}
