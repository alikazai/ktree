package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// Default to the current directory. Milestone 4+ could walk up to find
	// the repo root properly, but for now this matches running `wtree`
	// from inside whatever repo you care about — same expectation as `git`
	// itself.
	repoDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "wtree: couldn't determine current directory:", err)
		os.Exit(1)
	}

	p := tea.NewProgram(New(repoDir))
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "wtree:", err)
		os.Exit(1)
	}
}
