package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jmoiron/sqlx"
	"github.com/pricklywiggles/hone/internal/config"
	"github.com/pricklywiggles/hone/internal/store"
)

// ── shared list item ────────────────────────────────────────────────────────

type playlistItem struct {
	playlist store.Playlist
	active   bool
}

func (i playlistItem) Title() string { return i.playlist.Name }

func (i playlistItem) Description() string {
	if i.active {
		return fmt.Sprintf("● active · %d problems", i.playlist.ProblemCount)
	}
	return fmt.Sprintf("%d problems", i.playlist.ProblemCount)
}

func (i playlistItem) FilterValue() string { return i.playlist.Name }

// noneItem is the "None (all problems)" sentinel at the top of the list.
type noneItem struct{ active bool }

func (i noneItem) Title() string       { return "None (all problems)" }
func (i noneItem) Description() string {
	if i.active {
		return "● active · practice from the full problem set"
	}
	return "practice from the full problem set"
}
func (i noneItem) FilterValue() string { return "none all problems" }

func makePlaylistItems(playlists []store.Playlist, activeID *int) []list.Item {
	items := make([]list.Item, 0, len(playlists)+1)
	items = append(items, noneItem{active: activeID == nil})
	for _, p := range playlists {
		active := activeID != nil && *activeID == p.ID
		items = append(items, playlistItem{playlist: p, active: active})
	}
	return items
}

func newPlaylistList(items []list.Item, w, h int) list.Model {
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color("62")).
		BorderLeftForeground(lipgloss.Color("62"))
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(lipgloss.Color("241")).
		BorderLeftForeground(lipgloss.Color("62"))
	l := list.New(items, delegate, w, h)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62"))
	return l
}

// ── PlaylistSelectModel ──────────────────────────────────────────────────────
// Embeddable picker: standalone via tui.Run(), or check Done()/Selected() when
// embedded in a parent model. Calls tea.Quit on selection/cancel.

type PlaylistSelectModel struct {
	list     list.Model
	selected *store.Playlist
	done     bool
	canceled bool
}

func NewPlaylistSelectModel(playlists []store.Playlist, activeID *int) PlaylistSelectModel {
	l := newPlaylistList(makePlaylistItems(playlists, activeID), 44, 20)
	l.Title = "Select Playlist"
	return PlaylistSelectModel{list: l}
}

func (m PlaylistSelectModel) Selected() *store.Playlist { return m.selected }
func (m PlaylistSelectModel) Done() bool                { return m.done }
func (m PlaylistSelectModel) Canceled() bool            { return m.canceled }

func (m PlaylistSelectModel) Init() tea.Cmd { return nil }

func (m PlaylistSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.canceled, m.done = true, true
			return m, tea.Quit
		case "enter":
			if item, ok := m.list.SelectedItem().(playlistItem); ok {
				p := item.playlist
				m.selected, m.done = &p, true
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

// ── PlaylistHubModel ─────────────────────────────────────────────────────────
// Full playlist manager: view list, select active, create new — all in one TUI.

type hubState int

const (
	hubList   hubState = iota
	hubCreate
	hubPicker
)

type playlistsLoadedMsg struct{ playlists []store.Playlist }
type playlistHubErrMsg struct{ err error }

type PlaylistHubModel struct {
	state     hubState
	playlists []store.Playlist
	activeID  *int
	list      list.Model
	input     textinput.Model
	picker    PlaylistPickerModel
	statusMsg string
	width     int
	height    int
	db        *sqlx.DB
}

func NewPlaylistHubModel(db *sqlx.DB, activeID *int) PlaylistHubModel {
	ti := textinput.New()
	ti.Placeholder = "playlist name"
	ti.CharLimit = 80
	ti.Width = 30

	l := newPlaylistList(nil, 60, 20)
	l.Title = "Playlists"

	return PlaylistHubModel{
		db:       db,
		activeID: activeID,
		list:     l,
		input:    ti,
		width:    60,
		height:   24,
	}
}

func (m PlaylistHubModel) Init() tea.Cmd {
	return loadPlaylists(m.db)
}

func loadPlaylists(db *sqlx.DB) tea.Cmd {
	return func() tea.Msg {
		playlists, err := store.ListPlaylists(db)
		if err != nil {
			return playlistHubErrMsg{err}
		}
		return playlistsLoadedMsg{playlists}
	}
}

func activatePlaylist(db *sqlx.DB, id int) tea.Cmd {
	return func() tea.Msg {
		if err := config.SetActivePlaylist(id); err != nil {
			return playlistHubErrMsg{err}
		}
		return loadPlaylists(db)()
	}
}

func clearActivePlaylist(db *sqlx.DB) tea.Cmd {
	return func() tea.Msg {
		if err := config.ClearActivePlaylist(); err != nil {
			return playlistHubErrMsg{err}
		}
		return loadPlaylists(db)()
	}
}

func createPlaylist(db *sqlx.DB, name string) tea.Cmd {
	return func() tea.Msg {
		_, err := store.CreatePlaylist(db, name)
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE constraint failed") {
				return playlistHubErrMsg{fmt.Errorf("playlist %q already exists", name)}
			}
			return playlistHubErrMsg{err}
		}
		return loadPlaylists(db)()
	}
}

