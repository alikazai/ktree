package install

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type Target interface {
	Key() string
	Label() string
	Detect() (bool, error)
	ManagedPath() (string, error)
	InspectInstalled() (*InstalledArtifact, error)
	InstallBundle(bundle SkillBundle) error
	ExportBundle(bundle SkillBundle) (string, error)
}

type InstalledArtifact struct {
	Metadata *Metadata
	Content  string
}

var ErrManagedMetadataMalformed = errors.New("managed metadata malformed")

const (
	managedMetadataPrefix = "<!-- ktree:metadata "
	managedMetadataSuffix = " -->"
)

func ManagedBundleForTarget(bundle SkillBundle, target string) SkillBundle {
	bundle.Metadata = MetadataForTarget(bundle.Metadata, target)
	bundle.Content = normalizeContent(bundle.Content)
	return bundle
}

func EncodeManagedArtifact(bundle SkillBundle, target string) (string, error) {
	bundle = ManagedBundleForTarget(bundle, target)

	metadata, err := json.Marshal(bundle.Metadata)
	if err != nil {
		return "", err
	}

	return managedMetadataPrefix + string(metadata) + managedMetadataSuffix + "\n\n" + bundle.Content, nil
}

func DecodeManagedArtifact(content string) (*InstalledArtifact, error) {
	content = normalizeContent(normalizeLineEndings(content))
	if !strings.HasPrefix(content, managedMetadataPrefix) {
		return &InstalledArtifact{Content: content}, nil
	}

	rest := strings.TrimPrefix(content, managedMetadataPrefix)
	terminatorIndex := strings.Index(rest, managedMetadataSuffix)
	if terminatorIndex < 0 {
		return nil, fmt.Errorf("%w: missing metadata terminator", ErrManagedMetadataMalformed)
	}
	metadataJSON := rest[:terminatorIndex]
	body := strings.TrimLeft(rest[terminatorIndex+len(managedMetadataSuffix):], "\n")

	var metadata Metadata
	if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrManagedMetadataMalformed, err)
	}
	if err := validateManagedMetadata(metadata); err != nil {
		return nil, err
	}

	return &InstalledArtifact{Metadata: &metadata, Content: normalizeContent(body)}, nil
}

func validateManagedMetadata(metadata Metadata) error {
	missing := make([]string, 0, 4)
	if metadata.ID == "" {
		missing = append(missing, "ID")
	}
	if metadata.Version == "" {
		missing = append(missing, "Version")
	}
	if metadata.Hash == "" {
		missing = append(missing, "Hash")
	}
	if metadata.Target == "" {
		missing = append(missing, "Target")
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: missing required fields: %s", ErrManagedMetadataMalformed, strings.Join(missing, ", "))
	}
	return nil
}

func normalizeLineEndings(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	return strings.ReplaceAll(content, "\r", "\n")
}
