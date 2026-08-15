package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"alikazai/ktree/internal/git"
	"alikazai/ktree/internal/vault"
)

func TestCreateShowsBusyStateAfterSubmit(t *testing.T) {
	m := New("/repo")
	m.mode = modeCreate
	m.input.SetValue("feature/test")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)

	view := got.View()
	if !strings.Contains(view, "Creating worktree...") {
		t.Fatalf("expected busy create message in view, got:\n%s", view)
	}
	if strings.Contains(view, "enter to create") {
		t.Fatalf("expected create prompt to be replaced while busy, got:\n%s", view)
	}

	stillBusy, _ := got.Update(tea.KeyMsg{Type: tea.KeyEsc})
	busyView := stillBusy.(Model).View()
	if !strings.Contains(busyView, "Creating worktree...") {
		t.Fatalf("expected esc to be ignored while create is busy, got:\n%s", busyView)
	}
	if strings.Contains(busyView, "enter to create") {
		t.Fatalf("expected create prompt to stay blocked while busy, got:\n%s", busyView)
	}
}

func TestDeleteShowsBusyStateAfterConfirm(t *testing.T) {
	m := New("/repo")
	m.mode = modeDeleteConfirm
	m.worktrees = []git.Worktree{{Path: "/repo-worktrees/feature-test", Branch: "feature/test"}}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	got := updated.(Model)

	view := got.View()
	if !strings.Contains(view, "Deleting worktree...") {
		t.Fatalf("expected busy delete message in view, got:\n%s", view)
	}
	if strings.Contains(view, "Delete /repo-worktrees/feature-test? (y/N)") {
		t.Fatalf("expected delete confirm prompt to be replaced while busy, got:\n%s", view)
	}

	stillBusy, _ := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	busyView := stillBusy.(Model).View()
	if !strings.Contains(busyView, "Deleting worktree...") {
		t.Fatalf("expected conflicting key to be ignored while delete is busy, got:\n%s", busyView)
	}
	if strings.Contains(busyView, "Delete /repo-worktrees/feature-test? (y/N)") {
		t.Fatalf("expected delete prompt to stay blocked while busy, got:\n%s", busyView)
	}
}

func TestCreateFailureShowsBannerAndInlineError(t *testing.T) {
	m := New("/repo")
	m.mode = modeCreate
	m.input.SetValue("feature/test")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)

	failed, _ := got.Update(worktreeCreatedMsg{err: errBoom{}})
	failedModel := failed.(Model)

	view := failedModel.View()
	if !strings.Contains(view, "error: boom") {
		t.Fatalf("expected top-level error banner, got:\n%s", view)
	}
	if strings.Count(view, "error: boom") < 2 {
		t.Fatalf("expected both banner and inline create error, got:\n%s", view)
	}
	if !strings.Contains(view, "enter to create") {
		t.Fatalf("expected create prompt to be restored after failure, got:\n%s", view)
	}
}

func TestCreateInlineErrorSurvivesLateWorktreeReload(t *testing.T) {
	m := New("/repo")
	m.mode = modeCreate
	m.input.SetValue("feature/test")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	failed, _ := updated.(Model).Update(worktreeCreatedMsg{err: errBoom{}})
	reloaded, _ := failed.(Model).Update(worktreesLoadedMsg{worktrees: []git.Worktree{{Path: "/repo", Branch: "main"}}})
	view := reloaded.(Model).View()
	if strings.Count(view, "error: boom") < 2 {
		t.Fatalf("expected late reload to preserve both banner and inline create errors, got:\n%s", view)
	}
}

func TestDeleteFailurePromptsForceDeleteAndShowsBanner(t *testing.T) {
	m := New("/repo")
	m.mode = modeDeleteConfirm
	m.worktrees = []git.Worktree{{Path: "/repo-worktrees/feature-test", Branch: "feature/test"}}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	failed, _ := updated.(Model).Update(worktreeDeletedMsg{err: errBoom{}, forced: false})
	got := failed.(Model)

	view := got.View()
	if got.mode != modeDeleteForce {
		t.Fatalf("expected force-delete mode after delete failure, got %v", got.mode)
	}
	if !strings.Contains(view, "Force delete /repo-worktrees/feature-test? (y/N)") {
		t.Fatalf("expected force-delete prompt, got:\n%s", view)
	}
	if strings.Count(view, "boom") < 2 {
		t.Fatalf("expected both banner and inline delete errors, got:\n%s", view)
	}
}

