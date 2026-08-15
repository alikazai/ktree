// Package ui implements the Bubble Tea terminal interface for ktree.
//
// Bubble Tea follows The Elm Architecture: a single Model holds all state,
// Update() is a pure function that takes a Msg and returns a new Model (plus
// optional Cmd to run), and View() renders the Model to a string. There's no
// direct mutation from event handlers like you'd see in a typical GUI
// framework — everything flows through Update.
package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"alikazai/ktree/internal/git"
	"alikazai/ktree/internal/vault"
)

type viewMode int

const (
	modeList viewMode = iota
	modeCreate
	modeDeleteConfirm
	modeDeleteForce
)

// worktreeStatus holds the per-worktree status fetched concurrently after the
// worktree list loads.
type worktreeStatus struct {
	dirty       bool
	ahead       int
	behind      int
	hasUpstream bool
}

type operationKind int

const (
	operationCreate operationKind = iota
	operationDelete
	operationCopyVault
)

type pendingOperation struct {
	kind   operationKind
	label  string
	target string
}

// Model is the full state of the TUI at any point in time. Bubble Tea
// re-renders View() from this struct on every Update — there's no
// persistent widget tree to mutate.
type Model struct {
	repoDir           string
	worktrees         []git.Worktree
	statuses          map[string]worktreeStatus
	cursor            int
	err               error
	bannerErr         error
	width             int
	height            int
	mode              viewMode
	input             textinput.Model
	spinner           spinner.Model
	pending           *pendingOperation
	selected          string // path printed to stdout after quit (switch action)
	pendingBranch     string // branch held across the vault-copy-confirm → create → copy sequence
	copyVaultOnCreate bool   // true when a vault copy should follow worktree creation
	vaultModel        *vault.VaultModel
}

// New constructs the initial model for a given repo directory. It does NOT
// fetch worktree data yet — that happens via the loadWorktrees command,
// fired from Init(), so the UI can render immediately and update once data
// arrives rather than blocking startup on a subprocess call.
func New(repoDir string) Model {
	ti := textinput.New()
	ti.Placeholder = "feature/my-branch"
	ti.CharLimit = 100
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	return Model{repoDir: repoDir, input: ti, spinner: sp}
}

// Selected returns the worktree path chosen by the user (switch action),
// or "" if the user quit without selecting.
func (m Model) Selected() string { return m.selected }

// Init is called once when the program starts. Returning loadWorktrees here
// kicks off the initial git call without blocking the first render.
func (m Model) Init() tea.Cmd {
	return m.loadWorktrees
}

func (m *Model) startPendingOperation(kind operationKind, label, target string) {
	m.pending = &pendingOperation{kind: kind, label: label, target: target}
	m.bannerErr = nil
	m.err = nil
}

