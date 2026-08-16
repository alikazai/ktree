package installui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestViewShowsChooserTable(t *testing.T) {
	m := New([]TargetRow{
		{Key: "claude", Label: "Claude", Status: "current"},
		{Key: "codex", Label: "Codex", Status: "not installed"},
	}, false)

	plain := ansi.Strip(m.View())
	if !strings.Contains(plain, "Target") || !strings.Contains(plain, "Status") {
		t.Fatalf("chooser view = %q, want Target and Status headers", plain)
	}
	if !strings.Contains(plain, "Claude") || !strings.Contains(plain, "current") {
		t.Fatalf("chooser view = %q, want Claude current row", plain)
	}
	if !strings.Contains(plain, "Codex") || !strings.Contains(plain, "not installed") {
		t.Fatalf("chooser view = %q, want Codex not installed row", plain)
	}
	if !strings.Contains(plain, "> ") {
		t.Fatalf("chooser view = %q, want selected row marker", plain)
	}
	if !strings.Contains(plain, "enter to install") {
		t.Fatalf("chooser view = %q, want chooser help text", plain)
	}
	if !strings.Contains(plain, "esc to cancel") {
		t.Fatalf("chooser view = %q, want cancel help text", plain)
	}
}

func TestEnterSelectsHighlightedTarget(t *testing.T) {
	m := New([]TargetRow{
		{Key: "claude", Label: "Claude", Status: "current"},
		{Key: "codex", Label: "Codex", Status: "not installed"},
	}, false)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	chosen, cmd := updated.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := chosen.(Model)

	if got.Selected() != "codex" {
		t.Fatalf("Selected() = %q, want %q", got.Selected(), "codex")
	}
	if cmd == nil {
		t.Fatal("enter cmd = nil, want tea.Quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("enter cmd() = %T, want tea.QuitMsg", cmd())
	}
	if !strings.Contains(ansi.Strip(got.View()), "Codex") {
		t.Fatalf("chooser view = %q, want selected target row retained", got.View())
	}
}

func TestViewShowsExportCopyInExportMode(t *testing.T) {
	m := New([]TargetRow{{Key: "claude", Label: "Claude", Status: "current"}}, true)

	plain := ansi.Strip(m.View())
	if !strings.Contains(plain, "Export ktree skill") {
		t.Fatalf("chooser view = %q, want export title", plain)
	}
	if !strings.Contains(plain, "enter to export") {
		t.Fatalf("chooser view = %q, want export help text", plain)
	}
	if strings.Contains(plain, "enter to install") {
		t.Fatalf("chooser view = %q, do not want install help text in export mode", plain)
	}
}
