package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/pricklywiggles/hone/internal/store"
)

// playlistItem implements list.Item for store.Playlist.
type playlistItem struct {
	playlist store.Playlist
	active   bool
}

func (i playlistItem) Title() string {
	if i.active {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("62")).Bold(true).Render("* " + i.playlist.Name)
	}
	return "  " + i.playlist.Name
}

func (i playlistItem) Description() string {
	return fmt.Sprintf("  %d problems", i.playlist.ProblemCount)
}

func (i playlistItem) FilterValue() string { return i.playlist.Name }

// PlaylistSelectModel is a bubbles/list-based playlist picker.
// Standalone use: call tui.Run(m) then m.Selected() / m.Canceled().
// Embedded use: check Done() in parent Update after routing messages to it.
// NOTE: calls tea.Quit on selection/cancel — refactor for embedding in stats dashboard.
type PlaylistSelectModel struct {
	list     list.Model
	selected *store.Playlist
	done     bool
	canceled bool
}

func NewPlaylistSelectModel(playlists []store.Playlist, activeID *int) PlaylistSelectModel {
	items := make([]list.Item, len(playlists))
	for i, p := range playlists {
		active := activeID != nil && *activeID == p.ID
		items[i] = playlistItem{playlist: p, active: active}
	}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color("62")).
		BorderLeftForeground(lipgloss.Color("62"))
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(lipgloss.Color("241")).
		BorderLeftForeground(lipgloss.Color("62"))

	l := list.New(items, delegate, 44, 20)
	l.Title = "Select Playlist"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62"))

	return PlaylistSelectModel{list: l}
}

func (m PlaylistSelectModel) Selected() *store.Playlist  { return m.selected }
func (m PlaylistSelectModel) Done() bool                 { return m.done }
func (m PlaylistSelectModel) Canceled() bool             { return m.canceled }

func (m PlaylistSelectModel) Init() tea.Cmd { return nil }

func (m PlaylistSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.canceled = true
			m.done = true
			return m, tea.Quit
		case "enter":
			if item, ok := m.list.SelectedItem().(playlistItem); ok {
				p := item.playlist
				m.selected = &p
				m.done = true
				return m, tea.Quit
			}
		}
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height-2)
		return m, nil
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m PlaylistSelectModel) View() string {
	return "\n" + m.list.View()
}
