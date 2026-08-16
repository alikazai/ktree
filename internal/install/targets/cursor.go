package targets

import (
	"path/filepath"

	"alikazai/ktree/internal/install"
)

func Cursor() install.Target {
	return newFileTarget(
		"cursor",
		"Cursor",
		"cursor",
		func(home string) string { return filepath.Join(home, ".cursor") },
		func(home string) string { return filepath.Join(home, ".cursor", "skills", "ktree", "SKILL.md") },
	)
}
