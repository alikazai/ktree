package cli

import "fmt"

type Mode int

const (
	ModeWorktreeUI Mode = iota
	ModeShellInit
	ModeInstallSkill
)

type InstallSkillOptions struct {
	Target string
	Export bool
	All    bool
}

type App struct {
	Mode         Mode
	Shell        string
	InstallSkill InstallSkillOptions
}

func Parse(args []string) (App, error) {
	if len(args) == 0 {
		return App{Mode: ModeWorktreeUI}, nil
	}

	if len(args) == 2 && args[0] == "--init-shell" {
		return App{Mode: ModeShellInit, Shell: args[1]}, nil
	}
	if args[0] == "--init-shell" {
		return App{}, fmt.Errorf("--init-shell requires exactly one shell argument")
	}

	if args[0] == "install" {
		if len(args) < 2 {
			return App{}, fmt.Errorf("install requires a subcommand")
		}
		if args[1] != "skill" {
			return App{}, fmt.Errorf("unknown install subcommand %q", args[1])
		}

		app := App{Mode: ModeInstallSkill}
		for _, arg := range args[2:] {
			switch arg {
			case "--export":
				app.InstallSkill.Export = true
			case "--all":
				app.InstallSkill.All = true
			default:
				if len(arg) > 1 && arg[0] == '-' {
					return App{}, fmt.Errorf("unknown install skill flag %q", arg)
				}
				if app.InstallSkill.Target != "" {
					return App{}, fmt.Errorf("install skill accepts at most one target")
				}
				app.InstallSkill.Target = arg
			}
		}
		if app.InstallSkill.All && app.InstallSkill.Target != "" {
			return App{}, fmt.Errorf("install skill does not accept both a target and --all")
		}
		return app, nil
	}

	return App{}, fmt.Errorf("unknown command %q", args[0])
}
