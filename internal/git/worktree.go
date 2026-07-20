package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Worktree represents a single entry from `git worktree list --porcelain`.
type Worktree struct {
	Path     string // absolute path to the worktree directory
	Head     string // full commit SHA currently checked out
	Branch   string // branch name, e.g. "refs/heads/main" -> "main"; empty if detached
	Bare     bool   // true for the main bare repo entry, if any
	Locked   bool   // true if `git worktree lock` was used on this entry
	Prunable bool   // true if git thinks this worktree's directory is gone
}

// List runs `git worktree list --porcelain` in the given repo directory
// and parses the output into a slice of Worktree.
//
// The porcelain format looks like this (entries separated by blank lines):
//
//	worktree /home/ali/code/jurii
//	HEAD a1b2c3d4...
//	branch refs/heads/main
//
//	worktree /home/ali/code/jurii-kyc
//	HEAD e5f6a7b8...
//	branch refs/heads/feature/kyc-flow
//
//	worktree /home/ali/code/jurii-old
//	HEAD 11223344...
//	detached
func List(repoDir string) ([]Worktree, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = repoDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git worktree list: %w (stderr: %s)", err, stderr.String())
	}

	return parsePorcelain(stdout.String()), nil
}

// parsePorcelain is split out from List so it can be unit tested without
// actually running git — pass it captured output as a string.
func parsePorcelain(output string) []Worktree {
	var worktrees []Worktree
	var current Worktree
	started := false

	flush := func() {
		if started {
			worktrees = append(worktrees, current)
		}
		current = Worktree{}
		started = false
	}

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")

		if line == "" {
			flush()
			continue
		}

		key, value, _ := strings.Cut(line, " ")

		switch key {
		case "worktree":
			flush() // saftey: should not normally fire since blank lines seperate entries
			current.Path = value
			started = true
		case "HEAD":
			current.Head = value
		case "branch":
			current.Branch = strings.TrimPrefix(value, "refs/heads/")
		case "bare":
			current.Bare = true
		case "locked":
			current.Locked = true
		case "prunable":
			current.Prunable = true
		case "detached":
			current.Branch = "" // explicit: HEAD is set but no branch
		}
	}

	flush() // catch the last entry, since there is no trailing blank line guaranteed

	return worktrees
}

// ShortHead returns the first 8 characters of the HEAD SHA, or "" if unset.
func (w Worktree) ShortHead() string {
	if len(w.Head) < 8 {
		return w.Head
	}

	return w.Head[:8]
}

// DisplayBranch returns the branch name, or "(detached)" if there isn't one.
func (w Worktree) DisplayBranch() string {
	if w.Branch == "" {
		return "(detached)"
	}
	return w.Branch
}

// AheadBehind returns how many commits the worktree's branch is ahead/behind
// its upstream. Returns (0, 0, false) if there's no upstream configured.
func AheadBehind(repoDir, branch string) (ahead, behind int, ok bool) {
	if branch == "" {
		return 0, 0, false
	}

	upstream := branch + "@{upstream}"
	cmd := exec.Command("git", "rev-list", "--left-right", "--count", branch+"..."+upstream)
	cmd.Dir = repoDir

	out, err := cmd.Output()
	if err != nil {
		// Most likely: no upstream set for this branch. Not a real error
		return 0, 0, false
	}

	fields := strings.Fields(string(out))
	if len(fields) != 2 {
		return 0, 0, false
	}

	ahead, errA := strconv.Atoi(fields[0])
	behind, errB := strconv.Atoi(fields[1])
	if errA != nil || errB != nil {
		return 0, 0, false
	}

	return ahead, behind, true
}

// IsDirty reports whether the worktree has uncommitted changes
// (staged, unstaged, or untracked files).
func IsDirty(worktreePath string) (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = worktreePath

	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("git status: %w", err)
	}

	return len(bytes.TrimSpace(out)) > 0, nil
}

// WorktreePath derives the directory for a new worktree.
// New worktrees live at <parent>/<repo>-worktrees/<branch-with-slashes-as-dashes>.
func WorktreePath(repoDir, branch string) string {
	parent := filepath.Dir(repoDir)
	repo := filepath.Base(repoDir)
	dirName := strings.ReplaceAll(branch, "/", "-")
	return filepath.Join(parent, repo+"-worktrees", dirName)
}

// AddWorktree runs `git worktree add <path> -b <branch>` in repoDir.
// Returns an error (including git's stderr) if the command fails.
func AddWorktree(repoDir, path, branch string) error {
	cmd := exec.Command("git", "worktree", "add", path, "-b", branch)
	cmd.Dir = repoDir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// RemoveWorktree runs `git worktree remove [--force] <path>` in repoDir.
func RemoveWorktree(repoDir, path string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, path)
	cmd := exec.Command("git", args...)
	cmd.Dir = repoDir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}
