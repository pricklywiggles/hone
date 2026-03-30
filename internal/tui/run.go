package tui

import tea "github.com/charmbracelet/bubbletea"

// Run starts a full-screen Bubble Tea program and returns the final model.
func Run(m tea.Model) (tea.Model, error) {
	return tea.NewProgram(m, tea.WithAltScreen()).Run()
}

// RunInline starts a Bubble Tea program without taking over the full screen.
// Output scrolls inline in the terminal, remaining visible after exit.
func RunInline(m tea.Model) (tea.Model, error) {
	return tea.NewProgram(m).Run()
}
