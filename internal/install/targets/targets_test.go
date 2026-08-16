package targets_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"alikazai/ktree/internal/install"
	"alikazai/ktree/internal/install/targets"
)

func TestTargetsExposeManagedFixtures(t *testing.T) {
	bundle := install.CoreSkillBundle()

	tests := []struct {
		name        string
		target      install.Target
		fixtureHome string
	}{
		{
			name:        "claude",
			target:      targets.Claude(),
			fixtureHome: filepath.Join("testdata", "claude", "home"),
		},
		{
			name:        "opencode",
			target:      targets.OpenCode(),
			fixtureHome: filepath.Join("testdata", "opencode", "home"),
		},
		{
			name:        "codex",
			target:      targets.Codex(),
			fixtureHome: filepath.Join("testdata", "codex", "home"),
		},
		{
			name:        "grok",
			target:      targets.Grok(),
			fixtureHome: filepath.Join("testdata", "grok", "home"),
		},
		{
			name:        "cursor",
			target:      targets.Cursor(),
			fixtureHome: filepath.Join("testdata", "cursor", "home"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
			t.Setenv("PATH", filepath.Join(home, "bin"))

			detected, err := tt.target.Detect()
			if err != nil {
				t.Fatalf("Detect() error = %v", err)
			}
			if detected {
				t.Fatal("Detect() = true, want false without fixture")
			}

			managedArtifact, managedPathRel := readFixtureArtifact(t, tt.fixtureHome)
			copyFixtureHome(t, tt.fixtureHome, home)

			detected, err = tt.target.Detect()
			if err != nil {
				t.Fatalf("Detect() error = %v", err)
			}
			if !detected {
				t.Fatal("Detect() = false, want true with fixture")
			}

			managedPath, err := tt.target.ManagedPath()
			if err != nil {
				t.Fatalf("ManagedPath() error = %v", err)
			}
			if managedPath != filepath.Join(home, managedPathRel) {
				t.Fatalf("ManagedPath() = %q, want %q", managedPath, filepath.Join(home, managedPathRel))
			}

			installed, err := tt.target.InspectInstalled()
			if err != nil {
				t.Fatalf("InspectInstalled() error = %v", err)
			}
			if installed == nil {
				t.Fatal("InspectInstalled() = nil, want artifact")
			}
			if installed.Content != bundle.Content {
				t.Fatalf("InspectInstalled().Content = %q, want %q", installed.Content, bundle.Content)
			}
			if installed.Metadata == nil {
				t.Fatal("InspectInstalled().Metadata = nil, want metadata")
			}
			if installed.Metadata.ID != bundle.Metadata.ID {
				t.Fatalf("InspectInstalled().Metadata.ID = %q, want %q", installed.Metadata.ID, bundle.Metadata.ID)
			}
			if installed.Metadata.Version != bundle.Metadata.Version {
				t.Fatalf("InspectInstalled().Metadata.Version = %q, want %q", installed.Metadata.Version, bundle.Metadata.Version)
			}
			if installed.Metadata.Hash != bundle.Metadata.Hash {
				t.Fatalf("InspectInstalled().Metadata.Hash = %q, want %q", installed.Metadata.Hash, bundle.Metadata.Hash)
			}
			if installed.Metadata.Target != tt.target.Key() {
				t.Fatalf("InspectInstalled().Metadata.Target = %q, want %q", installed.Metadata.Target, tt.target.Key())
			}

			exported, err := tt.target.ExportBundle(bundle)
			if err != nil {
				t.Fatalf("ExportBundle() error = %v", err)
			}
			if exported != managedArtifact {
				t.Fatalf("ExportBundle() = %q, want fixture %q", exported, managedArtifact)
			}

			installHome := t.TempDir()
			t.Setenv("HOME", installHome)
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(installHome, ".config"))
			t.Setenv("PATH", filepath.Join(installHome, "bin"))

			if err := tt.target.InstallBundle(bundle); err != nil {
				t.Fatalf("InstallBundle() error = %v", err)
			}

			managedPath, err = tt.target.ManagedPath()
			if err != nil {
				t.Fatalf("ManagedPath() error = %v", err)
			}
			artifact, err := os.ReadFile(managedPath)
			if err != nil {
				t.Fatalf("ReadFile() error = %v", err)
			}
			if string(artifact) != managedArtifact {
				t.Fatalf("installed artifact = %q, want fixture %q", string(artifact), managedArtifact)
			}
		})
	}
}

