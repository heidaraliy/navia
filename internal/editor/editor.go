package editor

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
	"github.com/heidaraliy/navia/internal/textsafe"
)

type Mode int

const (
	Normal Mode = iota
	Insert
	Visual
	VisualLine
	Command
	Search
)

func (m Mode) String() string {
	switch m {
	case Insert:
		return "INSERT"
	case Visual:
		return "VISUAL"
	case VisualLine:
		return "V-LINE"
	case Command:
		return "COMMAND"
	case Search:
		return "SEARCH"
	default:
		return "NORMAL"
	}
}

type ActionKind int

const (
	ActionNone ActionKind = iota
	ActionStatus
	ActionSave
	ActionClose
	ActionCloseForce
	ActionQuitAll
	ActionQuitAllForce
	ActionOpen
	ActionNextTab
	ActionPrevTab
	ActionListTabs
	ActionJumpBack
	ActionJumpForward
	ActionDefinition
	ActionReferences
	ActionExternal
	ActionTheme
)

type Action struct {
	Kind    ActionKind
	Message string
	Path    string
	Force   bool
}

type Buffer struct {
	Path       string
	Name       string
	Lines      []string
	Row        int
	Col        int
	Mode       Mode
	Dirty      bool
	ModifiedAt time.Time
	FileMode   os.FileMode
	Register   string

	command   string
	query     string
	lastQuery string
	count     string
	pending   string
	opCount   int
	visualRow int
	visualCol int
	lastFind  findState
	undo      []snapshot
	redo      []snapshot
	insertTxn bool
}

type Range struct {
	StartRow int
	StartCol int
	EndRow   int
	EndCol   int
}

type findState struct {
	Target    rune
	Backward  bool
	Till      bool
	Available bool
}

type snapshot struct {
	lines []string
	row   int
	col   int
}

const DefaultMaxBytes int64 = 1024 * 1024

var (
	ErrDirectory = errors.New("cannot edit a directory")
	ErrBinary    = errors.New("cannot edit a binary file")
	ErrTooLarge  = errors.New("file is too large for Navia editor")
	ErrChanged   = errors.New("file changed on disk; use :w! to overwrite")
)

func Open(path string, maxBytes int64) (*Buffer, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, ErrDirectory
	}
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBytes
	}
	if info.Size() > maxBytes {
		return nil, ErrTooLarge
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if bytes.Contains(data, []byte{0}) {
		return nil, ErrBinary
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) > 1 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		lines = []string{""}
	}
	return &Buffer{
		Path:       path,
		Name:       filepath.Base(path),
		Lines:      lines,
		Mode:       Normal,
		ModifiedAt: info.ModTime(),
		FileMode:   info.Mode(),
		visualRow:  -1,
	}, nil
}

func NewScratch(path string) *Buffer {
	return &Buffer{
		Path:      path,
		Name:      filepath.Base(path),
		Lines:     []string{""},
		Mode:      Normal,
		FileMode:  0o644,
		Dirty:     true,
		visualRow: -1,
	}
}

func (b *Buffer) Value() string {
	return strings.Join(b.Lines, "\n")
}

