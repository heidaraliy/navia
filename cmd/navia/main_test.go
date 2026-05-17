package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/heidaraliy/navia/internal/app"
)

type fakeProgram struct {
	err error
}

func (p fakeProgram) Run() (tea.Model, error) {
	return nil, p.err
}

func TestDisplayVersionUsesInjectedVersion(t *testing.T) {
	old := version
	t.Cleanup(func() { version = old })

	version = "1.2.3"
	if got := displayVersion(); got != "1.2.3" {
		t.Fatalf("displayVersion() = %q, want %q", got, "1.2.3")
	}
}

func TestDisplayVersionTrimsLeadingV(t *testing.T) {
	old := version
	t.Cleanup(func() { version = old })

	version = "v1.2.3"
	if got := displayVersion(); got != "1.2.3" {
		t.Fatalf("displayVersion() = %q, want %q", got, "1.2.3")
	}
}

func TestRunVersion(t *testing.T) {
	old := version
	version = "v1.2.3"
	t.Cleanup(func() { version = old })

	var stdout, stderr bytes.Buffer
	code := run([]string{"--version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if got := strings.TrimSpace(stdout.String()); got != "navia 1.2.3" {
		t.Fatalf("stdout = %q", got)
	}
}

func TestRunRejectsConflictingSearchModes(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-s", "needle", "-f", "file"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "use only one") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunRejectsConflictingStartupModes(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--d", "-s", "needle"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "one startup mode") {
		t.Fatalf("stderr = %q", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"--patch", "-", "-f", "file"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("patch conflict code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "one startup mode") {
		t.Fatalf("patch conflict stderr = %q", stderr.String())
	}
}

func TestRunRejectsBadFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--missing"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "flag provided but not defined") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunStartsProgramWithSearchModesAndConfigWarning(t *testing.T) {
	dir := t.TempDir()
	configHome := filepath.Join(dir, "config")
	t.Setenv("XDG_CONFIG_HOME", configHome)
	mustWrite(t, filepath.Join(configHome, "navia", "config.toml"), "invalid line\n")

	oldNewProgram := newProgram
	defer func() { newProgram = oldNewProgram }()

	for _, args := range [][]string{{dir}, {"-s", "needle", dir}, {"-f", "nav", dir}, {"--d", dir}} {
		called := false
		newProgram = func(model app.Model) programRunner {
			called = true
			return fakeProgram{}
		}
		var stdout, stderr bytes.Buffer
		code := run(args, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("run(%v) code = %d, stderr=%q", args, code, stderr.String())
		}
		if !called {
			t.Fatalf("run(%v) did not start program", args)
		}
	}
}

func TestRunStartsProgramWithPatchReview(t *testing.T) {
	dir := t.TempDir()
	oldNewProgram := newProgram
	oldStdin := stdin
	defer func() {
		newProgram = oldNewProgram
		stdin = oldStdin
	}()

	patch := strings.Join([]string{
		"diff --git a/a.txt b/a.txt",
		"index 1111111..2222222 100644",
		"--- a/a.txt",
		"+++ b/a.txt",
		"@@ -1 +1 @@",
		"-old",
		"+new",
		"",
	}, "\n")
	stdin = strings.NewReader(patch)
	called := false
	newProgram = func(model app.Model) programRunner {
		called = true
		return fakeProgram{}
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--patch", "-", "--patch-label", "PR #7", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d, stderr=%q", code, stderr.String())
	}
	if !called {
		t.Fatal("patch review did not start program")
	}
}

func TestRunReportsModelAndProgramErrors(t *testing.T) {
	oldNewProgram := newProgram
	defer func() { newProgram = oldNewProgram }()

	var stdout, stderr bytes.Buffer
	code := run([]string{filepath.Join(t.TempDir(), "missing")}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("missing root code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "navia:") {
		t.Fatalf("stderr = %q", stderr.String())
	}

	dir := t.TempDir()
	newProgram = func(model app.Model) programRunner {
		return fakeProgram{err: errors.New("program failed")}
	}
	stdout.Reset()
	stderr.Reset()
	code = run([]string{dir}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("program error code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "program failed") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestMainExitsWithRunCode(t *testing.T) {
	oldArgs := os.Args
	oldExit := exitFn
	defer func() {
		os.Args = oldArgs
		exitFn = oldExit
	}()

	os.Args = []string{"navia", "-s", "needle", "-f", "file"}
	type exitCode int
	exitFn = func(code int) {
		panic(exitCode(code))
	}
	defer func() {
		recovered := recover()
		got, ok := recovered.(exitCode)
		if !ok {
			t.Fatalf("main did not exit with exitCode panic: %#v", recovered)
		}
		if got != 2 {
			t.Fatalf("exit = %d, want 2", got)
		}
	}()
	main()
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
