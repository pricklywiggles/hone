package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jmoiron/sqlx"
	"github.com/pricklywiggles/hone/internal/config"
	"github.com/pricklywiggles/hone/internal/store"
)

type tabID int

const (
	tabStats     tabID = iota
	tabProblems
	tabPlaylists
	tabTopics
)

// tabActivatedMsg is sent to a tab model when it becomes the active tab.
// Tabs use this to refresh their data.
type tabActivatedMsg struct{}

// ── Styles ────────────────────────────────────────────────────────────────────

var (
	dashLogoStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAccent)

	dashActiveTabStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorAccent).
				Underline(true).
				Padding(0, 1)

	dashInactiveTabStyle = lipgloss.NewStyle().
				Foreground(colorDim).
				Padding(0, 1)

	dashDividerStyle = lipgloss.NewStyle().
				Foreground(colorDimBg)

	dashTabBarStyle = lipgloss.NewStyle().
			PaddingLeft(1)
)

// ── Dashboard model ───────────────────────────────────────────────────────────

type DashboardModel struct {
	active     tabID
	stats      StatsTabModel
	problems   ProblemsTabModel
	playlists  PlaylistHubModel
	topics     TopicsTabModel
	width      int
	height     int
	db         *sqlx.DB
	profileDir string
	filter     store.PracticeFilter
}

func NewDashboardModel(db *sqlx.DB, profileDir string, filter store.PracticeFilter) DashboardModel {
	tabH := 3  // tab bar height (2 lines + divider)
	contentH := 24 - tabH

	return DashboardModel{
		active:     tabStats,
		stats:      NewStatsTabModel(db, filter, contentH),
		problems:   NewProblemsTabModel(db, profileDir, filter, contentH),
		playlists:  NewPlaylistHubModel(db, profileDir, filter.PlaylistID),
		topics:     NewTopicsTabModel(db, contentH),
		db:         db,
		profileDir: profileDir,
		filter:     filter,
	}
}

func (m DashboardModel) Init() tea.Cmd {
	return tea.Batch(
		m.stats.loadCmd(),
		m.problems.loadCmd(),
		m.topics.loadCmd(),
		m.playlists.Init(),
	)
}

func (m DashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		contentH := msg.Height - 3
		m.stats = m.stats.withHeight(contentH)
		m.problems = m.problems.withHeight(contentH)
		m.topics = m.topics.withHeight(contentH)
		// Pass adjusted size to playlists so it doesn't overflow past the tab bar.
		adjusted := tea.WindowSizeMsg{Width: msg.Width, Height: contentH}
		var cmd tea.Cmd
		pm, c := m.playlists.Update(adjusted)
		m.playlists = pm.(PlaylistHubModel)
		cmd = c
		return m, cmd

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			if !m.activeTabFiltering() {
				return m, Pop()
			}
		case "p":
			if !m.activeTabFiltering() {
				filter := m.filter
				db := m.db
				profileDir := m.profileDir
				return m, func() tea.Msg {
					problem, srsState, isDue, err := store.PickNext(db, filter)
					if err != nil || problem == nil {
						return nil
					}
					return PushMsg{Model: NewPracticeModel(db, profileDir, problem, srsState, isDue, filter)}
				}
			}
		case "a":
			if !m.activeTabFiltering() && m.active != tabPlaylists {
				add := NewAddModel(m.db, m.profileDir, "")
				add.standalone = false
				return m, func() tea.Msg { return PushMsg{Model: add} }
			}
		case "1":
			if !m.activeTabFiltering() {
				return m.switchTab(tabStats)
			}
		case "2":
			if !m.activeTabFiltering() {
				return m.switchTab(tabProblems)
			}
		case "3":
			if !m.activeTabFiltering() {
				return m.switchTab(tabPlaylists)
			}
		case "4":
			if !m.activeTabFiltering() {
				return m.switchTab(tabTopics)
			}
		case "left":
			if !m.activeTabFiltering() {
				prev := (int(m.active) - 1 + 4) % 4
				return m.switchTab(tabID(prev))
			}
		case "right":
			if !m.activeTabFiltering() {
				next := (int(m.active) + 1) % 4
				return m.switchTab(tabID(next))
			}
		}
	}

	return m.routeToActive(msg)
}