func (b *Buffer) Save(force bool) error {
	if b.Path == "" {
		return errors.New("buffer has no path")
	}
	if info, err := os.Stat(b.Path); err == nil && !force && !b.ModifiedAt.IsZero() && info.ModTime().After(b.ModifiedAt) {
		return ErrChanged
	}
	dir := filepath.Dir(b.Path)
	tmp, err := os.CreateTemp(dir, ".navia-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.WriteString(b.Value()); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	mode := b.FileMode
	if mode == 0 {
		mode = 0o644
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		return err
	}
	if err := os.Rename(tmpName, b.Path); err != nil {
		return err
	}
	info, err := os.Stat(b.Path)
	if err == nil {
		b.ModifiedAt = info.ModTime()
		b.FileMode = info.Mode()
	}
	b.Dirty = false
	return nil
}

func (b *Buffer) HandleKey(key string) Action {
	switch b.Mode {
	case Insert:
		return b.handleInsert(key)
	case Visual, VisualLine:
		return b.handleVisual(key)
	case Command:
		return b.handleCommand(key)
	case Search:
		return b.handleSearch(key)
	default:
		return b.handleNormal(key)
	}
}

func (b *Buffer) handleNormal(key string) Action {
	if b.pending != "" {
		first := b.pending
		if isCountKey(key) && first != "ctrl+w" {
			if key != "0" || b.count != "" {
				b.count += key
				return Action{}
			}
		}
		b.pending = ""
		if first == "f" || first == "F" || first == "t" || first == "T" {
			b.applyFind([]rune(key)[0], first == "F" || first == "T", first == "t" || first == "T", b.takeCount())
			return Action{}
		}
		if len(first) == 2 && (first[1] == 'f' || first[1] == 'F' || first[1] == 't' || first[1] == 'T') {
			r, ok := b.findRangeCount([]rune(key)[0], first[1] == 'F' || first[1] == 'T', first[1] == 't' || first[1] == 'T', b.effectiveCount())
			if ok {
				b.applyOperator(first[:1], r)
			}
			return Action{}
		}
		if len(first) == 2 && (first[1] == 'a' || first[1] == 'i') {
			r, ok := b.textObjectRange(rune(key[0]), first[1] == 'a', b.effectiveCount())
			if ok {
				b.applyOperator(first[:1], r)
			}
			return Action{}
		}
		if first == "d" || first == "c" || first == "y" {
			switch key {
			case "w":
				r := b.motionWordRange(b.effectiveCount())
				b.applyOperator(first, r)
			case "$":
				b.applyOperator(first, Range{StartRow: b.Row, StartCol: b.Col, EndRow: b.Row, EndCol: len(b.Lines[b.Row])})
			case "a", "i":
				b.pending = first + key
			case "f", "F", "t", "T":
				b.pending = first + key
			case first:
				b.applyLineOperator(first, b.effectiveCount())
			default:
				b.clearCounts()
				return Action{Kind: ActionStatus, Message: "Unknown command: " + first + key}
			}
			return Action{}
		}
		switch first + key {
		case "gg":
			count, specified := b.takeCountWithSpecified()
			if specified {
				b.Row = count - 1
			} else {
				b.Row = 0
			}
			b.clamp()
		case "gd":
			b.clearCounts()
			return Action{Kind: ActionDefinition}
		case "gr":
			b.clearCounts()
			return Action{Kind: ActionReferences}
		case "gt":
			b.clearCounts()
			return Action{Kind: ActionNextTab}
		case "gT":
			b.clearCounts()
			return Action{Kind: ActionPrevTab}
		case "dd":
			b.applyLineOperator("d", b.effectiveCount())
		case "yy":
			b.applyLineOperator("y", b.effectiveCount())
		case "dw":
			b.applyOperator("d", b.motionWordRange(b.effectiveCount()))
		case "d$":
			b.deleteToEnd()
		case "yw":
			b.applyOperator("y", b.motionWordRange(b.effectiveCount()))
		case "y$":
			b.Register = b.Lines[b.Row][min(b.Col, len(b.Lines[b.Row])):]
		default:
			b.clearCounts()
			return Action{Kind: ActionStatus, Message: "Unknown command: " + first + key}
		}
		return Action{}
	}
	if isCountKey(key) {
		if key == "0" && b.count == "" {
			b.Col = 0
		} else {
			b.count += key
		}
		return Action{}
	}
	count, countSpecified := b.takeCountWithSpecified()
	switch key {
	case "ctrl+w":
		b.pending = "ctrl+w"
	case "h", "left":
		repeat(count, b.moveLeft)
	case "j", "down":
		repeat(count, b.moveDown)
	case "k", "up":
		repeat(count, b.moveUp)
	case "l", "right":
		repeat(count, b.moveRight)
	case "ctrl+d":
		b.pageDown(10)
	case "ctrl+u":
		b.pageUp(10)
	case "ctrl+o":
		return Action{Kind: ActionJumpBack}
	case "ctrl+i", "tab":
		return Action{Kind: ActionJumpForward}
	case "w":
		repeat(count, b.wordRight)
	case "b":
		repeat(count, b.wordLeft)
	case "e":
		repeat(count, b.wordEnd)
	case "0", "home":
		b.Col = 0
	case "$", "end":
		b.Col = len(b.Lines[b.Row])
	case "G":
		if countSpecified {
			b.Row = count - 1
		} else {
			b.Row = len(b.Lines) - 1
		}
		b.clamp()
	case "d", "y", "c":
		b.opCount = count
		b.pending = key
	case "g", "f", "F", "t", "T":
		if countSpecified {
			b.count = strconv.Itoa(count)
		}
		b.pending = key
	case ";":
		b.repeatFind(false, count)
	case ",":
		b.repeatFind(true, count)
	case "i":
		b.Mode = Insert
	case "a":
		b.moveRightInsert()
		b.Mode = Insert
	case "I":
		b.Col = firstNonSpace(b.Lines[b.Row])
		b.Mode = Insert
	case "A":
		b.Col = len(b.Lines[b.Row])
		b.Mode = Insert
	case "o":
		b.pushUndo()
		b.Row++
		b.Lines = insertLine(b.Lines, b.Row, "")
		b.Col = 0
		b.Dirty = true
		b.Mode = Insert
	case "O":
		b.pushUndo()
		b.Lines = insertLine(b.Lines, b.Row, "")
		b.Col = 0
		b.Dirty = true
		b.Mode = Insert
	case "x", "delete":
		b.deleteRune()
	case "p":
		b.pasteAfter()
	case "P":
		b.pasteBefore()
	case "u":
		b.undoLast()
	case "ctrl+r":
		b.redoLast()
	case " ", "space":
		return b.toggleMarkdownCheckbox()
	case "v":
		b.startVisual(Visual)
	case "V":
		b.startVisual(VisualLine)
	case ":":
		b.Mode = Command
		b.command = ""
	case "/":
		b.Mode = Search
		b.query = ""
	case "n":
		b.findNext(1)
	case "N":
		b.findNext(-1)
	}
	if b.Mode == Insert {
		b.clampInsert()
	} else {
		b.clamp()
	}
	return Action{}
}

func (b *Buffer) handleInsert(key string) Action {
	switch key {
	case "esc", "ctrl+[":
		b.Mode = Normal
		b.insertTxn = false
		if b.Col > 0 {
			b.Col--
		}
	case "enter":
		b.beginInsertUndo()
		line := b.Lines[b.Row]
		left, right := line[:b.Col], line[b.Col:]
		b.Lines[b.Row] = left
		b.Lines = insertLine(b.Lines, b.Row+1, right)
		b.Row++
		b.Col = 0
		b.Dirty = true
	case "backspace", "ctrl+h":
		b.backspace()
	case "delete":
		b.deleteRune()
	case "left":
		b.moveLeft()
	case "right":
		b.moveRightInsert()
	case "up":
		b.moveUp()
	case "down":
		b.moveDown()
	case "ctrl+d":
		b.pageDown(10)
	case "ctrl+u":
		b.pageUp(10)
	case "tab":
		b.insertText("\t")
	case "space":
		b.insertText(" ")
	default:
		if len([]rune(key)) == 1 {
			b.insertText(key)
		}
	}
	b.clampInsert()
	return Action{}
}

func (b *Buffer) handleVisual(key string) Action {
	switch key {
	case "esc", "v", "V":
		b.Mode = Normal
		b.visualRow = -1
	case "h", "left":
		b.moveLeft()
	case "j", "down":
		b.moveDown()
	case "k", "up":
		b.moveUp()
	case "l", "right":
		b.moveRight()
	case "ctrl+d":
		b.pageDown(10)
	case "ctrl+u":
		b.pageUp(10)
	case "y":
		b.Register = b.selectionText()
		b.Mode = Normal
		b.visualRow = -1
	case "d":
		b.pushUndo()
		b.Register = b.selectionText()
		b.deleteSelection()
		b.Mode = Normal
		b.visualRow = -1
		b.Dirty = true
	}
	return Action{}
}

func (b *Buffer) handleCommand(key string) Action {
	switch key {
	case "esc":
		b.Mode = Normal
		b.command = ""
	case "enter":
		cmd := b.command
		b.command = ""
		b.Mode = Normal
		return b.Execute(cmd)
	case "backspace", "ctrl+h":
		if b.command != "" {
			b.command = b.command[:len(b.command)-1]
		}
	default:
		if len([]rune(key)) == 1 {
			b.command += key
		}
	}
	return Action{}
}

func (b *Buffer) handleSearch(key string) Action {
	switch key {
	case "esc":
		b.Mode = Normal
		b.query = ""
	case "enter":
		b.lastQuery = b.query
		b.query = ""
		b.Mode = Normal
		b.findNext(1)
	case "backspace", "ctrl+h":
		if b.query != "" {
			b.query = b.query[:len(b.query)-1]
		}
	default:
		if len([]rune(key)) == 1 {
			b.query += key
		}
	}
	return Action{}
}

func (b *Buffer) Execute(cmd string) Action {
	cmd = strings.TrimSpace(cmd)
	switch {
	case cmd == "w":
		return Action{Kind: ActionSave}
	case cmd == "w!":
		return Action{Kind: ActionSave, Force: true}
	case cmd == "q":
		return Action{Kind: ActionClose}
	case cmd == "q!" || cmd == "bd!":
		return Action{Kind: ActionCloseForce}
	case cmd == "wq":
		return Action{Kind: ActionSave, Message: "close"}
	case cmd == "qa":
		return Action{Kind: ActionQuitAll}
	case cmd == "qa!":
		return Action{Kind: ActionQuitAllForce}
	case cmd == "bd":
		return Action{Kind: ActionClose}
	case cmd == "bnext" || cmd == "bn":
		return Action{Kind: ActionNextTab}
	case cmd == "bprev" || cmd == "bp":
		return Action{Kind: ActionPrevTab}
	case cmd == "buffers" || cmd == "ls" || cmd == "bl":
		return Action{Kind: ActionListTabs}
	case cmd == "nvim":
		return Action{Kind: ActionExternal}
	case cmd == "theme":
		return Action{Kind: ActionTheme}
	case strings.HasPrefix(cmd, "theme "):
		return Action{Kind: ActionTheme, Message: strings.TrimSpace(strings.TrimPrefix(cmd, "theme "))}
	case strings.HasPrefix(cmd, "e "):
		return Action{Kind: ActionOpen, Path: strings.TrimSpace(strings.TrimPrefix(cmd, "e "))}
	case cmd == "$":
		b.Row = len(b.Lines) - 1
		b.clamp()
	case isNumber(cmd):
		n, _ := strconv.Atoi(cmd)
		b.Row = max(0, n-1)
		b.clamp()
	case strings.HasPrefix(cmd, "%s/") || strings.HasPrefix(cmd, "s/"):
		if err := b.substitute(cmd); err != nil {
			return Action{Kind: ActionStatus, Message: err.Error()}
		}
	default:
		return Action{Kind: ActionStatus, Message: "Unknown command: :" + cmd}
	}
	return Action{}
}

func (b *Buffer) CommandLine() string {
	if b.Mode == Command {
		return ":" + b.command
	}
	if b.Mode == Search {
		return "/" + b.query
	}
	return ""
}

func (b *Buffer) NormalCommandLine() string {
	if b.Mode != Normal {
		return ""
	}
	if b.pending == "" && b.count == "" {
		return ""
	}
	if b.pending == "ctrl+w" {
		return "ctrl+w"
	}
	prefix := ""
	if b.opCount > 1 {
		prefix = strconv.Itoa(b.opCount)
	}
	return prefix + b.pending + b.count
}

func (b *Buffer) CursorLine() int { return b.Row + 1 }
func (b *Buffer) CursorCol() int  { return b.Col + 1 }
func (b *Buffer) SearchQuery() string {
	if b.Mode == Search && b.query != "" {
		return b.query
	}
	return b.lastQuery
}

func (b *Buffer) IsMarkdown() bool {
	return isMarkdownPath(b.Path)
}

func (b *Buffer) toggleMarkdownCheckbox() Action {
	if !isMarkdownPath(b.Path) {
		return Action{}
	}
	if b.Row < 0 || b.Row >= len(b.Lines) {
		return Action{}
	}
	line := b.Lines[b.Row]
	idx, checked, ok := markdownCheckboxMarker(line)
	if !ok {
		return Action{Kind: ActionStatus, Message: "No Markdown checkbox on this line."}
	}
	b.pushUndo()
	next := 'x'
	message := "Checked task."
	if checked {
		next = ' '
		message = "Unchecked task."
	}
	runes := []rune(line)
	runes[idx] = next
	b.Lines[b.Row] = string(runes)
	b.Dirty = true
	return Action{Kind: ActionStatus, Message: message}
}

func isMarkdownPath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".markdown", ".mdown", ".mkd":
		return true
	default:
		return false
	}
}