// Update handles every incoming message and returns the new model state
// plus an optional command to run next. This is the only place model
// fields should change.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.pending != nil {
		switch msg := msg.(type) {
		case spinner.TickMsg:
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		case tea.KeyMsg:
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			return m, nil
		}
	}

	// When the vault sub-model is active it owns the screen. ctrl+c is
	// intercepted first so quit always works regardless of vault state.
	if m.vaultModel != nil {
		if key, ok := msg.(tea.KeyMsg); ok && key.String() == "ctrl+c" {
			return m, tea.Quit
		}
		updated, cmd := m.vaultModel.Update(msg)
		m.vaultModel = &updated
		if m.vaultModel.Done() {
			shouldCopy := m.vaultModel.ShouldCopy()
			if err := m.vaultModel.Err(); err != nil {
				m.err = err
				m.bannerErr = err
			}
			m.vaultModel = nil
			m.copyVaultOnCreate = shouldCopy
			if m.pendingBranch != "" {
				// Copy-confirm flow: create the worktree now that user has responded.
				m.startPendingOperation(operationCreate, "Creating worktree...", m.pendingBranch)
				return m, tea.Batch(m.spinner.Tick, m.createWorktree(m.pendingBranch))
			}
			// Init flow: return to the list.
			return m, m.loadWorktrees
		}
		return m, cmd
	}

	// In create mode, handle window size and key messages. Everything else
	// (including worktreeCreatedMsg) falls through to the main switch.
	if m.mode == modeCreate {
		switch msg := msg.(type) {
		case tea.WindowSizeMsg:
			m.width = msg.Width
			m.height = msg.Height
			return m, nil
		case tea.KeyMsg:
			if msg.String() != "ctrl+c" {
				m.bannerErr = nil
			}
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "esc":
				m.mode = modeList
				m.err = nil
				m.bannerErr = nil
				m.input.Blur()
				return m, nil
			case "enter":
				branch := strings.TrimSpace(m.input.Value())
				if branch == "" {
					return m, nil
				}
				m.input.Blur()
				if vault.VaultExists(m.repoDir) {
					m.pendingBranch = branch
					v := vault.NewCopyConfirmFlow(m.repoDir, branch)
					m.vaultModel = &v
					m.mode = modeList
					return m, m.vaultModel.Init()
				}
				m.startPendingOperation(operationCreate, "Creating worktree...", branch)
				return m, tea.Batch(m.spinner.Tick, m.createWorktree(branch))
			default:
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				return m, cmd
			}
		}
		// Any other message type (worktreeCreatedMsg, statusLoadedMsg, etc.)
		// falls through to the main switch below.
	}

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case worktreesLoadedMsg:
		m.worktrees = msg.worktrees
		if m.mode == modeList {
			m.err = msg.err
		}
		if msg.err != nil && m.bannerErr == nil {
			m.bannerErr = msg.err
		}
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
		m.pending = nil
		if msg.err != nil {
			m.err = msg.err
			m.bannerErr = msg.err
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
		m.bannerErr = nil
		m.statuses = nil
		return m, m.loadWorktrees

	case worktreeCreatedMsg:
		m.pending = nil
		if msg.err != nil {
			m.err = msg.err
			m.bannerErr = msg.err
			m.mode = modeCreate
			m.input.Focus()
			return m, textinput.Blink
		}
		if m.copyVaultOnCreate {
			m.copyVaultOnCreate = false
			m.startPendingOperation(operationCopyVault, "Copying vault...", git.WorktreePath(m.repoDir, m.pendingBranch))
			return m, tea.Batch(m.spinner.Tick, m.copyVault(git.WorktreePath(m.repoDir, m.pendingBranch)))
		}
		m.mode = modeList
		m.err = nil
		m.bannerErr = nil
		m.statuses = nil
		m.pendingBranch = ""
		return m, m.loadWorktrees

	case vaultCopiedMsg:
		m.pending = nil
		m.mode = modeList
		m.pendingBranch = ""
		m.statuses = nil
		if msg.err != nil {
			m.err = msg.err
			m.bannerErr = msg.err
		} else {
			m.err = nil
			m.bannerErr = nil
		}
		return m, m.loadWorktrees

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		m.bannerErr = nil
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
				m.bannerErr = nil
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
					m.bannerErr = nil
				}
			case "n":
				m.mode = modeCreate
				m.err = nil
				m.bannerErr = nil
				m.input.SetValue("")
				m.input.Focus()
				return m, textinput.Blink
			case "v":
				m.err = nil
				m.bannerErr = nil
				v := vault.NewInitFlow(m.repoDir)
				m.vaultModel = &v
				return m, m.vaultModel.Init()
			}
			return m, nil

		case modeDeleteConfirm:
			switch msg.String() {
			case "y", "Y":
				wt := m.worktrees[m.cursor]
				m.startPendingOperation(operationDelete, "Deleting worktree...", wt.Path)
				return m, tea.Batch(m.spinner.Tick, m.deleteWorktree(wt, false))
			case "n", "N", "esc":
				m.mode = modeList
				m.err = nil
				m.bannerErr = nil
			}
			return m, nil

		case modeDeleteForce:
			switch msg.String() {
			case "y", "Y":
				wt := m.worktrees[m.cursor]
				m.startPendingOperation(operationDelete, "Deleting worktree...", wt.Path)
				return m, tea.Batch(m.spinner.Tick, m.deleteWorktree(wt, true))
			case "n", "N", "esc":
				m.mode = modeList
				m.err = nil
				m.bannerErr = nil
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
	if m.vaultModel != nil {
		return m.vaultModel.View()
	}

	var b strings.Builder

	b.WriteString(titleStyle.Render(fmt.Sprintf("Worktrees: %s", m.repoDir)))
	b.WriteString("\n\n")

	if m.bannerErr != nil {
		b.WriteString(errStyle.Render(fmt.Sprintf("error: %v", m.bannerErr)))
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

	if m.pending != nil {
		b.WriteString("\n")
		b.WriteString(busyStyle.Render(fmt.Sprintf("  %s %s", m.spinner.View(), m.pending.label)))
		b.WriteString("\n")
		if m.pending.target != "" {
			b.WriteString(pathStyle.Render(fmt.Sprintf("  %s", m.pending.target)))
			b.WriteString("\n")
		}
		b.WriteString(helpStyle.Render("working..."))
		return b.String()
	}

	switch m.mode {
	case modeList:
		b.WriteString(helpStyle.Render("↑/k up · ↓/j down · enter switch · n new · d delete · r refresh · v vault · q quit"))

	case modeCreate:
		branch := m.input.Value()
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("New worktree") + "\n")
		b.WriteString("  Branch: " + m.input.View() + "\n")
		if branch != "" {
			preview := git.WorktreePath(m.repoDir, branch)
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
