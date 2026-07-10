// Package ui implements the Bubble Tea terminal interface for wtree.
//
// Bubble Tea follows The Elm Architecture: a single Model holds all state,
// Update() is a pure function that takes a Msg and returns a new Model (plus
// optional Cmd to run), and View() renders the Model to a string. There's no
// direct mutation from event handlers like you'd see in a typical GUI
// framework — everything flows through Update.
package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type viewMode int

const (
	modeList viewMode = iota
	modeCreate
	modeDeleteConfirm
	modeDeleteForce
)

// Styling. Lipgloss styles are immutable and composable — define them once,
// reuse everywhere. Keeping them at package level (not inside the model)
// avoids rebuilding them every render.
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212")).
			Padding(0, 1)

	selectedStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("57")).
			Foreground(lipgloss.Color("230")).
			Bold(true)

	cleanDot = lipgloss.NewStyle().Foreground(lipgloss.Color("76")).Render("●")  // green
	dirtyDot = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("●") // amber
	staleDot = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("○") // grey

	branchStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	pathStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	helpStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Padding(1, 1, 0, 1)
	errStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

// worktreeStatus holds the per-worktree status fetched concurrently after the
// worktree list loads.
type worktreeStatus struct {
	dirty       bool
	ahead       int
	behind      int
	hasUpstream bool
}

// Model is the full state of the TUI at any point in time. Bubble Tea
// re-renders View() from this struct on every Update — there's no
// persistent widget tree to mutate.
type Model struct {
	repoDir   string
	worktrees []Worktree
	statuses  map[string]worktreeStatus
	cursor    int
	err       error
	width     int
	height    int
	mode      viewMode
	input     textinput.Model
	selected  string // path printed to stdout after quit (switch action)
}

// New constructs the initial model for a given repo directory. It does NOT
// fetch worktree data yet — that happens via the loadWorktrees command,
// fired from Init(), so the UI can render immediately and update once data
// arrives rather than blocking startup on a subprocess call.
func New(repoDir string) Model {
	ti := textinput.New()
	ti.Placeholder = "feature/my-branch"
	ti.CharLimit = 100
	return Model{repoDir: repoDir, input: ti}
}

// --- Messages -----------------------------------------------------------
//
// In Bubble Tea, anything that happens asynchronously (subprocess finishing,
// timer firing, network response) is represented as a Msg type and delivered
// to Update(). This keeps Update() the single place state changes happen.

// worktreesLoadedMsg carries the result of running `git worktree list`.
type worktreesLoadedMsg struct {
	worktrees []Worktree
	err       error
}

// statusLoadedMsg carries the status result for a single worktree.
type statusLoadedMsg struct {
	path   string
	status worktreeStatus
}

type worktreeCreatedMsg struct{ err error }

type worktreeDeletedMsg struct {
	err    error
	forced bool // true when this was a --force attempt
}

// loadWorktrees returns a tea.Cmd — a function Bubble Tea will run in the
// background and turn into a Msg once it completes. This is how you do I/O
// (subprocesses, network calls) without blocking the render loop.
func (m Model) loadWorktrees() tea.Msg {
	wts, err := List(m.repoDir)
	return worktreesLoadedMsg{worktrees: wts, err: err}
}

// loadStatus fetches dirty/ahead-behind for a single worktree. It's called
// once per worktree via tea.Batch so all statuses load concurrently.
func (m Model) loadStatus(wt Worktree) tea.Cmd {
	return func() tea.Msg {
		dirty, _ := IsDirty(wt.Path)
		ahead, behind, hasUpstream := AheadBehind(m.repoDir, wt.Branch)
		return statusLoadedMsg{
			path: wt.Path,
			status: worktreeStatus{
				dirty:       dirty,
				ahead:       ahead,
				behind:      behind,
				hasUpstream: hasUpstream,
			},
		}
	}
}

func (m Model) deleteWorktree(wt Worktree, force bool) tea.Cmd {
	return func() tea.Msg {
		err := RemoveWorktree(m.repoDir, wt.Path, force)
		return worktreeDeletedMsg{err: err, forced: force}
	}
}

func (m Model) createWorktree(branch string) tea.Cmd {
	return func() tea.Msg {
		path := WorktreePath(m.repoDir, branch)
		err := AddWorktree(m.repoDir, path, branch)
		return worktreeCreatedMsg{err: err}
	}
}

// --- Elm architecture: Init / Update / View -----------------------------

// Init is called once when the program starts. Returning loadWorktrees here
// kicks off the initial git call without blocking the first render.
func (m Model) Init() tea.Cmd {
	return m.loadWorktrees
}

