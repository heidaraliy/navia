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
	"github.com/heidaraliy/navia/internal/textsafe"
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
	if isMarkdownPath(path) {
		return highlightMarkdownLine(line, query)
	}
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

const (
	ansiReset           = "\x1b[0m"
	markdownH1Style     = "\x1b[1m\x1b[38;2;248;250;252m\x1b[48;2;24;78;119m"
	markdownH2Style     = "\x1b[1m\x1b[4m\x1b[38;2;125;211;252m"
	markdownH3Style     = "\x1b[1m\x1b[38;2;134;239;172m"
	markdownH4Style     = "\x1b[1m\x1b[38;2;255;203;107m"
	markdownH5Style     = "\x1b[38;2;255;158;100m"
	markdownH6Style     = "\x1b[38;2;198;208;245m"
	markdownMarkerStyle = "\x1b[1m\x1b[38;2;125;211;252m"
	markdownQuoteStyle  = "\x1b[38;2;148;163;184m"
	markdownCodeStyle   = "\x1b[38;2;255;203;107m\x1b[48;2;39;39;42m"
	markdownLinkStyle   = "\x1b[4m\x1b[38;2;125;211;252m"
	markdownDimStyle    = "\x1b[38;2;100;116;139m"
	markdownDoneStyle   = "\x1b[1m\x1b[38;2;134;239;172m"
	markdownOpenStyle   = "\x1b[1m\x1b[38;2;255;203;107m"
)

func isMarkdownPath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".markdown", ".mdown", ".mkd":
		return true
	default:
		return false
	}
}

func highlightMarkdownLine(line, query string) string {
	trimmed := strings.TrimLeft(line, " \t")
	if trimmed == "" {
		return styleMarkdownPart("", line, query)
	}
	if level, ok := markdownHeadingLevel(trimmed); ok {
		return styleMarkdownPart(markdownHeadingStyle(level), line, query)
	}
	if isMarkdownRule(trimmed) {
		return styleMarkdownPart(markdownDimStyle, line, query)
	}
	if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
		return styleMarkdownPart(markdownCodeStyle, line, query)
	}
	if strings.HasPrefix(trimmed, ">") {
		indent := len(line) - len(trimmed)
		return styleMarkdownPart("", line[:indent], query) +
			styleMarkdownPart(markdownQuoteStyle, trimmed, query)
	}
	if markerEnd, checkboxStart, checkboxEnd, checked, ok := markdownListParts(trimmed); ok {
		indent := len(line) - len(trimmed)
		var out strings.Builder
		out.WriteString(styleMarkdownPart("", line[:indent], query))
		out.WriteString(styleMarkdownPart(markdownMarkerStyle, trimmed[:markerEnd], query))
		if checkboxStart >= 0 {
			out.WriteString(styleMarkdownPart("", trimmed[markerEnd:checkboxStart], query))
			style := markdownOpenStyle
			if checked {
				style = markdownDoneStyle
			}
			out.WriteString(styleMarkdownPart(style, trimmed[checkboxStart:checkboxEnd], query))
			out.WriteString(highlightMarkdownInline(trimmed[checkboxEnd:], query, ""))
		} else {
			out.WriteString(highlightMarkdownInline(trimmed[markerEnd:], query, ""))
		}
		return out.String()
	}
	return highlightMarkdownInline(line, query, "")
}

func markdownHeadingLevel(line string) (int, bool) {
	level := 0
	for level < len(line) && level < 6 && line[level] == '#' {
		level++
	}
	return level, level > 0 && level < len(line) && isMarkdownByteSpace(line[level])
}

func markdownHeadingStyle(level int) string {
	switch level {
	case 1:
		return markdownH1Style
	case 2:
		return markdownH2Style
	case 3:
		return markdownH3Style
	case 4:
		return markdownH4Style
	case 5:
		return markdownH5Style
	default:
		return markdownH6Style
	}
}

func isMarkdownRule(line string) bool {
	trimmed := strings.TrimSpace(line)
	if len(trimmed) < 3 {
		return false
	}
	first := trimmed[0]
	if first != '-' && first != '*' && first != '_' {
		return false
	}
	for i := 0; i < len(trimmed); i++ {
		if trimmed[i] != first {
			return false
		}
	}
	return true
}

