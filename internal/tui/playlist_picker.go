package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jmoiron/sqlx"
	"github.com/pricklywiggles/hone/internal/store"
)

// playlistPickerDoneMsg is sent when the user confirms their selection.
type playlistPickerDoneMsg struct{ added, removed int }

// ── Styles ────────────────────────────────────────────────────────────────────

var (
	pickerCheckedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	pickerUncheckedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	pickerSelectedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("62")).Bold(true)
	pickerDiffStyle      = map[string]lipgloss.Style{
		"easy":   lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
		"medium": lipgloss.NewStyle().Foreground(lipgloss.Color("11")),
		"hard":   lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
	}
)

// ── Model ─────────────────────────────────────────────────────────────────────

type PlaylistPickerModel struct {
	playlistID   int
	playlistName string
	all          []store.SimpleProblem
	checked      []bool
	selected     []bool
	visible      []int
	cursor       int
	filterInput  textinput.Model
	height       int
	db           *sqlx.DB
	help         help.Model
}

func NewPlaylistPickerModel(db *sqlx.DB, playlistID int, playlistName string, height int) PlaylistPickerModel {
	ti := textinput.New()
	ti.Placeholder = "search problems…"
	ti.CharLimit = 80
	ti.Width = 40

	return PlaylistPickerModel{
		db:           db,
		playlistID:   playlistID,
		playlistName: playlistName,
		height:       height,
		filterInput:  ti,
		help:         newHelpModel(),
	}
}

func (m PlaylistPickerModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.loadCmd())
}

func (m PlaylistPickerModel) loadCmd() tea.Cmd {
	return func() tea.Msg {
		problems, checked, err := store.ListAllProblemsForPicker(m.db, m.playlistID)
		if err != nil {
			return playlistPickerLoadedMsg{err: err}
		}
		return playlistPickerLoadedMsg{problems: problems, checked: checked}
	}
}

type playlistPickerLoadedMsg struct {
	problems []store.SimpleProblem
	checked  []bool
	err      error
}

func (m PlaylistPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case playlistPickerLoadedMsg:
		if msg.err != nil {
			return m, nil
		}
		m.all = msg.problems
		m.checked = msg.checked
		m.selected = make([]bool, len(msg.checked))
		copy(m.selected, msg.checked)
		m.rebuildVisible()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	var cmd tea.Cmd
	m.filterInput, cmd = m.filterInput.Update(msg)
	return m, cmd
}

func (m PlaylistPickerModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "esc":
		// Handled by the parent (PlaylistHubModel intercepts PopMsg).
		return m, Pop()

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}

	case "down", "j":
		if m.cursor < len(m.visible)-1 {
			m.cursor++
		}

	case " ":
		if len(m.visible) > 0 && m.cursor < len(m.visible) {
			idx := m.visible[m.cursor]
			m.selected[idx] = !m.selected[idx]
		}

	case "enter":
		return m, m.confirmCmd()

	default:
		var cmd tea.Cmd
		m.filterInput, cmd = m.filterInput.Update(msg)
		m.rebuildVisible()
		m.cursor = 0
		return m, cmd
	}

	return m, nil
}

func (m PlaylistPickerModel) confirmCmd() tea.Cmd {
	return func() tea.Msg {
		added, removed := 0, 0
		for i, sp := range m.all {
			wasIn := m.checked[i]
			nowIn := m.selected[i]
			if !wasIn && nowIn {
				if err := store.AddProblemToPlaylist(m.db, m.playlistID, sp.ID); err == nil {
					added++
				}
			} else if wasIn && !nowIn {
				if err := store.RemoveProblemFromPlaylist(m.db, m.playlistID, sp.ID); err == nil {
					removed++
				}
			}
		}
		return playlistPickerDoneMsg{added: added, removed: removed}
	}
}

func (m *PlaylistPickerModel) rebuildVisible() {
	query := strings.ToLower(m.filterInput.Value())
	m.visible = m.visible[:0]
	for i, p := range m.all {
		if query == "" || strings.Contains(strings.ToLower(p.Title), query) {
			m.visible = append(m.visible, i)
		}
	}
}

func (m PlaylistPickerModel) View() string {
	var b strings.Builder

	b.WriteString("\n  ")
	b.WriteString(statsSectionStyle.Render(fmt.Sprintf(`Add problems to "%s"`, m.playlistName)))
	b.WriteString("\n\n  ")
	b.WriteString(m.filterInput.View())
	b.WriteString("\n\n")

	// Compute visible window
	viewH := m.height - 8
	if viewH < 4 {
		viewH = 4
	}
	start := 0
	if m.cursor >= viewH {
		start = m.cursor - viewH + 1
	}
	end := start + viewH
	if end > len(m.visible) {
		end = len(m.visible)
	}

	for i := start; i < end; i++ {
		idx := m.visible[i]
		p := m.all[idx]
		isCursor := i == m.cursor

		check := pickerUncheckedStyle.Render("[ ]")
		if m.selected[idx] {
			check = pickerCheckedStyle.Render("[✓]")
		}

		diffS, ok := pickerDiffStyle[p.Difficulty]
		if !ok {
			diffS = lipgloss.NewStyle()
		}
		diff := diffS.Render(fmt.Sprintf("%-6s", p.Difficulty))

		title := truncate(p.Title, 36)
		if isCursor {
			title = pickerSelectedStyle.Render(title)
			b.WriteString("  > " + check + "  " + title + "  " + diff + "\n")
		} else {
			b.WriteString("    " + check + "  " + title + "  " + diff + "\n")
		}
	}

	if len(m.visible) == 0 {
		b.WriteString("  " + statsDimStyle.Render("no problems found") + "\n")
	}

	// Count selected
	selectedCount := 0
	for _, s := range m.selected {
		if s {
			selectedCount++
		}
	}

	b.WriteString(fmt.Sprintf("\n  %s  %s\n",
		m.help.View(pickerKeyMap{}),
		pickerCheckedStyle.Render(fmt.Sprintf("%d selected", selectedCount)),
	))

	return b.String()
}
