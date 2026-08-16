package installui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type TargetRow struct {
	Key    string
	Label  string
	Status string
}

type Model struct {
	rows     []TargetRow
	cursor   int
	selected string
	export   bool
}

func New(rows []TargetRow, export bool) Model {
	cloned := make([]TargetRow, len(rows))
	copy(cloned, rows)
	return Model{rows: cloned, export: export}
}

func (m Model) WithSelected(target string) Model {
	m.selected = target
	return m
}

func (m Model) Selected() string { return m.selected }

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.rows)-1 {
				m.cursor++
			}
		case "enter":
			if len(m.rows) == 0 {
				return m, nil
			}
			m.selected = m.rows[m.cursor].Key
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m Model) View() string {
	var b strings.Builder
	b.WriteString(m.title())
	b.WriteString("\n\n")
	b.WriteString("  Target      Status\n")
	for i, row := range m.rows {
		prefix := "  "
		if i == m.cursor {
			prefix = "> "
		}
		b.WriteString(prefix)
		b.WriteString(padRight(row.Label, 11))
		b.WriteString("  ")
		b.WriteString(row.Status)
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(m.actionHelp())
	b.WriteString(", esc to cancel")
	return b.String()
}

func (m Model) title() string {
	if m.export {
		return "Export ktree skill"
	}
	return "Install ktree skill"
}

func (m Model) actionHelp() string {
	if m.export {
		return "enter to export"
	}
	return "enter to install"
}

func padRight(value string, width int) string {
	if len(value) >= width {
		return value
	}
	return value + strings.Repeat(" ", width-len(value))
}
