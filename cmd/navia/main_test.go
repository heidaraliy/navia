package main

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/heidaraliy/navia/internal/app"
)

type fakeProgram struct{ err error }

func (p fakeProgram) Run() (tea.Model, error) { return nil, p.err }

func TestRunVersion(t *testing.T) {
	old := version
	version = "v2.0.0"
	t.Cleanup(func() { version = old })
	var out, errOut bytes.Buffer
	if code := run([]string{"--version"}, &out, &errOut); code != 0 || strings.TrimSpace(out.String()) != "navia 2.0.0" {
		t.Fatalf("code/output = %d %q", code, out.String())
	}
}
func TestRunRejectsBadFlags(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"--patch", "-"}, &out, &errOut); code != 2 {
		t.Fatalf("code=%d", code)
	}
}
func TestRunStartsNavigator(t *testing.T) {
	dir := t.TempDir()
	old := newProgram
	t.Cleanup(func() { newProgram = old })
	called := false
	newProgram = func(app.Model) programRunner { called = true; return fakeProgram{} }
	var out, errOut bytes.Buffer
	if code := run([]string{dir}, &out, &errOut); code != 0 || !called {
		t.Fatalf("code/called=%d/%v stderr=%q", code, called, errOut.String())
	}
}
func TestRunReportsErrors(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{filepath.Join(t.TempDir(), "missing")}, &out, &errOut); code != 1 {
		t.Fatalf("code=%d", code)
	}
	dir := t.TempDir()
	old := newProgram
	t.Cleanup(func() { newProgram = old })
	newProgram = func(app.Model) programRunner { return fakeProgram{errors.New("program failed")} }
	out.Reset()
	errOut.Reset()
	if code := run([]string{dir}, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "program failed") {
		t.Fatalf("code/stderr=%d/%q", code, errOut.String())
	}
}