func markdownCheckboxMarker(line string) (int, bool, bool) {
	runes := []rune(line)
	i := skipMarkdownSpace(runes, 0)
	if i >= len(runes) {
		return 0, false, false
	}
	if isMarkdownBullet(runes[i]) && i+1 < len(runes) && isMarkdownSpace(runes[i+1]) {
		i += 2
	} else {
		start := i
		for i < len(runes) && runes[i] >= '0' && runes[i] <= '9' {
			i++
		}
		if i == start || i >= len(runes) || (runes[i] != '.' && runes[i] != ')') || i+1 >= len(runes) || !isMarkdownSpace(runes[i+1]) {
			i = start
		} else {
			i += 2
		}
	}
	i = skipMarkdownSpace(runes, i)
	if i+2 >= len(runes) || runes[i] != '[' || runes[i+2] != ']' {
		return 0, false, false
	}
	switch runes[i+1] {
	case ' ':
		return i + 1, false, true
	case 'x', 'X':
		return i + 1, true, true
	default:
		return 0, false, false
	}
}

func skipMarkdownSpace(runes []rune, i int) int {
	for i < len(runes) && isMarkdownSpace(runes[i]) {
		i++
	}
	return i
}

func isMarkdownSpace(r rune) bool {
	return r == ' ' || r == '\t'
}

