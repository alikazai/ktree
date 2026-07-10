package main

import "testing"

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
