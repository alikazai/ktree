package ui

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

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

func TestBranchFromSelectedRefusesDirtyWorktree(t *testing.T) {
	m := New("/repo")
	m.worktrees = []git.Worktree{{Path: "/repo-worktrees/feature-auth", Branch: "feature/auth"}}
	m.statuses = map[string]worktreeStatus{
		"/repo-worktrees/feature-auth": {dirty: true},
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	got := updated.(Model)
	if got.mode != modeList {
		t.Fatalf("expected dirty selected worktree to stay in list mode, got %v", got.mode)
	}
	if got.pending != nil {
		t.Fatalf("expected dirty selected worktree not to enter busy state, got pending %+v", *got.pending)
	}
	if got.input.Value() != "" {
		t.Fatalf("expected dirty selected worktree not to prefill branch input, got %q", got.input.Value())
	}

	view := got.View()
	if !strings.Contains(view, "cannot branch from dirty worktree") {
		t.Fatalf("expected dirty-branch refusal, got:\n%s", view)
	}
}

func TestBranchFromSelectedRefusesLoadingWorktreeStatus(t *testing.T) {
	m := New("/repo")
	m.worktrees = []git.Worktree{{Path: "/repo-worktrees/feature-auth", Branch: "feature/auth"}}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	got := updated.(Model)
	if got.mode != modeList {
		t.Fatalf("expected loading selected worktree to stay in list mode, got %v", got.mode)
	}
	if got.pending != nil {
		t.Fatalf("expected loading selected worktree not to enter busy state, got pending %+v", *got.pending)
	}
	if got.input.Value() != "" {
		t.Fatalf("expected loading selected worktree not to prefill branch input, got %q", got.input.Value())
	}

	view := got.View()
	if !strings.Contains(view, "cannot branch while worktree status is still loading") {
		t.Fatalf("expected loading-status refusal, got:\n%s", view)
	}
}

func TestBranchFromSelectedRefusesUntrustedDirtyStatus(t *testing.T) {
	m := New("/repo")
	m.width = 120
	m.worktrees = []git.Worktree{{Path: "/repo-worktrees/feature-auth", Branch: "feature/auth"}}

	updated, _ := m.Update(statusLoadedMsg{
		path: "/repo-worktrees/feature-auth",
		status: worktreeStatus{
			dirty: false,
			ahead: 2,
		},
		dirtyKnown: false,
	})
	got := updated.(Model)

	if _, known := got.statuses["/repo-worktrees/feature-auth"]; known {
		t.Fatal("expected untrusted dirty status not to be recorded as known")
	}

	blocked, _ := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	blockedModel := blocked.(Model)
	if blockedModel.mode != modeList {
		t.Fatalf("expected untrusted dirty status to keep branch-from-selected blocked, got %v", blockedModel.mode)
	}

	view := ansi.Strip(blockedModel.View())
	if !strings.Contains(view, "cannot branch while worktree status is still loading") {
		t.Fatalf("expected untrusted dirty status to block branching like loading status, got:\n%s", view)
	}
	if !strings.Contains(view, "loading") {
		t.Fatalf("expected untrusted dirty status to render as loading, got:\n%s", view)
	}
	if strings.Contains(view, "clean") {
		t.Fatalf("expected untrusted dirty status not to render as clean, got:\n%s", view)
	}
}

func TestBranchFromSelectedPrefillsFromSelectedBranch(t *testing.T) {
	m := New("/repo")
	m.worktrees = []git.Worktree{{Path: "/repo-worktrees/feature-auth", Branch: "feature/auth"}}
	m.statuses = map[string]worktreeStatus{
		"/repo-worktrees/feature-auth": {dirty: false},
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	got := updated.(Model)
	if got.pending != nil {
		t.Fatalf("expected branch-from-selected prompt not to be busy yet, got pending %+v", *got.pending)
	}
	if !got.input.Focused() {
		t.Fatal("expected clean selected worktree to focus branch input")
	}
	if got.input.Value() != "feature/auth" {
		t.Fatalf("expected branch input to be prefilled from selected branch, got %q", got.input.Value())
	}

	afterView := ansi.Strip(got.View())
	if !strings.Contains(strings.ToLower(afterView), "from selected") {
		t.Fatalf("expected branch-from-selected rendered prompt, got:\n%s", afterView)
	}
	if !strings.Contains(afterView, "enter to create") {
		t.Fatalf("expected dedicated branch-from-selected flow to advertise submit, got:\n%s", afterView)
	}
	if !strings.Contains(afterView, "esc to cancel") {
		t.Fatalf("expected dedicated branch-from-selected flow to keep cancel help, got:\n%s", afterView)
	}
}

func TestBranchFromSelectedDetachedDoesNotPrefillBranch(t *testing.T) {
	m := New("/repo")
	m.worktrees = []git.Worktree{{Path: "/repo-worktrees/detached", Head: "abcdef12", Branch: ""}}
	m.statuses = map[string]worktreeStatus{
		"/repo-worktrees/detached": {dirty: false},
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	got := updated.(Model)
	if got.pending != nil {
		t.Fatalf("expected detached selected worktree not to enter busy state, got pending %+v", *got.pending)
	}
	if !got.input.Focused() {
		t.Fatal("expected detached selected worktree to focus branch input")
	}
	if got.input.Value() != "" {
		t.Fatalf("expected detached selected worktree to leave branch input empty, got %q", got.input.Value())
	}

	afterView := ansi.Strip(got.View())
	if !strings.Contains(strings.ToLower(afterView), "from selected") {
		t.Fatalf("expected branch-from-selected rendered prompt, got:\n%s", afterView)
	}
}

func TestBranchFromSelectedSubmitStartsBusyState(t *testing.T) {
	m := New("/repo")
	m.worktrees = []git.Worktree{{Path: "/repo-worktrees/feature-auth", Branch: "feature/auth"}}
	m.statuses = map[string]worktreeStatus{
		"/repo-worktrees/feature-auth": {dirty: false},
	}

	opened, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	branching := opened.(Model)
	branching.input.SetValue("feature/child")

	updated, _ := branching.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)
	if got.pending == nil {
		t.Fatalf("expected branch-from-selected submit to enter busy state, got pending %#v", got.pending)
	}
	if got.pending.kind != operationCreate {
		t.Fatalf("expected branch-from-selected submit to reuse create pending kind, got %v", got.pending.kind)
	}
	if got.pending.label != "Creating worktree..." {
		t.Fatalf("expected branch-from-selected busy label, got %q", got.pending.label)
	}
	if got.pending.target != "feature/child" {
		t.Fatalf("expected branch-from-selected pending target to match typed branch, got %q", got.pending.target)
	}
	view := got.View()
	if !strings.Contains(view, "Creating worktree...") {
		t.Fatalf("expected branch-from-selected submit to enter busy state, got:\n%s", view)
	}
	if strings.Contains(view, "Branch from selected") {
		t.Fatalf("expected branch-from-selected prompt to be replaced while busy, got:\n%s", view)
	}
}

func TestBranchFromSelectedSubmitKeepsOriginalBaseAfterReloadClamp(t *testing.T) {
	repoDir := t.TempDir()
	worktreeRoot := filepath.Join(filepath.Dir(repoDir), filepath.Base(repoDir)+"-worktrees")
	if err := os.MkdirAll(worktreeRoot, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q): %v", worktreeRoot, err)
	}
	argvFile, cwdFile := installFakeGitForUITests(t)

	m := New(repoDir)
	m.worktrees = []git.Worktree{
		{Path: repoDir, Branch: "main"},
		{Path: filepath.Join(worktreeRoot, "feature-auth"), Branch: "feature/auth"},
	}
	m.statuses = map[string]worktreeStatus{
		repoDir:                            {dirty: false},
		filepath.Join(worktreeRoot, "feature-auth"): {dirty: false},
	}
	m.cursor = 1

	opened, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	branching := opened.(Model)

	reloaded, _ := branching.Update(worktreesLoadedMsg{worktrees: []git.Worktree{{Path: repoDir, Branch: "main"}}})
	afterReload := reloaded.(Model)
	afterReload.input.SetValue("feature/child")

	_, cmd := afterReload.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected branch-from-selected submit to return create command")
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("cmd() returned %T, want tea.BatchMsg", msg)
	}
	var created worktreeCreatedMsg
	var found bool
	for _, item := range batch {
		if item == nil {
			continue
		}
		if got, ok := item().(worktreeCreatedMsg); ok {
			created = got
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("cmd() batch = %#v, want worktreeCreatedMsg", batch)
	}
	if created.err != nil {
		t.Fatalf("branch-from-selected submit err = %v", created.err)
	}

	assertGitInvocationForUITests(
		t,
		argvFile,
		cwdFile,
		[]string{"worktree", "add", git.WorktreePath(repoDir, "feature/child"), "-b", "feature/child", "feature/auth"},
		repoDir,
	)
}

func TestCreateWorktreeFromSelectedDetachedUsesHeadAsBaseRef(t *testing.T) {
	repoDir := t.TempDir()
	worktreeRoot := filepath.Join(filepath.Dir(repoDir), filepath.Base(repoDir)+"-worktrees")
	if err := os.MkdirAll(worktreeRoot, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q): %v", worktreeRoot, err)
	}
	argvFile, cwdFile := installFakeGitForUITests(t)

	m := New(repoDir)
	cmd := m.createWorktreeFromSelected(git.Worktree{Head: "abcdef12"}, "feature/child")
	msg := cmd()
	created, ok := msg.(worktreeCreatedMsg)
	if !ok {
		t.Fatalf("cmd() returned %T, want worktreeCreatedMsg", msg)
	}
	if created.err != nil {
		t.Fatalf("createWorktreeFromSelected() err = %v", created.err)
	}

	assertGitInvocationForUITests(
		t,
		argvFile,
		cwdFile,
		[]string{"worktree", "add", git.WorktreePath(repoDir, "feature/child"), "-b", "feature/child", "abcdef12"},
		repoDir,
	)
}

