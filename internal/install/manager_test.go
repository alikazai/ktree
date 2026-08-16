package install

import (
	"fmt"
	"strings"
	"testing"

	"alikazai/ktree/internal/installui"
)

func TestNormalizeContent(t *testing.T) {
	input := "\n\n# ktree core skill\n\nThis bundled skill payload is the canonical installer baseline.\n\n\n"
	got := normalizeContent(input)
	want := "# ktree core skill\n\nThis bundled skill payload is the canonical installer baseline.\n"
	if got != want {
		t.Fatalf("normalizeContent() = %q, want %q", got, want)
	}
}

func TestCoreSkillBundle(t *testing.T) {
	bundle := CoreSkillBundle()

	if bundle.Content == "" {
		t.Fatal("CoreSkillBundle().Content is empty")
	}
	if bundle.Metadata.ID != "skill-core" {
		t.Fatalf("CoreSkillBundle().Metadata.ID = %q, want %q", bundle.Metadata.ID, "skill-core")
	}
	if bundle.Metadata.Hash == "" {
		t.Fatal("CoreSkillBundle().Metadata.Hash is empty")
	}
	if !strings.HasSuffix(bundle.Content, "\n") {
		t.Fatalf("CoreSkillBundle().Content = %q, want trailing newline", bundle.Content)
	}
	if bundle.Metadata.Hash != hashContent(bundle.Content) {
		t.Fatalf("CoreSkillBundle().Metadata.Hash = %q, want computed hash %q", bundle.Metadata.Hash, hashContent(bundle.Content))
	}
	if bundle.Content != normalizeContent(coreSkillContent) {
		t.Fatalf("CoreSkillBundle().Content = %q, want normalized embedded content %q", bundle.Content, normalizeContent(coreSkillContent))
	}
	if bundle.Metadata.Target != TargetShared {
		t.Fatalf("CoreSkillBundle().Metadata.Target = %q, want %q", bundle.Metadata.Target, TargetShared)
	}
}

func TestMetadataForTarget(t *testing.T) {
	bundle := CoreSkillBundle()

	got := MetadataForTarget(bundle.Metadata, "fish")

	if got.Target != "fish" {
		t.Fatalf("MetadataForTarget().Target = %q, want %q", got.Target, "fish")
	}
	if got.ID != bundle.Metadata.ID {
		t.Fatalf("MetadataForTarget().ID = %q, want %q", got.ID, bundle.Metadata.ID)
	}
	if got.Version != bundle.Metadata.Version {
		t.Fatalf("MetadataForTarget().Version = %q, want %q", got.Version, bundle.Metadata.Version)
	}
	if got.Hash != bundle.Metadata.Hash {
		t.Fatalf("MetadataForTarget().Hash = %q, want %q", got.Hash, bundle.Metadata.Hash)
	}
	if bundle.Metadata.Target != TargetShared {
		t.Fatalf("CoreSkillBundle().Metadata.Target changed to %q, want %q", bundle.Metadata.Target, TargetShared)
	}
}

func TestCompareMetadata(t *testing.T) {
	canonical := CoreSkillBundle().Metadata

	t.Run("current", func(t *testing.T) {
		status := CompareMetadata(canonical, canonical)
		if status != StatusCurrent {
			t.Fatalf("CompareMetadata() = %q, want %q", status, StatusCurrent)
		}
	})

	t.Run("outdated version", func(t *testing.T) {
		installed := canonical
		installed.Version = "0.9.0"

		status := CompareMetadata(installed, canonical)
		if status != StatusOutdated {
			t.Fatalf("CompareMetadata() = %q, want %q", status, StatusOutdated)
		}
	})

	t.Run("modified hash", func(t *testing.T) {
		installed := canonical
		installed.Hash = "hash-modified"

		status := CompareMetadata(installed, canonical)
		if status != StatusModified {
			t.Fatalf("CompareMetadata() = %q, want %q", status, StatusModified)
		}
	})

	t.Run("unmanaged metadata", func(t *testing.T) {
		installed := canonical
		installed.Target = "fish"

		status := CompareMetadata(installed, canonical)
		if status != StatusUnmanaged {
			t.Fatalf("CompareMetadata() = %q, want %q", status, StatusUnmanaged)
		}
	})
}