func (m PlaylistHubModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case playlistsLoadedMsg:
		m.playlists = msg.playlists
		m.list.SetItems(makePlaylistItems(m.playlists, m.activeID))
		m.resizeList()
		return m, nil

	case playlistHubErrMsg:
		m.statusMsg = hubErrStyle.Render("✗ " + msg.err.Error())
		m.state = hubList
		return m, nil

	case playlistPickerDoneMsg:
		m.state = hubList
		m.statusMsg = hubOKStyle.Render(fmt.Sprintf("✓ %d added, %d removed", msg.added, msg.removed))
		return m, loadPlaylists(m.db)

	case PopMsg:
		if m.state == hubPicker {
			m.state = hubList
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resizeList()
		return m, nil

	case tea.KeyMsg:
		if m.state == hubCreate {
			return m.updateCreate(msg)
		}
		if m.state == hubPicker {
			newPicker, cmd := m.picker.Update(msg)
			m.picker = newPicker.(PlaylistPickerModel)
			return m, cmd
		}
		return m.updateList(msg)
	}

	if m.state == hubPicker {
		newPicker, cmd := m.picker.Update(msg)
		m.picker = newPicker.(PlaylistPickerModel)
		return m, cmd
	}
	if m.state == hubList {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m PlaylistHubModel) updateList(msg tea.KeyMsg) (PlaylistHubModel, tea.Cmd) {
	// While the list's filter input is active, pass all keys straight through.
	if m.list.FilterState() == list.Filtering {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}

	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "n":
		m.state = hubCreate
		m.input.SetValue("")
		m.input.Focus()
		m.statusMsg = ""
		m.resizeList()
		return m, textinput.Blink
	case "a":
		if item, ok := m.list.SelectedItem().(playlistItem); ok {
			m.state = hubPicker
			m.picker = NewPlaylistPickerModel(m.db, item.playlist.ID, item.playlist.Name, m.height)
			return m, m.picker.Init()
		}
	case "enter":
		switch item := m.list.SelectedItem().(type) {
		case noneItem:
			m.activeID = nil
			m.statusMsg = hubOKStyle.Render("✓ Playlist cleared — using all problems")
			return m, clearActivePlaylist(m.db)
		case playlistItem:
			id := item.playlist.ID
			m.activeID = &id
			m.statusMsg = hubOKStyle.Render(fmt.Sprintf("✓ Active playlist set to %q", item.playlist.Name))
			return m, activatePlaylist(m.db, id)
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m PlaylistHubModel) updateCreate(msg tea.KeyMsg) (PlaylistHubModel, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyEsc:
		m.state = hubList
		m.resizeList()
		return m, nil
	case tea.KeyEnter:
		name := strings.TrimSpace(m.input.Value())
		if name == "" {
			return m, nil
		}
		m.state = hubList
		m.statusMsg = hubOKStyle.Render(fmt.Sprintf("✓ Created playlist %q", name))
		m.resizeList()
		return m, createPlaylist(m.db, name)
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *PlaylistHubModel) resizeList() {
	reserved := 4 // help line + status line + padding
	if m.state == hubCreate {
		reserved += 3
	}
	h := m.height - reserved
	if h < 4 {
		h = 4
	}
	m.list.SetSize(m.width, h)
}

// Styles for the hub
var (
	hubOKStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	hubErrStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	hubHelpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	hubInputLabelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("62")).Bold(true)
)

func (m PlaylistHubModel) View() string {
	if m.state == hubPicker {
		return m.picker.View()
	}

	var b strings.Builder

	b.WriteString(m.list.View())
	b.WriteString("\n")

	if m.state == hubCreate {
		b.WriteString("\n  ")
		b.WriteString(hubInputLabelStyle.Render("New playlist:"))
		b.WriteString("  ")
		b.WriteString(m.input.View())
		b.WriteString("\n  ")
		b.WriteString(hubHelpStyle.Render("enter to create • esc to cancel"))
		b.WriteString("\n")
	} else {
		if m.statusMsg != "" {
			b.WriteString("  ")
			b.WriteString(m.statusMsg)
			b.WriteString("\n")
		}
		b.WriteString("  ")
		b.WriteString(hubHelpStyle.Render("enter: set active • a: add problems • n: new • /: filter • q: quit"))
		b.WriteString("\n")
	}

	return b.String()
}
