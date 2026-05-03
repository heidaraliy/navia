package editor

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOpenRefusesBinaryAndLargeFiles(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.WriteFile(bin, []byte{'a', 0, 'b'}, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(bin, 1024); !errors.Is(err, ErrBinary) {
		t.Fatalf("expected binary refusal, got %v", err)
	}
	large := filepath.Join(dir, "large.txt")
	if err := os.WriteFile(large, []byte("abcdef"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(large, 3); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("expected large refusal, got %v", err)
	}
}

func TestInsertUndoRedoAndSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := Open(path, 1024)
	if err != nil {
		t.Fatal(err)
	}
	b.HandleKey("A")
	b.HandleKey("!")
	b.HandleKey("esc")
	if got := b.Value(); got != "hello!" {
		t.Fatalf("value = %q", got)
	}
	b.HandleKey("u")
	if got := b.Value(); got != "hello" {
		t.Fatalf("undo value = %q", got)
	}
	b.HandleKey("ctrl+r")
	if got := b.Value(); got != "hello!" {
		t.Fatalf("redo value = %q", got)
	}
	if err := b.Save(false); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello!" {
		t.Fatalf("saved = %q", data)
	}
}

func TestInsertModeTypingUsesOneUndoSnapshot(t *testing.T) {
	b := NewScratch("x.txt")
	b.Lines = []string{""}
	b.Dirty = false

	b.HandleKey("i")
	b.HandleKey("a")
	b.HandleKey("b")
	b.HandleKey("c")
	if len(b.undo) != 1 {
		t.Fatalf("undo snapshots while typing = %d, want 1", len(b.undo))
	}
	b.HandleKey("esc")
	b.HandleKey("u")
	if got := b.Value(); got != "" {
		t.Fatalf("undo value = %q, want empty", got)
	}
}

func TestInsertModeSpaceAndLiteralTextInput(t *testing.T) {
	b := NewScratch("x.txt")
	b.Lines = []string{"hello"}
	b.Dirty = false

	b.HandleKey("A")
	b.HandleKey("space")
	b.InsertText("world\nfrom paste")
	if got := b.Value(); got != "hello world\nfrom paste" {
		t.Fatalf("value = %q", got)
	}
	if b.Row != 1 || b.Col != len("from paste") {
		t.Fatalf("cursor = %d:%d", b.Row, b.Col)
	}

	b.HandleKey("esc")
	b.HandleKey("u")
	if got := b.Value(); got != "hello" {
		t.Fatalf("undo value = %q, want original", got)
	}
}

func TestVisualLineDeleteYanksBlock(t *testing.T) {
	b := NewScratch("x.txt")
	b.Lines = []string{"one", "two", "three"}
	b.Dirty = false
	b.HandleKey("V")
	b.HandleKey("j")
	b.HandleKey("d")
	if b.Register != "one\ntwo\n" {
		t.Fatalf("register = %q", b.Register)
	}
	if got := b.Value(); got != "three" {
		t.Fatalf("value = %q", got)
	}
}

func TestExGotoAndSubstitute(t *testing.T) {
	b := NewScratch("x.txt")
	b.Lines = []string{"alpha", "beta beta", "gamma"}
	if act := b.Execute("2"); act.Kind != ActionNone || b.CursorLine() != 2 {
		t.Fatalf("goto action=%v line=%d", act.Kind, b.CursorLine())
	}
	if act := b.Execute("s/beta/delta/"); act.Kind != ActionNone {
		t.Fatalf("sub action=%v msg=%q", act.Kind, act.Message)
	}
	if got := b.Lines[1]; got != "delta beta" {
		t.Fatalf("line = %q", got)
	}
	if act := b.Execute("%s/a/A/g"); act.Kind != ActionNone {
		t.Fatalf("global sub action=%v msg=%q", act.Kind, act.Message)
	}
	if got := b.Value(); got != "AlphA\ndeltA betA\ngAmmA" {
		t.Fatalf("value = %q", got)
	}
}

func TestSaveDetectsExternalChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := Open(path, 1024)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond)
	if err := os.WriteFile(path, []byte("external"), 0o644); err != nil {
		t.Fatal(err)
	}
	b.HandleKey("A")
	b.HandleKey("b")
	if err := b.Save(false); !errors.Is(err, ErrChanged) {
		t.Fatalf("expected changed error, got %v", err)
	}
	if err := b.Save(true); err != nil {
		t.Fatal(err)
	}
}

func TestVisibleExpandsTabsUnderCursor(t *testing.T) {
	b := NewScratch("x.go")
	b.Lines = []string{"\t\"os\""}
	b.Row = 0
	b.Col = 0
	lines := b.Visible(40, 1)
	if len(lines) != 1 {
		t.Fatalf("lines = %d", len(lines))
	}
	if got, want := lines[0], "1 \x1b[7m \x1b[27m   \"os\""; got != want {
		t.Fatalf("rendered = %q, want %q", got, want)
	}
}

func TestVisibleBoundsHugeSingleLineWrapping(t *testing.T) {
	b := NewScratch("huge.txt")
	b.Lines = []string{strings.Repeat("x", 20000)}
	lines := b.Visible(20, 5)
	if len(lines) != 5 {
		t.Fatalf("lines = %d, want 5", len(lines))
	}
	joined := strings.Join(lines, "\n")
	if len(joined) > 2000 {
		t.Fatalf("visible output too large: %d bytes", len(joined))
	}
}

func TestCtrlDAndCtrlUPageMovement(t *testing.T) {
	b := NewScratch("x.txt")
	b.Lines = []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11"}
	b.HandleKey("ctrl+d")
	if b.Row != 10 {
		t.Fatalf("row after ctrl+d = %d", b.Row)
	}
	b.HandleKey("ctrl+u")
	if b.Row != 0 {
		t.Fatalf("row after ctrl+u = %d", b.Row)
	}
}

func TestBufferAndJumpCommands(t *testing.T) {
	b := NewScratch("x.txt")
	cases := map[string]ActionKind{
		"bn":      ActionNextTab,
		"bnext":   ActionNextTab,
		"bp":      ActionPrevTab,
		"bprev":   ActionPrevTab,
		"bl":      ActionListTabs,
		"buffers": ActionListTabs,
		"ls":      ActionListTabs,
	}
	for cmd, want := range cases {
		if got := b.Execute(cmd).Kind; got != want {
			t.Fatalf(":%s = %v, want %v", cmd, got, want)
		}
	}
	if got := b.HandleKey("ctrl+o").Kind; got != ActionJumpBack {
		t.Fatalf("ctrl+o = %v", got)
	}
	if got := b.HandleKey("tab").Kind; got != ActionJumpForward {
		t.Fatalf("tab/ctrl+i = %v", got)
	}
}

func TestMarkdownTaskCheckboxToggle(t *testing.T) {
	b := NewScratch("tasks.md")
	b.Lines = []string{
		"# Plan",
		"- [ ] first",
		"2. [X] second",
	}
	b.Row = 1
	b.Dirty = false

	action := b.HandleKey(" ")
	if action.Kind != ActionStatus || action.Message != "Checked task." {
		t.Fatalf("space action = %#v", action)
	}
	if got := b.Lines[1]; got != "- [x] first" {
		t.Fatalf("checked line = %q", got)
	}
	if !b.Dirty {
		t.Fatal("toggle should mark buffer dirty")
	}

	b.HandleKey("u")
	if got := b.Lines[1]; got != "- [ ] first" {
		t.Fatalf("undo line = %q", got)
	}

	b.Row = 2
	action = b.HandleKey("space")
	if action.Kind != ActionStatus || action.Message != "Unchecked task." {
		t.Fatalf("checked task action = %#v", action)
	}
	if got := b.Lines[2]; got != "2. [ ] second" {
		t.Fatalf("unchecked line = %q", got)
	}

	b.Row = 0
	action = b.HandleKey("space")
	if action.Kind != ActionStatus || action.Message != "No Markdown checkbox on this line." {
		t.Fatalf("plain markdown line action = %#v", action)
	}

	plain := NewScratch("tasks.txt")
	plain.Lines = []string{"- [ ] no-op"}
	if action := plain.HandleKey("space"); action.Kind != ActionNone || plain.Lines[0] != "- [ ] no-op" {
		t.Fatalf("plain text toggle action=%#v line=%q", action, plain.Lines[0])
	}
}

