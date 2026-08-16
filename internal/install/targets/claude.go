package targets

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"

	"alikazai/ktree/internal/install"
)

type pathResolver func() (string, error)

type fileTarget struct {
	key        string
	label      string
	binaryName string
	rootPath   pathResolver
	filePath   pathResolver
}

func Claude() install.Target {
	return newFileTarget(
		"claude",
		"Claude",
		"claude",
		func(home string) string { return filepath.Join(home, ".claude") },
		func(home string) string { return filepath.Join(home, ".claude", "skills", "ktree", "SKILL.md") },
	)
}

func newFileTarget(key, label, binaryName string, rootBuilder, fileBuilder func(home string) string) install.Target {
	return fileTarget{
		key:        key,
		label:      label,
		binaryName: binaryName,
		rootPath: func() (string, error) {
			home, err := homeDir()
			if err != nil {
				return "", err
			}
			return rootBuilder(home), nil
		},
		filePath: func() (string, error) {
			home, err := homeDir()
			if err != nil {
				return "", err
			}
			return fileBuilder(home), nil
		},
	}
}

func (t fileTarget) Key() string {
	return t.key
}

func (t fileTarget) Label() string {
	return t.label
}

func (t fileTarget) Detect() (bool, error) {
	root, err := t.rootPath()
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(root); err == nil {
		return true, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}

	_, err = exec.LookPath(t.binaryName)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, exec.ErrNotFound) {
		return false, nil
	}

	return false, err
}

func (t fileTarget) ManagedPath() (string, error) {
	return t.filePath()
}

func (t fileTarget) InspectInstalled() (*install.InstalledArtifact, error) {
	managedPath, err := t.ManagedPath()
	if err != nil {
		return nil, err
	}

	artifact, err := os.ReadFile(managedPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	decoded, err := install.DecodeManagedArtifact(string(artifact))
	if err != nil {
		return nil, err
	}
	return decoded, nil
}

func (t fileTarget) InstallBundle(bundle install.SkillBundle) error {
	managedPath, err := t.ManagedPath()
	if err != nil {
		return err
	}

	artifact, err := t.ExportBundle(bundle)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(managedPath), 0o755); err != nil {
		return err
	}

	return os.WriteFile(managedPath, []byte(artifact), 0o644)
}

func (t fileTarget) ExportBundle(bundle install.SkillBundle) (string, error) {
	return install.EncodeManagedArtifact(bundle, t.key)
}

func homeDir() (string, error) {
	return os.UserHomeDir()
}