// Update handles every incoming message and returns the new model state
// plus an optional command to run next. This is the only place model
// fields should change.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// In create mode, route everything through the textinput component first.
	// Special keys (esc, enter) are intercepted before the input sees them.
	if m.mode == modeCreate {
		if wm, ok := msg.(tea.WindowSizeMsg); ok {
			m.width = wm.Width
			m.height = wm.Height
			return m, nil
		}
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "esc":
				m.mode = modeList
				m.err = nil
				m.input.Blur()
				return m, nil
			case "enter":
				branch := strings.TrimSpace(m.input.Value())
				if branch == "" {
					return m, nil
				}
				m.input.Blur()
				return m, m.createWorktree(branch)
			}
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case worktreesLoadedMsg:
		m.worktrees = msg.worktrees
		m.err = msg.err
		if m.cursor >= len(m.worktrees) {
			m.cursor = max(0, len(m.worktrees)-1)
		}
		m.statuses = make(map[string]worktreeStatus)
		var cmds []tea.Cmd
		for _, wt := range m.worktrees {
			cmds = append(cmds, m.loadStatus(wt))
		}
		return m, tea.Batch(cmds...)

	case statusLoadedMsg:
		if m.statuses == nil {
			m.statuses = make(map[string]worktreeStatus)
		}
		m.statuses[msg.path] = msg.status
		return m, nil

	case worktreeDeletedMsg:
		if msg.err != nil {
			m.err = msg.err
			if !msg.forced {
				// First attempt failed — offer force delete
				m.mode = modeDeleteForce
			} else {
				// Force attempt also failed — give up, go back to list
				m.mode = modeList
			}
			return m, nil
		}
		m.mode = modeList
		m.err = nil
		m.statuses = nil
		return m, m.loadWorktrees

	case worktreeCreatedMsg:
		if msg.err != nil {
			m.err = msg.err
			// Stay in modeCreate so the user can fix the branch name
			return m, nil
		}
		m.mode = modeList
		m.err = nil
		m.statuses = nil
		return m, m.loadWorktrees

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		switch m.mode {
		case modeList:
			switch msg.String() {
			case "q":
				return m, tea.Quit
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down", "j":
				if m.cursor < len(m.worktrees)-1 {
					m.cursor++
				}
			case "r":
				m.statuses = nil
				return m, m.loadWorktrees
			case "enter":
				if len(m.worktrees) > 0 {
					m.selected = m.worktrees[m.cursor].Path
					return m, tea.Quit
				}
			case "d":
				if len(m.worktrees) > 0 {
					m.mode = modeDeleteConfirm
					m.err = nil
				}
			case "n":
				m.mode = modeCreate
				m.err = nil
				m.input.SetValue("")
				m.input.Focus()
				return m, textinput.Blink
			}
			return m, nil

		case modeDeleteConfirm:
			switch msg.String() {
			case "y", "Y":
				wt := m.worktrees[m.cursor]
				return m, m.deleteWorktree(wt, false)
			case "n", "N", "esc":
				m.mode = modeList
				m.err = nil
			}
			return m, nil

		case modeDeleteForce:
			switch msg.String() {
			case "y", "Y":
				wt := m.worktrees[m.cursor]
				return m, m.deleteWorktree(wt, true)
			case "n", "N", "esc":
				m.mode = modeList
				m.err = nil
			}
			return m, nil
		}
	}

	return m, nil
}

// View renders the current model state to a string. Bubble Tea calls this
// after every Update and redraws the terminal with the result — you never
// write to the terminal directly.
func (m Model) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(fmt.Sprintf("Worktrees: %s", m.repoDir)))
	b.WriteString("\n\n")

	if m.err != nil && m.mode == modeList {
		b.WriteString(errStyle.Render(fmt.Sprintf("error: %v", m.err)))
		b.WriteString("\n")
	}

	if len(m.worktrees) == 0 && m.err == nil {
		b.WriteString(pathStyle.Render("  loading...\n"))
	}

	for i, wt := range m.worktrees {
		st, known := m.statuses[wt.Path]
		var dot string
		switch {
		case !known:
			dot = staleDot
		case st.dirty:
			dot = dirtyDot
		default:
			dot = cleanDot
		}

		syncInfo := ""
		if known && st.hasUpstream {
			syncInfo = fmt.Sprintf(" ↑%d ↓%d", st.ahead, st.behind)
		}

		line := fmt.Sprintf("%s  %-28s %s%s",
			dot,
			branchStyle.Render(wt.DisplayBranch()),
			pathStyle.Render(wt.Path),
			pathStyle.Render(syncInfo),
		)

		if i == m.cursor {
			b.WriteString(selectedStyle.Render("> " + line))
		} else {
			b.WriteString("  " + line)
		}
		b.WriteString("\n")
	}

	switch m.mode {
	case modeList:
		b.WriteString(helpStyle.Render("↑/k up · ↓/j down · enter switch · n new · d delete · r refresh · q quit"))

	case modeCreate:
		branch := m.input.Value()
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("New worktree") + "\n")
		b.WriteString("  Branch: " + m.input.View() + "\n")
		if branch != "" {
			preview := WorktreePath(m.repoDir, branch)
			b.WriteString(pathStyle.Render(fmt.Sprintf("  Path:   %s", preview)) + "\n")
		}
		if m.err != nil {
			b.WriteString(errStyle.Render(fmt.Sprintf("  error: %v", m.err)) + "\n")
		}
		b.WriteString(helpStyle.Render("enter to create · esc to cancel"))

	case modeDeleteConfirm:
		if len(m.worktrees) > 0 {
			b.WriteString("\n")
			b.WriteString(errStyle.Render(fmt.Sprintf("  Delete %s? (y/N)", m.worktrees[m.cursor].Path)))
			b.WriteString("\n")
			b.WriteString(helpStyle.Render("y yes · n/esc cancel"))
		}

	case modeDeleteForce:
		if len(m.worktrees) > 0 {
			b.WriteString("\n")
			if m.err != nil {
				b.WriteString(errStyle.Render(fmt.Sprintf("  %v", m.err)))
				b.WriteString("\n")
			}
			b.WriteString(errStyle.Render(fmt.Sprintf("  Force delete %s? (y/N)", m.worktrees[m.cursor].Path)))
			b.WriteString("\n")
			b.WriteString(helpStyle.Render("y yes · n/esc cancel"))
		}
	}

	return b.String()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