func TestDeriveStatus(t *testing.T) {
	bundle := CoreSkillBundle()
	canonical := bundle.Metadata

	t.Run("detection unknown", func(t *testing.T) {
		status := DeriveStatus(TargetShared, false, nil, canonical)
		if status.Status != StatusDetectionUnknown {
			t.Fatalf("DeriveStatus().Status = %q, want %q", status.Status, StatusDetectionUnknown)
		}
	})

	t.Run("not installed", func(t *testing.T) {
		status := DeriveStatus(TargetShared, true, nil, canonical)
		if status.Status != StatusNotInstalled {
			t.Fatalf("DeriveStatus().Status = %q, want %q", status.Status, StatusNotInstalled)
		}
	})

	t.Run("managed install", func(t *testing.T) {
		installed := canonical

		status := DeriveStatus(TargetShared, true, &installed, canonical)
		if status.Status != StatusCurrent {
			t.Fatalf("DeriveStatus().Status = %q, want %q", status.Status, StatusCurrent)
		}
		if status.Target != TargetShared {
			t.Fatalf("DeriveStatus().Target = %q, want %q", status.Target, TargetShared)
		}
		if status.Installed == nil {
			t.Fatal("DeriveStatus().Installed = nil, want metadata")
		}
	})

	t.Run("target mismatch is unmanaged", func(t *testing.T) {
		installed := canonical
		installed.Target = "zsh"

		status := DeriveStatus(TargetShared, true, &installed, canonical)
		if status.Status != StatusUnmanaged {
			t.Fatalf("DeriveStatus().Status = %q, want %q", status.Status, StatusUnmanaged)
		}
	})

	t.Run("target-stamped install is current", func(t *testing.T) {
		installed := MetadataForTarget(canonical, "fish")

		status := DeriveStatus("fish", true, &installed, canonical)
		if status.Status != StatusCurrent {
			t.Fatalf("DeriveStatus().Status = %q, want %q", status.Status, StatusCurrent)
		}
		if status.Canonical.Target != "fish" {
			t.Fatalf("DeriveStatus().Canonical.Target = %q, want %q", status.Canonical.Target, "fish")
		}
	})
}

func TestManagerExecuteAllInstallsDetectedTargetsOnly(t *testing.T) {
	detected := &stubTarget{key: "claude", detected: true}
	undetected := &stubTarget{key: "codex", detected: false}
	manager := NewManager([]Target{detected, undetected})

	output, err := manager.ExecuteAll()
	if err != nil {
		t.Fatalf("ExecuteAll() error = %v", err)
	}
	if output != "installed ktree skill for: claude" {
		t.Fatalf("ExecuteAll() output = %q, want %q", output, "installed ktree skill for: claude")
	}

	if detected.detectCalls != 1 {
		t.Fatalf("detected target Detect() calls = %d, want 1", detected.detectCalls)
	}
	if detected.installCalls != 1 {
		t.Fatalf("detected target InstallBundle() calls = %d, want 1", detected.installCalls)
	}
	if undetected.detectCalls != 1 {
		t.Fatalf("undetected target Detect() calls = %d, want 1", undetected.detectCalls)
	}
	if undetected.installCalls != 0 {
		t.Fatalf("undetected target InstallBundle() calls = %d, want 0", undetected.installCalls)
	}
}

func TestManagerExecuteAllReturnsDetectError(t *testing.T) {
	wantErr := fmt.Errorf("boom")
	manager := NewManager([]Target{&stubTarget{key: "claude", detectErr: wantErr}})

	output, err := manager.ExecuteAll()
	if output != "failed targets: claude (detect: boom)" {
		t.Fatalf("ExecuteAll() output = %q, want %q", output, "failed targets: claude (detect: boom)")
	}
	if err == nil || err.Error() != "install skill completed with failures" {
		t.Fatalf("ExecuteAll() error = %v, want %q", err, "install skill completed with failures")
	}
}

