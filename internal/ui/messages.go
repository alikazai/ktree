package ui

import "alikazai/ktree/internal/git"

type worktreesLoadedMsg struct {
	worktrees []git.Worktree
	err       error
}

type statusLoadedMsg struct {
	path       string
	status     worktreeStatus
	dirtyKnown bool
	probeErr   error
}

type worktreeCreatedMsg struct{ err error }

type worktreePulledMsg struct{ err error }

type worktreeDeletedMsg struct {
	err    error
	forced bool
}

type vaultCopiedMsg struct{ err error }