func markdownListParts(line string) (markerEnd, checkboxStart, checkboxEnd int, checked bool, ok bool) {
	if len(line) < 2 {
		return 0, -1, -1, false, false
	}
	i := 0
	if isMarkdownBulletByte(line[i]) && i+1 < len(line) && isMarkdownByteSpace(line[i+1]) {
		i += 2
	} else {
		start := i
		for i < len(line) && line[i] >= '0' && line[i] <= '9' {
			i++
		}
		if i == start || i >= len(line) || (line[i] != '.' && line[i] != ')') || i+1 >= len(line) || !isMarkdownByteSpace(line[i+1]) {
			return 0, -1, -1, false, false
		}
		i += 2
	}
	markerEnd = i
	i = skipMarkdownByteSpace(line, i)
	if i+2 < len(line) && line[i] == '[' && line[i+2] == ']' {
		switch line[i+1] {
		case ' ':
			return markerEnd, i, i + 3, false, true
		case 'x', 'X':
			return markerEnd, i, i + 3, true, true
		}
	}
	return markerEnd, -1, -1, false, true
}

func highlightMarkdownInline(line, query, baseStyle string) string {
	var out strings.Builder
	for line != "" {
		codeStart := strings.Index(line, "`")
		linkStart, linkEnd, ok := nextMarkdownLink(line)
		switch {
		case ok && (codeStart < 0 || linkStart < codeStart):
			out.WriteString(styleMarkdownPart(baseStyle, line[:linkStart], query))
			closeBracket := strings.Index(line[linkStart:linkEnd], "](") + linkStart
			out.WriteString(styleMarkdownPart(markdownLinkStyle, line[linkStart:closeBracket+1], query))
			out.WriteString(styleMarkdownPart(markdownDimStyle, line[closeBracket+1:linkEnd], query))
			line = line[linkEnd:]
		case codeStart >= 0:
			codeEnd := strings.Index(line[codeStart+1:], "`")
			if codeEnd < 0 {
				out.WriteString(styleMarkdownPart(baseStyle, line, query))
				line = ""
				continue
			}
			codeEnd += codeStart + 2
			out.WriteString(styleMarkdownPart(baseStyle, line[:codeStart], query))
			out.WriteString(styleMarkdownPart(markdownCodeStyle, line[codeStart:codeEnd], query))
			line = line[codeEnd:]
		default:
			out.WriteString(styleMarkdownPart(baseStyle, line, query))
			line = ""
		}
	}
	return out.String()
}

func nextMarkdownLink(line string) (int, int, bool) {
	for start := strings.Index(line, "["); start >= 0; {
		closeRel := strings.Index(line[start:], "](")
		if closeRel < 0 {
			return 0, 0, false
		}
		closeBracket := start + closeRel
		closeParenRel := strings.Index(line[closeBracket+2:], ")")
		if closeParenRel >= 0 {
			return start, closeBracket + 3 + closeParenRel, true
		}
		nextRel := strings.Index(line[start+1:], "[")
		if nextRel < 0 {
			return 0, 0, false
		}
		start += nextRel + 1
	}
	return 0, 0, false
}

func styleMarkdownPart(style, value, query string) string {
	if value == "" {
		return ""
	}
	value = textsafe.Content(value)
	if query == "" || !strings.Contains(strings.ToLower(value), strings.ToLower(query)) {
		if style == "" {
			return value
		}
		return style + value + ansiReset
	}
	lowerQuery := strings.ToLower(query)
	var out strings.Builder
	if style != "" {
		out.WriteString(style)
	}
	for len(value) > 0 {
		idx := strings.Index(strings.ToLower(value), lowerQuery)
		if idx < 0 {
			out.WriteString(value)
			break
		}
		if idx > 0 {
			out.WriteString(value[:idx])
		}
		end := idx + len(query)
		out.WriteString("\x1b[38;2;17;24;39m\x1b[48;2;229;231;146m")
		out.WriteString(value[idx:end])
		out.WriteString(ansiReset)
		if style != "" {
			out.WriteString(style)
		}
		value = value[end:]
	}
	if style != "" {
		out.WriteString(ansiReset)
	}
	return out.String()
}

func skipMarkdownByteSpace(line string, i int) int {
	for i < len(line) && isMarkdownByteSpace(line[i]) {
		i++
	}
	return i
}

func isMarkdownByteSpace(b byte) bool {
	return b == ' ' || b == '\t'
}

func isMarkdownBulletByte(b byte) bool {
	return b == '-' || b == '*' || b == '+'
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
	value = textsafe.Content(value)
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
	line = textsafe.Content(line)
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