func TestManagerExecuteAllReturnsNoSupportedTargetsDetected(t *testing.T) {
	manager := NewManager([]Target{
		&stubTarget{key: "claude", detected: false},
		&stubTarget{key: "codex", detected: false},
	})

	output, err := manager.ExecuteAll()
	if output != "" {
		t.Fatalf("ExecuteAll() output = %q, want empty string", output)
	}
	if err == nil || err.Error() != "no supported install targets detected" {
		t.Fatalf("ExecuteAll() error = %v, want %q", err, "no supported install targets detected")
	}
}

func TestManagerExecuteAllAggregatesPartialFailuresInTargetOrder(t *testing.T) {
	manager := NewManager([]Target{
		&stubTarget{key: "cursor", detected: true},
		&stubTarget{key: "claude", detected: true, installErr: fmt.Errorf("permission denied")},
		&stubTarget{key: "codex", detectErr: fmt.Errorf("boom")},
		&stubTarget{key: "grok", detected: false},
		&stubTarget{key: "opencode", detected: true},
	})

	output, err := manager.ExecuteAll()
	wantOutput := "installed ktree skill for: cursor, opencode\nfailed targets: claude (install: permission denied), codex (detect: boom)"
	if output != wantOutput {
		t.Fatalf("ExecuteAll() output = %q, want %q", output, wantOutput)
	}
	if err == nil || err.Error() != "install skill completed with failures" {
		t.Fatalf("ExecuteAll() error = %v, want %q", err, "install skill completed with failures")
	}
	if strings.Index(output, "cursor") > strings.Index(output, "opencode") {
		t.Fatalf("ExecuteAll() output = %q, want successes in target order", output)
	}
	if strings.Index(output, "claude") > strings.Index(output, "codex") {
		t.Fatalf("ExecuteAll() output = %q, want failures in target order", output)
	}
	if strings.Contains(output, "grok") {
		t.Fatalf("ExecuteAll() output = %q, want undetected targets omitted from summary", output)
	}
	if strings.Contains(output, "cursor, claude") {
		t.Fatalf("ExecuteAll() output = %q, want failed targets excluded from success summary", output)
	}
	if strings.Contains(output, "codex") && strings.Contains(output, "installed ktree skill for: cursor, opencode, codex") {
		t.Fatalf("ExecuteAll() output = %q, want detect failures excluded from success summary", output)
	}
	if strings.Contains(output, "grok") {
		t.Fatalf("ExecuteAll() output = %q, want undetected targets omitted from summary", output)
	}
	if strings.Contains(output, "permission denied") && strings.Contains(output, "boom") == false {
		t.Fatalf("ExecuteAll() output = %q, want all failure reasons included", output)
	}
	if strings.Contains(output, "install: permission denied") == false || strings.Contains(output, "detect: boom") == false {
		t.Fatalf("ExecuteAll() output = %q, want categorized failure reasons", output)
	}
	if strings.Contains(output, "failed targets: claude") == false {
		t.Fatalf("ExecuteAll() output = %q, want failure summary", output)
	}
	if strings.Contains(output, "installed ktree skill for: cursor, opencode") == false {
		t.Fatalf("ExecuteAll() output = %q, want success summary", output)
	}
	if strings.Count(output, "\n") != 1 {
		t.Fatalf("ExecuteAll() output = %q, want exactly two summary lines", output)
	}
	if strings.Contains(output, "grok") {
		t.Fatalf("ExecuteAll() output = %q, want undetected targets omitted from summary", output)
	}
	if strings.Contains(output, "cursor") == false || strings.Contains(output, "opencode") == false {
		t.Fatalf("ExecuteAll() output = %q, want successful targets listed", output)
	}
	if strings.Contains(output, "claude") == false || strings.Contains(output, "codex") == false {
		t.Fatalf("ExecuteAll() output = %q, want failed targets listed", output)
	}
	if strings.Contains(output, "failed targets: codex") {
		t.Fatalf("ExecuteAll() output = %q, want failure order to follow target order", output)
	}
	if strings.Contains(output, "installed ktree skill for: opencode, cursor") {
		t.Fatalf("ExecuteAll() output = %q, want success order to follow target order", output)
	}
	if strings.Contains(output, "failed targets: codex (detect: boom), claude (install: permission denied)") {
		t.Fatalf("ExecuteAll() output = %q, want failure order to follow target order", output)
	}
	if strings.Contains(output, "no supported install targets detected") {
		t.Fatalf("ExecuteAll() output = %q, want partial failure summary instead of unsupported error", output)
	}
	if strings.Contains(output, "failed targets: ") == false {
		t.Fatalf("ExecuteAll() output = %q, want failure summary prefix", output)
	}
	if strings.Contains(output, "installed ktree skill for: ") == false {
		t.Fatalf("ExecuteAll() output = %q, want success summary prefix", output)
	}
	if strings.Contains(output, "\nfailed targets: claude (install: permission denied), codex (detect: boom)") == false {
		t.Fatalf("ExecuteAll() output = %q, want failures on second line", output)
	}
	if strings.HasSuffix(output, " ") {
		t.Fatalf("ExecuteAll() output = %q, want trimmed summary", output)
	}
	if strings.Contains(output, "  ") {
		t.Fatalf("ExecuteAll() output = %q, want concise spacing", output)
	}
}

