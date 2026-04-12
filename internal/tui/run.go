package tui

import tea "charm.land/bubbletea/v2"

type altScreenModel struct{ inner tea.Model }

func (a altScreenModel) Init() tea.Cmd                           { return a.inner.Init() }
func (a altScreenModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m, cmd := a.inner.Update(msg)
	a.inner = m
	return a, cmd
}
func (a altScreenModel) View() tea.View {
	v := a.inner.View()
	v.AltScreen = true
	return v
}

// Run starts a full-screen Bubble Tea program and returns the final model.
func Run(m tea.Model) (tea.Model, error) {
	final, err := tea.NewProgram(altScreenModel{inner: m}).Run()
	if a, ok := final.(altScreenModel); ok {
		return a.inner, err
	}
	return final, err
}

// RunInline starts a Bubble Tea program without taking over the full screen.
// Output scrolls inline in the terminal, remaining visible after exit.
func RunInline(m tea.Model) (tea.Model, error) {
	return tea.NewProgram(m).Run()
}