func TestForcedDeleteFailureReturnsToListWithBanner(t *testing.T) {
	m := New("/repo")
	m.mode = modeDeleteForce
	m.worktrees = []git.Worktree{{Path: "/repo-worktrees/feature-test", Branch: "feature/test"}}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	failed, _ := updated.(Model).Update(worktreeDeletedMsg{err: errBoom{}, forced: true})
	got := failed.(Model)

	view := got.View()
	if got.mode != modeList {
		t.Fatalf("expected list mode after forced delete failure, got %v", got.mode)
	}
	if !strings.Contains(view, "error: boom") {
		t.Fatalf("expected top-level error banner, got:\n%s", view)
	}
	if strings.Contains(view, "Force delete /repo-worktrees/feature-test? (y/N)") {
		t.Fatalf("expected force prompt to be dismissed, got:\n%s", view)
	}
}

func TestCreateSuccessWithVaultCopyStaysBusy(t *testing.T) {
	m := New("/repo")
	m.mode = modeCreate
	m.input.SetValue("feature/test")
	m.pendingBranch = "feature/test"
	m.copyVaultOnCreate = true

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	succeeded, _ := updated.(Model).Update(worktreeCreatedMsg{})
	got := succeeded.(Model)

	view := got.View()
	if !strings.Contains(view, "Copying vault...") {
		t.Fatalf("expected copy-vault busy message after create succeeds, got:\n%s", view)
	}
	if !strings.Contains(view, "/repo-worktrees/feature-test") {
		t.Fatalf("expected copy-vault target path, got:\n%s", view)
	}
}

func TestNewActionClearsBannerError(t *testing.T) {
	m := New("/repo")
	m.bannerErr = errBoom{}
	m.worktrees = []git.Worktree{{Path: "/repo-worktrees/feature-test", Branch: "feature/test"}}

	view := m.View()
	if !strings.Contains(view, "error: boom") {
		t.Fatalf("expected initial banner error, got:\n%s", view)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	cleared := updated.(Model).View()
	if strings.Contains(cleared, "error: boom") {
		t.Fatalf("expected new action to clear banner error, got:\n%s", cleared)
	}
	if !strings.Contains(cleared, "New worktree") {
		t.Fatalf("expected new worktree prompt after clearing error, got:\n%s", cleared)
	}
}

func TestPendingOperationStillAllowsCtrlC(t *testing.T) {
	m := New("/repo")
	m.mode = modeCreate
	m.input.SetValue("feature/test")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_, cmd := updated.(Model).Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected ctrl+c to quit while busy")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg from ctrl+c, got %T", cmd())
	}
}

func TestVaultConfirmedCreateStartsBusyState(t *testing.T) {
	m := New("/repo")
	m.pendingBranch = "feature/test"
	v := vault.NewCopyConfirmFlow("/repo", "feature/test")
	m.vaultModel = &v

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	view := updated.(Model).View()
	if !strings.Contains(view, "Creating worktree...") {
		t.Fatalf("expected busy create message after vault confirmation, got:\n%s", view)
	}
}

