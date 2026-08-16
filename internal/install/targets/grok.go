package targets

import (
	"path/filepath"

	"alikazai/ktree/internal/install"
)

func Grok() install.Target {
	return newFileTarget(
		"grok",
		"Grok",
		"grok",
		func(home string) string { return filepath.Join(home, ".grok") },
		func(home string) string { return filepath.Join(home, ".grok", "skills", "ktree", "SKILL.md") },
	)
}
