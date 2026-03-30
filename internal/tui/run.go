package tui

import tea "github.com/charmbracelet/bubbletea"

// Run starts a full-screen Bubble Tea program and returns the final model.
func Run(m tea.Model) (tea.Model, error) {
	return tea.NewProgram(m, tea.WithAltScreen()).Run()
}
