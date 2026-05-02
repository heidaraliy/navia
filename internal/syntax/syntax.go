package syntax

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

type Renderer struct {
	Name  string
	style *chroma.Style
	cache *lineCache
}

var fallbackStyle = styles.Get("dracula")

const maxCachedLines = 4096

type lineCache struct {
	mu    sync.Mutex
	order []string
	lines map[string]string
}

func Names() []string {
	names := []string{"navia", "dim", "mono", "amber"}
	sort.Strings(names)
	return names
}

func New(name string) Renderer {
	if name == "" {
		name = "navia"
	}
	style := styleFor(name)
	if style == nil {
		name = "navia"
		style = styleFor(name)
	}
	if style == nil {
		style = fallbackStyle
	}
	return Renderer{Name: name, style: style, cache: &lineCache{lines: make(map[string]string)}}
}

func Valid(name string) bool {
	return styleFor(name) != nil
}

func (r Renderer) HighlightLine(path, line string) string {
	return r.HighlightLineWithSearch(path, line, "")
}

func (r Renderer) HighlightLineWithSearch(path, line, query string) string {
	key := r.cacheKey(path, line, query)
	if cached, ok := r.cacheGet(key); ok {
		return cached
	}
	rendered := r.highlightLineWithSearch(path, line, query)
	r.cacheSet(key, rendered)
	return rendered
}

func (r Renderer) highlightLineWithSearch(path, line, query string) string {
	if r.style == nil {
		return highlightPlainSearch(line, query)
	}
	lexer := lexers.Match(path)
	if lexer == nil {
		lexer = lexers.Match(filepath.Base(path))
	}
	if lexer == nil {
		return highlightPlainSearch(line, query)
	}
	iterator, err := lexer.Tokenise(nil, line)
	if err != nil {
		return highlightPlainSearch(line, query)
	}
	var out strings.Builder
	for _, token := range iterator.Tokens() {
		entry := r.style.Get(token.Type)
		out.WriteString(renderTokenValue(entry, token.Value, query))
	}
	return out.String()
}

func (r Renderer) cacheKey(path, line, query string) string {
	return r.Name + "\x00" + path + "\x00" + query + "\x00" + line
}

func (r Renderer) cacheGet(key string) (string, bool) {
	if r.cache == nil {
		return "", false
	}
	r.cache.mu.Lock()
	defer r.cache.mu.Unlock()
	value, ok := r.cache.lines[key]
	return value, ok
}

func (r Renderer) cacheSet(key, value string) {
	if r.cache == nil {
		return
	}
	r.cache.mu.Lock()
	defer r.cache.mu.Unlock()
	if _, exists := r.cache.lines[key]; exists {
		r.cache.lines[key] = value
		return
	}
	if len(r.cache.order) >= maxCachedLines {
		delete(r.cache.lines, r.cache.order[0])
		copy(r.cache.order, r.cache.order[1:])
		r.cache.order[len(r.cache.order)-1] = key
	} else {
		r.cache.order = append(r.cache.order, key)
	}
	r.cache.lines[key] = value
}

func renderTokenValue(entry chroma.StyleEntry, value, query string) string {
	if query == "" {
		return styleValue(entry, value)
	}
	lowerValue := strings.ToLower(value)
	lowerQuery := strings.ToLower(query)
	if lowerQuery == "" || !strings.Contains(lowerValue, lowerQuery) {
		return styleValue(entry, value)
	}
	var out strings.Builder
	for len(value) > 0 {
		idx := strings.Index(strings.ToLower(value), lowerQuery)
		if idx < 0 {
			out.WriteString(styleValue(entry, value))
			break
		}
		if idx > 0 {
			out.WriteString(styleValue(entry, value[:idx]))
		}
		end := idx + len(query)
		out.WriteString("\x1b[38;2;17;24;39m\x1b[48;2;229;231;146m")
		out.WriteString(value[idx:end])
		out.WriteString("\x1b[0m")
		value = value[end:]
	}
	return out.String()
}

func styleValue(entry chroma.StyleEntry, value string) string {
	if value == "" {
		return ""
	}
	var out strings.Builder
	if entry.Colour.IsSet() {
		out.WriteString(fmt.Sprintf("\x1b[38;2;%d;%d;%dm", entry.Colour.Red(), entry.Colour.Green(), entry.Colour.Blue()))
	}
	if entry.Bold == chroma.Yes {
		out.WriteString("\x1b[1m")
	}
	out.WriteString(value)
	if entry.Colour.IsSet() || entry.Bold == chroma.Yes {
		out.WriteString("\x1b[0m")
	}
	return out.String()
}

func highlightPlainSearch(line, query string) string {
	if query == "" || !strings.Contains(strings.ToLower(line), strings.ToLower(query)) {
		return line
	}
	var out strings.Builder
	for len(line) > 0 {
		idx := strings.Index(strings.ToLower(line), strings.ToLower(query))
		if idx < 0 {
			out.WriteString(line)
			break
		}
		out.WriteString(line[:idx])
		end := idx + len(query)
		out.WriteString("\x1b[38;2;17;24;39m\x1b[48;2;229;231;146m")
		out.WriteString(line[idx:end])
		out.WriteString("\x1b[0m")
		line = line[end:]
	}
	return out.String()
}

func styleFor(name string) *chroma.Style {
	switch strings.ToLower(name) {
	case "navia":
		return styles.Get("dracula")
	case "dim":
		return styles.Get("nord")
	case "mono":
		return styles.Get("bw")
	case "amber":
		return styles.Get("monokai")
	default:
		return nil
	}
}
