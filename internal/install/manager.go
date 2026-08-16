package install

import (
	"fmt"
	"strings"

	"alikazai/ktree/internal/installui"
)

type Status string

const (
	StatusNotInstalled     Status = "not_installed"
	StatusCurrent          Status = "current"
	StatusOutdated         Status = "outdated"
	StatusModified         Status = "modified"
	StatusUnmanaged        Status = "unmanaged"
	StatusDetectionUnknown Status = "detection_unknown"
)

type TargetStatus struct {
	Target    string
	Status    Status
	Installed *Metadata
	Canonical Metadata
}

type Manager struct {
	bundle  SkillBundle
	targets []Target
}

func NewManager(targets []Target) Manager {
	cloned := make([]Target, len(targets))
	copy(cloned, targets)
	return Manager{bundle: CoreSkillBundle(), targets: cloned}
}

func (m Manager) Options() ([]installui.TargetRow, error) {
	rows := make([]installui.TargetRow, 0, len(m.targets))
	for _, target := range m.targets {
		detected, err := target.Detect()
		if err != nil {
			return nil, fmt.Errorf("detect %s: %w", target.Key(), err)
		}
		if !detected {
			continue
		}

		installed, err := target.InspectInstalled()
		if err != nil {
			return nil, fmt.Errorf("inspect %s: %w", target.Key(), err)
		}

		var installedMetadata *Metadata
		if installed != nil {
			installedMetadata = installed.Metadata
		}

		status := DeriveStatus(target.Key(), true, installedMetadata, m.bundle.Metadata)
		rows = append(rows, installui.TargetRow{
			Key:    target.Key(),
			Label:  target.Label(),
			Status: statusLabel(status.Status),
		})
	}

	return rows, nil
}

func (m Manager) Execute(targetKey string, export bool) (string, error) {
	target, err := m.lookup(targetKey)
	if err != nil {
		return "", err
	}
	if export {
		return target.ExportBundle(m.bundle)
	}
	if err := target.InstallBundle(m.bundle); err != nil {
		return "", err
	}
	return "", nil
}

func (m Manager) ExecuteAll() (string, error) {
	var successes []string
	var failures []string
	detectedCount := 0

	for _, target := range m.targets {
		detected, err := target.Detect()
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s (detect: %v)", target.Key(), err))
			continue
		}
		if !detected {
			continue
		}
		detectedCount++
		if err := target.InstallBundle(m.bundle); err != nil {
			failures = append(failures, fmt.Sprintf("%s (install: %v)", target.Key(), err))
			continue
		}
		successes = append(successes, target.Key())
	}

	if detectedCount == 0 && len(failures) == 0 {
		return "", fmt.Errorf("no supported install targets detected")
	}

	return formatExecuteAllSummary(successes, failures), executeAllError(failures)
}

func formatExecuteAllSummary(successes, failures []string) string {
	parts := make([]string, 0, 2)
	if len(successes) > 0 {
		parts = append(parts, "installed ktree skill for: "+strings.Join(successes, ", "))
	}
	if len(failures) > 0 {
		parts = append(parts, "failed targets: "+strings.Join(failures, ", "))
	}
	return strings.Join(parts, "\n")
}

func executeAllError(failures []string) error {
	if len(failures) == 0 {
		return nil
	}
	return fmt.Errorf("install skill completed with failures")
}

func (m Manager) lookup(targetKey string) (Target, error) {
	for _, target := range m.targets {
		if target.Key() == targetKey {
			return target, nil
		}
	}
	return nil, fmt.Errorf("unknown install target %q", targetKey)
}

func statusLabel(status Status) string {
	switch status {
	case StatusCurrent:
		return "current"
	case StatusOutdated:
		return "outdated"
	case StatusModified:
		return "modified"
	case StatusUnmanaged:
		return "unmanaged"
	case StatusDetectionUnknown:
		return "unknown"
	default:
		return "not installed"
	}
}

func CompareMetadata(installed, canonical Metadata) Status {
	if installed.ID != canonical.ID || installed.Target != canonical.Target {
		return StatusUnmanaged
	}
	if installed.Version != canonical.Version {
		return StatusOutdated
	}
	if installed.Hash != canonical.Hash {
		return StatusModified
	}
	return StatusCurrent
}

func DeriveStatus(target string, detectionKnown bool, installed *Metadata, canonical Metadata) TargetStatus {
	canonical = MetadataForTarget(canonical, target)

	status := TargetStatus{
		Target:    target,
		Installed: installed,
		Canonical: canonical,
	}

	if !detectionKnown {
		status.Status = StatusDetectionUnknown
		return status
	}
	if installed == nil {
		status.Status = StatusNotInstalled
		return status
	}

	status.Status = CompareMetadata(*installed, canonical)
	return status
}