func TestBranchFromSelectedCreateFailureRestoresInputFlow(t *testing.T) {
	m := New("/repo")
	m.worktrees = []git.Worktree{{Path: "/repo-worktrees/feature-auth", Branch: "feature/auth"}}
	m.statuses = map[string]worktreeStatus{
		"/repo-worktrees/feature-auth": {dirty: false},
	}

	opened, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	branching := opened.(Model)
	branching.input.SetValue("feature/child")

	started, _ := branching.Update(tea.KeyMsg{Type: tea.KeyEnter})
	failed, _ := started.(Model).Update(worktreeCreatedMsg{err: errBoom{}})
	got := failed.(Model)

	if got.mode != modeCreateFromSelected {
		t.Fatalf("expected selected-base create failure to restore branch-from-selected mode, got %v", got.mode)
	}
	if got.pending != nil {
		t.Fatalf("expected selected-base create failure to clear busy state, got pending %+v", *got.pending)
	}
	if !got.input.Focused() {
		t.Fatal("expected selected-base create failure to refocus branch input")
	}
	if got.input.Value() != "feature/child" {
		t.Fatalf("expected selected-base create failure to preserve typed branch, got %q", got.input.Value())
	}
	view := got.View()
	if !strings.Contains(view, "Branch from selected") {
		t.Fatalf("expected selected-base create failure to restore branch-from-selected prompt, got:\n%s", view)
	}
	if !strings.Contains(view, "error: boom") {
		t.Fatalf("expected selected-base create failure to show error, got:\n%s", view)
	}
}

func installFakeGitForUITests(t *testing.T) (argvFile string, cwdFile string) {
	t.Helper()

	binDir := t.TempDir()
	argvFile = filepath.Join(binDir, "git-argv")
	cwdFile = filepath.Join(binDir, "git-cwd")
	gitPath := filepath.Join(binDir, "git")
	script := "#!/bin/sh\npwd >\"$FAKE_GIT_CWD_FILE\"\nprintf '%s\\n' \"$@\" >\"$FAKE_GIT_ARGV_FILE\"\nexit 0\n"
	if err := os.WriteFile(gitPath, []byte(script), 0o755); err != nil {
		t.Fatalf("os.WriteFile(%q): %v", gitPath, err)
	}

	t.Setenv("FAKE_GIT_ARGV_FILE", argvFile)
	t.Setenv("FAKE_GIT_CWD_FILE", cwdFile)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	return argvFile, cwdFile
}

func assertGitInvocationForUITests(t *testing.T, argvFile, cwdFile string, wantArgs []string, wantDir string) {
	t.Helper()

	argvBytes, err := os.ReadFile(argvFile)
	if err != nil {
		t.Fatalf("os.ReadFile(%q): %v", argvFile, err)
	}
	cwdBytes, err := os.ReadFile(cwdFile)
	if err != nil {
		t.Fatalf("os.ReadFile(%q): %v", cwdFile, err)
	}

	gotArgs := strings.Split(strings.TrimSpace(string(argvBytes)), "\n")
	if len(gotArgs) != len(wantArgs) {
		t.Fatalf("git argv length = %d, want %d (%#v)", len(gotArgs), len(wantArgs), gotArgs)
	}
	for i := range wantArgs {
		if gotArgs[i] != wantArgs[i] {
			t.Fatalf("git argv = %#v, want %#v", gotArgs, wantArgs)
		}
	}
	if gotDir := strings.TrimSpace(string(cwdBytes)); gotDir != wantDir {
		t.Fatalf("git cwd = %q, want %q", gotDir, wantDir)
	}
}

