package git

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParsePorcelain(t *testing.T) {
	input := `worktree /home/ali/code/jurii
HEAD a1b2c3d4e5f6a7b8c9d0
branch refs/heads/main

worktree /home/ali/code/jurii-kyc
HEAD e5f6a7b8c9d0a1b2c3d4
branch refs/heads/feature/kyc-flow

worktree /home/ali/code/jurii-old
HEAD 1122334455667788990a
detached
`

	got := parsePorcelain(input)

	if len(got) != 3 {
		t.Fatalf("expected 3 worktrees, got %d", len(got))
	}

	if got[0].Path != "/home/ali/code/jurii" {
		t.Errorf("worktree[0].Path = %q, want /home/ali/code/jurii", got[0].Path)
	}
	if got[0].Branch != "main" {
		t.Errorf("worktree[0].Branch = %q, want main", got[0].Branch)
	}

	if got[1].Branch != "feature/kyc-flow" {
		t.Errorf("worktree[1].Branch = %q, want feature/kyc-flow", got[1].Branch)
	}

	// Detached HEAD worktree should have an empty branch.
	if got[2].Branch != "" {
		t.Errorf("worktree[2].Branch = %q, want empty (detached)", got[2].Branch)
	}
	if got[2].DisplayBranch() != "(detached)" {
		t.Errorf("worktree[2].DisplayBranch() = %q, want (detached)", got[2].DisplayBranch())
	}
}

func TestParsePorcelainEmpty(t *testing.T) {
	got := parsePorcelain("")
	if len(got) != 0 {
		t.Errorf("expected 0 worktrees for empty input, got %d", len(got))
	}
}

func TestParsePorcelainNoTrailingBlankLine(t *testing.T) {
	// Real git output doesn't always end with a trailing blank line —
	// make sure the last entry still gets flushed.
	input := `worktree /home/ali/code/solo
HEAD abcd1234
branch refs/heads/main`

	got := parsePorcelain(input)
	if len(got) != 1 {
		t.Fatalf("expected 1 worktree, got %d", len(got))
	}
	if got[0].Path != "/home/ali/code/solo" {
		t.Errorf("Path = %q, want /home/ali/code/solo", got[0].Path)
	}
}

func TestParsePorcelainFlags(t *testing.T) {
	input := `worktree /home/ali/code/bare-repo
HEAD 0000000000000000000000000000000000000000
bare

worktree /home/ali/code/locked-wt
HEAD aabbccdd
branch refs/heads/feature/locked
locked worktree locked by user

worktree /home/ali/code/prunable-wt
HEAD 11223344
branch refs/heads/old-branch
prunable gitdir file points to non-existent location
`

	got := parsePorcelain(input)
	if len(got) != 3 {
		t.Fatalf("expected 3 worktrees, got %d", len(got))
	}
	if !got[0].Bare {
		t.Errorf("worktree[0].Bare = false, want true")
	}
	if !got[1].Locked {
		t.Errorf("worktree[1].Locked = false, want true")
	}
	if !got[2].Prunable {
		t.Errorf("worktree[2].Prunable = false, want true")
	}
}

func TestWorktreePath(t *testing.T) {
	cases := []struct {
		repoDir string
		branch  string
		want    string
	}{
		{"/home/ali/code/myapp", "main", "/home/ali/code/myapp-worktrees/main"},
		{"/home/ali/code/myapp", "feature/kyc", "/home/ali/code/myapp-worktrees/feature-kyc"},
		{"/home/ali/code/myapp", "feat/a/b", "/home/ali/code/myapp-worktrees/feat-a-b"},
	}
	for _, tc := range cases {
		got := WorktreePath(tc.repoDir, tc.branch)
		if got != tc.want {
			t.Errorf("WorktreePath(%q, %q) = %q, want %q", tc.repoDir, tc.branch, got, tc.want)
		}
	}
}

func TestAddWorktreeFromBase(t *testing.T) {
	repoDir := t.TempDir()
	worktreePath := filepath.Join(t.TempDir(), "feature-child")
	argvFile, cwdFile := installFakeGit(t)

	if err := AddWorktreeFromBase(repoDir, worktreePath, "feature/child", "feature/base"); err != nil {
		t.Fatalf("AddWorktreeFromBase() error = %v", err)
	}

	assertGitInvocation(t, argvFile, cwdFile, []string{"worktree", "add", worktreePath, "-b", "feature/child", "feature/base"}, repoDir)
}

func TestPullWorktree(t *testing.T) {
	worktreePath := t.TempDir()
	argvFile, cwdFile := installFakeGit(t)

	if err := PullWorktree(worktreePath); err != nil {
		t.Fatalf("PullWorktree() error = %v", err)
	}

	assertGitInvocation(t, argvFile, cwdFile, []string{"pull"}, worktreePath)
}

func installFakeGit(t *testing.T) (argvFile string, cwdFile string) {
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

func assertGitInvocation(t *testing.T, argvFile, cwdFile string, wantArgs []string, wantDir string) {
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
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("git argv = %#v, want %#v", gotArgs, wantArgs)
	}
	if gotDir := strings.TrimSpace(string(cwdBytes)); gotDir != wantDir {
		t.Fatalf("git cwd = %q, want %q", gotDir, wantDir)
	}
}