func isMarkdownBullet(r rune) bool {
	return r == '-' || r == '*' || r == '+'
}

func (b *Buffer) Visible(width, height int) []string {
	return b.visible(width, height, nil)
}

func (b *Buffer) VisibleHighlighted(width, height int, highlight func(path, line string) string) []string {
	return b.visible(width, height, highlight)
}

func (b *Buffer) visible(width, height int, highlight func(path, line string) string) []string {
	if height <= 0 {
		return nil
	}
	start := b.Row - height/2
	if start < 0 {
		start = 0
	}
	if start+height > len(b.Lines) {
		start = len(b.Lines) - height
	}
	if start < 0 {
		start = 0
	}
	end := min(len(b.Lines), start+height)
	gutterW := len(strconv.Itoa(max(1, end)))
	out := make([]string, 0, height)
	for i := start; i < len(b.Lines) && len(out) < height; i++ {
		prefix := fmt.Sprintf("%*d ", gutterW, i+1)
		lineText, col := visibleLineWindow(b.Lines[i], -1, width, height)
		line := renderVisibleLine(b.Path, lineText, col, false, highlight)
		if i == b.Row {
			lineText, col = visibleLineWindow(b.Lines[i], b.Col, width, height)
			line = renderVisibleLine(b.Path, lineText, col, b.Mode == Insert, highlight)
		}
		wrapped := wrapDisplayLimit(prefix+line, strings.Repeat(" ", gutterW+1), width, height-len(out))
		for _, visual := range wrapped {
			out = append(out, visual)
			if len(out) >= height {
				break
			}
		}
	}
	for len(out) < height {
		out = append(out, "   ~ ")
	}
	return out
}

func renderVisibleLine(path, line string, col int, insert bool, highlight func(path, line string) string) string {
	if highlight == nil {
		return renderLine(line, col, insert)
	}
	return renderHighlightedLine(highlight(path, line), col, insert)
}

func visibleLineWindow(line string, col, width, height int) (string, int) {
	budget := width * max(1, height)
	if budget < 400 {
		budget = 400
	}
	if budget > 4000 {
		budget = 4000
	}
	end, truncated := byteAfterRunes(line, budget)
	if !truncated {
		return line, col
	}
	if col < 0 {
		return line[:end] + " ...", col
	}
	if col > len(line) {
		col = len(line)
	}
	col = clampByteToRuneBoundary(line, col)
	start := byteBeforeRunes(line, col, budget/2)
	end = byteAfterRunesFrom(line, start, budget)
	prefix := ""
	if start > 0 {
		prefix = "... "
	}
	suffix := ""
	if end < len(line) {
		suffix = " ..."
	}
	return prefix + line[start:end] + suffix, col - start + len(prefix)
}

func byteAfterRunes(s string, maxRunes int) (int, bool) {
	if maxRunes <= 0 {
		return 0, s != ""
	}
	count := 0
	for i := range s {
		if count == maxRunes {
			return i, true
		}
		count++
	}
	return len(s), false
}

func byteAfterRunesFrom(s string, start, maxRunes int) int {
	if start < 0 {
		start = 0
	}
	if start >= len(s) || maxRunes <= 0 {
		return start
	}
	end, _ := byteAfterRunes(s[start:], maxRunes)
	return start + end
}

func byteBeforeRunes(s string, end, maxRunes int) int {
	if end <= 0 {
		return 0
	}
	if end > len(s) {
		end = len(s)
	}
	start := end
	for i := 0; i < maxRunes && start > 0; i++ {
		_, size := utf8.DecodeLastRuneInString(s[:start])
		if size == 0 {
			return start
		}
		start -= size
	}
	return start
}

func clampByteToRuneBoundary(s string, col int) int {
	if col < 0 {
		return 0
	}
	if col > len(s) {
		return len(s)
	}
	for col > 0 && col < len(s) && !utf8.RuneStart(s[col]) {
		col--
	}
	return col
}

func (b *Buffer) startVisual(mode Mode) {
	b.Mode = mode
	b.visualRow = b.Row
	b.visualCol = b.Col
}

func (b *Buffer) insertText(s string) {
	b.beginInsertUndo()
	line := b.Lines[b.Row]
	b.Lines[b.Row] = line[:b.Col] + s + line[b.Col:]
	b.Col += len([]rune(s))
	b.Dirty = true
}

// InsertText inserts literal terminal text at the cursor without interpreting it
// as editor commands.
func (b *Buffer) InsertText(text string) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	if text == "" {
		return
	}
	b.beginInsertUndo()
	line := b.Lines[b.Row]
	left, right := line[:b.Col], line[b.Col:]
	parts := strings.Split(text, "\n")
	if len(parts) == 1 {
		b.Lines[b.Row] = left + text + right
		b.Col += len(text)
		b.Dirty = true
		b.clampInsert()
		return
	}
	b.Lines[b.Row] = left + parts[0]
	insertAt := b.Row + 1
	for _, part := range parts[1 : len(parts)-1] {
		b.Lines = insertLine(b.Lines, insertAt, part)
		insertAt++
	}
	last := parts[len(parts)-1]
	b.Lines = insertLine(b.Lines, insertAt, last+right)
	b.Row = insertAt
	b.Col = len(last)
	b.Dirty = true
	b.clampInsert()
}

