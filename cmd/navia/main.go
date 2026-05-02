package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/heidaraliy/navia/internal/app"
	"github.com/heidaraliy/navia/internal/config"
)

const version = "0.1.0"

func main() {
	showVersion := flag.Bool("version", false, "show version")
	textSearch := flag.String("s", "", "start in recursive text search with query")
	fileSearch := flag.String("f", "", "start in recursive file-name search with query")
	flag.Parse()

	if *showVersion {
		fmt.Println("navia " + version)
		return
	}
	if *textSearch != "" && *fileSearch != "" {
		fmt.Fprintln(os.Stderr, "navia: use only one of --s or --f")
		os.Exit(2)
	}

	root := "."
	if flag.NArg() > 0 {
		root = flag.Arg(0)
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "navia: %v\n", err)
		os.Exit(1)
	}

	cfg, warning := config.Load()
	var model app.Model
	switch {
	case *textSearch != "":
		model, err = app.NewWithSearch(abs, cfg, app.StartupSearch{Mode: app.SearchText, Query: *textSearch})
	case *fileSearch != "":
		model, err = app.NewWithSearch(abs, cfg, app.StartupSearch{Mode: app.SearchFiles, Query: *fileSearch})
	default:
		model, err = app.New(abs, cfg)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "navia: %v\n", err)
		os.Exit(1)
	}
	if warning != "" {
		model.SetStatus(warning)
	}

	program := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "navia: %v\n", err)
		os.Exit(1)
	}
}
