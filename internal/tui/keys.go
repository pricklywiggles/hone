package tui

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
)

func newHelpModel() help.Model {
	h := help.New()
	h.Styles.ShortKey = lipgloss.NewStyle().Foreground(lipgloss.Color("62")).Bold(true)
	h.Styles.ShortDesc = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	h.Styles.ShortSeparator = lipgloss.NewStyle().Foreground(lipgloss.Color("237"))
	h.ShortSeparator = "  •  "
	return h
}

// ── Stats tab ────────────────────────────────────────────────────────────────

type statsKeyMap struct{}

func (statsKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "practice")),
		key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add problem")),
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	}
}
func (statsKeyMap) FullHelp() [][]key.Binding { return nil }

// ── Problems tab ─────────────────────────────────────────────────────────────

type problemsKeyMap struct{ filtering bool }

func (k problemsKeyMap) ShortHelp() []key.Binding {
	if k.filtering {
		return []key.Binding{
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter/esc", "done filtering")),
		}
	}
	return []key.Binding{
		key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "practice next")),
		key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add problem")),
		key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "sort")),
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "practice this")),
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	}
}
func (problemsKeyMap) FullHelp() [][]key.Binding { return nil }

// ── Topics tab ───────────────────────────────────────────────────────────────

type topicsKeyMap struct{}

func (topicsKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "practice")),
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "set topic")),
		key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "sort")),
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	}
}
func (topicsKeyMap) FullHelp() [][]key.Binding { return nil }

// ── Playlist hub ─────────────────────────────────────────────────────────────

type playlistKeyMap struct{ creating bool }

func (k playlistKeyMap) ShortHelp() []key.Binding {
	if k.creating {
		return []key.Binding{
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "create")),
			key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
		}
	}
	return []key.Binding{
		key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "practice")),
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "set active")),
		key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add problems")),
		key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new")),
		key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
	}
}
func (playlistKeyMap) FullHelp() [][]key.Binding { return nil }

// ── Playlist picker ──────────────────────────────────────────────────────────

type pickerKeyMap struct{}

func (pickerKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑↓", "navigate")),
		key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "toggle")),
		key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add new")),
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
	}
}
func (pickerKeyMap) FullHelp() [][]key.Binding { return nil }

// ── Practice ─────────────────────────────────────────────────────────────────

type practiceWaitingKeyMap struct{}

func (practiceWaitingKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
	}
}
func (practiceWaitingKeyMap) FullHelp() [][]key.Binding { return nil }

type practiceDoneKeyMap struct{}

func (practiceDoneKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("n", "enter"), key.WithHelp("n/enter", "next problem")),
		key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
	}
}
func (practiceDoneKeyMap) FullHelp() [][]key.Binding { return nil }

// ── Add ──────────────────────────────────────────────────────────────────────

type addInputKeyMap struct{}

func (addInputKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "submit URL")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
	}
}
func (addInputKeyMap) FullHelp() [][]key.Binding { return nil }

type addDoneKeyMap struct{}

func (addDoneKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("any"), key.WithHelp("any key", "exit")),
	}
}
func (addDoneKeyMap) FullHelp() [][]key.Binding { return nil }