func TestInspectInstalledReturnsUnmanagedArtifactForPlainMarkdown(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	copyFixtureHome(t, filepath.Join("testdata", "claude", "plain", "home"), home)

	target := targets.Claude()

	installed, err := target.InspectInstalled()
	if err != nil {
		t.Fatalf("InspectInstalled() error = %v", err)
	}
	if installed == nil {
		t.Fatal("InspectInstalled() = nil, want unmanaged artifact")
	}
	if installed.Metadata != nil {
		t.Fatalf("InspectInstalled().Metadata = %+v, want nil", installed.Metadata)
	}
	if installed.Content != "# plain skill\n" {
		t.Fatalf("InspectInstalled().Content = %q, want %q", installed.Content, "# plain skill\n")
	}
}

func TestInspectInstalledReturnsMalformedMetadataError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	copyFixtureHome(t, filepath.Join("testdata", "claude", "malformed-json", "home"), home)

	installed, err := targets.Claude().InspectInstalled()
	if !errors.Is(err, install.ErrManagedMetadataMalformed) {
		t.Fatalf("InspectInstalled() error = %v, want %v", err, install.ErrManagedMetadataMalformed)
	}
	if installed != nil {
		t.Fatalf("InspectInstalled() = %+v, want nil on malformed metadata", installed)
	}
}

func TestInspectInstalledReturnsMissingFieldMetadataError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	copyFixtureHome(t, filepath.Join("testdata", "claude", "missing-fields", "home"), home)

	installed, err := targets.Claude().InspectInstalled()
	if !errors.Is(err, install.ErrManagedMetadataMalformed) {
		t.Fatalf("InspectInstalled() error = %v, want %v", err, install.ErrManagedMetadataMalformed)
	}
	if installed != nil {
		t.Fatalf("InspectInstalled() = %+v, want nil on missing field metadata", installed)
	}
}

func TestInspectInstalledReturnsMalformedTerminatorError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	copyFixtureHome(t, filepath.Join("testdata", "claude", "truncated", "home"), home)

	installed, err := targets.Claude().InspectInstalled()
	if !errors.Is(err, install.ErrManagedMetadataMalformed) {
		t.Fatalf("InspectInstalled() error = %v, want %v", err, install.ErrManagedMetadataMalformed)
	}
	if installed != nil {
		t.Fatalf("InspectInstalled() = %+v, want nil on malformed terminator", installed)
	}
}

func TestDetectFallsBackToPATHLookup(t *testing.T) {
	home := t.TempDir()
	binDir := filepath.Join(home, "bin")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("PATH", binDir)
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "claude"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	detected, err := targets.Claude().Detect()
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if !detected {
		t.Fatal("Detect() = false, want true from PATH fallback")
	}
}

func TestOpenCodeManagedPathFallsBackToHomeConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")

	managedPath, err := targets.OpenCode().ManagedPath()
	if err != nil {
		t.Fatalf("ManagedPath() error = %v", err)
	}
	want := filepath.Join(home, ".config", "opencode", "skills", "ktree", "SKILL.md")
	if managedPath != want {
		t.Fatalf("ManagedPath() = %q, want %q", managedPath, want)
	}
}

func TestOpenCodeManagedPathIgnoresRelativeXDGConfigHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "relative-config")

	managedPath, err := targets.OpenCode().ManagedPath()
	if err != nil {
		t.Fatalf("ManagedPath() error = %v", err)
	}
	want := filepath.Join(home, ".config", "opencode", "skills", "ktree", "SKILL.md")
	if managedPath != want {
		t.Fatalf("ManagedPath() = %q, want %q", managedPath, want)
	}
}

