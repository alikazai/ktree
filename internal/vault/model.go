package vault

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type vaultMode int

const (
	vaultModeDiscovering vaultMode = iota
	vaultModeInit
	vaultModeCopyConfirm
)

var (
	helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Padding(1, 1, 0, 1)
	pathStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	errStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

type envFilesDiscoveredMsg struct {
	files []string
	err   error
}

type vaultInitializedMsg struct{ err error }

// VaultModel is a Bubble Tea sub-model owning the vault feature UI flows.
// Embed via pointer in the root model; delegate Update and View when non-nil.
type VaultModel struct {
	repoDir    string
	mode       vaultMode
	files      []string
	branch     string // copy confirm flow only
	done       bool
	shouldCopy bool
	err        error
}

// NewInitFlow starts the vault initialisation flow (triggered by the "v" key).
// The caller must fire Init() as a Cmd to begin file discovery.
func NewInitFlow(repoDir string) VaultModel {
	return VaultModel{repoDir: repoDir, mode: vaultModeDiscovering}
}

// NewCopyConfirmFlow starts the vault copy-confirm flow (triggered during
// worktree creation when a vault already exists).
func NewCopyConfirmFlow(repoDir, branch string) VaultModel {
	return VaultModel{repoDir: repoDir, mode: vaultModeCopyConfirm, branch: branch}
}

// Init returns the first Cmd for the flow. For the init flow this kicks off
// env-file discovery; for copy confirm there is nothing to do upfront.
func (v VaultModel) Init() tea.Cmd {
	if v.mode == vaultModeDiscovering {
		return v.discoverEnvFiles
	}
	return nil
}

func (v VaultModel) discoverEnvFiles() tea.Msg {
	files, err := DiscoverEnvFiles(v.repoDir)
	return envFilesDiscoveredMsg{files: files, err: err}
}

func (v VaultModel) runInitVault() tea.Cmd {
	return func() tea.Msg {
		return vaultInitializedMsg{err: InitVault(v.repoDir, v.files)}
	}
}

// Done reports whether the flow has finished (success, cancel, or error).
func (v VaultModel) Done() bool { return v.done }

// ShouldCopy reports whether the user confirmed a vault copy. Only meaningful
// after a copy-confirm flow where Done() is true.
func (v VaultModel) ShouldCopy() bool { return v.shouldCopy }

// Err returns any error that caused the flow to terminate early.
func (v VaultModel) Err() error { return v.err }

func (v VaultModel) Update(msg tea.Msg) (VaultModel, tea.Cmd) {
	switch msg := msg.(type) {
	case envFilesDiscoveredMsg:
		if msg.err != nil {
			v.err = msg.err
			v.done = true
			return v, nil
		}
		if len(msg.files) == 0 {
			v.err = fmt.Errorf("no .env files found in repo")
			v.done = true
			return v, nil
		}
		v.files = msg.files
		v.mode = vaultModeInit
		return v, nil

	case vaultInitializedMsg:
		v.err = msg.err
		v.done = true
		return v, nil

	case tea.KeyMsg:
		switch v.mode {
		case vaultModeInit:
			switch msg.String() {
			case "y", "Y":
				return v, v.runInitVault()
			case "n", "N", "esc":
				v.done = true
				return v, nil
			}
		case vaultModeCopyConfirm:
			switch msg.String() {
			case "y", "Y", "enter":
				v.shouldCopy = true
				v.done = true
				return v, nil
			case "n", "N", "esc":
				v.shouldCopy = false
				v.done = true
				return v, nil
			}
		}
	}
	return v, nil
}

func (v VaultModel) View() string {
	var b strings.Builder

	switch v.mode {
	case vaultModeDiscovering:
		b.WriteString(helpStyle.Render("Discovering .env files..."))

	case vaultModeInit:
		b.WriteString(helpStyle.Render("Vault files discovered:") + "\n\n")
		for _, f := range v.files {
			b.WriteString(pathStyle.Render(fmt.Sprintf("    %s", f)) + "\n")
		}
		b.WriteString("\n")
		b.WriteString(helpStyle.Render(fmt.Sprintf("Initialize vault at %s? (y/N)", VaultDir(v.repoDir))))
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("y yes · n/esc cancel"))

	case vaultModeCopyConfirm:
		b.WriteString(helpStyle.Render(fmt.Sprintf("Copy vault to %s? (Y/n)", v.branch)))
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("enter/y yes · n/esc no"))
	}

	return b.String()
}
