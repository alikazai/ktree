package install

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"strings"
)

const coreSkillID = "skill-core"

//go:embed testdata/skill-core.md
var coreSkillContent string

var coreSkillBundle = func() SkillBundle {
	content := normalizeContent(coreSkillContent)
	return SkillBundle{
		Metadata: Metadata{
			ID:      coreSkillID,
			Version: "1.0.0",
			Hash:    hashContent(content),
			Target:  TargetShared,
		},
		Content: content,
	}
}()

func CoreSkillBundle() SkillBundle {
	return coreSkillBundle
}

func normalizeContent(content string) string {
	return strings.TrimSpace(content) + "\n"
}

func hashContent(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}