func (b *Buffer) backspace() {
	if b.Row == 0 && b.Col == 0 {
		return
	}
	b.beginInsertUndo()
	if b.Col > 0 {
		line := b.Lines[b.Row]
		b.Lines[b.Row] = line[:b.Col-1] + line[b.Col:]
		b.Col--
	} else {
		prevLen := len(b.Lines[b.Row-1])
		b.Lines[b.Row-1] += b.Lines[b.Row]
		b.Lines = append(b.Lines[:b.Row], b.Lines[b.Row+1:]...)
		b.Row--
		b.Col = prevLen
	}
	b.Dirty = true
}

func (b *Buffer) deleteRune() {
	line := b.Lines[b.Row]
	if b.Col >= len(line) {
		if b.Row >= len(b.Lines)-1 {
			return
		}
		if b.Mode == Insert {
			b.beginInsertUndo()
		} else {
			b.pushUndo()
		}
		b.Lines[b.Row] += b.Lines[b.Row+1]
		b.Lines = append(b.Lines[:b.Row+1], b.Lines[b.Row+2:]...)
		b.Dirty = true
		return
	}
	if b.Mode == Insert {
		b.beginInsertUndo()
	} else {
		b.pushUndo()
	}
	b.Register = line[b.Col : b.Col+1]
	b.Lines[b.Row] = line[:b.Col] + line[b.Col+1:]
	b.Dirty = true
}

func (b *Buffer) deleteWord() {
	b.pushUndo()
	start := b.Col
	b.wordRight()
	end := b.Col
	line := b.Lines[b.Row]
	if start > end {
		start, end = end, start
	}
	b.Register = line[start:end]
	b.Lines[b.Row] = line[:start] + line[end:]
	b.Col = start
	b.Dirty = true
}

func (b *Buffer) deleteToEnd() {
	b.pushUndo()
	line := b.Lines[b.Row]
	b.Register = line[b.Col:]
	b.Lines[b.Row] = line[:b.Col]
	b.Dirty = true
}

func (b *Buffer) applyFind(target rune, backward, till bool, count int) {
	if r, ok := b.findRangeCount(target, backward, till, count); ok {
		b.Row = r.EndRow
		b.Col = r.EndCol
		b.lastFind = findState{Target: target, Backward: backward, Till: till, Available: true}
	}
}

func (b *Buffer) repeatFind(reverse bool, count int) {
	if !b.lastFind.Available {
		return
	}
	backward := b.lastFind.Backward
	if reverse {
		backward = !backward
	}
	b.applyFind(b.lastFind.Target, backward, b.lastFind.Till, count)
}

func (b *Buffer) findRangeCount(target rune, backward, till bool, count int) (Range, bool) {
	if count <= 0 {
		count = 1
	}
	startRow, startCol := b.Row, b.Col
	var r Range
	var ok bool
	for i := 0; i < count; i++ {
		r, ok = b.findRange(target, backward, till)
		if !ok {
			b.Row, b.Col = startRow, startCol
			return Range{}, false
		}
		b.Row, b.Col = r.EndRow, r.EndCol
	}
	b.Row, b.Col = startRow, startCol
	r.StartRow, r.StartCol = startRow, startCol
	return r, true
}

func (b *Buffer) findRange(target rune, backward, till bool) (Range, bool) {
	line := []rune(b.Lines[b.Row])
	if len(line) == 0 {
		return Range{}, false
	}
	start := b.Col
	if backward {
		for i := min(start-1, len(line)-1); i >= 0; i-- {
			if line[i] == target {
				col := i
				if till {
					col = min(i+1, len(line))
				}
				return Range{StartRow: b.Row, StartCol: b.Col, EndRow: b.Row, EndCol: col}, true
			}
		}
		return Range{}, false
	}
	for i := start + 1; i < len(line); i++ {
		if line[i] == target {
			col := i
			if till {
				col = max(start, i-1)
			}
			return Range{StartRow: b.Row, StartCol: b.Col, EndRow: b.Row, EndCol: col}, true
		}
	}
	return Range{}, false
}

func (b *Buffer) motionWordRange(count int) Range {
	if count <= 0 {
		count = 1
	}
	start := b.Col
	line := b.Lines[b.Row]
	end := start
	for i := 0; i < count; i++ {
		for end < len(line) && !unicode.IsSpace(rune(line[end])) {
			end++
		}
		for end < len(line) && unicode.IsSpace(rune(line[end])) {
			end++
		}
	}
	return Range{StartRow: b.Row, StartCol: start, EndRow: b.Row, EndCol: end}
}

func (b *Buffer) textObjectRange(object rune, around bool, count int) (Range, bool) {
	switch object {
	case 'w':
		return b.wordObjectRange(around, count), true
	case '"', '\'', '`':
		return b.pairObjectRange(object, object, around)
	case '(', ')':
		return b.pairObjectRange('(', ')', around)
	case '[', ']':
		return b.pairObjectRange('[', ']', around)
	case '{', '}':
		return b.pairObjectRange('{', '}', around)
	default:
		return Range{}, false
	}
}

func (b *Buffer) wordObjectRange(around bool, count int) Range {
	if count <= 0 {
		count = 1
	}
	line := b.Lines[b.Row]
	if line == "" {
		return Range{StartRow: b.Row, StartCol: 0, EndRow: b.Row, EndCol: 0}
	}
	start := min(b.Col, len(line)-1)
	for start > 0 && !unicode.IsSpace(rune(line[start-1])) {
		start--
	}
	end := start
	for i := 0; i < count; i++ {
		for end < len(line) && unicode.IsSpace(rune(line[end])) {
			end++
		}
		for end < len(line) && !unicode.IsSpace(rune(line[end])) {
			end++
		}
		if around || i < count-1 {
			for end < len(line) && unicode.IsSpace(rune(line[end])) {
				end++
			}
		}
	}
	return Range{StartRow: b.Row, StartCol: start, EndRow: b.Row, EndCol: end}
}

