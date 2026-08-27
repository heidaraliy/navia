package fs

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

const MaxSearchResults = 500
const MaxIndexedFiles = 20000

type SearchMatch struct {
	Entry   FileEntry
	Line    int
	Column  int
	Score   int
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
		if path == root && len(tokens) > 0 {
			return nil
		}
		if score, ok := fileMatchScore(root, path, query, tokens); len(tokens) == 0 || ok {
			info, err := d.Info()
			if err != nil {
				return nil
			}
			matches = append(matches, SearchMatch{Entry: NewEntry(path, info), Score: score})
			if len(tokens) == 0 && len(matches) >= MaxIndexedFiles {
				return filepath.SkipAll
			}
		}
		return nil
	})
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].Score != matches[j].Score {
			return matches[i].Score > matches[j].Score
		}
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
	_, ok := fileMatchScore(root, path, "", tokens)
	return ok
}

func fileMatchScore(root, path, query string, tokens []string) (int, bool) {
	name := strings.ToLower(filepath.Base(path))
	rel := strings.ToLower(searchRelativePath(root, path))
	score := 0
	normalizedQuery := strings.Join(searchTokens(query), "")
	if normalizedQuery != "" && strings.Contains(strings.ReplaceAll(name, " ", ""), normalizedQuery) {
		score += 200
	}
	for _, token := range tokens {
		switch {
		case strings.Contains(name, token):
			score += 100
		case strings.Contains(rel, token):
			score += 60
		case fuzzyToken(name, token):
			score += 20
		case fuzzyToken(rel, token):
			score += 10
		default:
			return 0, false
		}
	}
	return score, true
}

func fuzzyToken(value, token string) bool {
	if token == "" {
		return true
	}
	tokenRunes := []rune(token)
	i := 0
	for _, r := range value {
		if r == tokenRunes[i] {
			i++
			if i == len(tokenRunes) {
				return true
			}
		}
	}
	return false
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

func SearchSymbolDefinitions(root, symbol string, maxBytes int64, opts ScanOptions) ([]SearchMatch, error) {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return nil, nil
	}
	patterns := definitionPatterns(symbol)
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
		fileMatches := searchFileSymbolDefinitions(path, symbol, maxBytes, patterns, MaxSearchResults-len(matches))
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
		return nil
	})
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].Score != matches[j].Score {
			return matches[i].Score > matches[j].Score
		}
		if matches[i].Entry.Path != matches[j].Entry.Path {
			return matches[i].Entry.Path < matches[j].Entry.Path
		}
		return matches[i].Line < matches[j].Line
	})
	return matches, err
}

func SearchSymbolReferences(root, symbol string, maxBytes int64, opts ScanOptions) ([]SearchMatch, error) {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
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
		fileMatches := searchFileSymbolReferences(path, symbol, maxBytes, MaxSearchResults-len(matches))
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
		return nil
	})
	return matches, err
}

type definitionPattern struct {
	re    *regexp.Regexp
	score int
}

func definitionPatterns(symbol string) []definitionPattern {
	quoted := regexp.QuoteMeta(symbol)
	ws := `\s*`
	return []definitionPattern{
		{regexp.MustCompile(`^` + ws + `func\s+(?:\([^)]*\)\s*)?` + quoted + `\s*\(`), 120},
		{regexp.MustCompile(`^` + ws + `(?:type|const|var)\s+` + quoted + `\b`), 115},
		{regexp.MustCompile(`^` + ws + `(?:export\s+)?(?:async\s+)?function\s+` + quoted + `\s*\(`), 120},
		{regexp.MustCompile(`^` + ws + `(?:export\s+)?(?:const|let|var)\s+` + quoted + `\b`), 115},
		{regexp.MustCompile(`^` + ws + `(?:export\s+)?(?:class|interface|type|enum)\s+` + quoted + `\b`), 115},
		{regexp.MustCompile(`^` + ws + `(?:public\s+|private\s+|protected\s+|static\s+|async\s+)*` + quoted + `\s*\(`), 90},
		{regexp.MustCompile(`^` + ws + `(?:class|struct|enum(?:\s+class)?)\s+` + quoted + `\b`), 115},
		{regexp.MustCompile(`^` + ws + `(?:[\w:<>,~*&]+\s+)+(?:(?:\w+::)*` + quoted + `|` + quoted + `)\s*\([^;]*\)\s*(?:(?:const|override|final|noexcept)\s*)*(?:\{|:)`), 95},
		{regexp.MustCompile(`^` + ws + `(?:local\s+)?function\s+(?:[\w.]+[:.])?` + quoted + `\s*\(`), 120},
		{regexp.MustCompile(`^` + ws + `(?:local\s+)?` + quoted + `\s*=\s*function\s*\(`), 110},
	}
}

func searchFileSymbolDefinitions(path, symbol string, maxBytes int64, patterns []definitionPattern, maxMatches int) []SearchMatch {
	data := readSearchFile(path, maxBytes)
	if len(data) == 0 {
		return nil
	}
	var matches []SearchMatch
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		bestScore := 0
		for _, pattern := range patterns {
			if pattern.re.MatchString(line) && pattern.score > bestScore {
				bestScore = pattern.score
			}
		}
		if bestScore == 0 {
			continue
		}
		matches = append(matches, SearchMatch{
			Line:    lineNo,
			Column:  symbolColumn(line, symbol) + 1,
			Score:   bestScore,
			Snippet: strings.TrimSpace(line),
		})
		if len(matches) >= maxMatches {
			return matches
		}
	}
	return matches
}

func searchFileSymbolReferences(path, symbol string, maxBytes int64, maxMatches int) []SearchMatch {
	data := readSearchFile(path, maxBytes)
	if len(data) == 0 {
		return nil
	}
	var matches []SearchMatch
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		col := symbolColumn(line, symbol)
		if col < 0 {
			continue
		}
		matches = append(matches, SearchMatch{
			Line:    lineNo,
			Column:  col + 1,
			Snippet: strings.TrimSpace(line),
		})
		if len(matches) >= maxMatches {
			return matches
		}
	}
	return matches
}

func readSearchFile(path string, maxBytes int64) []byte {
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
	return data
}

func symbolColumn(line, symbol string) int {
	for offset := 0; offset <= len(line)-len(symbol); {
		idx := strings.Index(line[offset:], symbol)
		if idx < 0 {
			return -1
		}
		idx += offset
		end := idx + len(symbol)
		if isSymbolBoundary(line, idx, end) {
			return idx
		}
		offset = idx + len(symbol)
	}
	return -1
}

func isSymbolBoundary(line string, start, end int) bool {
	return (start == 0 || !isIdentifierRune(rune(line[start-1]))) &&
		(end >= len(line) || !isIdentifierRune(rune(line[end])))
}

func isIdentifierRune(r rune) bool {
	return r == '_' || r == '$' || unicode.IsLetter(r) || unicode.IsDigit(r)
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
