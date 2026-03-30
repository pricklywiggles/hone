package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
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
	table  table.Model
	rows   []store.TopicStat
	loaded bool
	height int
	db     *sqlx.DB
}

func NewTopicsTabModel(db *sqlx.DB, height int) TopicsTabModel {
	t := newTopicsTable(nil, topicsBodyHeight(height))
	return TopicsTabModel{table: t, db: db, height: height}
}

func topicsBodyHeight(h int) int {
	reserved := 5
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
	b.WriteString(statsDimStyle.Render("r: refresh"))
	return b.String()
}

// ── Table helpers ─────────────────────────────────────────────────────────────

func newTopicsTable(rows []table.Row, height int) table.Model {
	cols := []table.Column{
		{Title: "Topic", Width: 26},
		{Title: "Problems", Width: 9},
		{Title: "Mastered", Width: 9},
		{Title: "Due Today", Width: 10},
		{Title: "Success Rate", Width: 14},
	}

	t := table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(height),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("237")).
		BorderBottom(true).
		Foreground(lipgloss.Color("241")).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("15")).
		Background(lipgloss.Color("62")).
		Bold(true)
	t.SetStyles(s)

	return t
}

func buildTopicRows(rows []store.TopicStat) []table.Row {
	out := make([]table.Row, len(rows))
	for i, r := range rows {
		rateStr := statsDimStyle.Render("—")
		if r.SuccessRate >= 0 {
			pct := int(r.SuccessRate * 100)
			color := rateColor(pct)
			rateStr = lipgloss.NewStyle().Foreground(color).Render(fmt.Sprintf("%d%%", pct))
			rateStr += "  " + renderBar(r.SuccessRate, 6, string(color))
		}

		dueStr := statsDimStyle.Render("—")
		if r.DueToday > 0 {
			dueStr = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render(fmt.Sprintf("%d", r.DueToday))
		}

		out[i] = table.Row{
			truncate(r.Name, 26),
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
