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
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"alikazai/ktree/internal/git"
	"alikazai/ktree/internal/vault"
)

const (
	tableBranchWidth = 28
	tableStateWidth  = 8
	tableSyncWidth   = 12
	tablePrefixWidth = 5
	tableBranchMin   = 12
	tableBranchTight = 4
	tableStateTight  = 5
	tableSyncTight   = 5
	tablePathTight   = 28
	tablePathMin     = 1
)

type viewMode int

const (
	modeList viewMode = iota
	modeCreate
	modeCreateFromSelected
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
	operationPull
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
	statusProbeErrs   map[string]error
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
	selectedBase      git.Worktree
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

func (m Model) selectedWorktree() (git.Worktree, bool) {
	if len(m.worktrees) == 0 || m.cursor < 0 || m.cursor >= len(m.worktrees) {
		return git.Worktree{}, false
	}
	return m.worktrees[m.cursor], true
}

func errProbeFailed(path, cause string) error {
	return fmt.Errorf("worktree status probe failed for %s: %s", path, cause)
}

func (m Model) selectedStatusForAction(wt git.Worktree, action string) (worktreeStatus, error) {
	if err := m.statusProbeErrs[wt.Path]; err != nil {
		return worktreeStatus{}, fmt.Errorf("cannot %s: %w", action, err)
	}
	st, known := m.statuses[wt.Path]
	if !known {
		return worktreeStatus{}, fmt.Errorf("cannot %s while worktree status is still loading", action)
	}
	return st, nil
}

func (m Model) canBranchFromSelected(wt git.Worktree) error {
	st, err := m.selectedStatusForAction(wt, "branch")
	if err != nil {
		return err
	}
	if st.dirty {
		return fmt.Errorf("cannot branch from dirty worktree")
	}
	return nil
}

func (m Model) canPullSelected(wt git.Worktree) error {
	st, err := m.selectedStatusForAction(wt, "pull")
	if err != nil {
		return err
	}
	if st.dirty {
		return fmt.Errorf("cannot pull dirty worktree")
	}
	if wt.Branch == "" {
		return fmt.Errorf("cannot pull detached worktree")
	}
	if !st.hasUpstream {
		return fmt.Errorf("cannot pull worktree without upstream")
	}
	return nil
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
	if m.mode == modeCreate || m.mode == modeCreateFromSelected {
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
				m.selectedBase = git.Worktree{}
				m.input.Blur()
				return m, nil
			case "enter":
				branch := strings.TrimSpace(m.input.Value())
				if branch == "" {
					return m, nil
				}
				if m.mode == modeCreateFromSelected {
					baseRef := m.selectedBase.Branch
					if baseRef == "" {
						baseRef = m.selectedBase.Head
					}
					if baseRef == "" {
						return m, nil
					}
					m.input.Blur()
					m.startPendingOperation(operationCreate, "Creating worktree...", branch)
					return m, tea.Batch(m.spinner.Tick, m.createWorktreeFromSelected(m.selectedBase, branch))
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
		m.statusProbeErrs = make(map[string]error)
		var cmds []tea.Cmd
		for _, wt := range m.worktrees {
			cmds = append(cmds, m.loadStatus(wt))
		}
		return m, tea.Batch(cmds...)

	case statusLoadedMsg:
		if m.statuses == nil {
			m.statuses = make(map[string]worktreeStatus)
		}
		if m.statusProbeErrs == nil {
			m.statusProbeErrs = make(map[string]error)
		}
		if !msg.dirtyKnown {
			delete(m.statuses, msg.path)
			if msg.probeErr != nil {
				m.statusProbeErrs[msg.path] = msg.probeErr
			} else {
				delete(m.statusProbeErrs, msg.path)
			}
			return m, nil
		}
		m.statuses[msg.path] = msg.status
		delete(m.statusProbeErrs, msg.path)
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
		m.statusProbeErrs = nil
		return m, m.loadWorktrees

	case worktreeCreatedMsg:
		m.pending = nil
		if msg.err != nil {
			m.err = msg.err
			m.bannerErr = msg.err
			if m.mode != modeCreateFromSelected {
				m.mode = modeCreate
			}
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
		m.statusProbeErrs = nil
		m.pendingBranch = ""
		m.selectedBase = git.Worktree{}
		return m, m.loadWorktrees

	case worktreePulledMsg:
		m.pending = nil
		if msg.err != nil {
			m.err = msg.err
			m.bannerErr = msg.err
			m.mode = modeList
			return m, nil
		}
		m.mode = modeList
		m.err = nil
		m.bannerErr = nil
		m.statuses = nil
		m.statusProbeErrs = nil
		return m, m.loadWorktrees

	case vaultCopiedMsg:
		m.pending = nil
		m.mode = modeList
		m.pendingBranch = ""
		m.statuses = nil
		m.statusProbeErrs = nil
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
				m.statusProbeErrs = nil
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
				m.selectedBase = git.Worktree{}
				m.input.SetValue("")
				m.input.Focus()
				return m, textinput.Blink
			case "b":
				wt, ok := m.selectedWorktree()
				if !ok {
					return m, nil
				}
				if err := m.canBranchFromSelected(wt); err != nil {
					m.err = err
					m.bannerErr = err
					return m, nil
				}
				m.mode = modeCreateFromSelected
				m.err = nil
				m.bannerErr = nil
				m.pendingBranch = ""
				m.copyVaultOnCreate = false
				m.selectedBase = wt
				m.input.SetValue(wt.Branch)
				m.input.Focus()
				return m, textinput.Blink
			case "p":
				wt, ok := m.selectedWorktree()
				if !ok {
					return m, nil
				}
				if err := m.canPullSelected(wt); err != nil {
					m.err = err
					m.bannerErr = err
					return m, nil
				}
				m.startPendingOperation(operationPull, "Pulling worktree...", wt.Path)
				return m, tea.Batch(m.spinner.Tick, m.pullWorktree(wt))
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

func stateLabel(st worktreeStatus, known bool) string {
	if !known {
		return "loading"
	}
	if st.dirty {
		return "dirty"
	}
	return "clean"
}

func syncLabel(st worktreeStatus, known bool) string {
	if !known || !st.hasUpstream {
		return "-"
	}
	return fmt.Sprintf("↑%d ↓%d", st.ahead, st.behind)
}

func fitTableCell(text string, width int) string {
	if width <= 0 {
		return ""
	}

	text = ansi.Truncate(text, width, "")
	padding := width - lipgloss.Width(text)
	if padding <= 0 {
		return text
	}
	return text + strings.Repeat(" ", padding)
}

func truncatePathStart(path string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(path) <= width {
		return path
	}
	if width <= 3 {
		return strings.Repeat(".", width)
	}

	trimmed := path
	for lipgloss.Width("..."+trimmed) > width && len(trimmed) > 0 {
		_, size := utf8.DecodeRuneInString(trimmed)
		trimmed = trimmed[size:]
	}
	if !strings.HasPrefix(trimmed, "/") {
		if slash := strings.Index(trimmed, "/"); slash > 0 {
			candidate := trimmed[slash:]
			if lipgloss.Width("..."+candidate) <= width {
				trimmed = candidate
			}
		}
	}

	return "..." + trimmed
}

func useNarrowPathStartTruncation(pathWidth int) bool {
	return pathWidth > 0 && pathWidth < tablePathTight
}

func tableWidth(total int) int {
	if total <= 0 {
		return 80
	}
	return total
}

func shrinkWidth(width *int, minWidth int, deficit *int) {
	if *deficit <= 0 || *width <= minWidth {
		return
	}

	shrink := min(*deficit, *width-minWidth)
	*width -= shrink
	*deficit -= shrink
}

func tableColumnWidths(total int) (branchWidth, stateWidth, syncWidth, pathWidth int) {
	branchWidth = tableBranchWidth
	stateWidth = tableStateWidth
	syncWidth = tableSyncWidth

	available := tableWidth(total) - tablePrefixWidth - 3
	pathWidth = available - branchWidth - stateWidth - syncWidth
	if pathWidth >= tablePathMin {
		return branchWidth, stateWidth, syncWidth, pathWidth
	}

	deficit := tablePathMin - pathWidth
	shrinkWidth(&branchWidth, tableBranchMin, &deficit)
	shrinkWidth(&stateWidth, tableStateTight, &deficit)
	shrinkWidth(&syncWidth, tableSyncTight, &deficit)
	shrinkWidth(&branchWidth, tableBranchTight, &deficit)
	shrinkWidth(&stateWidth, 1, &deficit)
	shrinkWidth(&syncWidth, 1, &deficit)
	shrinkWidth(&branchWidth, 1, &deficit)
	shrinkWidth(&stateWidth, 0, &deficit)
	shrinkWidth(&syncWidth, 0, &deficit)
	shrinkWidth(&branchWidth, 0, &deficit)

	pathWidth = available - branchWidth - stateWidth - syncWidth
	if pathWidth < 0 {
		pathWidth = 0
	}

	return branchWidth, stateWidth, syncWidth, pathWidth
}

func useCompactTableFallback(total, branchWidth, stateWidth, syncWidth, pathWidth int) bool {
	if total <= 0 {
		return false
	}

	return branchWidth < tableBranchTight ||
		stateWidth < tableStateTight ||
		syncWidth < tableSyncTight ||
		pathWidth <= 0
}

func renderTinyTableRow(total int, selected bool, dot string, primary string) string {
	remaining := total
	if remaining <= 0 {
		return ""
	}

	var b strings.Builder
	if remaining == 1 {
		if selected {
			b.WriteString(">")
		} else {
			b.WriteString(" ")
		}
		remaining--
	} else if remaining > 1 {
		if selected {
			b.WriteString("> ")
		} else {
			b.WriteString("  ")
		}
		remaining -= 2
	}
	if remaining > 0 {
		b.WriteString(dot)
		remaining--
	}
	if remaining > 0 {
		b.WriteString(fitTableCell(primary, remaining))
	}

	row := b.String()
	if selected {
		return selectedStyle.Render(row)
	}
	return row
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

	if len(m.worktrees) > 0 {
		branchWidth, stateWidth, syncWidth, pathWidth := tableColumnWidths(m.width)
		// Fall back once the condensed table drops below the minimum readable widths.
		tinyTable := useCompactTableFallback(m.width, branchWidth, stateWidth, syncWidth, pathWidth)
		if !tinyTable {
			header := strings.Repeat(" ", tablePrefixWidth) +
				fitTableCell("Branch", branchWidth) + " " +
				fitTableCell("State", stateWidth) + " " +
				fitTableCell("Ahead/Behind", syncWidth) + " " +
				fitTableCell("Path", pathWidth)
			b.WriteString(headerStyle.Render(header))
			b.WriteString("\n")
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

			if tinyTable {
				b.WriteString(renderTinyTableRow(m.width, i == m.cursor, dot, wt.DisplayBranch()))
				b.WriteString("\n")
				continue
			}

			line := dot + "  " +
				branchStyle.Render(fitTableCell(wt.DisplayBranch(), branchWidth)) + " " +
				fitTableCell(stateLabel(st, known), stateWidth) + " " +
				fitTableCell(syncLabel(st, known), syncWidth) + " " +
				pathStyle.Render(fitTableCell(func() string {
					if useNarrowPathStartTruncation(pathWidth) && lipgloss.Width(wt.Path) > pathWidth {
						return truncatePathStart(wt.Path, pathWidth)
					}
					return wt.Path
				}(), pathWidth))

			if i == m.cursor {
				b.WriteString(selectedStyle.Render("> " + line))
			} else {
				b.WriteString("  " + line)
			}
			b.WriteString("\n")
		}
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
		b.WriteString(helpStyle.Render("↑/k up · ↓/j down · enter switch · n new · b branch · p pull · d delete · r refresh · v vault · q quit"))

	case modeCreate, modeCreateFromSelected:
		branch := m.input.Value()
		b.WriteString("\n")
		if m.mode == modeCreateFromSelected {
			b.WriteString(helpStyle.Render("Branch from selected") + "\n")
		} else {
			b.WriteString(helpStyle.Render("New worktree") + "\n")
		}
		b.WriteString("  Branch: " + m.input.View() + "\n")
		if branch != "" {
			preview := git.WorktreePath(m.repoDir, branch)
			b.WriteString(pathStyle.Render(fmt.Sprintf("  Path:   %s", preview)) + "\n")
		}
		if m.err != nil {
			b.WriteString(errStyle.Render(fmt.Sprintf("  error: %v", m.err)) + "\n")
		}
		if m.mode == modeCreateFromSelected {
			b.WriteString(helpStyle.Render("type branch name · enter to create · esc to cancel"))
		} else {
			b.WriteString(helpStyle.Render("enter to create · esc to cancel"))
		}

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
