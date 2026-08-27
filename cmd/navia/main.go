package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/heidaraliy/navia/internal/app"
	"github.com/heidaraliy/navia/internal/config"
)

var version = "dev"

type programRunner interface{ Run() (tea.Model, error) }

var exitFn = os.Exit
var newProgram = func(model app.Model) programRunner {
	return tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
}

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
	flags.Usage = func() {
		fmt.Fprintln(flags.Output(), "Usage: navia [-d] [path]\n\nBrowse files and review Git changes without mutating either.")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *showVersion {
		fmt.Fprintln(stdout, "navia "+displayVersion())
		return 0
	}
	if flags.NArg() > 1 {
		flags.Usage()
		return 2
	}
	start := "."
	if flags.NArg() == 1 {
		start = flags.Arg(0)
	}
	cfg, warning := config.Load()
	model, err := app.New(start, cfg, *diffMode)
	if err != nil {
		fmt.Fprintln(stderr, "navia:", err)
		return 1
	}
	if warning != "" {
		model.SetStatus(warning)
	}
	program := newProgram(model)
	if _, err := program.Run(); err != nil {
		fmt.Fprintln(stderr, "navia:", err)
		return 1
	}
	return 0
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
