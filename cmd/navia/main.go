package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/heidaraliy/navia/internal/app"
	"github.com/heidaraliy/navia/internal/config"
)

var version = "dev"

type programRunner interface {
	Run() (tea.Model, error)
}

var (
	exitFn               = os.Exit
	stdin      io.Reader = os.Stdin
	newProgram           = func(model app.Model) programRunner {
		return tea.NewProgram(model, tea.WithAltScreen())
	}
)

func main() {
	if code := run(os.Args[1:], os.Stdout, os.Stderr); code != 0 {
		exitFn(code)
	}
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("navia", flag.ContinueOnError)
	flags.SetOutput(stderr)
	showVersion := flags.Bool("version", false, "show version")
	diffMode := flags.Bool("d", false, "start in diff mode")
	textSearch := flags.String("s", "", "start in recursive text search with query")
	fileSearch := flags.String("f", "", "start in recursive file-name search with query")
	patchReview := flags.String("patch", "", "start in patch review mode from file (- for stdin)")
	patchReviewShort := flags.String("P", "", "start in patch review mode from file (- for stdin)")
	patchLabel := flags.String("patch-label", "", "label shown for patch review mode")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	if *showVersion {
		fmt.Fprintln(stdout, "navia "+displayVersion())
		return 0
	}
	if *textSearch != "" && *fileSearch != "" {
		fmt.Fprintln(stderr, "navia: use only one of --s or --f")
		return 2
	}
	patchSource := *patchReview
	if *patchReviewShort != "" {
		if patchSource != "" {
			fmt.Fprintln(stderr, "navia: use only one of --patch or -P")
			return 2
		}
		patchSource = *patchReviewShort
	}
	startupModes := 0
	if *diffMode {
		startupModes++
	}
	if *textSearch != "" || *fileSearch != "" {
		startupModes++
	}
	if patchSource != "" {
		startupModes++
	}
	if startupModes > 1 {
		fmt.Fprintln(stderr, "navia: use only one startup mode")
		return 2
	}

	root := "."
	if flags.NArg() > 0 {
		root = flags.Arg(0)
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		fmt.Fprintf(stderr, "navia: %v\n", err)
		return 1
	}

	cfg, warning := config.Load()
	var model app.Model
	switch {
	case *diffMode:
		model, err = app.NewWithDiff(abs, cfg)
	case patchSource != "":
		patch, label, readErr := readPatchReview(patchSource)
		if readErr != nil {
			fmt.Fprintf(stderr, "navia: %v\n", readErr)
			return 1
		}
		if strings.TrimSpace(*patchLabel) != "" {
			label = *patchLabel
		}
		model, err = app.NewWithPatchReview(abs, cfg, app.StartupPatchReview{Label: label, Data: patch})
	case *textSearch != "":
		model, err = app.NewWithSearch(abs, cfg, app.StartupSearch{Mode: app.SearchText, Query: *textSearch})
	case *fileSearch != "":
		model, err = app.NewWithSearch(abs, cfg, app.StartupSearch{Mode: app.SearchFiles, Query: *fileSearch})
	default:
		model, err = app.NewFromPath(abs, cfg)
	}
	if err != nil {
		fmt.Fprintf(stderr, "navia: %v\n", err)
		return 1
	}
	if warning != "" {
		model.SetStatus(warning)
	}

	program := newProgram(model)
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(stderr, "navia: %v\n", err)
		return 1
	}
	return 0
}

func readPatchReview(source string) ([]byte, string, error) {
	if source == "-" {
		data, err := io.ReadAll(stdin)
		return data, "stdin patch", err
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return nil, "", err
	}
	return data, filepath.Base(source), nil
}

func displayVersion() string {
	display := version
	if display == "" || display == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			display = info.Main.Version
		}
	}
	return strings.TrimPrefix(display, "v")
}
