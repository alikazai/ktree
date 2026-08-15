package ui

import "github.com/charmbracelet/lipgloss"

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
	busyStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	errStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)
