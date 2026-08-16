package targets

import (
	"path/filepath"

	"alikazai/ktree/internal/install"
)

func Codex() install.Target {
	return newFileTarget(
		"codex",
		"Codex",
		"codex",
		func(home string) string { return filepath.Join(home, ".codex") },
		func(home string) string { return filepath.Join(home, ".codex", "skills", "ktree", "SKILL.md") },
	)
}