func (b *Buffer) applyLineOperator(op string, count int) {
	if count <= 0 {
		count = 1
	}
	end := min(len(b.Lines), b.Row+count)
	text := strings.Join(b.Lines[b.Row:end], "\n") + "\n"
	switch op {
	case "y":
		b.Register = text
	case "d", "c":
		b.pushUndo()
		b.Register = text
		b.Lines = append(b.Lines[:b.Row], b.Lines[end:]...)
		if len(b.Lines) == 0 {
			b.Lines = []string{""}
		}
		b.Dirty = true
		b.clamp()
		if op == "c" {
			b.Mode = Insert
		}
	}
}

func (b *Buffer) pairObjectRange(open, close rune, around bool) (Range, bool) {
	line := []rune(b.Lines[b.Row])
	left := -1
	for i := min(b.Col, len(line)-1); i >= 0; i-- {
		if line[i] == open {
			left = i
			break
		}
	}
	right := -1
	for i := max(0, b.Col); i < len(line); i++ {
		if line[i] == close {
			right = i
			break
		}
	}
	if left < 0 || right < 0 || right < left {
		return Range{}, false
	}
	if !around {
		left++
	}
	if around {
		right++
	}
	return Range{StartRow: b.Row, StartCol: left, EndRow: b.Row, EndCol: right}, true
}

func (b *Buffer) applyOperator(op string, r Range) {
	b.normalizeRange(&r)
	text := b.rangeText(r)
	switch op {
	case "y":
		b.Register = text
	case "d", "c":
		b.pushUndo()
		b.Register = text
		b.deleteRange(r)
		b.Dirty = true
		if op == "c" {
			b.Mode = Insert
		}
	}
}

func (b *Buffer) normalizeRange(r *Range) {
	if r.StartRow > r.EndRow || (r.StartRow == r.EndRow && r.StartCol > r.EndCol) {
		r.StartRow, r.EndRow = r.EndRow, r.StartRow
		r.StartCol, r.EndCol = r.EndCol, r.StartCol
	}
	r.StartRow = min(max(0, r.StartRow), len(b.Lines)-1)
	r.EndRow = min(max(0, r.EndRow), len(b.Lines)-1)
	r.StartCol = min(max(0, r.StartCol), len(b.Lines[r.StartRow]))
	r.EndCol = min(max(0, r.EndCol), len(b.Lines[r.EndRow]))
}

func (b *Buffer) rangeText(r Range) string {
	if r.StartRow == r.EndRow {
		return b.Lines[r.StartRow][r.StartCol:r.EndCol]
	}
	return ""
}

func (b *Buffer) deleteRange(r Range) {
	if r.StartRow != r.EndRow {
		return
	}
	line := b.Lines[r.StartRow]
	b.Lines[r.StartRow] = line[:r.StartCol] + line[r.EndCol:]
	b.Row = r.StartRow
	b.Col = r.StartCol
	b.clampInsert()
}

func (b *Buffer) pasteAfter() {
	if b.Register == "" {
		return
	}
	b.pushUndo()
	if strings.HasSuffix(b.Register, "\n") {
		lines := strings.Split(strings.TrimSuffix(b.Register, "\n"), "\n")
		idx := b.Row + 1
		for _, line := range lines {
			b.Lines = insertLine(b.Lines, idx, line)
			idx++
		}
		b.Row++
		b.Col = 0
	} else {
		b.moveRightInsert()
		line := b.Lines[b.Row]
		b.Lines[b.Row] = line[:b.Col] + b.Register + line[b.Col:]
		b.Col += len(b.Register)
	}
	b.Dirty = true
}

func (b *Buffer) pasteBefore() {
	if b.Register == "" {
		return
	}
	b.pushUndo()
	if strings.HasSuffix(b.Register, "\n") {
		lines := strings.Split(strings.TrimSuffix(b.Register, "\n"), "\n")
		idx := b.Row
		for _, line := range lines {
			b.Lines = insertLine(b.Lines, idx, line)
			idx++
		}
		b.Col = 0
	} else {
		line := b.Lines[b.Row]
		b.Lines[b.Row] = line[:b.Col] + b.Register + line[b.Col:]
	}
	b.Dirty = true
}

func (b *Buffer) selectionText() string {
	if b.visualRow < 0 {
		return ""
	}
	a, c := b.visualRow, b.Row
	if a > c {
		a, c = c, a
	}
	if b.Mode == VisualLine {
		return strings.Join(b.Lines[a:c+1], "\n") + "\n"
	}
	if b.visualRow != b.Row {
		return strings.Join(b.Lines[a:c+1], "\n")
	}
	start, end := b.visualCol, b.Col
	if start > end {
		start, end = end, start
	}
	line := b.Lines[b.Row]
	end = min(end+1, len(line))
	return line[start:end]
}

func (b *Buffer) deleteSelection() {
	if b.visualRow < 0 {
		return
	}
	a, c := b.visualRow, b.Row
	if a > c {
		a, c = c, a
	}
	if b.Mode == VisualLine || b.visualRow != b.Row {
		b.Lines = append(b.Lines[:a], b.Lines[c+1:]...)
		if len(b.Lines) == 0 {
			b.Lines = []string{""}
		}
		b.Row = min(a, len(b.Lines)-1)
		b.Col = 0
		return
	}
	start, end := b.visualCol, b.Col
	if start > end {
		start, end = end, start
	}
	line := b.Lines[b.Row]
	end = min(end+1, len(line))
	b.Lines[b.Row] = line[:start] + line[end:]
	b.Col = start
}

func (b *Buffer) findNext(dir int) {
	if b.lastQuery == "" {
		return
	}
	row := b.Row
	for i := 0; i < len(b.Lines); i++ {
		row += dir
		if row < 0 {
			row = len(b.Lines) - 1
		}
		if row >= len(b.Lines) {
			row = 0
		}
		if idx := strings.Index(strings.ToLower(b.Lines[row]), strings.ToLower(b.lastQuery)); idx >= 0 {
			b.Row = row
			b.Col = idx
			return
		}
	}
}