func TestFindMotionsAndRepeats(t *testing.T) {
	b := NewScratch("x.txt")
	b.Lines = []string{"banana"}
	b.HandleKey("f")
	b.HandleKey("a")
	if b.Col != 1 {
		t.Fatalf("fa col = %d", b.Col)
	}
	b.HandleKey(";")
	if b.Col != 3 {
		t.Fatalf("; col = %d", b.Col)
	}
	b.HandleKey(",")
	if b.Col != 1 {
		t.Fatalf(", col = %d", b.Col)
	}
}

func TestOperatorTextObjects(t *testing.T) {
	b := NewScratch("x.txt")
	b.Lines = []string{"alpha beta gamma"}
	b.Col = 7
	b.HandleKey("d")
	b.HandleKey("a")
	b.HandleKey("w")
	if got := b.Value(); got != "alpha gamma" {
		t.Fatalf("daw = %q", got)
	}
	b.Lines = []string{`call("value")`}
	b.Row, b.Col = 0, 6
	b.HandleKey("c")
	b.HandleKey("i")
	b.HandleKey(`"`)
	if got := b.Value(); got != `call("")` {
		t.Fatalf("ci quote = %q", got)
	}
	if b.Mode != Insert {
		t.Fatalf("mode = %v, want Insert", b.Mode)
	}
}

func TestNormalCountsMoveAndOperate(t *testing.T) {
	b := NewScratch("x.txt")
	b.Lines = []string{
		"alpha beta gamma delta",
		"one",
		"two",
		"three",
		"four",
		"five",
		"six",
		"seven",
		"eight",
		"nine",
		"ten",
	}
	b.HandleKey("3")
	b.HandleKey("w")
	if b.Col != len("alpha beta gamma ") {
		t.Fatalf("3w col = %d", b.Col)
	}
	b.Row, b.Col = 0, 0
	b.HandleKey("1")
	b.HandleKey("0")
	b.HandleKey("j")
	if b.Row != 10 {
		t.Fatalf("10j row = %d", b.Row)
	}
	b.Row, b.Col = 0, 6
	b.HandleKey("2")
	b.HandleKey("d")
	b.HandleKey("a")
	b.HandleKey("w")
	if got := b.Lines[0]; got != "alpha delta" {
		t.Fatalf("2daw line = %q", got)
	}
}

func TestNormalCommandLineShowsPendingOperatorSequence(t *testing.T) {
	b := NewScratch("x.txt")
	b.Lines = []string{"alpha beta"}
	b.HandleKey("c")
	if got := b.NormalCommandLine(); got != "c" {
		t.Fatalf("pending c = %q", got)
	}
	b.HandleKey("a")
	if got := b.NormalCommandLine(); got != "ca" {
		t.Fatalf("pending ca = %q", got)
	}
}

func TestNormalCommandLineShowsOperatorAndMotionCounts(t *testing.T) {
	b := NewScratch("x.txt")
	b.Lines = []string{"alpha beta gamma delta"}
	b.HandleKey("2")
	b.HandleKey("d")
	if got := b.NormalCommandLine(); got != "2d" {
		t.Fatalf("pending 2d = %q", got)
	}
	b.HandleKey("3")
	if got := b.NormalCommandLine(); got != "2d3" {
		t.Fatalf("pending 2d3 = %q", got)
	}
}

func TestOperatorMotionCountsMultiply(t *testing.T) {
	b := NewScratch("x.txt")
	b.Lines = []string{"alpha beta gamma delta epsilon zeta eta"}
	b.HandleKey("2")
	b.HandleKey("d")
	b.HandleKey("3")
	b.HandleKey("w")
	if got := b.Lines[0]; got != "eta" {
		t.Fatalf("2d3w line = %q", got)
	}
}
