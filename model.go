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

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

// Model is the full state of the TUI at any point in time. Bubble Tea
// re-renders View() from this struct on every Update — there's no
// persistent widget tree to mutate.
type Model struct {
	repoDir   string
	worktrees []Worktree
	cursor    int   // index into worktrees of the currently highlighted row
	err       error // last error, shown in the status line if non-nil
	width     int   // terminal width, set on first WindowSizeMsg
	height    int
}

// New constructs the initial model for a given repo directory. It does NOT
// fetch worktree data yet — that happens via the loadWorktrees command,
// fired from Init(), so the UI can render immediately and update once data
// arrives rather than blocking startup on a subprocess call.
func New(repoDir string) Model {
	return Model{repoDir: repoDir}
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

// loadWorktrees returns a tea.Cmd — a function Bubble Tea will run in the
// background and turn into a Msg once it completes. This is how you do I/O
// (subprocesses, network calls) without blocking the render loop.
func (m Model) loadWorktrees() tea.Msg {
	wts, err := List(m.repoDir)
	return worktreesLoadedMsg{worktrees: wts, err: err}
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
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case "down", "j":
			if m.cursor < len(m.worktrees)-1 {
				m.cursor++
			}
			return m, nil

		case "r":
			// Re-run the git call. Milestone 3 will also refresh
			// dirty/ahead-behind status here.
			return m, m.loadWorktrees
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

	if m.err != nil {
		b.WriteString(errStyle.Render(fmt.Sprintf("error: %v", m.err)))
		b.WriteString("\n")
	}

	if len(m.worktrees) == 0 && m.err == nil {
		b.WriteString(pathStyle.Render("  loading...\n"))
	}

	for i, wt := range m.worktrees {
		dot := cleanDot // Milestone 3 will swap this based on actual git status
		line := fmt.Sprintf("%s  %-28s %s",
			dot,
			branchStyle.Render(wt.DisplayBranch()),
			pathStyle.Render(wt.Path),
		)

		if i == m.cursor {
			b.WriteString(selectedStyle.Render("> " + line))
		} else {
			b.WriteString("  " + line)
		}
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render("↑/k up · ↓/j down · r refresh · q quit"))

	return b.String()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