func TestBranchFromSelectedCreateIgnoresStaleVaultCopyStateFromFailedNormalCreate(t *testing.T) {
	m := New("/repo")
	m.worktrees = []git.Worktree{{Path: "/repo-worktrees/feature-auth", Branch: "feature/auth"}}
	m.statuses = map[string]worktreeStatus{
		"/repo-worktrees/feature-auth": {dirty: false},
	}
	m.mode = modeCreate
	m.input.SetValue("feature/old")
	m.pendingBranch = "feature/old"
	m.copyVaultOnCreate = true

	failedNormalCreate, _ := m.Update(worktreeCreatedMsg{err: errBoom{}})
	afterFailure := failedNormalCreate.(Model)
	if afterFailure.mode != modeCreate {
		t.Fatalf("expected failed normal create to restore create mode, got %v", afterFailure.mode)
	}

	returned, _ := afterFailure.Update(tea.KeyMsg{Type: tea.KeyEsc})
	backAtList := returned.(Model)
	if backAtList.mode != modeList {
		t.Fatalf("expected esc after failed normal create to return to list mode, got %v", backAtList.mode)
	}

	opened, _ := backAtList.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	branching := opened.(Model)
	if branching.mode != modeCreateFromSelected {
		t.Fatalf("expected b to open branch-from-selected mode, got %v", branching.mode)
	}
	branching.input.SetValue("feature/child")

	started, _ := branching.Update(tea.KeyMsg{Type: tea.KeyEnter})
	succeeded, _ := started.(Model).Update(worktreeCreatedMsg{})
	got := succeeded.(Model)

	if got.pending != nil {
		t.Fatalf("expected successful branch-from-selected create to finish without vault copy busy state, got pending %+v", *got.pending)
	}
	if got.copyVaultOnCreate {
		t.Fatal("expected branch-from-selected create to clear stale vault copy flag")
	}
	if got.pendingBranch != "" {
		t.Fatalf("expected branch-from-selected create to clear stale pending branch, got %q", got.pendingBranch)
	}
	view := got.View()
	if strings.Contains(view, "Copying vault...") {
		t.Fatalf("expected branch-from-selected create not to trigger stale vault copy, got:\n%s", view)
	}
}

