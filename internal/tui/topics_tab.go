package tui

import (
	"cmp"
	"fmt"
	"image/color"
	"slices"
	"strings"

	"charm.land/bubbles/v2/help"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jmoiron/sqlx"
	"github.com/pricklywiggles/hone/internal/store"
)

// ── Sort modes ────────────────────────────────────────────────────────────────

type topicSortMode int

const (
	topicSortAlpha    topicSortMode = iota // default: alphabetical
	topicSortMastered                      // % mastered descending
	topicSortWeakest                       // weakest success rate first
)

func (s topicSortMode) label() string {
	switch s {
	case topicSortMastered:
		return "% mastered"
	case topicSortWeakest:
		return "weakest first"
	default:
		return "alphabetical"
	}
}

// ── Messages ──────────────────────────────────────────────────────────────────

type topicsLoadedMsg struct{ rows []store.TopicStat }
type topicsErrMsg struct{ err error }
type topicSetMsg struct{ name string }
type topicClearedMsg struct{}

// ── Model ─────────────────────────────────────────────────────────────────────

type TopicsTabModel struct {
	table         colorTable
	rows          []store.TopicStat
	sorted        []store.TopicStat
	sort          topicSortMode
	loaded        bool
	height        int
	db            *sqlx.DB
	activeTopicID *int
	statusMsg     string
	help          help.Model
}

func NewTopicsTabModel(db *sqlx.DB, height int) TopicsTabModel {
	t := newTopicsTable(nil, topicsBodyHeight(height))
	activeTopicID, _ := store.ActiveTopicID(db)
	return TopicsTabModel{table: t, db: db, height: height, activeTopicID: activeTopicID, help: newHelpModel()}
}

func (m *TopicsTabModel) applySort() {
	sorted := slices.Clone(m.rows)
	switch m.sort {
	case topicSortMastered:
		slices.SortStableFunc(sorted, func(a, b store.TopicStat) int {
			pa, pb := 0.0, 0.0
			if a.Total > 0 {
				pa = float64(a.Mastered) / float64(a.Total)
			}
			if b.Total > 0 {
				pb = float64(b.Mastered) / float64(b.Total)
			}
			return cmp.Compare(pb, pa)
		})
	case topicSortAlpha:
		slices.SortStableFunc(sorted, func(a, b store.TopicStat) int {
			return cmp.Compare(a.Name, b.Name)
		})
	default: // topicSortWeakest: success rate asc, unattempted last
		slices.SortStableFunc(sorted, func(a, b store.TopicStat) int {
			if a.SuccessRate < 0 && b.SuccessRate < 0 {
				return 0
			}
			if a.SuccessRate < 0 {
				return 1
			}
			if b.SuccessRate < 0 {
				return -1
			}
			return cmp.Compare(a.SuccessRate, b.SuccessRate)
		})
	}
	m.sorted = sorted
	m.table.SetRows(buildTopicRows(sorted, m.activeTopicID))
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
	m.activeTopicID, _ = store.ActiveTopicID(m.db)
	return m, m.loadCmd()
}

func (m TopicsTabModel) Init() tea.Cmd { return nil }

func (m TopicsTabModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case topicsLoadedMsg:
		m.rows = msg.rows
		m.applySort()
		m.loaded = true
		return m, nil

	case topicsErrMsg:
		m.loaded = true
		return m, nil

	case topicSetMsg:
		m.statusMsg = hubOKStyle.Render(fmt.Sprintf("✓ Practicing topic %q", msg.name))
		m.applySort()
		return m, nil

	case topicClearedMsg:
		m.statusMsg = hubOKStyle.Render("✓ Topic filter cleared")
		m.applySort()
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "r":
			return m, m.loadCmd()
		case "s":
			m.sort = (m.sort + 1) % 3
			m.applySort()
			return m, nil
		case "enter":
			if idx := m.table.cursor; idx >= 0 && idx < len(m.sorted) {
				topic := m.sorted[idx]
				if m.activeTopicID != nil && *m.activeTopicID == topic.ID {
					m.activeTopicID = nil
					db := m.db
					return m, func() tea.Msg {
						_ = store.ClearActiveTopic(db)
						return topicClearedMsg{}
					}
				}
				id := topic.ID
				m.activeTopicID = &id
				db := m.db
				return m, func() tea.Msg {
					_ = store.SetActiveTopic(db, topic.ID)
					return topicSetMsg{name: topic.Name}
				}
			}
		}
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m TopicsTabModel) View() tea.View {
	if !m.loaded {
		return tea.NewView("\n  " + statsDimStyle.Render("loading…"))
	}

	var b strings.Builder
	b.WriteString("\n  ")
	b.WriteString(statsSectionStyle.Render(fmt.Sprintf("%d topics", len(m.rows))))
	b.WriteString("  " + statsDimStyle.Render("sorted by "+m.sort.label()))
	b.WriteString("\n\n")
	b.WriteString(m.table.View())
	b.WriteString("\n")
	if m.statusMsg != "" {
		b.WriteString("  " + m.statusMsg + "\n")
	} else {
		b.WriteString("\n")
	}
	b.WriteString("  ")
	b.WriteString(m.help.View(topicsKeyMap{}))
	return tea.NewView(b.String())
}

// ── Table helpers ─────────────────────────────────────────────────────────────

func newTopicsTable(_ [][]string, height int) colorTable {
	return newColorTable([]ctColumn{
		{title: "Topic", width: 22},
		{title: "Progress", width: 16},
		{title: "Total", width: 7},
		{title: "Mastered", width: 10},
		{title: "Pass", width: 6},
		{title: "Fail", width: 6},
		{title: "Due", width: 5},
		{title: "Rate", width: 6},
	}, height)
}

func buildTopicRows(rows []store.TopicStat, activeTopicID *int) [][]string {
	out := make([][]string, len(rows))
	for i, r := range rows {
		notMastered := r.Attempted - r.Mastered
		if notMastered < 0 {
			notMastered = 0
		}
		bar := renderSegmentedBar(r.Total, 14,
			barSegment{value: r.Mastered, color: barMasteredColor},
			barSegment{value: notMastered, color: barAttemptColor},
		)

		rateStr := statsDimStyle.Render("—")
		if total := r.Successes + r.Failures; total > 0 {
			pct := int(float64(r.Successes) / float64(total) * 100)
			rateStr = lipgloss.NewStyle().Foreground(rateColor(pct)).Render(fmt.Sprintf("%d%%", pct))
		}

		dueStr := statsDimStyle.Render("—")
		if r.DueToday > 0 {
			dueStr = lipgloss.NewStyle().Foreground(colorWarning).Render(fmt.Sprintf("%d", r.DueToday))
		}

		name := r.Name
		if activeTopicID != nil && *activeTopicID == r.ID {
			name = "* " + name
		}
		out[i] = []string{
			truncate(name, 22),
			bar,
			fmt.Sprintf("%d", r.Total),
			fmt.Sprintf("%d", r.Mastered),
			fmt.Sprintf("%d", r.Successes),
			fmt.Sprintf("%d", r.Failures),
			dueStr,
			rateStr,
		}
	}
	return out
}

func rateColor(pct int) color.Color {
	switch {
	case pct >= 75:
		return colorSuccess
	case pct >= 50:
		return colorWarning
	default:
		return colorDanger
	}
}
