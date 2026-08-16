package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"alikazai/ktree/internal/cli"
	"alikazai/ktree/internal/install"
	"alikazai/ktree/internal/install/targets"
	"alikazai/ktree/internal/installui"
	"alikazai/ktree/internal/ui"
)

type program interface {
	Run() (tea.Model, error)
}

type installManager interface {
	Options() ([]installui.TargetRow, error)
	Execute(target string, export bool) (string, error)
	ExecuteAll() (string, error)
}

func main() {
	err := run(
		os.Args[1:],
		os.Getwd,
		func(repoDir string) program {
			return tea.NewProgram(ui.New(repoDir))
		},
		func(rows []installui.TargetRow, export bool) program {
			return tea.NewProgram(installui.New(rows, export))
		},
		install.NewManager(targets.All()),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ktree:", err)
		os.Exit(1)
	}
}

func run(args []string, getwd func() (string, error), newProgram func(string) program, newInstallProgram func([]installui.TargetRow, bool) program, installer installManager) error {
	app, err := cli.Parse(args)
	if err != nil {
		return err
	}

	switch app.Mode {
	case cli.ModeShellInit:
		return printShellInit(app.Shell)
	case cli.ModeInstallSkill:
		return runInstallSkill(app.InstallSkill, newInstallProgram, installer)
	}

	repoDir, err := getwd()
	if err != nil {
		return fmt.Errorf("couldn't determine current directory: %w", err)
	}

	p := newProgram(repoDir)
	result, err := p.Run()
	if err != nil {
		return err
	}
	if m, ok := result.(ui.Model); ok && m.Selected() != "" {
		fmt.Println(m.Selected())
	}

	return nil
}

func runInstallSkill(options cli.InstallSkillOptions, newInstallProgram func([]installui.TargetRow, bool) program, installer installManager) error {
	if options.All {
		if options.Export {
			return fmt.Errorf("install skill --all does not support --export")
		}
		output, err := installer.ExecuteAll()
		if output != "" {
			fmt.Println(output)
		}
		if err != nil {
			return err
		}
		return nil
	}

	target := options.Target
	if target == "" {
		rows, err := installer.Options()
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return fmt.Errorf("no supported install targets detected")
		}

		result, err := newInstallProgram(rows, options.Export).Run()
		if err != nil {
			return err
		}
		model, ok := result.(installui.Model)
		if !ok || model.Selected() == "" {
			return nil
		}
		target = model.Selected()
	}

	output, err := installer.Execute(target, options.Export)
	if err != nil {
		return err
	}
	if output != "" {
		fmt.Println(output)
	}
	return nil
}

func printShellInit(shell string) error {
	switch shell {
	case "bash", "zsh":
		fmt.Println(`ktw() { local d; d=$(ktree) && [ -n "$d" ] && cd "$d"; }`)
		return nil
	case "fish":
		fmt.Println(`function ktw; set d (ktree); and test -n "$d"; and cd $d; end`)
		return nil
	default:
		return fmt.Errorf("unknown shell %q — supported: bash, zsh, fish", shell)
	}
}
