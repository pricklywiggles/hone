package tui

import tea "charm.land/bubbletea/v2"

// PushMsg pushes a new model onto the navigation stack.
type PushMsg struct{ Model tea.Model }

// PopMsg pops the top model off the navigation stack.
// If the stack would become empty, the program quits.
type PopMsg struct{}

// Router is a stack-based model router. Navigation is driven by PushMsg/PopMsg.
// All sub-models can remain unaware of the stack — they just send PopMsg to "go back".
type Router struct {
	stack []tea.Model
}

// Pop returns a Cmd that sends PopMsg, navigating back to the previous screen.
func Pop() tea.Cmd {
	return func() tea.Msg { return PopMsg{} }
}

func NewRouter(initial tea.Model) Router {
	return Router{stack: []tea.Model{initial}}
}

func (r Router) Init() tea.Cmd {
	if len(r.stack) == 0 {
		return tea.Quit
	}
	return r.stack[0].Init()
}

func (r Router) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case PushMsg:
		r.stack = append(r.stack, msg.Model)
		return r, msg.Model.Init()

	case PopMsg:
		if len(r.stack) <= 1 {
			return r, tea.Quit
		}
		r.stack = r.stack[:len(r.stack)-1]
		return r, nil
	}

	if len(r.stack) == 0 {
		return r, tea.Quit
	}
	top := r.stack[len(r.stack)-1]
	newTop, cmd := top.Update(msg)
	r.stack[len(r.stack)-1] = newTop
	return r, cmd
}

func (r Router) View() tea.View {
	if len(r.stack) == 0 {
		return tea.NewView("")
	}
	return r.stack[len(r.stack)-1].View()
}
