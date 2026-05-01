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
	flag.Parse()

	if *showVersion {
		fmt.Println("navia " + version)
		return
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
	model, err := app.New(abs, cfg)
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
