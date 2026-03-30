package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jmoiron/sqlx"
	"github.com/pricklywiggles/hone/internal/store"
)

// ── Messages ──────────────────────────────────────────────────────────────────

type topicsLoadedMsg struct{ rows []store.TopicStat }
type topicsErrMsg struct{ err error }

// ── Model ─────────────────────────────────────────────────────────────────────

type TopicsTabModel struct {
	table  colorTable
	rows   []store.TopicStat
	loaded bool
	height int
	db     *sqlx.DB
	help   help.Model
}

func NewTopicsTabModel(db *sqlx.DB, height int) TopicsTabModel {
	t := newTopicsTable(nil, topicsBodyHeight(height))
	return TopicsTabModel{table: t, db: db, height: height, help: newHelpModel()}
}

func topicsBodyHeight(h int) int {
	reserved := 8 // header(2) + count line(3) + help(3)
	if h-reserved < 5 {
		return 5
	}
	return h - reserved
}

func (m TopicsTabModel) withHeight(h int) TopicsTabModel {
	m.height = h
	m.table.SetHeight(topicsBodyHeight(h))
	return m
}

func (m TopicsTabModel) loadCmd() tea.Cmd {
	return func() tea.Msg {
		rows, err := store.GetTopicStats(m.db)
		if err != nil {
			return topicsErrMsg{err}
		}
		return topicsLoadedMsg{rows}
	}
}

func (m TopicsTabModel) activated() (TopicsTabModel, tea.Cmd) {
	return m, m.loadCmd()
}

func (m TopicsTabModel) Init() tea.Cmd { return nil }

func (m TopicsTabModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case topicsLoadedMsg:
		m.rows = msg.rows
		m.table.SetRows(buildTopicRows(msg.rows))
		m.loaded = true
		return m, nil

	case topicsErrMsg:
		m.loaded = true
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "r" {
			return m, m.loadCmd()
		}
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m TopicsTabModel) View() string {
	if !m.loaded {
		return "\n  " + statsDimStyle.Render("loading…")
	}

	var b strings.Builder
	b.WriteString("\n  ")
	b.WriteString(statsSectionStyle.Render(fmt.Sprintf("%d topics", len(m.rows))))
	b.WriteString("\n\n")
	b.WriteString(m.table.View())
	b.WriteString("\n\n  ")
	b.WriteString(m.help.View(topicsKeyMap{}))
	return b.String()
}

// ── Table helpers ─────────────────────────────────────────────────────────────

func newTopicsTable(_ [][]string, height int) colorTable {
	return newColorTable([]ctColumn{
		{title: "Topic", width: 22},
		{title: "Progress", width: 16},
		{title: "Total", width: 6},
		{title: "Mastered", width: 9},
		{title: "Due", width: 5},
		{title: "Rate", width: 6},
	}, height)
}

func buildTopicRows(rows []store.TopicStat) [][]string {
	out := make([][]string, len(rows))
	for i, r := range rows {
		notMastered := r.Attempted - r.Mastered
		if notMastered < 0 {
			notMastered = 0
		}
		bar := renderSegmentedBar(r.Total, 16,
			barSegment{value: r.Mastered, color: barMasteredColor},
			barSegment{value: notMastered, color: barAttemptColor},
		)

		rateStr := statsDimStyle.Render("—")
		if r.SuccessRate >= 0 {
			pct := int(r.SuccessRate * 100)
			rateStr = lipgloss.NewStyle().Foreground(rateColor(pct)).Render(fmt.Sprintf("%d%%", pct))
		}

		dueStr := statsDimStyle.Render("—")
		if r.DueToday > 0 {
			dueStr = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render(fmt.Sprintf("%d", r.DueToday))
		}

		out[i] = []string{
			truncate(r.Name, 22),
			bar,
			fmt.Sprintf("%d", r.Total),
			fmt.Sprintf("%d", r.Mastered),
			dueStr,
			rateStr,
		}
	}
	return out
}

func rateColor(pct int) lipgloss.Color {
	switch {
	case pct >= 75:
		return lipgloss.Color("10") // green
	case pct >= 50:
		return lipgloss.Color("11") // yellow
	default:
		return lipgloss.Color("9") // red
	}
}