func TestManagerOptionsReturnsDetectedTargetsWithStatusLabelsInTargetOrder(t *testing.T) {
	bundle := CoreSkillBundle()
	current := MetadataForTarget(bundle.Metadata, "codex")
	modified := MetadataForTarget(bundle.Metadata, "claude")
	modified.Hash = "custom-hash"

	manager := NewManager([]Target{
		&stubTarget{key: "codex", label: "Codex", detected: true, installed: &InstalledArtifact{Metadata: &current}},
		&stubTarget{key: "claude", label: "Claude", detected: true, installed: &InstalledArtifact{Metadata: &modified}},
		&stubTarget{key: "cursor", label: "Cursor", detected: false},
	})

	rows, err := manager.Options()
	if err != nil {
		t.Fatalf("Options() error = %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("len(Options()) = %d, want %d", len(rows), 2)
	}
	if rows[0] != (installui.TargetRow{Key: "codex", Label: "Codex", Status: "current"}) {
		t.Fatalf("Options()[0] = %+v, want current Codex row", rows[0])
	}
	if rows[1] != (installui.TargetRow{Key: "claude", Label: "Claude", Status: "modified"}) {
		t.Fatalf("Options()[1] = %+v, want modified Claude row", rows[1])
	}
}

type stubTarget struct {
	key          string
	label        string
	detected     bool
	detectErr    error
	inspectErr   error
	installed    *InstalledArtifact
	installErr   error
	detectCalls  int
	installCalls int
}

func (t *stubTarget) Key() string {
	if t.key != "" {
		return t.key
	}
	return "stub"
}

func (t *stubTarget) Label() string {
	if t.label != "" {
		return t.label
	}
	return "Stub"
}

func (t *stubTarget) Detect() (bool, error) {
	t.detectCalls++
	return t.detected, t.detectErr
}

func (t *stubTarget) ManagedPath() (string, error) { return "", nil }

func (t *stubTarget) InspectInstalled() (*InstalledArtifact, error) {
	return t.installed, t.inspectErr
}

func (t *stubTarget) InstallBundle(SkillBundle) error {
	t.installCalls++
	return t.installErr
}

func (t *stubTarget) ExportBundle(SkillBundle) (string, error) { return "", nil }