func TestPullSelectedRefusesDirtyWorktree(t *testing.T) {
	m := New("/repo")
	m.worktrees = []git.Worktree{{Path: "/repo-worktrees/feature-auth", Branch: "feature/auth"}}
	m.statuses = map[string]worktreeStatus{
		"/repo-worktrees/feature-auth": {dirty: true, hasUpstream: true},
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	got := updated.(Model)
	if got.pending != nil {
		t.Fatalf("expected dirty selected worktree not to enter busy state, got pending %+v", *got.pending)
	}
	if got.err == nil || got.err.Error() != "cannot pull dirty worktree" {
		t.Fatalf("expected dirty-pull refusal error, got %#v", got.err)
	}
	if got.bannerErr == nil || got.bannerErr.Error() != "cannot pull dirty worktree" {
		t.Fatalf("expected dirty-pull refusal banner, got %#v", got.bannerErr)
	}
	if !strings.Contains(got.View(), "cannot pull dirty worktree") {
		t.Fatalf("expected dirty-pull refusal in view, got:\n%s", got.View())
	}
}

func TestPullSelectedRefusesLoadingStatus(t *testing.T) {
	m := New("/repo")
	m.worktrees = []git.Worktree{{Path: "/repo-worktrees/feature-auth", Branch: "feature/auth"}}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	got := updated.(Model)
	if got.pending != nil {
		t.Fatalf("expected loading selected worktree not to enter busy state, got pending %+v", *got.pending)
	}
	if got.err == nil || got.err.Error() != "cannot pull while worktree status is still loading" {
		t.Fatalf("expected loading-pull refusal error, got %#v", got.err)
	}
	if got.bannerErr == nil || got.bannerErr.Error() != "cannot pull while worktree status is still loading" {
		t.Fatalf("expected loading-pull refusal banner, got %#v", got.bannerErr)
	}
	if !strings.Contains(got.View(), "cannot pull while worktree status is still loading") {
		t.Fatalf("expected loading-pull refusal in view, got:\n%s", got.View())
	}
}

func TestPullSelectedPrefersProbeErrorOverLoadingStatus(t *testing.T) {
	m := New("/repo")
	m.worktrees = []git.Worktree{{Path: "/repo-worktrees/feature-auth", Branch: "feature/auth"}}

	updated, _ := m.Update(statusLoadedMsg{
		path:       "/repo-worktrees/feature-auth",
		dirtyKnown: false,
		probeErr:   errProbeFailed("/repo-worktrees/feature-auth", "git status failed"),
	})
	blocked, _ := updated.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	got := blocked.(Model)

	if got.pending != nil {
		t.Fatalf("expected failed-probe selected worktree not to enter busy state, got pending %+v", *got.pending)
	}
	if got.err == nil || got.err.Error() != "cannot pull: worktree status probe failed for /repo-worktrees/feature-auth: git status failed" {
		t.Fatalf("expected probe-error pull refusal, got %#v", got.err)
	}
	if got.bannerErr == nil || got.bannerErr.Error() != "cannot pull: worktree status probe failed for /repo-worktrees/feature-auth: git status failed" {
		t.Fatalf("expected probe-error pull banner, got %#v", got.bannerErr)
	}
	view := ansi.Strip(got.View())
	if !strings.Contains(view, "cannot pull: worktree status probe failed for /repo-worktrees/feature-auth: git status failed") {
		t.Fatalf("expected probe-error pull refusal in view, got:\n%s", view)
	}
	if !strings.Contains(view, "loading") {
		t.Fatalf("expected failed probe to remain untrusted/loading in table, got:\n%s", view)
	}
}

func TestBranchFromSelectedPrefersProbeErrorOverLoadingStatus(t *testing.T) {
	m := New("/repo")
	m.worktrees = []git.Worktree{{Path: "/repo-worktrees/feature-auth", Branch: "feature/auth"}}

	updated, _ := m.Update(statusLoadedMsg{
		path:       "/repo-worktrees/feature-auth",
		dirtyKnown: false,
		probeErr:   errProbeFailed("/repo-worktrees/feature-auth", "git status failed"),
	})
	blocked, _ := updated.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	got := blocked.(Model)

	if got.mode != modeList {
		t.Fatalf("expected failed-probe selected worktree to stay in list mode, got %v", got.mode)
	}
	if got.pending != nil {
		t.Fatalf("expected failed-probe selected worktree not to enter busy state, got pending %+v", *got.pending)
	}
	view := ansi.Strip(got.View())
	if !strings.Contains(view, "cannot branch: worktree status probe failed for /repo-worktrees/feature-auth: git status failed") {
		t.Fatalf("expected probe-error branch refusal, got:\n%s", view)
	}
	if !strings.Contains(view, "loading") {
		t.Fatalf("expected failed probe to remain untrusted/loading in table, got:\n%s", view)
	}
}

func TestPullSelectedRefusesDetachedWorktree(t *testing.T) {
	m := New("/repo")
	m.worktrees = []git.Worktree{{Path: "/repo-worktrees/detached", Head: "abcdef12", Branch: ""}}
	m.statuses = map[string]worktreeStatus{
		"/repo-worktrees/detached": {dirty: false, ahead: 0, behind: 0, hasUpstream: true},
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	got := updated.(Model)
	if got.pending != nil {
		t.Fatalf("expected detached selected worktree not to enter busy state, got pending %+v", *got.pending)
	}
	if got.err == nil || got.err.Error() != "cannot pull detached worktree" {
		t.Fatalf("expected detached-pull refusal error, got %#v", got.err)
	}
	if got.bannerErr == nil || got.bannerErr.Error() != "cannot pull detached worktree" {
		t.Fatalf("expected detached-pull refusal banner, got %#v", got.bannerErr)
	}
	if !strings.Contains(got.View(), "cannot pull detached worktree") {
		t.Fatalf("expected detached-pull refusal in view, got:\n%s", got.View())
	}
}

func TestPullSelectedRefusesWorktreeWithoutUpstream(t *testing.T) {
	m := New("/repo")
	m.worktrees = []git.Worktree{{Path: "/repo-worktrees/feature-auth", Branch: "feature/auth"}}
	m.statuses = map[string]worktreeStatus{
		"/repo-worktrees/feature-auth": {dirty: false, hasUpstream: false},
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	got := updated.(Model)
	if got.pending != nil {
		t.Fatalf("expected no-upstream selected worktree not to enter busy state, got pending %+v", *got.pending)
	}
	if got.err == nil || got.err.Error() != "cannot pull worktree without upstream" {
		t.Fatalf("expected no-upstream refusal error, got %#v", got.err)
	}
	if got.bannerErr == nil || got.bannerErr.Error() != "cannot pull worktree without upstream" {
		t.Fatalf("expected no-upstream refusal banner, got %#v", got.bannerErr)
	}
	if !strings.Contains(got.View(), "cannot pull worktree without upstream") {
		t.Fatalf("expected no-upstream refusal in view, got:\n%s", got.View())
	}
}

func TestPullSelectedStartsBusyState(t *testing.T) {
	m := New("/repo")
	m.worktrees = []git.Worktree{{Path: "/repo-worktrees/feature-auth", Branch: "feature/auth"}}
	m.statuses = map[string]worktreeStatus{
		"/repo-worktrees/feature-auth": {dirty: false, hasUpstream: true},
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	got := updated.(Model)
	if got.pending == nil {
		t.Fatalf("expected pull to enter busy state, got pending %#v", got.pending)
	}
	if got.pending.label != "Pulling worktree..." {
		t.Fatalf("expected pull busy label, got %q", got.pending.label)
	}
	if got.pending.target != "/repo-worktrees/feature-auth" {
		t.Fatalf("expected pull target to match selected worktree, got %q", got.pending.target)
	}
	view := got.View()
	if !strings.Contains(view, "Pulling worktree...") {
		t.Fatalf("expected busy pull status in view, got:\n%s", view)
	}
}

func TestViewShowsWorktreeTableHeader(t *testing.T) {
	m := New("/repo")
	m.width = 120
	m.worktrees = []git.Worktree{{Path: "/repo", Branch: "main"}}
	m.statuses = map[string]worktreeStatus{
		"/repo": {
			dirty:       false,
			ahead:       0,
			behind:      0,
			hasUpstream: true,
		},
	}

	view := m.View()
	plainView := ansi.Strip(view)
	lines := strings.Split(plainView, "\n")
	headerIndex := -1
	rowIndex := -1
	for i, line := range lines {
		if strings.Contains(line, "Branch") &&
			strings.Contains(line, "State") &&
			strings.Contains(line, "Ahead/Behind") &&
			strings.Contains(line, "Path") {
			branchIndex := strings.Index(line, "Branch")
			stateIndex := strings.Index(line, "State")
			syncIndex := strings.Index(line, "Ahead/Behind")
			pathIndex := strings.Index(line, "Path")
			if branchIndex < stateIndex && stateIndex < syncIndex && syncIndex < pathIndex {
				headerIndex = i
			}
		}
		if strings.Contains(line, "main") && strings.Contains(line, "/repo") {
			rowIndex = i
		}
	}
	if headerIndex == -1 {
		t.Fatalf("expected dedicated header row with Branch, State, Ahead/Behind, Path in order, got:\n%s", plainView)
	}
	if rowIndex == -1 {
		t.Fatalf("expected worktree data row containing branch and path, got:\n%s", plainView)
	}
	if headerIndex >= rowIndex {
		t.Fatalf("expected header row to appear before worktree data row, got:\n%s", plainView)
	}

	headerLine := lines[headerIndex]
	rowLine := lines[rowIndex]
	assertColumnStartsMatch(t, headerLine, rowLine, map[string]string{
		"Branch":       "main",
		"State":        "clean",
		"Ahead/Behind": "↑0 ↓0",
		"Path":         "/repo",
	})
}

func TestViewShowsExplicitStateAndSyncColumns(t *testing.T) {
	m := New("/repo")
	m.width = 120
	m.worktrees = []git.Worktree{{Path: "/repo-worktrees/feature-auth", Branch: "feature/auth"}}
	m.statuses = map[string]worktreeStatus{
		"/repo-worktrees/feature-auth": {
			dirty:       true,
			ahead:       3,
			behind:      0,
			hasUpstream: true,
		},
	}

	view := m.View()
	var row string
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "feature/auth") && strings.Contains(line, "/repo-worktrees/feature-auth") {
			row = line
			break
		}
	}
	if row == "" {
		t.Fatalf("expected worktree row containing branch and path, got:\n%s", view)
	}
	plainRow := ansi.Strip(row)
	if !regexp.MustCompile(`feature/auth\s+dirty\s+↑3 ↓0\s+/repo-worktrees/feature-auth`).MatchString(strings.ToLower(plainRow)) {
		t.Fatalf("expected worktree row to include explicit dirty state and sync column before path, got row:\n%s\nfull view:\n%s", row, view)
	}
}

func TestViewKeepsAllColumnsVisibleOnWideTerminal(t *testing.T) {
	m := New("/repo")
	m.width = 120
	m.worktrees = []git.Worktree{{Path: "/repo-worktrees/feature-auth", Branch: "feature/auth"}}
	m.statuses = map[string]worktreeStatus{
		"/repo-worktrees/feature-auth": {
			dirty:       false,
			ahead:       2,
			behind:      1,
			hasUpstream: true,
		},
	}

	view := m.View()
	plainView := ansi.Strip(view)
	lines := strings.Split(plainView, "\n")
	headerLine, rowLine := findHeaderAndRow(lines, "feature/auth", "/repo-worktrees/feature-auth")
	if headerLine == "" {
		t.Fatalf("expected table header on wide terminal, got:\n%s", plainView)
	}
	if rowLine == "" {
		t.Fatalf("expected worktree row on wide terminal, got:\n%s", plainView)
	}

	assertColumnStartsMatch(t, headerLine, rowLine, map[string]string{
		"Branch":       "feature/auth",
		"State":        "clean",
		"Ahead/Behind": "↑2 ↓1",
		"Path":         "/repo-worktrees/feature-auth",
	})

	if got := lipgloss.Width(headerLine); got != m.width {
		t.Fatalf("expected wide terminal header to use full width %d, got %d\nheader: %q\nfull view:\n%s", m.width, got, headerLine, plainView)
	}
	if got := lipgloss.Width(rowLine); got != m.width {
		t.Fatalf("expected wide terminal row to use full width %d, got %d\nrow: %q\nfull view:\n%s", m.width, got, rowLine, plainView)
	}

	if !strings.Contains(rowLine, "/repo-worktrees/feature-auth") {
		t.Fatalf("expected full path on wide terminal, got row:\n%s\nfull view:\n%s", rowLine, plainView)
	}
	if strings.Contains(rowLine, "...") {
		t.Fatalf("expected wide terminal row to avoid path truncation, got row:\n%s\nfull view:\n%s", rowLine, plainView)
	}
	if !strings.Contains(strings.ToLower(rowLine), "clean") {
		t.Fatalf("expected clean state column on wide terminal, got row:\n%s\nfull view:\n%s", rowLine, plainView)
	}
}

func TestViewKeepsSelectedRowAlignedWithHeader(t *testing.T) {
	m := New("/repo")
	m.width = 120
	m.worktrees = []git.Worktree{
		{Path: "/repo", Branch: "main"},
		{Path: "/repo-worktrees/feature-auth", Branch: "feature/auth"},
	}
	m.statuses = map[string]worktreeStatus{
		"/repo": {
			dirty:       false,
			ahead:       0,
			behind:      0,
			hasUpstream: true,
		},
		"/repo-worktrees/feature-auth": {
			dirty:       true,
			ahead:       2,
			behind:      1,
			hasUpstream: true,
		},
	}
	m.cursor = 1

	plainView := ansi.Strip(m.View())
	lines := strings.Split(plainView, "\n")
	headerLine, rowLine := findHeaderAndRow(lines, "feature/auth", "/repo-worktrees/feature-auth")
	if headerLine == "" {
		t.Fatalf("expected table header with selected row present, got:\n%s", plainView)
	}
	if rowLine == "" {
		t.Fatalf("expected selected second-row worktree, got:\n%s", plainView)
	}
	if !strings.HasPrefix(rowLine, "> ") {
		t.Fatalf("expected second row to be selected, got row:\n%s\nfull view:\n%s", rowLine, plainView)
	}

	assertColumnStartsMatch(t, headerLine, rowLine, map[string]string{
		"Branch":       "feature/auth",
		"State":        "dirty",
		"Ahead/Behind": "↑2 ↓1",
		"Path":         "/repo-worktrees/feature-auth",
	})
}

func TestTableColumnWidthsStayWithinNarrowTerminalBudget(t *testing.T) {
	total := 38

	branchWidth, stateWidth, syncWidth, pathWidth := tableColumnWidths(total)
	used := tablePrefixWidth + 3 + branchWidth + stateWidth + syncWidth + pathWidth

	if used > total {
		t.Fatalf("expected table width to fit terminal width %d, got %d (branch=%d state=%d sync=%d path=%d)", total, used, branchWidth, stateWidth, syncWidth, pathWidth)
	}
	if branchWidth <= 0 || stateWidth <= 0 || syncWidth <= 0 || pathWidth <= 0 {
		t.Fatalf("expected all narrow-width columns to remain nonzero, got branch=%d state=%d sync=%d path=%d", branchWidth, stateWidth, syncWidth, pathWidth)
	}
}

func TestViewClipsRenderedTableCellsOnNarrowTerminal(t *testing.T) {
	m := New("/repo")
	m.width = 38
	m.worktrees = []git.Worktree{{
		Path:   "/repo-worktrees/feature-auth-with-a-very-long-path",
		Branch: "feature/auth-with-a-very-long-branch",
	}}
	m.statuses = map[string]worktreeStatus{
		"/repo-worktrees/feature-auth-with-a-very-long-path": {
			dirty:       true,
			ahead:       12,
			behind:      3,
			hasUpstream: true,
		},
	}

	plainView := ansi.Strip(m.View())
	lines := strings.Split(plainView, "\n")
	var headerLine string
	var rowLine string
	for _, line := range lines {
		if headerLine == "" && strings.Contains(line, "Branch") && strings.Contains(line, "State") && strings.Contains(line, "Ahead/Behind") {
			headerLine = line
		}
		if rowLine == "" && strings.Contains(line, "dirty") && strings.Contains(line, "↑12 ↓3") {
			rowLine = line
		}
	}
	if headerLine == "" {
		t.Fatalf("expected narrow table header, got:\n%s", plainView)
	}
	if rowLine == "" {
		t.Fatalf("expected narrow worktree row, got:\n%s", plainView)
	}

	if got := lipgloss.Width(headerLine); got != m.width {
		t.Fatalf("expected narrow header width %d, got %d\nheader: %q\nfull view:\n%s", m.width, got, headerLine, plainView)
	}
	if got := lipgloss.Width(rowLine); got != m.width {
		t.Fatalf("expected narrow row width %d, got %d\nrow: %q\nfull view:\n%s", m.width, got, rowLine, plainView)
	}
	if strings.Contains(headerLine, "Path") {
		t.Fatalf("expected narrow header path cell to be clipped, got header:\n%q", headerLine)
	}
	if strings.Contains(rowLine, "feature/auth-with-a-very-long-branch") {
		t.Fatalf("expected narrow row branch cell to be clipped, got row:\n%q", rowLine)
	}
	if strings.Contains(rowLine, "/repo-worktrees/feature-auth-with-a-very-long-path") {
		t.Fatalf("expected narrow row path cell to be clipped, got row:\n%q", rowLine)
	}
}

func TestViewKeepsEndClippedPathOnNormalWidthTerminal(t *testing.T) {
	m := New("/repo")
	m.width = 90
	m.worktrees = []git.Worktree{{
		Path:   "/home/ali/code/myapp-worktrees/feature-auth",
		Branch: "feature/auth",
	}}
	m.statuses = map[string]worktreeStatus{
		"/home/ali/code/myapp-worktrees/feature-auth": {
			dirty:       false,
			hasUpstream: false,
		},
	}

	plainView := ansi.Strip(m.View())
	lines := strings.Split(plainView, "\n")
	var rowLine string
	for _, line := range lines {
		if strings.Contains(line, "feature/auth") && strings.Contains(line, "clean") {
			rowLine = line
			break
		}
	}
	if rowLine == "" {
		t.Fatalf("expected normal-width worktree row, got:\n%s", plainView)
	}

	if !strings.Contains(rowLine, "/home/ali/code/") {
		t.Fatalf("expected normal-width path cell to keep its leading path segment, got row:\n%q\nfull view:\n%s", rowLine, plainView)
	}
	if strings.Contains(rowLine, ".../myapp-worktrees/feature-auth") {
		t.Fatalf("expected normal-width path cell to avoid start truncation, got row:\n%q\nfull view:\n%s", rowLine, plainView)
	}
	if strings.Contains(rowLine, "/home/ali/code/myapp-worktrees/feature-auth") {
		t.Fatalf("expected normal-width path cell to remain clipped to the column, got row:\n%q\nfull view:\n%s", rowLine, plainView)
	}
	if got := lipgloss.Width(rowLine); got != m.width {
		t.Fatalf("expected normal-width row width %d, got %d\nrow: %q\nfull view:\n%s", m.width, got, rowLine, plainView)
	}
}

func TestViewStartTruncatesPathOnNarrowTerminal(t *testing.T) {
	m := New("/repo")
	m.width = 80
	m.worktrees = []git.Worktree{{
		Path:   "/home/ali/code/myapp-worktrees/task-four-target",
		Branch: "topic/branch-prefix-should-stay-visible-but-tail-clips",
	}}
	m.statuses = map[string]worktreeStatus{
		"/home/ali/code/myapp-worktrees/task-four-target": {
			dirty:       false,
			hasUpstream: false,
		},
	}

	plainView := ansi.Strip(m.View())
	lines := strings.Split(plainView, "\n")
	var rowLine string
	for _, line := range lines {
		if strings.Contains(line, "topic/branch-prefix") && strings.Contains(line, "clean") {
			rowLine = line
			break
		}
	}
	if rowLine == "" {
		t.Fatalf("expected narrow worktree row, got:\n%s", plainView)
	}

	if !strings.Contains(rowLine, "...") || !strings.Contains(rowLine, "task-four-target") {
		t.Fatalf("expected narrow path cell to start-truncate and retain the path suffix, got row:\n%q\nfull view:\n%s", rowLine, plainView)
	}
	if strings.Contains(rowLine, "/home/ali/code/myapp-worktrees/task-four-target") {
		t.Fatalf("expected full path to be truncated on narrow terminal, got row:\n%q\nfull view:\n%s", rowLine, plainView)
	}
	if !strings.Contains(rowLine, "topic/branch-prefix") {
		t.Fatalf("expected overlong branch column to keep its leading segment, got row:\n%q\nfull view:\n%s", rowLine, plainView)
	}
	if strings.Contains(rowLine, "...tail-clips") {
		t.Fatalf("expected overlong branch column to keep end clipping instead of start truncation, got row:\n%q\nfull view:\n%s", rowLine, plainView)
	}
	if got := lipgloss.Width(rowLine); got != m.width {
		t.Fatalf("expected narrow row width %d, got %d\nrow: %q\nfull view:\n%s", m.width, got, rowLine, plainView)
	}
}

func TestViewKeepsEndClippedPathAtTask4StartTruncationThreshold(t *testing.T) {
	m := New("/repo")
	m.width = 84
	m.worktrees = []git.Worktree{{
		Path:   "/home/ali/code/myapp-worktrees/task-four-target",
		Branch: "topic/branch-prefix-should-stay-visible-but-tail-clips",
	}}
	m.statuses = map[string]worktreeStatus{
		"/home/ali/code/myapp-worktrees/task-four-target": {
			dirty:       false,
			hasUpstream: false,
		},
	}

	_, _, _, pathWidth := tableColumnWidths(m.width)
	if pathWidth != 28 {
		t.Fatalf("expected threshold test to render with path width 28, got %d", pathWidth)
	}

	plainView := ansi.Strip(m.View())
	lines := strings.Split(plainView, "\n")
	var rowLine string
	for _, line := range lines {
		if strings.Contains(line, "topic/branch-prefix") && strings.Contains(line, "clean") {
			rowLine = line
			break
		}
	}
	if rowLine == "" {
		t.Fatalf("expected threshold worktree row, got:\n%s", plainView)
	}

	if !strings.Contains(rowLine, "/home/ali/code/") {
		t.Fatalf("expected path at width 28 to keep its leading segment, got row:\n%q\nfull view:\n%s", rowLine, plainView)
	}
	if strings.Contains(rowLine, ".../myapp-worktrees/task-four-target") {
		t.Fatalf("expected path at width 28 to keep normal end clipping, got row:\n%q\nfull view:\n%s", rowLine, plainView)
	}
	if strings.Contains(rowLine, "/home/ali/code/myapp-worktrees/task-four-target") {
		t.Fatalf("expected path at width 28 to remain clipped to the column, got row:\n%q\nfull view:\n%s", rowLine, plainView)
	}
	if got := lipgloss.Width(rowLine); got != m.width {
		t.Fatalf("expected threshold row width %d, got %d\nrow: %q\nfull view:\n%s", m.width, got, rowLine, plainView)
	}
}

func TestViewStartTruncatesPathBelowTask4Threshold(t *testing.T) {
	m := New("/repo")
	m.width = 83
	m.worktrees = []git.Worktree{{
		Path:   "/home/ali/code/myapp-worktrees/task-four-target",
		Branch: "topic/branch-prefix-should-stay-visible-but-tail-clips",
	}}
	m.statuses = map[string]worktreeStatus{
		"/home/ali/code/myapp-worktrees/task-four-target": {
			dirty:       false,
			hasUpstream: false,
		},
	}

	_, _, _, pathWidth := tableColumnWidths(m.width)
	if pathWidth != 27 {
		t.Fatalf("expected threshold test to render with path width 27, got %d", pathWidth)
	}

	plainView := ansi.Strip(m.View())
	lines := strings.Split(plainView, "\n")
	var rowLine string
	for _, line := range lines {
		if strings.Contains(line, "topic/branch-prefix") && strings.Contains(line, "clean") {
			rowLine = line
			break
		}
	}
	if rowLine == "" {
		t.Fatalf("expected threshold worktree row, got:\n%s", plainView)
	}

	if !strings.Contains(rowLine, "...") || !strings.Contains(rowLine, "task-four-target") {
		t.Fatalf("expected path at width 27 to start-truncate and keep the suffix, got row:\n%q\nfull view:\n%s", rowLine, plainView)
	}
	if strings.Contains(rowLine, "/home/ali/code/myapp-worktrees/task-four-target") {
		t.Fatalf("expected full path at width 27 to be truncated, got row:\n%q\nfull view:\n%s", rowLine, plainView)
	}
	if got := lipgloss.Width(rowLine); got != m.width {
		t.Fatalf("expected threshold row width %d, got %d\nrow: %q\nfull view:\n%s", m.width, got, rowLine, plainView)
	}
}

func TestViewShowsLoadingStateTextWhenStatusUnknown(t *testing.T) {
	m := New("/repo")
	m.width = 120
	m.worktrees = []git.Worktree{{Path: "/repo-worktrees/feature-auth", Branch: "feature/auth"}}

	plainView := ansi.Strip(m.View())
	lines := strings.Split(plainView, "\n")
	headerLine, rowLine := findHeaderAndRow(lines, "feature/auth", "/repo-worktrees/feature-auth")
	if headerLine == "" {
		t.Fatalf("expected table header when rendering unknown status, got:\n%s", plainView)
	}
	if rowLine == "" {
		t.Fatalf("expected worktree row when rendering unknown status, got:\n%s", plainView)
	}

	assertColumnStartsMatch(t, headerLine, rowLine, map[string]string{
		"Branch":       "feature/auth",
		"State":        "loading",
		"Ahead/Behind": "-",
		"Path":         "/repo-worktrees/feature-auth",
	})

	if !regexp.MustCompile(`feature/auth\s+loading\s+-\s+/repo-worktrees/feature-auth`).MatchString(rowLine) {
		t.Fatalf("expected unknown status row to show loading state and dash sync placeholder, got row:\n%s\nfull view:\n%s", rowLine, plainView)
	}
}

func TestViewShowsDashWhenNoUpstreamExists(t *testing.T) {
	m := New("/repo")
	m.width = 120
	m.worktrees = []git.Worktree{{Path: "/repo-worktrees/feature-auth", Branch: "feature/auth"}}
	m.statuses = map[string]worktreeStatus{
		"/repo-worktrees/feature-auth": {
			dirty:       false,
			hasUpstream: false,
		},
	}

	plainView := ansi.Strip(m.View())
	lines := strings.Split(plainView, "\n")
	headerLine, rowLine := findHeaderAndRow(lines, "feature/auth", "/repo-worktrees/feature-auth")
	if headerLine == "" {
		t.Fatalf("expected table header when rendering missing upstream, got:\n%s", plainView)
	}
	if rowLine == "" {
		t.Fatalf("expected worktree row when rendering missing upstream, got:\n%s", plainView)
	}

	assertColumnStartsMatch(t, headerLine, rowLine, map[string]string{
		"Branch":       "feature/auth",
		"State":        "clean",
		"Ahead/Behind": "-",
		"Path":         "/repo-worktrees/feature-auth",
	})

	if !regexp.MustCompile(`feature/auth\s+clean\s+-\s+/repo-worktrees/feature-auth`).MatchString(rowLine) {
		t.Fatalf("expected no-upstream row to show clean state and dash sync placeholder, got row:\n%s\nfull view:\n%s", rowLine, plainView)
	}
}

func TestViewUsesCompactFallbackBeforeColumnsCollapse(t *testing.T) {
	m := New("/repo")
	m.width = 11
	m.worktrees = []git.Worktree{{
		Path:   "/repo-worktrees/feature-auth-with-a-very-long-path",
		Branch: "feature/auth-with-a-very-long-branch",
	}}
	m.statuses = map[string]worktreeStatus{
		"/repo-worktrees/feature-auth-with-a-very-long-path": {
			dirty:       true,
			ahead:       12,
			behind:      3,
			hasUpstream: true,
		},
	}

	plainView := ansi.Strip(m.View())
	if strings.Contains(plainView, "Branch") || strings.Contains(plainView, "State") || strings.Contains(plainView, "Ahead/Behind") {
		t.Fatalf("expected width-11 view to use compact fallback instead of collapsed table header, got:\n%s", plainView)
	}

	lines := strings.Split(plainView, "\n")
	var rowLine string
	for _, line := range lines {
		if rowLine == "" && strings.HasPrefix(line, "> ") {
			rowLine = line
		}
	}
	if rowLine == "" {
		t.Fatalf("expected width-11 compact worktree row, got:\n%s", plainView)
	}

	if got := lipgloss.Width(rowLine); got > m.width {
		t.Fatalf("expected width-11 compact row width to stay within %d, got %d\nrow: %q\nfull view:\n%s", m.width, got, rowLine, plainView)
	}
	if !strings.Contains(rowLine, "feature/") {
		t.Fatalf("expected width-11 compact row to retain branch content, got row:\n%q\nfull view:\n%s", rowLine, plainView)
	}
}

func TestViewUsesCompactFallbackAtFirstUnreadableBoundary(t *testing.T) {
	m := New("/repo")
	m.width = 22
	m.worktrees = []git.Worktree{{
		Path:   "/repo-worktrees/feature-auth-with-a-very-long-path",
		Branch: "feature/auth-with-a-very-long-branch",
	}}
	m.statuses = map[string]worktreeStatus{
		"/repo-worktrees/feature-auth-with-a-very-long-path": {
			dirty:       true,
			ahead:       12,
			behind:      3,
			hasUpstream: true,
		},
	}

	branchWidth, stateWidth, syncWidth, pathWidth := tableColumnWidths(m.width)
	if branchWidth != tableBranchTight || stateWidth != tableStateTight-1 || syncWidth != tableSyncTight || pathWidth != tablePathMin {
		t.Fatalf("expected width-22 boundary widths branch=%d state=%d sync=%d path=%d, got branch=%d state=%d sync=%d path=%d",
			tableBranchTight, tableStateTight-1, tableSyncTight, tablePathMin,
			branchWidth, stateWidth, syncWidth, pathWidth)
	}

	plainView := ansi.Strip(m.View())
	if strings.Contains(plainView, "Branch") || strings.Contains(plainView, "State") || strings.Contains(plainView, "Ahead/Behind") {
		t.Fatalf("expected width-22 view to use compact fallback instead of unreadable table header, got:\n%s", plainView)
	}

	lines := strings.Split(plainView, "\n")
	var rowLine string
	for _, line := range lines {
		if rowLine == "" && strings.HasPrefix(line, "> ") {
			rowLine = line
		}
	}
	if rowLine == "" {
		t.Fatalf("expected width-22 compact worktree row, got:\n%s", plainView)
	}
	if got := lipgloss.Width(rowLine); got > m.width {
		t.Fatalf("expected width-22 compact row width to stay within %d, got %d\nrow: %q\nfull view:\n%s", m.width, got, rowLine, plainView)
	}
	if !strings.Contains(rowLine, "feature/auth") {
		t.Fatalf("expected width-22 compact row to retain meaningful branch content, got row:\n%q\nfull view:\n%s", rowLine, plainView)
	}
}

func TestViewStaysWithinTinyTerminalBudget(t *testing.T) {
	m := New("/repo")
	m.width = 7
	m.worktrees = []git.Worktree{{
		Path:   "/repo-worktrees/feature-auth-with-a-very-long-path",
		Branch: "feature/auth-with-a-very-long-branch",
	}}
	m.statuses = map[string]worktreeStatus{
		"/repo-worktrees/feature-auth-with-a-very-long-path": {
			dirty:       true,
			ahead:       12,
			behind:      3,
			hasUpstream: true,
		},
	}

	plainView := ansi.Strip(m.View())
	lines := strings.Split(plainView, "\n")
	var rowLine string
	for _, line := range lines {
		if rowLine == "" && strings.HasPrefix(line, ">") {
			rowLine = line
		}
	}
	if rowLine == "" {
		t.Fatalf("expected tiny-width worktree row, got:\n%s", plainView)
	}

	if got := lipgloss.Width(rowLine); got > m.width {
		t.Fatalf("expected tiny-width row to stay within %d, got %d\nrow: %q\nfull view:\n%s", m.width, got, rowLine, plainView)
	}
}

func TestViewUsesCompactFallbackAtWidthEight(t *testing.T) {
	m := New("/repo")
	m.width = 8
	m.worktrees = []git.Worktree{{
		Path:   "/repo-worktrees/feature-auth-with-a-very-long-path",
		Branch: "feature/auth-with-a-very-long-branch",
	}}
	m.statuses = map[string]worktreeStatus{
		"/repo-worktrees/feature-auth-with-a-very-long-path": {
			dirty:       true,
			ahead:       12,
			behind:      3,
			hasUpstream: true,
		},
	}

	plainView := ansi.Strip(m.View())
	if strings.Contains(plainView, "Branch") || strings.Contains(plainView, "State") || strings.Contains(plainView, "Ahead/Behind") {
		t.Fatalf("expected width-8 view to use compact fallback instead of table header, got:\n%s", plainView)
	}

	lines := strings.Split(plainView, "\n")
	var rowLine string
	for _, line := range lines {
		if rowLine == "" && strings.HasPrefix(line, ">") {
			rowLine = line
		}
	}
	if rowLine == "" {
		t.Fatalf("expected width-8 compact worktree row, got:\n%s", plainView)
	}
	if got := lipgloss.Width(rowLine); got > m.width {
		t.Fatalf("expected width-8 row to stay within %d, got %d\nrow: %q\nfull view:\n%s", m.width, got, rowLine, plainView)
	}
	if !strings.Contains(rowLine, "fea") {
		t.Fatalf("expected width-8 compact row to retain meaningful branch content, got row:\n%q\nfull view:\n%s", rowLine, plainView)
	}
}

func TestCompactFallbackKeepsSelectedRowDistinctWithoutShiftingText(t *testing.T) {
	m := New("/repo")
	m.width = 8
	m.worktrees = []git.Worktree{
		{Path: "/repo", Branch: "main"},
		{Path: "/repo-worktrees/feature-auth", Branch: "feature/auth"},
	}
	m.statuses = map[string]worktreeStatus{
		"/repo": {
			dirty:       false,
			ahead:       0,
			behind:      0,
			hasUpstream: true,
		},
		"/repo-worktrees/feature-auth": {
			dirty:       true,
			ahead:       2,
			behind:      1,
			hasUpstream: true,
		},
	}
	m.cursor = 1

	plainView := ansi.Strip(m.View())
	lines := strings.Split(plainView, "\n")
	var firstRow string
	var selectedRow string
	for _, line := range lines {
		if firstRow == "" && strings.HasPrefix(line, "  ") {
			firstRow = line
		}
		if selectedRow == "" && strings.HasPrefix(line, "> ") {
			selectedRow = line
		}
	}

	if firstRow == "" || selectedRow == "" {
		t.Fatalf("expected compact fallback to render aligned selected and unselected rows, got:\n%s", plainView)
	}
	if got := lipgloss.Width(firstRow); got > m.width {
		t.Fatalf("expected unselected compact row width to stay within %d, got %d\nrow: %q\nfull view:\n%s", m.width, got, firstRow, plainView)
	}
	if got := lipgloss.Width(selectedRow); got > m.width {
		t.Fatalf("expected selected compact row width to stay within %d, got %d\nrow: %q\nfull view:\n%s", m.width, got, selectedRow, plainView)
	}
	if !strings.HasPrefix(firstRow, "  ") {
		t.Fatalf("expected compact fallback to reserve the same prefix width for unselected rows, got row:\n%q", firstRow)
	}
	if !strings.HasPrefix(selectedRow, "> ") {
		t.Fatalf("expected compact fallback selected row marker to remain visible, got row:\n%q", selectedRow)
	}
}

func findHeaderAndRow(lines []string, branch, path string) (string, string) {
	var headerLine string
	var rowLine string

	for _, line := range lines {
		if headerLine == "" && strings.Contains(line, "Branch") &&
			strings.Contains(line, "State") &&
			strings.Contains(line, "Ahead/Behind") &&
			strings.Contains(line, "Path") {
			headerLine = line
		}
		if rowLine == "" && strings.Contains(line, branch) && strings.Contains(line, path) {
			rowLine = line
		}
	}

	return headerLine, rowLine
}

func findLineWithPrefix(lines []string, prefix string) string {
	for _, line := range lines {
		if strings.HasPrefix(line, prefix) {
			return line
		}
	}

	return ""
}

func assertColumnStartsMatch(t *testing.T, headerLine, rowLine string, columns map[string]string) {
	t.Helper()

	for header, value := range columns {
		headerIndex := strings.Index(headerLine, header)
		if headerIndex == -1 {
			t.Fatalf("expected header %q in line:\n%s", header, headerLine)
		}

		valueIndex := strings.Index(rowLine, value)
		if valueIndex == -1 {
			t.Fatalf("expected value %q in row:\n%s", value, rowLine)
		}

		headerColumn := lipgloss.Width(headerLine[:headerIndex])
		valueColumn := lipgloss.Width(rowLine[:valueIndex])
		if headerColumn != valueColumn {
			t.Fatalf("expected header %q and value %q to start in the same column, got %d vs %d\nheader: %s\nrow:    %s", header, value, headerColumn, valueColumn, headerLine, rowLine)
		}
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
