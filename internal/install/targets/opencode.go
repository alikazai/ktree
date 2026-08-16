package targets

import (
	"os"
	"path/filepath"

	"alikazai/ktree/internal/install"
)

func OpenCode() install.Target {
	return fileTarget{
		key:        "opencode",
		label:      "OpenCode",
		binaryName: "opencode",
		rootPath: func() (string, error) {
			return xdgConfigPath("opencode")
		},
		filePath: func() (string, error) {
			root, err := xdgConfigPath("opencode")
			if err != nil {
				return "", err
			}
			return filepath.Join(root, "skills", "ktree", "SKILL.md"), nil
		},
	}
}

func xdgConfigPath(name string) (string, error) {
	if root := os.Getenv("XDG_CONFIG_HOME"); filepath.IsAbs(root) {
		return filepath.Join(root, name), nil
	}

	home, err := homeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".config", name), nil
}