func (b *Buffer) substitute(cmd string) error {
	all := strings.HasPrefix(cmd, "%s/")
	body := strings.TrimPrefix(strings.TrimPrefix(cmd, "%s/"), "s/")
	parts := strings.Split(body, "/")
	if len(parts) < 2 {
		return errors.New("invalid substitute command")
	}
	re, err := regexp.Compile(parts[0])
	if err != nil {
		return err
	}
	repl := parts[1]
	global := len(parts) > 2 && strings.Contains(parts[2], "g")
	b.pushUndo()
	start, end := b.Row, b.Row
	if all {
		start, end = 0, len(b.Lines)-1
	}
	for i := start; i <= end; i++ {
		if global {
			b.Lines[i] = re.ReplaceAllString(b.Lines[i], repl)
		} else {
			b.Lines[i] = replaceFirst(re, b.Lines[i], repl)
		}
	}
	b.Dirty = true
	return nil
}

func replaceFirst(re *regexp.Regexp, s, repl string) string {
	loc := re.FindStringIndex(s)
	if loc == nil {
		return s
	}
	return s[:loc[0]] + re.ReplaceAllString(s[loc[0]:loc[1]], repl) + s[loc[1]:]
}

func isCountKey(key string) bool {
	return len(key) == 1 && key[0] >= '0' && key[0] <= '9'
}

func repeat(count int, fn func()) {
	if count <= 0 {
		count = 1
	}
	for i := 0; i < count; i++ {
		fn()
	}
}

func (b *Buffer) takeCount() int {
	count, _ := b.takeCountWithSpecified()
	return count
}

func (b *Buffer) takeCountWithSpecified() (int, bool) {
	if b.count == "" {
		return 1, false
	}
	count, err := strconv.Atoi(b.count)
	b.count = ""
	if err != nil || count <= 0 {
		return 1, true
	}
	return count, true
}

func (b *Buffer) effectiveCount() int {
	motionCount := b.takeCount()
	operatorCount := b.opCount
	b.opCount = 0
	if operatorCount <= 0 {
		operatorCount = 1
	}
	return operatorCount * motionCount
}

func (b *Buffer) clearCounts() {
	b.count = ""
	b.opCount = 0
}

func (b *Buffer) pushUndo() {
	b.undo = append(b.undo, snapshot{lines: cloneLines(b.Lines), row: b.Row, col: b.Col})
	if len(b.undo) > 200 {
		b.undo = b.undo[1:]
	}
	b.redo = nil
}

func (b *Buffer) beginInsertUndo() {
	if b.insertTxn {
		return
	}
	b.pushUndo()
	b.insertTxn = true
}

func (b *Buffer) undoLast() {
	if len(b.undo) == 0 {
		return
	}
	s := b.undo[len(b.undo)-1]
	b.undo = b.undo[:len(b.undo)-1]
	b.redo = append(b.redo, snapshot{lines: cloneLines(b.Lines), row: b.Row, col: b.Col})
	b.Lines, b.Row, b.Col = cloneLines(s.lines), s.row, s.col
	b.Dirty = true
	b.clamp()
}

func (b *Buffer) redoLast() {
	if len(b.redo) == 0 {
		return
	}
	s := b.redo[len(b.redo)-1]
	b.redo = b.redo[:len(b.redo)-1]
	b.undo = append(b.undo, snapshot{lines: cloneLines(b.Lines), row: b.Row, col: b.Col})
	b.Lines, b.Row, b.Col = cloneLines(s.lines), s.row, s.col
	b.Dirty = true
	b.clamp()
}

func (b *Buffer) moveLeft() {
	if b.Col > 0 {
		b.Col--
	} else if b.Row > 0 {
		b.Row--
		b.Col = max(0, len(b.Lines[b.Row])-1)
	}
}

func (b *Buffer) moveRight() {
	if b.Col < max(0, len(b.Lines[b.Row])-1) {
		b.Col++
	} else if b.Row < len(b.Lines)-1 {
		b.Row++
		b.Col = 0
	}
}

func (b *Buffer) moveRightInsert() {
	if b.Col < len(b.Lines[b.Row]) {
		b.Col++
	}
}

func (b *Buffer) moveUp() {
	if b.Row > 0 {
		b.Row--
	}
	b.clamp()
}

func (b *Buffer) moveDown() {
	if b.Row < len(b.Lines)-1 {
		b.Row++
	}
	b.clamp()
}

func (b *Buffer) pageUp(lines int) {
	if lines <= 0 {
		lines = 10
	}
	b.Row = max(0, b.Row-lines)
	b.clamp()
}

func (b *Buffer) pageDown(lines int) {
	if lines <= 0 {
		lines = 10
	}
	b.Row = min(len(b.Lines)-1, b.Row+lines)
	b.clamp()
}

func (b *Buffer) wordRight() {
	line := b.Lines[b.Row]
	i := b.Col
	for i < len(line) && !unicode.IsSpace(rune(line[i])) {
		i++
	}
	for i < len(line) && unicode.IsSpace(rune(line[i])) {
		i++
	}
	if i >= len(line) && b.Row < len(b.Lines)-1 {
		b.Row++
		b.Col = firstNonSpace(b.Lines[b.Row])
		return
	}
	b.Col = min(i, len(line))
}

func (b *Buffer) wordLeft() {
	line := b.Lines[b.Row]
	i := b.Col - 1
	for i > 0 && unicode.IsSpace(rune(line[i])) {
		i--
	}
	for i > 0 && !unicode.IsSpace(rune(line[i-1])) {
		i--
	}
	if i < 0 && b.Row > 0 {
		b.Row--
		b.Col = firstNonSpace(b.Lines[b.Row])
		return
	}
	b.Col = max(0, i)
}