func TestOpenCodeUsesCustomXDGConfigHomeRoundTrip(t *testing.T) {
	home := t.TempDir()
	xdgHome := filepath.Join(home, "xdg-config")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdgHome)

	target := targets.OpenCode()
	bundle := install.CoreSkillBundle()

	managedPath, err := target.ManagedPath()
	if err != nil {
		t.Fatalf("ManagedPath() error = %v", err)
	}
	want := filepath.Join(xdgHome, "opencode", "skills", "ktree", "SKILL.md")
	if managedPath != want {
		t.Fatalf("ManagedPath() = %q, want %q", managedPath, want)
	}

	if err := target.InstallBundle(bundle); err != nil {
		t.Fatalf("InstallBundle() error = %v", err)
	}

	installed, err := target.InspectInstalled()
	if err != nil {
		t.Fatalf("InspectInstalled() error = %v", err)
	}
	if installed == nil || installed.Metadata == nil {
		t.Fatal("InspectInstalled() = nil metadata, want managed artifact")
	}
	if installed.Metadata.Target != "opencode" {
		t.Fatalf("InspectInstalled().Metadata.Target = %q, want %q", installed.Metadata.Target, "opencode")
	}
	if installed.Content != bundle.Content {
		t.Fatalf("InspectInstalled().Content = %q, want %q", installed.Content, bundle.Content)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("Stat(%q) error = %v", want, err)
	}
	if detected, err := target.Detect(); err != nil {
		t.Fatalf("Detect() error = %v", err)
	} else if !detected {
		t.Fatal("Detect() = false, want true with custom XDG config root")
	}
}

func TestRegistryProvidesStableTargetOrderAndLookup(t *testing.T) {
	all := targets.All()
	if len(all) != 5 {
		t.Fatalf("len(All()) = %d, want %d", len(all), 5)
	}

	wantKeys := []string{"claude", "opencode", "codex", "grok", "cursor"}
	wantLabels := []string{"Claude", "OpenCode", "Codex", "Grok", "Cursor"}
	for i, target := range all {
		if target.Key() != wantKeys[i] {
			t.Fatalf("All()[%d].Key() = %q, want %q", i, target.Key(), wantKeys[i])
		}
		if target.Label() != wantLabels[i] {
			t.Fatalf("All()[%d].Label() = %q, want %q", i, target.Label(), wantLabels[i])
		}
	}

	if _, ok := targets.Lookup("missing"); ok {
		t.Fatal("Lookup(\"missing\") succeeded, want false")
	}
	for _, key := range wantKeys {
		target, ok := targets.Lookup(key)
		if !ok {
			t.Fatalf("Lookup(%q) = false, want true", key)
		}
		if target.Key() != key {
			t.Fatalf("Lookup(%q).Key() = %q, want %q", key, target.Key(), key)
		}
	}
}

func readFixtureArtifact(t *testing.T, fixtureHome string) (string, string) {
	t.Helper()

	managedPathRel := filepath.Join(detectManagedArtifactRel(t, fixtureHome))
	content, err := os.ReadFile(filepath.Join(fixtureHome, managedPathRel))
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", filepath.Join(fixtureHome, managedPathRel), err)
	}

	return string(content), managedPathRel
}

func detectManagedArtifactRel(t *testing.T, fixtureHome string) string {
	t.Helper()

	var managedPathRel string
	err := filepath.WalkDir(fixtureHome, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Base(path) != "SKILL.md" {
			return nil
		}
		rel, relErr := filepath.Rel(fixtureHome, path)
		if relErr != nil {
			return relErr
		}
		managedPathRel = rel
		return fs.SkipAll
	})
	if err != nil {
		t.Fatalf("WalkDir(%q) error = %v", fixtureHome, err)
	}
	if managedPathRel == "" {
		t.Fatalf("fixture %q did not contain SKILL.md", fixtureHome)
	}

	return managedPathRel
}

func copyFixtureHome(t *testing.T, fixtureHome, destHome string) {
	t.Helper()

	err := filepath.WalkDir(fixtureHome, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(fixtureHome, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		destPath := filepath.Join(destHome, rel)
		if d.IsDir() {
			return os.MkdirAll(destPath, 0o755)
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return err
		}
		return os.WriteFile(destPath, content, 0o644)
	})
	if err != nil {
		t.Fatalf("copyFixtureHome(%q) error = %v", fixtureHome, err)
	}
}
