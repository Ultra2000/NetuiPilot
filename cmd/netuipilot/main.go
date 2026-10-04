package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/Ultra2000/netuipilot/internal/config"
	"github.com/Ultra2000/netuipilot/internal/style"
	"github.com/Ultra2000/netuipilot/internal/ui"
)

var (
	version = "dev"
	commit  = "none"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Printf("netuipilot %s (%s)\n", version, commit)
		return
	}

	cfg := config.Load()

	t := cfg.Theme
	style.ApplyTheme(t.Primary, t.Secondary, t.Accent, t.Success, t.Warning, t.Danger)

	m := ui.NewModel(version, cfg)
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
