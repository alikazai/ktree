# ktree

A terminal UI for managing git worktrees. Built as a learning project — see
the roadmap at the bottom for what's implemented vs. what's next.

## Setup

```
go mod tidy
go build -o ktree .
./ktree
```

Run it from inside any git repo with at least one worktree. If you've never
made one yet, set one up first to test against:

```
cd ~/code/jurii
git worktree add ../jurii-test -b test/ktree-demo
cd ~/code/jurii
./ktree
```

### Note on go.mod

You'll see a couple of `replace` directives redirecting `golang.org/x/sys`
and `golang.org/x/exp` to their GitHub mirrors. That was a workaround for
the sandboxed environment I built this in, which couldn't resolve Go's
vanity import paths (golang.org redirects). On your own machine you almost
certainly don't need these — feel free to delete the `replace` lines and run
`go mod tidy` again to get the canonical golang.org/x modules instead.

Also note: `bubbletea` is pinned to v0.27.1 and `lipgloss` to v0.13.0
because the latest versions require Go 1.24+. If you're on a newer Go
toolchain, `go get -u ./...` should be safe to pull the latest of both.

## Project structure

```
main.go                    — entrypoint, wires up the Bubble Tea program
internal/git/worktree.go   — shells out to git, parses porcelain output
internal/git/worktree_test.go
internal/ui/model.go       — Bubble Tea model (Elm architecture: Init/Update/View)
```

The split matters: `internal/git` knows nothing about the UI, and
`internal/ui` knows nothing about how data was fetched — it just reacts to
`worktreesLoadedMsg`. That separation is what makes the git package
testable without spinning up a terminal (see `worktree_test.go`, which
tests `parsePorcelain` against captured string output rather than running
real git commands).

## What's implemented (Milestones 1–2)

- Parses `git worktree list --porcelain` into a typed `[]Worktree`
- Renders a scrollable list in a Bubble Tea TUI
- Keyboard nav: `↑/k` `↓/j` to move, `r` to refresh, `q` to quit

## Roadmap

**Milestone 3 — status enrichment.** `git.IsDirty` and `git.AheadBehind` are
already written in `worktree.go` but not wired into the UI yet. The task:
after `loadWorktrees` fetches the list, fire off a `tea.Cmd` per worktree
(or a batched one) that calls these and feeds the result back as a new Msg,
then swap `cleanDot`/`dirtyDot` in `View()` based on the real status instead
of always showing clean. Watch out for: doing this sequentially will feel
slow with more than a few worktrees — look at `tea.Batch` to fan these out
concurrently.

**Milestone 4 — actions.**

- `n` — prompt for a branch name (Bubble Tea's `textinput` component is the
  natural fit), then run `git worktree add <path> -b <branch>`.
- `d` — confirm, then `git worktree remove <path>`.
- `enter` — this is the fiddly one. A child process can't change your
  shell's working directory. The standard trick: print the selected path to
  stdout on quit, and wrap the binary in a shell function that does
  `cd "$(ktree --select)"`. Worth researching how `zoxide` or `fzf`-based
  cd helpers solve this — same problem.

**Milestone 5 — polish.** Detect worktrees whose branch was already merged
into main (via `git branch --merged`) and show them greyed out with a
"merged" tag; add `/` to filter the list by typing; add a `.ktree.yaml` for
per-project defaults.