func (m DashboardModel) switchTab(t tabID) (DashboardModel, tea.Cmd) {
	m.active = t
	m.filter = store.PracticeFilter{
		PlaylistID: config.ActivePlaylistID(),
		TopicID:    config.ActiveTopicID(),
	}
	m.problems.filter = m.filter
	m.stats.filter = m.filter
	var cmd tea.Cmd
	switch t {
	case tabStats:
		m.stats, cmd = m.stats.activated()
	case tabProblems:
		m.problems, cmd = m.problems.activated()
	case tabPlaylists:
		cmd = loadPlaylists(m.db)
	case tabTopics:
		m.topics, cmd = m.topics.activated()
	}
	return m, cmd
}

func (m DashboardModel) routeToActive(msg tea.Msg) (DashboardModel, tea.Cmd) {
	switch m.active {
	case tabStats:
		newModel, cmd := m.stats.Update(msg)
		m.stats = newModel.(StatsTabModel)
		return m, cmd

	case tabProblems:
		newModel, cmd := m.problems.Update(msg)
		m.problems = newModel.(ProblemsTabModel)
		return m, cmd

	case tabPlaylists:
		newModel, cmd := m.playlists.Update(msg)
		m.playlists = newModel.(PlaylistHubModel)
		return m, cmd

	case tabTopics:
		newModel, cmd := m.topics.Update(msg)
		m.topics = newModel.(TopicsTabModel)
		return m, cmd
	}
	return m, nil
}

func (m DashboardModel) View() string {
	var b strings.Builder
	b.WriteString(m.renderTabBar())
	b.WriteString("\n")

	switch m.active {
	case tabStats:
		b.WriteString(m.stats.View())
	case tabProblems:
		b.WriteString(m.problems.View())
	case tabPlaylists:
		b.WriteString(m.playlists.View())
	case tabTopics:
		b.WriteString(m.topics.View())
	}

	return b.String()
}

func (m DashboardModel) renderTabBar() string {
	type tabDef struct {
		id    tabID
		label string
	}
	tabs := []tabDef{
		{tabStats, "1:Stats"},
		{tabProblems, "2:Problems"},
		{tabPlaylists, "3:Playlists"},
		{tabTopics, "4:Topics"},
	}

	logo := dashLogoStyle.Render("hone")

	var parts []string
	for _, t := range tabs {
		if t.id == m.active {
			parts = append(parts, dashActiveTabStyle.Render(t.label))
		} else {
			parts = append(parts, dashInactiveTabStyle.Render(t.label))
		}
	}

	// Active filter indicator at far right
	filterNote := ""
	if m.filter.PlaylistID != nil {
		filterNote = dashInactiveTabStyle.Render("● playlist active")
	} else if m.filter.TopicID != nil {
		filterNote = dashInactiveTabStyle.Render("● topic active")
	}

	tabRow := logo + "  " + strings.Join(parts, " ")
	if filterNote != "" && m.width > 60 {
		pad := m.width - lipgloss.Width(tabRow) - lipgloss.Width(filterNote) - 2
		if pad > 0 {
			tabRow += strings.Repeat(" ", pad) + filterNote
		}
	}

	divider := dashDividerStyle.Render(strings.Repeat("─", max(m.width, 60)))
	return dashTabBarStyle.Render(tabRow) + "\n" + divider
}

func (m DashboardModel) activeTabFiltering() bool {
	switch m.active {
	case tabPlaylists:
		return m.playlists.isFiltering()
	case tabProblems:
		return m.problems.isFiltering()
	}
	return false
}

func (m PlaylistHubModel) isFiltering() bool {
	return m.list.FilterState() == 1 // list.Filtering = 1
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
