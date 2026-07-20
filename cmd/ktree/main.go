package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"alikazai/ktree/internal/ui"
)

func main() {
	if len(os.Args) == 3 && os.Args[1] == "--init-shell" {
		printShellInit(os.Args[2])
		return
	}

	repoDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "ktree: couldn't determine current directory:", err)
		os.Exit(1)
	}

	p := tea.NewProgram(ui.New(repoDir))
	result, err := p.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "ktree:", err)
		os.Exit(1)
	}
	if m, ok := result.(ui.Model); ok && m.Selected() != "" {
		fmt.Println(m.Selected())
	}
}

func printShellInit(shell string) {
	switch shell {
	case "bash", "zsh":
		fmt.Println(`ktw() { local d; d=$(ktree) && [ -n "$d" ] && cd "$d"; }`)
	case "fish":
		fmt.Println(`function ktw; set d (ktree); and test -n "$d"; and cd $d; end`)
	default:
		fmt.Fprintf(os.Stderr, "unknown shell %q — supported: bash, zsh, fish\n", shell)
		os.Exit(1)
	}
}
