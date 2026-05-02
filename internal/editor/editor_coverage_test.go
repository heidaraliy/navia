package editor

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestModeStringVariants(t *testing.T) {
	cases := map[Mode]string{
		Normal:     "NORMAL",
		Insert:     "INSERT",
		Visual:     "VISUAL",
		VisualLine: "V-LINE",
		Command:    "COMMAND",
		Search:     "SEARCH",
		Mode(99):   "NORMAL",
	}
	for mode, want := range cases {
		if got := mode.String(); got != want {
			t.Fatalf("%v.String() = %q, want %q", int(mode), got, want)
		}
	}
}

func TestOpenVariantsAndSaveEdgeCases(t *testing.T) {
	dir := t.TempDir()
	if _, err := Open(dir, 1024); !errors.Is(err, ErrDirectory) {
		t.Fatalf("Open(dir) err = %v, want ErrDirectory", err)
	}
	if _, err := Open(filepath.Join(dir, "missing.txt"), 1024); err == nil {
		t.Fatal("Open(missing) succeeded")
	}

	crlf := filepath.Join(dir, "crlf.txt")
	if err := os.WriteFile(crlf, []byte("one\r\ntwo\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	b, err := Open(crlf, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := b.Value(); got != "one\ntwo" {
		t.Fatalf("normalized value = %q", got)
	}
	if b.Name != "crlf.txt" || b.Mode != Normal || b.FileMode.Perm() != 0o600 || b.visualRow != -1 {
		t.Fatalf("opened metadata = name %q mode %v perm %o visualRow %d", b.Name, b.Mode, b.FileMode.Perm(), b.visualRow)
	}

	empty := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	emptyBuf, err := Open(empty, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if got := emptyBuf.Lines; !reflect.DeepEqual(got, []string{""}) {
		t.Fatalf("empty lines = %#v", got)
	}

	scratch := NewScratch(filepath.Join(dir, "created.txt"))
	scratch.FileMode = 0
	scratch.Lines = []string{"created"}
	if err := scratch.Save(false); err != nil {
		t.Fatal(err)
	}
	if scratch.Dirty || scratch.FileMode.Perm() != 0o644 || scratch.ModifiedAt.IsZero() {
		t.Fatalf("saved scratch metadata dirty=%v mode=%o modified=%v", scratch.Dirty, scratch.FileMode.Perm(), scratch.ModifiedAt)
	}

	if err := NewScratch("").Save(false); err == nil || !strings.Contains(err.Error(), "no path") {
		t.Fatalf("Save without path err = %v", err)
	}
	missingDir := NewScratch(filepath.Join(dir, "missing", "x.txt"))
	if err := missingDir.Save(false); err == nil {
		t.Fatal("Save into missing directory succeeded")
	}
}

func TestCommandModeEditingAndExecuteActions(t *testing.T) {
	b := NewScratch("x.txt")
	b.HandleKey(":")
	for _, key := range []string{"t", "h", "e", "m", "e", " ", "d", "a", "r", "k"} {
		b.HandleKey(key)
	}
	if got := b.CommandLine(); got != ":theme dark" {
		t.Fatalf("command line = %q", got)
	}
	b.HandleKey("backspace")
	if got := b.CommandLine(); got != ":theme dar" {
		t.Fatalf("after backspace = %q", got)
	}
	b.HandleKey("esc")
	if b.Mode != Normal || b.CommandLine() != "" {
		t.Fatalf("command escape mode=%v line=%q", b.Mode, b.CommandLine())
	}

	b.HandleKey(":")
	for _, key := range []string{"e", " ", "n", "e", "x", "t", ".", "t", "x", "t"} {
		b.HandleKey(key)
	}
	act := b.HandleKey("enter")
	if act.Kind != ActionOpen || act.Path != "next.txt" {
		t.Fatalf(":e action = %#v", act)
	}

	cases := map[string]Action{
		"w":       {Kind: ActionSave},
		"w!":      {Kind: ActionSave, Force: true},
		"q":       {Kind: ActionClose},
		"q!":      {Kind: ActionCloseForce},
		"bd!":     {Kind: ActionCloseForce},
		"wq":      {Kind: ActionSave, Message: "close"},
		"qa":      {Kind: ActionQuitAll},
		"qa!":     {Kind: ActionQuitAllForce},
		"bd":      {Kind: ActionClose},
		"nvim":    {Kind: ActionExternal},
		"theme":   {Kind: ActionTheme},
		"theme x": {Kind: ActionTheme, Message: "x"},
	}
	for cmd, want := range cases {
		got := b.Execute(cmd)
		if got.Kind != want.Kind || got.Message != want.Message || got.Force != want.Force {
			t.Fatalf(":%s = %#v, want %#v", cmd, got, want)
		}
	}
	if act := b.Execute("nope"); act.Kind != ActionStatus || !strings.Contains(act.Message, ":nope") {
		t.Fatalf("unknown action = %#v", act)
	}
}

func TestSearchModeQueryAndNavigation(t *testing.T) {
	b := NewScratch("x.txt")
	b.Lines = []string{"alpha", "Beta", "gamma beta"}
	b.HandleKey("/")
	for _, key := range []string{"b", "e", "t", "a"} {
		b.HandleKey(key)
	}
	if got := b.CommandLine(); got != "/beta" {
		t.Fatalf("search command line = %q", got)
	}
	if got := b.SearchQuery(); got != "beta" {
		t.Fatalf("live search query = %q", got)
	}
	b.HandleKey("backspace")
	b.HandleKey("a")
	b.HandleKey("enter")
	if b.Mode != Normal || b.Row != 1 || b.Col != 0 || b.SearchQuery() != "beta" {
		t.Fatalf("after search mode=%v row=%d col=%d query=%q", b.Mode, b.Row, b.Col, b.SearchQuery())
	}
	b.HandleKey("n")
	if b.Row != 2 || b.Col != 6 {
		t.Fatalf("n row=%d col=%d", b.Row, b.Col)
	}
	b.HandleKey("N")
	if b.Row != 1 || b.Col != 0 {
		t.Fatalf("N row=%d col=%d", b.Row, b.Col)
	}
	b.HandleKey("/")
	b.HandleKey("x")
	b.HandleKey("esc")
	if b.Mode != Normal || b.CommandLine() != "" || b.SearchQuery() != "beta" {
		t.Fatalf("escaped search mode=%v line=%q last=%q", b.Mode, b.CommandLine(), b.SearchQuery())
	}
	b.lastQuery = ""
	b.findNext(1)
	if b.Row != 1 {
		t.Fatalf("findNext with no query moved row to %d", b.Row)
	}
}

func TestNormalInsertAndVisualMotions(t *testing.T) {
	b := NewScratch("x.txt")
	b.Lines = []string{"  alpha beta", "second", "third", "fourth"}
	b.Row, b.Col = 1, 2
	b.HandleKey("h")
	b.HandleKey("left")
	if b.Col != 0 {
		t.Fatalf("left col = %d", b.Col)
	}
	b.HandleKey("h")
	if b.Row != 0 || b.Col != len("  alpha beta")-1 {
		t.Fatalf("cross-line left row=%d col=%d", b.Row, b.Col)
	}
	b.HandleKey("l")
	b.HandleKey("right")
	if b.Row != 1 || b.Col != 1 {
		t.Fatalf("right wrap row=%d col=%d", b.Row, b.Col)
	}
	b.HandleKey("k")
	b.HandleKey("up")
	if b.Row != 0 {
		t.Fatalf("up row = %d", b.Row)
	}
	b.HandleKey("j")
	b.HandleKey("down")
	if b.Row != 2 {
		t.Fatalf("down row = %d", b.Row)
	}
	b.HandleKey("g")
	b.HandleKey("g")
	if b.Row != 0 {
		t.Fatalf("gg row = %d", b.Row)
	}
	b.HandleKey("3")
	b.HandleKey("G")
	if b.Row != 2 {
		t.Fatalf("3G row = %d", b.Row)
	}
	b.HandleKey("G")
	if b.Row != 3 {
		t.Fatalf("G row = %d", b.Row)
	}
	b.HandleKey("0")
	b.HandleKey("$")
	if b.Col != len(b.Lines[b.Row])-1 {
		t.Fatalf("$ col = %d", b.Col)
	}
	b.HandleKey("home")
	b.HandleKey("end")
	if b.Col != len(b.Lines[b.Row])-1 {
		t.Fatalf("home/end col = %d", b.Col)
	}

	b.Row, b.Col = 0, 0
	b.HandleKey("w")
	if b.Col != 2 {
		t.Fatalf("w col = %d", b.Col)
	}
	b.HandleKey("e")
	if b.Col != len("  alpha")-1 {
		t.Fatalf("e col = %d", b.Col)
	}
	b.HandleKey("b")
	if b.Col != 2 {
		t.Fatalf("b col = %d", b.Col)
	}

	b.HandleKey("I")
	if b.Mode != Insert || b.Col != 2 {
		t.Fatalf("I mode=%v col=%d", b.Mode, b.Col)
	}
	b.HandleKey("esc")
	b.HandleKey("A")
	if b.Mode != Insert || b.Col != len(b.Lines[b.Row]) {
		t.Fatalf("A mode=%v col=%d", b.Mode, b.Col)
	}
	b.HandleKey("enter")
	b.HandleKey("tab")
	b.HandleKey("backspace")
	b.HandleKey("delete")
	b.HandleKey("left")
	b.HandleKey("right")
	b.HandleKey("up")
	b.HandleKey("down")
	b.HandleKey("ctrl+d")
	b.HandleKey("ctrl+u")
	b.HandleKey("esc")
	if b.Mode != Normal {
		t.Fatalf("insert escape mode = %v", b.Mode)
	}

	b.Row, b.Col = 1, 0
	b.HandleKey("O")
	if b.Mode != Insert || b.Row != 1 || b.Lines[1] != "" {
		t.Fatalf("O mode=%v row=%d lines=%#v", b.Mode, b.Row, b.Lines)
	}
	b.HandleKey("esc")
	b.HandleKey("o")
	if b.Mode != Insert || b.Row != 2 || b.Lines[2] != "" {
		t.Fatalf("o mode=%v row=%d lines=%#v", b.Mode, b.Row, b.Lines)
	}
	b.HandleKey("esc")

	b.Lines = []string{"abcd", "efgh"}
	b.Row, b.Col = 0, 1
	b.HandleKey("v")
	b.HandleKey("l")
	b.HandleKey("y")
	if b.Mode != Normal || b.Register != "bc" {
		t.Fatalf("visual yank mode=%v register=%q", b.Mode, b.Register)
	}
	b.HandleKey("v")
	b.HandleKey("esc")
	if b.Mode != Normal || b.visualRow != -1 {
		t.Fatalf("visual esc mode=%v visualRow=%d", b.Mode, b.visualRow)
	}
	b.Row, b.Col = 0, 1
	b.HandleKey("v")
	b.HandleKey("l")
	b.HandleKey("d")
	if got := b.Lines[0]; got != "ad" {
		t.Fatalf("visual delete line = %q", got)
	}
	b.Lines = []string{"abcd", "efgh", "ijkl"}
	b.Row, b.Col = 2, 1
	b.HandleKey("v")
	b.HandleKey("k")
	b.HandleKey("y")
	if b.Register != "efgh\nijkl" {
		t.Fatalf("multi visual yank = %q", b.Register)
	}
	b.Row, b.Col = 2, 0
	b.HandleKey("V")
	b.HandleKey("k")
	b.HandleKey("d")
	if got := b.Value(); got != "abcd" {
		t.Fatalf("reverse visual line delete = %q", got)
	}
}

func TestOperatorsTextObjectsFindAndPaste(t *testing.T) {
	b := NewScratch("x.txt")
	b.Lines = []string{"alpha beta gamma delta"}
	b.HandleKey("y")
	b.HandleKey("w")
	if b.Register != "alpha " {
		t.Fatalf("yw register = %q", b.Register)
	}
	b.HandleKey("P")
	if got := b.Lines[0]; got != "alpha alpha beta gamma delta" {
		t.Fatalf("P char paste = %q", got)
	}
	b.HandleKey("u")
	b.HandleKey("p")
	if got := b.Lines[0]; got != "aalpha lpha beta gamma delta" {
		t.Fatalf("p char paste = %q", got)
	}
	b.HandleKey("u")

	b.Register = "line one\nline two\n"
	b.HandleKey("p")
	if got := b.Value(); got != "alpha beta gamma delta\nline one\nline two" {
		t.Fatalf("p line paste = %q", got)
	}
	b.HandleKey("u")
	b.Register = "before\n"
	b.HandleKey("P")
	if got := b.Value(); got != "before\nalpha beta gamma delta" {
		t.Fatalf("P line paste = %q", got)
	}

	b = NewScratch("x.txt")
	b.Lines = []string{"alpha beta gamma"}
	b.Row, b.Col = 0, 6
	b.HandleKey("d")
	b.HandleKey("$")
	if got := b.Value(); got != "alpha " || b.Register != "beta gamma" {
		t.Fatalf("d$ value=%q register=%q", got, b.Register)
	}
	b.HandleKey("u")
	b.HandleKey("y")
	b.HandleKey("$")
	if b.Register != "beta gamma" {
		t.Fatalf("y$ register = %q", b.Register)
	}
	b.HandleKey("d")
	b.HandleKey("d")
	if got := b.Value(); got != "" || b.Register != "alpha beta gamma\n" {
		t.Fatalf("dd value=%q register=%q", got, b.Register)
	}
	b.HandleKey("u")
	b.HandleKey("c")
	b.HandleKey("c")
	if b.Mode != Insert || b.Value() != "" {
		t.Fatalf("cc mode=%v value=%q", b.Mode, b.Value())
	}

	b = NewScratch("x.txt")
	b.Lines = []string{"one two three", "four five"}
	b.HandleKey("2")
	b.HandleKey("y")
	b.HandleKey("y")
	if b.Register != "one two three\nfour five\n" {
		t.Fatalf("2yy register = %q", b.Register)
	}

	b = NewScratch("x.txt")
	b.Lines = []string{`call(foo[bar] + {baz})`}
	for _, tc := range []struct {
		name string
		col  int
		keys []string
		want string
		mode Mode
	}{
		{name: "around parens", col: 5, keys: []string{"d", "a", "("}, want: "call", mode: Normal},
		{name: "inside brackets", col: 10, keys: []string{"c", "i", "]"}, want: `call(foo[] + {baz})`, mode: Insert},
		{name: "around braces", col: 17, keys: []string{"d", "a", "}"}, want: `call(foo[bar] + )`, mode: Normal},
	} {
		t.Run(tc.name, func(t *testing.T) {
			buf := NewScratch("x.txt")
			buf.Lines = []string{`call(foo[bar] + {baz})`}
			buf.Col = tc.col
			for _, key := range tc.keys {
				buf.HandleKey(key)
			}
			if got := buf.Value(); got != tc.want || buf.Mode != tc.mode {
				t.Fatalf("value=%q mode=%v, want %q/%v", got, buf.Mode, tc.want, tc.mode)
			}
		})
	}

	b = NewScratch("x.txt")
	b.Lines = []string{"abc def ghi"}
	b.HandleKey("d")
	if act := b.HandleKey("z"); act.Kind != ActionStatus {
		t.Fatalf("unknown pending action = %#v", act)
	}
	b.HandleKey("d")
	b.HandleKey("i")
	b.HandleKey("x")
	if got := b.Value(); got != "abc def ghi" {
		t.Fatalf("unknown text object changed value to %q", got)
	}

	b.Col = 0
	b.HandleKey("f")
	b.HandleKey("i")
	if b.Col != 10 {
		t.Fatalf("fi col = %d", b.Col)
	}
	b.HandleKey("T")
	b.HandleKey("d")
	if b.Col != 5 {
		t.Fatalf("Td col = %d", b.Col)
	}
	b.HandleKey("2")
	b.HandleKey("F")
	b.HandleKey("x")
	if b.Col != 5 {
		t.Fatalf("failed find should preserve col, got %d", b.Col)
	}

	b = NewScratch("x.txt")
	b.Lines = []string{"abc def ghi"}
	b.Col = 0
	b.HandleKey("d")
	b.HandleKey("t")
	b.HandleKey("g")
	if got := b.Value(); got != " ghi" {
		t.Fatalf("dtg value = %q", got)
	}
}

func TestUndoRedoBoundariesAndHelpers(t *testing.T) {
	b := NewScratch("x.txt")
	b.Lines = []string{"abc", "def"}
	b.HandleKey("u")
	b.HandleKey("ctrl+r")
	if got := b.Value(); got != "abc\ndef" {
		t.Fatalf("empty undo/redo changed value = %q", got)
	}
	b.Mode = Insert
	b.Row, b.Col = 0, 0
	b.HandleKey("backspace")
	if got := b.Value(); got != "abc\ndef" {
		t.Fatalf("backspace at origin changed value = %q", got)
	}
	b.Row, b.Col = 1, 0
	b.HandleKey("backspace")
	if got := b.Value(); got != "abcdef" || b.Row != 0 || b.Col != 3 {
		t.Fatalf("join backspace value=%q row=%d col=%d", got, b.Row, b.Col)
	}
	b.HandleKey("delete")
	if got := b.Value(); got != "abcef" {
		t.Fatalf("delete rune value = %q", got)
	}
	b.HandleKey("delete")
	b.HandleKey("delete")
	b.HandleKey("delete")
	b.HandleKey("delete")
	b.HandleKey("delete")
	if got := b.Value(); got != "abc" {
		t.Fatalf("delete at end value = %q", got)
	}

	for i := 0; i < 205; i++ {
		b.pushUndo()
		b.Lines = []string{string(rune('a' + i%26))}
	}
	if len(b.undo) != 200 {
		t.Fatalf("undo len = %d", len(b.undo))
	}

	b.Lines = []string{"", "  word"}
	b.Row, b.Col = 0, 0
	b.clamp()
	if b.Col != 0 || b.wordAtCursor() != "" {
		t.Fatalf("empty clamp/word col=%d word=%q", b.Col, b.wordAtCursor())
	}
	b.Row, b.Col = 1, 3
	if got := b.wordAtCursor(); got != "word" {
		t.Fatalf("wordAtCursor = %q", got)
	}
	b.Row, b.Col = 1, 0
	b.wordLeft()
	if b.Row != 0 || b.Col != 0 {
		t.Fatalf("wordLeft previous row=%d col=%d", b.Row, b.Col)
	}
	b.Row, b.Col = 0, 0
	b.wordRight()
	if b.Row != 1 || b.Col != 2 {
		t.Fatalf("wordRight next row=%d col=%d", b.Row, b.Col)
	}
	b.wordEnd()
	if b.Col != len("  word")-1 {
		t.Fatalf("wordEnd col = %d", b.Col)
	}

	lines := []string{"a"}
	lines = insertLine(lines, 1, "b")
	if !reflect.DeepEqual(lines, []string{"a", "b"}) {
		t.Fatalf("insertLine = %#v", lines)
	}
	if isNumber("") || isNumber("12x") || !isNumber("12") {
		t.Fatal("isNumber results were wrong")
	}
	if min(1, 2) != 1 || max(1, 2) != 2 || max(2, 1) != 2 {
		t.Fatal("min/max results were wrong")
	}
}

func TestVisibilityWrappingAndAccessors(t *testing.T) {
	b := NewScratch("x.txt")
	b.Lines = []string{"one", "two", "three"}
	b.Row, b.Col = 1, 1
	if b.CursorLine() != 2 || b.CursorCol() != 2 {
		t.Fatalf("cursor line/col = %d/%d", b.CursorLine(), b.CursorCol())
	}
	highlighted := b.VisibleHighlighted(8, 4, func(path, line string) string {
		if path != "x.txt" {
			t.Fatalf("highlight path = %q", path)
		}
		if strings.Contains(line, "█") {
			t.Fatalf("highlight received cursor-mutated line = %q", line)
		}
		return "\x1b[31m" + line + "\x1b[0m"
	})
	if len(highlighted) != 4 || !strings.Contains(highlighted[0], "\x1b[31m") {
		t.Fatalf("highlighted visible = %#v", highlighted)
	}
	if !strings.Contains(highlighted[1], "\x1b[31mt█o\x1b[0m") {
		t.Fatalf("highlighted cursor lost style = %#v", highlighted)
	}
	if got := b.Visible(0, 2); len(got) != 2 || got[0] != "" {
		t.Fatalf("zero-width visible = %#v", got)
	}
	if got := b.Visible(10, 0); got != nil {
		t.Fatalf("zero-height visible = %#v", got)
	}

	long := strings.Repeat("x", 5000)
	window, col := visibleLineWindow(long, 4500, 40, 2)
	if !strings.HasPrefix(window, "... ") || !strings.HasSuffix(window, " ...") || col <= 0 {
		t.Fatalf("window prefix/suffix/col = %q/%d", window[:min(10, len(window))], col)
	}
	window, col = visibleLineWindow(long, -1, 40, 2)
	if !strings.HasSuffix(window, " ...") || col != -1 {
		t.Fatalf("non-cursor window suffix/col = %q/%d", window[len(window)-5:], col)
	}
	if got := renderLine("ab", -1, false); got != "ab" {
		t.Fatalf("renderLine no cursor = %q", got)
	}
	if got := renderLine("ab", 2, true); got != "ab█" {
		t.Fatalf("renderLine insert end = %q", got)
	}
	if got := renderHighlightedLine("\x1b[31mword\x1b[0m", 1, false); got != "\x1b[31mw█rd\x1b[0m" {
		t.Fatalf("renderHighlightedLine cursor style = %q", got)
	}
	if got := tabWidth(5); got != 3 {
		t.Fatalf("tabWidth = %d", got)
	}
	if got := wrapDisplay("abcdef", "..", 3); !reflect.DeepEqual(got, []string{"abc", "..d", "..e", "..f"}) {
		t.Fatalf("wrapDisplay = %#v", got)
	}
	if got := wrapDisplayLimit("abcdef", ">", 3, 2); !reflect.DeepEqual(got, []string{"abc", ">de"}) {
		t.Fatalf("wrapDisplayLimit = %#v", got)
	}
	if got := wrapDisplayLimit("abc", ">", 0, 0); !reflect.DeepEqual(got, []string{""}) {
		t.Fatalf("wrapDisplayLimit zero width = %#v", got)
	}
}

func TestExecuteGotoSubstituteErrorsAndTimestamps(t *testing.T) {
	b := NewScratch("x.txt")
	b.Lines = []string{"one", "two", "three"}
	b.Row = 1
	if act := b.Execute("$"); act.Kind != ActionNone || b.Row != 2 {
		t.Fatalf(":$ action=%v row=%d", act.Kind, b.Row)
	}
	if act := b.Execute("99"); act.Kind != ActionNone || b.Row != 2 {
		t.Fatalf(":99 action=%v row=%d", act.Kind, b.Row)
	}
	if act := b.Execute("s/[//"); act.Kind != ActionStatus || act.Message == "" {
		t.Fatalf("bad regexp action = %#v", act)
	}
	if act := b.Execute("s/"); act.Kind != ActionStatus || !strings.Contains(act.Message, "invalid") {
		t.Fatalf("bad substitute action = %#v", act)
	}
	before := b.Value()
	if act := b.Execute("s/missing/repl/"); act.Kind != ActionNone || b.Value() != before {
		t.Fatalf("missing substitute action=%#v value=%q", act, b.Value())
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "mtime.txt")
	if err := os.WriteFile(path, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	opened, err := Open(path, 1024)
	if err != nil {
		t.Fatal(err)
	}
	opened.ModifiedAt = time.Time{}
	if err := os.WriteFile(path, []byte("external"), 0o644); err != nil {
		t.Fatal(err)
	}
	opened.Lines = []string{"local"}
	if err := opened.Save(false); err != nil {
		t.Fatalf("zero ModifiedAt save err = %v", err)
	}
}

func TestRemainingNormalModeAndHelperEdges(t *testing.T) {
	b := NewScratch("x.txt")
	b.Lines = []string{"one two", "three four", "five six"}
	b.HandleKey("ctrl+w")
	if got := b.NormalCommandLine(); got != "ctrl+w" {
		t.Fatalf("ctrl+w command line = %q", got)
	}
	if act := b.HandleKey("x"); act.Kind != ActionStatus {
		t.Fatalf("unknown ctrl+w action = %#v", act)
	}
	b.HandleKey("2")
	if got := b.NormalCommandLine(); got != "2" {
		t.Fatalf("count command line = %q", got)
	}
	b.HandleKey("g")
	b.HandleKey("g")
	if b.Row != 1 {
		t.Fatalf("2gg row = %d", b.Row)
	}
	for key, want := range map[string]ActionKind{
		"d": ActionDefinition,
		"r": ActionReferences,
		"t": ActionNextTab,
		"T": ActionPrevTab,
	} {
		b.HandleKey("g")
		if got := b.HandleKey(key).Kind; got != want {
			t.Fatalf("g%s action = %v, want %v", key, got, want)
		}
	}
	b.HandleKey("g")
	if act := b.HandleKey("x"); act.Kind != ActionStatus {
		t.Fatalf("unknown g action = %#v", act)
	}
	b.HandleKey(";")
	b.HandleKey(",")
	if got := b.HandleKey("ctrl+i").Kind; got != ActionJumpForward {
		t.Fatalf("ctrl+i action = %v", got)
	}

	b = NewScratch("x.txt")
	b.Lines = []string{"abcdef", "ghij"}
	b.Row, b.Col = 0, 2
	b.deleteWord()
	if got := b.Value(); got != "abcdef\nij" || b.Register != "gh" {
		t.Fatalf("deleteWord value=%q register=%q", got, b.Register)
	}
	b.Lines = []string{"abcdef"}
	b.Row, b.Col = 0, 3
	b.deleteToEnd()
	if got := b.Value(); got != "abc" || b.Register != "def" {
		t.Fatalf("deleteToEnd value=%q register=%q", got, b.Register)
	}

	r := Range{StartRow: 1, StartCol: 20, EndRow: 0, EndCol: -4}
	b.Lines = []string{"alpha", "beta"}
	b.normalizeRange(&r)
	if r != (Range{StartRow: 0, StartCol: 0, EndRow: 1, EndCol: 4}) {
		t.Fatalf("normalized range = %#v", r)
	}
	if got := b.rangeText(r); got != "" {
		t.Fatalf("multi-line rangeText = %q", got)
	}
	b.deleteRange(r)
	if got := b.Value(); got != "alpha\nbeta" {
		t.Fatalf("multi-line deleteRange changed value to %q", got)
	}

	b.Lines = []string{"abc"}
	b.Row, b.Col = 0, 1
	b.applyOperator("y", Range{StartRow: 0, StartCol: 3, EndRow: 0, EndCol: 1})
	if b.Register != "bc" {
		t.Fatalf("reverse yank register = %q", b.Register)
	}
	b.applyLineOperator("x", 1)
	if got := b.Value(); got != "abc" {
		t.Fatalf("unknown line operator changed value = %q", got)
	}

	if r, ok := b.textObjectRange('x', false, 1); ok || r != (Range{}) {
		t.Fatalf("unknown text object range = %#v/%v", r, ok)
	}
	b.Lines = []string{""}
	b.Row, b.Col = 0, 0
	if r := b.wordObjectRange(false, 0); r != (Range{StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 0}) {
		t.Fatalf("empty word object = %#v", r)
	}
	b.Lines = []string{"no pairs here"}
	if _, ok := b.pairObjectRange('(', ')', false); ok {
		t.Fatal("pairObjectRange found missing pair")
	}
	b.Lines = []string{""}
	if _, ok := b.findRange('x', false, false); ok {
		t.Fatal("findRange found target in empty line")
	}
	b.Lines = []string{"abc"}
	b.Row, b.Col = 0, 0
	if _, ok := b.findRangeCount('z', false, false, 2); ok || b.Col != 0 {
		t.Fatalf("failed findRangeCount ok=%v col=%d", ok, b.Col)
	}

	called := 0
	repeat(0, func() { called++ })
	if called != 1 {
		t.Fatalf("repeat(0) called %d times", called)
	}
	b.Lines = []string{"a", "b", "c"}
	b.Row = 1
	b.pageUp(0)
	if b.Row != 0 {
		t.Fatalf("pageUp default row = %d", b.Row)
	}
	b.pageDown(0)
	if b.Row != 2 {
		t.Fatalf("pageDown default row = %d", b.Row)
	}
}

func TestRemainingVisualAndWrappingEdges(t *testing.T) {
	b := NewScratch("x.txt")
	b.Lines = []string{"abc", "def", "ghi", "jkl", "mno", "pqr", "stu", "vwx", "yz", "last", "end"}
	b.Row, b.Col = 5, 1
	b.HandleKey("v")
	b.HandleKey("h")
	b.HandleKey("left")
	b.HandleKey("l")
	b.HandleKey("right")
	b.HandleKey("ctrl+u")
	b.HandleKey("ctrl+d")
	b.HandleKey("v")
	if b.Mode != Normal || b.visualRow != -1 {
		t.Fatalf("visual toggle mode=%v visualRow=%d", b.Mode, b.visualRow)
	}
	b.HandleKey("V")
	b.HandleKey("V")
	if b.Mode != Normal || b.visualRow != -1 {
		t.Fatalf("visual-line toggle mode=%v visualRow=%d", b.Mode, b.visualRow)
	}

	if got := renderLine("\tab", 0, false); got != "█   ab" {
		t.Fatalf("renderLine tab cursor = %q", got)
	}
	window, col := visibleLineWindow(strings.Repeat("x", 5000), 6000, 20, 1)
	if !strings.HasPrefix(window, "... ") || strings.HasSuffix(window, " ...") || col <= 0 {
		t.Fatalf("clamped visibleLineWindow col=%d len=%d", col, len(window))
	}
	ansiWrapped := wrapDisplayLimit("\x1b[31mabcdef\x1b[0m", ">", 3, 1)
	if len(ansiWrapped) != 1 || !strings.Contains(ansiWrapped[0], "\x1b[31m") {
		t.Fatalf("ansi wrapped = %#v", ansiWrapped)
	}
}

func TestRemainingSmallBranchEdges(t *testing.T) {
	b := NewScratch("x.txt")
	if got := b.NormalCommandLine(); got != "" {
		t.Fatalf("empty normal command line = %q", got)
	}
	b.Mode = Insert
	b.pending = "d"
	if got := b.NormalCommandLine(); got != "" {
		t.Fatalf("insert normal command line = %q", got)
	}

	b = NewScratch("x.txt")
	b.Lines = []string{"abc def"}
	b.Row, b.Col = 0, 0
	if r, ok := b.findRangeCount('d', false, false, 0); !ok || r.EndCol != 4 {
		t.Fatalf("findRangeCount default = %#v/%v", r, ok)
	}
	if r := b.motionWordRange(0); r.EndCol != 4 {
		t.Fatalf("motionWordRange default = %#v", r)
	}
	b.count = "bad"
	if count, specified := b.takeCountWithSpecified(); count != 1 || !specified {
		t.Fatalf("bad count result = %d/%v", count, specified)
	}
	b.opCount = -1
	if got := b.effectiveCount(); got != 1 {
		t.Fatalf("negative op effective count = %d", got)
	}

	b.Lines = []string{"abc def"}
	b.Row, b.Col = 0, 4
	if r := b.wordObjectRange(false, 2); r != (Range{StartRow: 0, StartCol: 4, EndRow: 0, EndCol: 7}) {
		t.Fatalf("inside word object = %#v", r)
	}
	b.applyLineOperator("d", 0)
	if got := b.Value(); got != "" || b.Register != "abc def\n" {
		t.Fatalf("default line delete value=%q register=%q", got, b.Register)
	}

	b = NewScratch("x.txt")
	b.Register = ""
	b.pasteAfter()
	b.pasteBefore()
	if got := b.Value(); got != "" {
		t.Fatalf("empty paste changed value = %q", got)
	}

	b.Lines = []string{"abc", "def"}
	b.visualRow = -1
	if got := b.selectionText(); got != "" {
		t.Fatalf("selection without visual anchor = %q", got)
	}
	b.deleteSelection()
	if got := b.Value(); got != "abc\ndef" {
		t.Fatalf("delete without visual anchor changed value = %q", got)
	}
	b.Mode = Visual
	b.visualRow, b.visualCol = 0, 2
	b.Row, b.Col = 0, 0
	if got := b.selectionText(); got != "abc" {
		t.Fatalf("reverse same-line selection = %q", got)
	}
	b.Mode = VisualLine
	b.visualRow = 0
	b.Row = 1
	b.deleteSelection()
	if got := b.Value(); got != "" || b.Row != 0 || b.Col != 0 {
		t.Fatalf("delete all selection value=%q row=%d col=%d", got, b.Row, b.Col)
	}

	b = NewScratch("x.txt")
	b.Lines = []string{"alpha", "beta"}
	b.Row = 0
	b.lastQuery = "missing"
	b.findNext(1)
	if b.Row != 0 {
		t.Fatalf("missing find moved row to %d", b.Row)
	}

	b.Lines = []string{"   ", "word"}
	b.Row, b.Col = 0, 2
	b.wordLeft()
	if b.Col != 0 {
		t.Fatalf("wordLeft over spaces col = %d", b.Col)
	}
	b.wordEnd()
	if b.Col != 2 {
		t.Fatalf("wordEnd on spaces col = %d", b.Col)
	}
	b.Row, b.Col = 1, 10
	b.clamp()
	if b.Col != len("word")-1 {
		t.Fatalf("clamp col = %d", b.Col)
	}
}
