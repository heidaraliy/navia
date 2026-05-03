package textsafe

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Display returns text that can be safely written to a terminal as user data.
// It preserves printable UTF-8 and renders control bytes visibly so filenames
// and file contents cannot smuggle terminal escape sequences.
func Display(s string) string {
	return sanitize(s, false)
}

// Content is like Display but preserves tab characters for editor/file content
// renderers that already handle tab expansion.
func Content(s string) string {
	return sanitize(s, true)
}

func sanitize(s string, allowTab bool) string {
	if s == "" {
		return ""
	}
	original := s
	var out strings.Builder
	changed := false
	for len(s) > 0 {
		r, size := utf8.DecodeRuneInString(s)
		if r == utf8.RuneError && size == 1 {
			writeByteEscape(&out, s[0])
			changed = true
			s = s[1:]
			continue
		}
		if isUnsafeControl(r, allowTab) {
			writeRuneEscape(&out, r)
			changed = true
		} else {
			out.WriteString(s[:size])
		}
		s = s[size:]
	}
	if !changed {
		return original
	}
	return out.String()
}

func isUnsafeControl(r rune, allowTab bool) bool {
	if allowTab && r == '\t' {
		return false
	}
	return r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f)
}

func writeRuneEscape(out *strings.Builder, r rune) {
	if r <= 0xff {
		writeByteEscape(out, byte(r))
		return
	}
	fmt.Fprintf(out, "\\u{%x}", r)
}

func writeByteEscape(out *strings.Builder, b byte) {
	switch b {
	case '\n':
		out.WriteString("\\n")
	case '\r':
		out.WriteString("\\r")
	case '\t':
		out.WriteString("\\t")
	case 0x1b:
		out.WriteString("\\x1b")
	default:
		fmt.Fprintf(out, "\\x%02x", b)
	}
}
