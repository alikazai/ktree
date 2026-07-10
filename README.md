# ktree

A terminal UI for managing git worktrees.

## Install

**Homebrew (macOS / Linux):**

```bash
brew tap alikazai/ktree
brew install ktree
```

**From source:**

```bash
go install github.com/alikazai/ktree@latest
```

## Usage

Run `ktree` from inside any git repo.

```
Worktrees: /home/ali/code/myapp

●  main                         /home/ali/code/myapp
○  feature/auth                 /home/ali/code/myapp-worktrees/feature-auth  ↑2 ↓0
●  feat/dashboard               /home/ali/code/myapp-worktrees/feat-dashboard

↑/k up · ↓/j down · enter switch · n new · d delete · r refresh · q quit
```

**Dot legend:** `●` green = clean, `●` amber = uncommitted changes, `○` grey = loading

### Keys

| Key | Action |
|-----|--------|
| `↑` / `k` | Move up |
| `↓` / `j` | Move down |
| `enter` | Switch to selected worktree (see shell setup below) |
| `n` | Create new worktree |
| `d` | Delete selected worktree (asks for confirmation) |
| `r` | Refresh list |
| `q` / `ctrl+c` | Quit |

### Shell setup for `enter` (cd to worktree)

A child process can't change your shell's working directory directly, so `enter` works by printing the selected path to stdout on exit. Wrap it in a shell function to get the `cd` behaviour:

**bash / zsh** — add to `~/.bashrc` or `~/.zshrc`:
```bash
eval "$(ktree --init-shell bash)"
```

**fish** — add to `~/.config/fish/config.fish`:
```fish
ktree --init-shell fish | source
```

Then use `ktw` to launch ktree with cd-on-select.

### Creating a worktree

Press `n`, type a branch name, and press `enter`. New worktrees are created at:

```
<parent>/<repo>-worktrees/<branch-name>
```

For example, branch `feature/auth` in repo `~/code/myapp` → `~/code/myapp-worktrees/feature-auth`.

### Deleting a worktree

Press `d` and confirm with `y`. If the worktree has uncommitted changes, ktree shows a second prompt offering a force delete.

## Project structure

```
main.go          — entrypoint; prints selected path after quit for shell cd trick
model.go         — Bubble Tea model (Elm architecture: Init / Update / View)
worktree.go      — git subprocess wrappers and porcelain parser
worktree_test.go — unit tests for parsePorcelain and WorktreePath
```

## What's implemented

- **Milestone 1** — parses `git worktree list --porcelain` into typed `[]Worktree`
- **Milestone 2** — scrollable Bubble Tea list with keyboard navigation
- **Milestone 3** — live dirty/clean status dots and ahead/behind counts, loaded concurrently via `tea.Batch`
- **Milestone 4** — create (`n`), delete (`d`), and switch (`enter` + shell wrapper)

## Roadmap

**Milestone 5 — polish**

- Merged-branch detection: grey out worktrees whose branch is already merged into main
- Filter/search: `/` to narrow the list by branch name
- Per-project config: `.ktree.yaml` for defaults (main branch name, post-create hook)