func (b *Buffer) wordEnd() {
	line := b.Lines[b.Row]
	i := b.Col
	if i < len(line) && !unicode.IsSpace(rune(line[i])) {
		i++
	}
	for i < len(line) && unicode.IsSpace(rune(line[i])) {
		i++
	}
	for i < len(line)-1 && !unicode.IsSpace(rune(line[i+1])) {
		i++
	}
	b.Col = min(i, max(0, len(line)-1))
}

func (b *Buffer) wordAtCursor() string {
	line := b.Lines[b.Row]
	if line == "" {
		return ""
	}
	start := min(b.Col, len(line)-1)
	for start > 0 && !unicode.IsSpace(rune(line[start-1])) {
		start--
	}
	end := min(b.Col, len(line)-1)
	for end < len(line) && !unicode.IsSpace(rune(line[end])) {
		end++
	}
	return line[start:end]
}

func (b *Buffer) clamp() {
	b.Row = min(max(0, b.Row), len(b.Lines)-1)
	if len(b.Lines[b.Row]) == 0 {
		b.Col = 0
		return
	}
	b.Col = min(max(0, b.Col), max(0, len(b.Lines[b.Row])-1))
}

func (b *Buffer) clampInsert() {
	b.Row = min(max(0, b.Row), len(b.Lines)-1)
	b.Col = min(max(0, b.Col), len(b.Lines[b.Row]))
}

func cloneLines(lines []string) []string {
	out := make([]string, len(lines))
	copy(out, lines)
	return out
}

func insertLine(lines []string, idx int, value string) []string {
	lines = append(lines, "")
	copy(lines[idx+1:], lines[idx:])
	lines[idx] = value
	return lines
}

func firstNonSpace(s string) int {
	for i, r := range s {
		if !unicode.IsSpace(r) {
			return i
		}
	}
	return 0
}

func isNumber(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func renderLine(s string, col int, insert bool) string {
	s = textsafe.Content(s)
	var out strings.Builder
	displayCol := 0
	cursorDrawn := false
	for i, r := range s {
		if i == col {
			cursorDrawn = true
			if r == '\t' {
				spaces := tabWidth(displayCol)
				writeCursorRune(&out, ' ')
				if spaces > 1 {
					out.WriteString(strings.Repeat(" ", spaces-1))
				}
				displayCol += spaces
				continue
			}
			writeCursorRune(&out, r)
			displayCol++
			continue
		}
		if r == '\t' {
			spaces := tabWidth(displayCol)
			out.WriteString(strings.Repeat(" ", spaces))
			displayCol += spaces
			continue
		}
		out.WriteRune(r)
		displayCol++
	}
	if col >= len(s) || (insert && col == len(s)) {
		cursorDrawn = true
		out.WriteRune('█')
	}
	if col >= 0 && !cursorDrawn {
		out.WriteRune('█')
	}
	return out.String()
}

func renderHighlightedLine(s string, col int, insert bool) string {
	var out strings.Builder
	displayCol := 0
	sourceByte := 0
	cursorDrawn := false
	for i := 0; i < len(s); {
		if strings.HasPrefix(s[i:], "\x1b[") {
			end := i + 2
			for end < len(s) && s[end] != 'm' {
				end++
			}
			if end < len(s) {
				end++
			}
			out.WriteString(s[i:end])
			i = end
			continue
		}
		r, size := rune(s[i]), 1
		if r >= utf8.RuneSelf {
			r, size = utf8.DecodeRuneInString(s[i:])
		}
		if sourceByte == col {
			cursorDrawn = true
			if r == '\t' {
				spaces := tabWidth(displayCol)
				writeCursorRune(&out, ' ')
				if spaces > 1 {
					out.WriteString(strings.Repeat(" ", spaces-1))
				}
				displayCol += spaces
			} else {
				writeCursorRune(&out, r)
				displayCol++
			}
			sourceByte += size
			i += size
			continue
		}
		if r == '\t' {
			spaces := tabWidth(displayCol)
			out.WriteString(strings.Repeat(" ", spaces))
			displayCol += spaces
			sourceByte += size
			i += size
			continue
		}
		out.WriteRune(r)
		displayCol++
		sourceByte += size
		i += size
	}
	if col >= sourceByte || (insert && col == sourceByte) {
		cursorDrawn = true
		out.WriteRune('█')
	}
	if col >= 0 && !cursorDrawn {
		out.WriteRune('█')
	}
	return out.String()
}

func writeCursorRune(out *strings.Builder, r rune) {
	out.WriteString("\x1b[7m")
	out.WriteRune(r)
	out.WriteString("\x1b[27m")
}

func tabWidth(displayCol int) int {
	return 4 - displayCol%4
}

func wrapDisplay(s, continuation string, width int) []string {
	return wrapDisplayLimit(s, continuation, width, 0)
}

func wrapDisplayLimit(s, continuation string, width, maxLines int) []string {
	if width <= 0 {
		return []string{""}
	}
	if strings.Contains(s, "\x1b[") {
		wrapped := strings.Split(ansi.Wrap(s, width, ""), "\n")
		for i := 1; i < len(wrapped); i++ {
			wrapped[i] = continuation + wrapped[i]
		}
		if maxLines > 0 && len(wrapped) > maxLines {
			return wrapped[:maxLines]
		}
		return wrapped
	}
	_, truncated := byteAfterRunes(s, width)
	if !truncated {
		return []string{s}
	}
	var lines []string
	first := true
	remaining := s
	for len(remaining) > 0 && (maxLines <= 0 || len(lines) < maxLines) {
		prefix := ""
		if !first {
			prefix = continuation
		}
		limit := width - utf8.RuneCountInString(prefix)
		if limit < 1 {
			limit = 1
		}
		end, _ := byteAfterRunes(remaining, limit)
		lines = append(lines, prefix+remaining[:end])
		remaining = remaining[end:]
		first = false
	}
	return lines
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