func TestVaultCopyFailureBannerPersistsUntilNextAction(t *testing.T) {
	m := New("/repo")
	m.pending = &pendingOperation{kind: operationCopyVault, label: "Copying vault...", target: "/repo-worktrees/feature-test"}

	updated, _ := m.Update(vaultCopiedMsg{err: errBoom{}})
	afterCopy := updated.(Model)
	if !strings.Contains(afterCopy.View(), "error: boom") {
		t.Fatalf("expected vault copy failure banner immediately after failure, got:\n%s", afterCopy.View())
	}

	reloaded, _ := afterCopy.Update(worktreesLoadedMsg{worktrees: []git.Worktree{{Path: "/repo", Branch: "main"}}})
	stillShowing := reloaded.(Model).View()
	if !strings.Contains(stillShowing, "error: boom") {
		t.Fatalf("expected banner to persist after reload, got:\n%s", stillShowing)
	}

	cleared, _ := reloaded.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if strings.Contains(cleared.(Model).View(), "error: boom") {
		t.Fatalf("expected next user action to clear vault copy banner, got:\n%s", cleared.(Model).View())
	}
}

func TestVaultCopyFailureBannerSurvivesReloadError(t *testing.T) {
	m := New("/repo")
	m.pending = &pendingOperation{kind: operationCopyVault, label: "Copying vault...", target: "/repo-worktrees/feature-test"}

	updated, _ := m.Update(vaultCopiedMsg{err: errBoom{}})
	reloaded, _ := updated.(Model).Update(worktreesLoadedMsg{err: errReload{}})
	view := reloaded.(Model).View()
	if !strings.Contains(view, "error: boom") {
		t.Fatalf("expected original vault copy error banner to persist through reload failure, got:\n%s", view)
	}
	if strings.Contains(view, "reload failed") {
		t.Fatalf("expected reload error not to replace the original vault copy failure banner, got:\n%s", view)
	}
}

func TestNavigationClearsBannerError(t *testing.T) {
	m := New("/repo")
	m.bannerErr = errBoom{}
	m.worktrees = []git.Worktree{{Path: "/repo", Branch: "main"}, {Path: "/repo-worktrees/feature-test", Branch: "feature/test"}}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	got := updated.(Model)
	if got.cursor != 1 {
		t.Fatalf("expected cursor to move, got %d", got.cursor)
	}
	if strings.Contains(got.View(), "error: boom") {
		t.Fatalf("expected navigation to clear banner error, got:\n%s", got.View())
	}
}

func TestTypingInCreateModeClearsBannerError(t *testing.T) {
	m := New("/repo")
	m.mode = modeCreate
	m.bannerErr = errBoom{}
	m.input.Focus()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	got := updated.(Model)
	if got.input.Value() != "f" {
		t.Fatalf("expected input to receive typed key, got %q", got.input.Value())
	}
	if strings.Contains(got.View(), "error: boom") {
		t.Fatalf("expected typing to clear banner error, got:\n%s", got.View())
	}
}

func TestRetryAfterVaultCopyDeclinedDoesNotCopyVault(t *testing.T) {
	m := New("/repo")
	m.pendingBranch = "feature/test"
	v := vault.NewCopyConfirmFlow("/repo", "feature/test")
	m.vaultModel = &v

	started, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	failed, _ := started.(Model).Update(worktreeCreatedMsg{err: errBoom{}})
	retrying := failed.(Model)

	v = vault.NewCopyConfirmFlow("/repo", "feature/test")
	retrying.vaultModel = &v
	declined, _ := retrying.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	succeeded, _ := declined.(Model).Update(worktreeCreatedMsg{})
	got := succeeded.(Model)

	if got.copyVaultOnCreate {
		t.Fatal("expected vault copy flag to be cleared after retrying with copy declined")
	}
	if strings.Contains(got.View(), "Copying vault...") {
		t.Fatalf("expected retry success without vault copy state, got:\n%s", got.View())
	}
}

func TestSpinnerStopsSchedulingAfterBusyStateEnds(t *testing.T) {
	m := New("/repo")
	m.mode = modeCreate
	m.input.SetValue("feature/test")

	started, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	completed, _ := started.(Model).Update(worktreeCreatedMsg{})
	_, cmd := completed.(Model).Update(spinner.TickMsg{Time: time.Now()})
	if cmd != nil {
		t.Fatal("expected spinner ticks to stop scheduling after pending work completes")
	}
}

type errBoom struct{}

func (errBoom) Error() string { return "boom" }

type errReload struct{}

func (errReload) Error() string { return "reload failed" }
