package cli

import "testing"

func TestParseInstallSkill(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want App
	}{
		{
			name: "default target",
			args: []string{"install", "skill"},
			want: App{Mode: ModeInstallSkill},
		},
		{
			name: "explicit target with export",
			args: []string{"install", "skill", "demo", "--export"},
			want: App{
				Mode: ModeInstallSkill,
				InstallSkill: InstallSkillOptions{
					Target: "demo",
					Export: true,
				},
			},
		},
		{
			name: "all targets",
			args: []string{"install", "skill", "--all"},
			want: App{
				Mode: ModeInstallSkill,
				InstallSkill: InstallSkillOptions{All: true},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse(tc.args)
			if err != nil {
				t.Fatalf("Parse(%q) error = %v", tc.args, err)
			}
			if got != tc.want {
				t.Fatalf("Parse(%q) = %#v, want %#v", tc.args, got, tc.want)
			}
		})
	}
}

func TestParseModes(t *testing.T) {
	t.Run("worktree ui", func(t *testing.T) {
		got, err := Parse(nil)
		if err != nil {
			t.Fatalf("Parse(nil) error = %v", err)
		}
		want := App{Mode: ModeWorktreeUI}
		if got != want {
			t.Fatalf("Parse(nil) = %#v, want %#v", got, want)
		}
	})

	t.Run("shell init", func(t *testing.T) {
		got, err := Parse([]string{"--init-shell", "fish"})
		if err != nil {
			t.Fatalf("Parse(init-shell) error = %v", err)
		}
		want := App{Mode: ModeShellInit, Shell: "fish"}
		if got != want {
			t.Fatalf("Parse(init-shell) = %#v, want %#v", got, want)
		}
	})
}

func TestParseErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "install missing subcommand",
			args: []string{"install"},
			want: "install requires a subcommand",
		},
		{
			name: "install unknown subcommand",
			args: []string{"install", "foo"},
			want: `unknown install subcommand "foo"`,
		},
		{
			name: "unknown top level arg",
			args: []string{"wat"},
			want: `unknown command "wat"`,
		},
		{
			name: "shell init missing shell",
			args: []string{"--init-shell"},
			want: "--init-shell requires exactly one shell argument",
		},
		{
			name: "shell init extra arg",
			args: []string{"--init-shell", "fish", "extra"},
			want: "--init-shell requires exactly one shell argument",
		},
		{
			name: "install skill target all conflict",
			args: []string{"install", "skill", "demo", "--all"},
			want: "install skill does not accept both a target and --all",
		},
		{
			name: "install skill unknown flag",
			args: []string{"install", "skill", "--wat"},
			want: `unknown install skill flag "--wat"`,
		},
		{
			name: "install skill multiple targets",
			args: []string{"install", "skill", "demo", "other"},
			want: "install skill accepts at most one target",
		},
	}

	for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				got, err := Parse(tc.args)
				if err == nil || err.Error() != tc.want {
					t.Fatalf("Parse(%q) error = %v, want %q; got %#v", tc.args, err, tc.want, got)
				}
			})
		}
}
