package ui

import (
	tea "github.com/charmbracelet/bubbletea"

	"alikazai/ktree/internal/git"
	"alikazai/ktree/internal/vault"
)

func (m Model) loadWorktrees() tea.Msg {
	wts, err := git.List(m.repoDir)
	return worktreesLoadedMsg{worktrees: wts, err: err}
}

func (m Model) loadStatus(wt git.Worktree) tea.Cmd {
	return func() tea.Msg {
		dirty, err := git.IsDirty(wt.Path)
		ahead, behind, hasUpstream := git.AheadBehind(m.repoDir, wt.Branch)
		var probeErr error
		if err != nil {
			probeErr = errProbeFailed(wt.Path, err.Error())
		}
		return statusLoadedMsg{
			path:       wt.Path,
			dirtyKnown: err == nil,
			probeErr:   probeErr,
			status: worktreeStatus{
				dirty:       dirty,
				ahead:       ahead,
				behind:      behind,
				hasUpstream: hasUpstream,
			},
		}
	}
}

func (m Model) createWorktree(branch string) tea.Cmd {
	return func() tea.Msg {
		path := git.WorktreePath(m.repoDir, branch)
		err := git.AddWorktree(m.repoDir, path, branch)
		return worktreeCreatedMsg{err: err}
	}
}

func (m Model) createWorktreeFromSelected(base git.Worktree, branch string) tea.Cmd {
	return func() tea.Msg {
		path := git.WorktreePath(m.repoDir, branch)
		baseRef := base.Branch
		if baseRef == "" {
			baseRef = base.Head
		}
		err := git.AddWorktreeFromBase(m.repoDir, path, branch, baseRef)
		return worktreeCreatedMsg{err: err}
	}
}

func (m Model) pullWorktree(wt git.Worktree) tea.Cmd {
	return func() tea.Msg {
		return worktreePulledMsg{err: git.PullWorktree(wt.Path)}
	}
}

func (m Model) deleteWorktree(wt git.Worktree, force bool) tea.Cmd {
	return func() tea.Msg {
		err := git.RemoveWorktree(m.repoDir, wt.Path, force)
		return worktreeDeletedMsg{err: err, forced: force}
	}
}

func (m Model) copyVault(worktreePath string) tea.Cmd {
	return func() tea.Msg {
		return vaultCopiedMsg{err: vault.CopyVaultToWorktree(m.repoDir, worktreePath)}
	}
}
