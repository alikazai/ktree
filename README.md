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

  Branch                       State    Ahead/Behind  Path
● main                         clean    -             /home/ali/code/myapp
> ● feature/auth               dirty    ↑2 ↓0         /home/ali/code/myapp-worktrees/feature-auth
○ feat/dashboard               loading  -             /home/ali/code/myapp-worktrees/feat-dashboard

↑/k up · ↓/j down · enter switch · n new · b branch · p pull · d delete · r refresh · v vault · q quit
```

**Dot legend:** `●` green = clean, `●` amber = uncommitted changes, `○` grey = loading

### Install bundled skills

`ktree` also ships a bundled `ktree` skill installer for supported agent CLIs.

```bash
# open the chooser and install into a detected target
ktree install skill

# install directly into a specific target
ktree install skill claude
ktree install skill opencode

# print the managed artifact instead of writing it
ktree install skill codex --export

# install into every detected target and print a summary
ktree install skill --all
```

Supported targets currently include Claude, OpenCode, Codex, Grok, and Cursor when their config location or binary is detected.

The chooser shows each detected target with its current status (`current`, `outdated`, `modified`, `unmanaged`, or `not installed`). `--all` installs across every detected target, reports successes and per-target failures in target order, and returns an error when nothing supported is detected.

### Keys

| Key | Action |
|-----|--------|
| `↑` / `k` | Move up |
| `↓` / `j` | Move down |
| `enter` | Switch to selected worktree (see shell setup below) |
| `n` | Create new worktree |
| `b` | Create a new branch/worktree from the selected worktree |
| `p` | Pull the selected worktree |
| `d` | Delete selected worktree (asks for confirmation) |
| `r` | Refresh list |
| `v` | Initialize vault from discovered `.env*` files |
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

While creation is running, ktree keeps the list visible and shows a bold in-progress status so it is clear the command is still working.

Press `b` to branch from the currently selected worktree into a new worktree. ktree refuses this when the selected worktree is dirty.

### Deleting a worktree

Press `d` and confirm with `y`. If the worktree has uncommitted changes, ktree shows a second prompt offering a force delete.

While deletion is running, ktree shows the same in-progress feedback. If create/delete fails, ktree restores the relevant prompt and keeps a visible error banner until your next action.

### Pulling a worktree

Press `p` to pull the selected worktree. ktree refuses this when the selected worktree is dirty, detached, or has no upstream configured.

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
