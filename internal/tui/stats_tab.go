package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jmoiron/sqlx"
	"github.com/pricklywiggles/hone/internal/store"
)

// ── Messages ──────────────────────────────────────────────────────────────────

type statsLoadedMsg struct {
	overview store.OverviewStats
	streak   int
	diff     []store.DiffStat
	topics   []store.TopicStat
	recent   []store.RecentAttempt
}

type statsErrMsg struct{ err error }

// ── Styles ────────────────────────────────────────────────────────────────────

var (
	statsSectionStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("62"))

	statsMetricNumStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("15"))

	statsMetricLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("241"))

	statsMetricCardStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("62")).
				Padding(0, 2).
				Width(18)

	statsMetricCardDueStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("11")).
				Padding(0, 2).
				Width(18)

	statsDimStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	statsOKStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	statsFailStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
)

// ── Model ─────────────────────────────────────────────────────────────────────

type StatsTabModel struct {
	loaded   bool
	loading  bool
	err      error
	overview store.OverviewStats
	streak   int
	diff     []store.DiffStat
	topics   []store.TopicStat
	recent   []store.RecentAttempt
	height   int
	db       *sqlx.DB
}

func NewStatsTabModel(db *sqlx.DB, height int) StatsTabModel {
	return StatsTabModel{db: db, height: height}
}

func (m StatsTabModel) withHeight(h int) StatsTabModel {
	m.height = h
	return m
}

func (m StatsTabModel) loadCmd() tea.Cmd {
	return func() tea.Msg {
		overview, err := store.GetOverviewStats(m.db)
		if err != nil {
			return statsErrMsg{err}
		}
		streak, err := store.GetStreak(m.db)
		if err != nil {
			return statsErrMsg{err}
		}
		diff, err := store.GetDiffStats(m.db)
		if err != nil {
			return statsErrMsg{err}
		}
		topics, err := store.GetTopicStats(m.db)
		if err != nil {
			return statsErrMsg{err}
		}
		recent, err := store.GetRecentAttempts(m.db, 8)
		if err != nil {
			return statsErrMsg{err}
		}
		return statsLoadedMsg{overview: overview, streak: streak, diff: diff, topics: topics, recent: recent}
	}
}

func (m StatsTabModel) activated() (StatsTabModel, tea.Cmd) {
	m.loading = true
	return m, m.loadCmd()
}

func (m StatsTabModel) Init() tea.Cmd { return nil }

func (m StatsTabModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case statsLoadedMsg:
		m.overview = msg.overview
		m.streak = msg.streak
		m.diff = msg.diff
		m.topics = msg.topics
		m.recent = msg.recent
		m.loaded = true
		m.loading = false
		m.err = nil
		return m, nil

	case statsErrMsg:
		m.err = msg.err
		m.loading = false
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "r" {
			m.loading = true
			return m, m.loadCmd()
		}
	}
	return m, nil
}

func (m StatsTabModel) View() string {
	if !m.loaded {
		return "\n  " + statsDimStyle.Render("loading…")
	}
	if m.err != nil {
		return "\n  " + statsFailStyle.Render("error: "+m.err.Error())
	}

	var b strings.Builder

	// ── Metric cards ─────────────────────────────────────────────────────────
	b.WriteString("\n")
	cards := m.renderMetricCards()
	b.WriteString(cards)
	b.WriteString("\n")

	// ── Overall progress ─────────────────────────────────────────────────────
	b.WriteString("\n  ")
	b.WriteString(statsSectionStyle.Render("Progress"))
	b.WriteString("\n\n")
	if m.overview.Total > 0 {
		b.WriteString(renderLabeledBar("  Mastered ", m.overview.Mastered, m.overview.Total, "220", 32))
		b.WriteString("\n")
		b.WriteString(renderLabeledBar("  Attempted", m.overview.AttemptedOnce, m.overview.Total, "62", 32))
		b.WriteString("\n")
		b.WriteString(renderLabeledBar("  Untouched", m.overview.Untouched, m.overview.Total, "241", 32))
	}

	// ── By difficulty ─────────────────────────────────────────────────────────
	if len(m.diff) > 0 {
		b.WriteString("\n\n  ")
		b.WriteString(statsSectionStyle.Render("By Difficulty"))
		b.WriteString("\n\n")
		for _, d := range m.diff {
			color := diffBarColor(d.Difficulty)
			label := fmt.Sprintf("  %-6s", d.Difficulty)
			suffix := fmt.Sprintf("  %d/%d   mastered %d", d.Attempted, d.Total, d.Mastered)
			b.WriteString(label + renderBar(float64(d.Attempted)/safeDiv(d.Total), 28, color) + statsDimStyle.Render(suffix))
			b.WriteString("\n")
		}
	}

	// ── Weakest topics ────────────────────────────────────────────────────────
	if len(m.topics) > 0 {
		b.WriteString("\n  ")
		b.WriteString(statsSectionStyle.Render("Weakest Topics"))
		b.WriteString("  ")
		b.WriteString(statsDimStyle.Render("sorted by success rate ↑"))
		b.WriteString("\n\n")
		limit := 6
		for i, t := range m.topics {
			if i >= limit {
				break
			}
			rateStr := "no attempts"
			barPct := 0.0
			if t.SuccessRate >= 0 {
				rateStr = fmt.Sprintf("%d%%", int(t.SuccessRate*100))
				barPct = t.SuccessRate
			}
			dueStr := ""
			if t.DueToday > 0 {
				dueStr = fmt.Sprintf("  %s", lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render(fmt.Sprintf("%d due", t.DueToday)))
			}
			name := fmt.Sprintf("  %-24s", truncate(t.Name, 24))
			b.WriteString(name + renderBar(barPct, 14, "62") + "  " + statsDimStyle.Render(rateStr) + "  " + statsDimStyle.Render(fmt.Sprintf("%d/%d", t.Mastered, t.Total)) + dueStr)
			b.WriteString("\n")
		}
	}

	// ── Recent attempts ───────────────────────────────────────────────────────
	if len(m.recent) > 0 {
		b.WriteString("\n  ")
		b.WriteString(statsSectionStyle.Render("Recent"))
		b.WriteString("\n\n")
		for _, a := range m.recent {
			icon := statsOKStyle.Render("✓")
			if a.Result != "success" {
				icon = statsFailStyle.Render("✗")
			}
			diffStyle := lipgloss.NewStyle().Foreground(diffColor(a.Difficulty))
			dur := formatDuration(time.Duration(a.DurationSec) * time.Second)
			when := timeAgo(a.StartedAt)
			title := fmt.Sprintf("%-30s", truncate(a.Title, 30))
			b.WriteString(fmt.Sprintf("  %s  %s  %s  %s  %s\n",
				icon,
				title,
				diffStyle.Render(fmt.Sprintf("%-6s", a.Difficulty)),
				statsDimStyle.Render(fmt.Sprintf("%5s", dur)),
				statsDimStyle.Render(when),
			))
		}
	}

	b.WriteString("\n  " + statsDimStyle.Render("r: refresh"))

	return b.String()
}

func (m StatsTabModel) renderMetricCards() string {
	streakLabel := "day streak"
	if m.streak == 1 {
		streakLabel = "day streak"
	}
	streakIcon := "🔥"
	if m.streak == 0 {
		streakIcon = "💤"
	}

	masteredPct := 0
	if m.overview.Total > 0 {
		masteredPct = m.overview.Mastered * 100 / m.overview.Total
	}

	card1 := statsMetricCardStyle.Render(
		statsMetricNumStyle.Render(fmt.Sprintf("%s %d", streakIcon, m.streak)) + "\n" +
			statsMetricLabelStyle.Render(streakLabel),
	)
	card2 := statsMetricCardDueStyle.Render(
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11")).Render(fmt.Sprintf("📅 %d", m.overview.DueToday)) + "\n" +
			statsMetricLabelStyle.Render("due today"),
	)
	card3 := statsMetricCardStyle.Render(
		statsMetricNumStyle.Render(fmt.Sprintf("%d", m.overview.Total)) + "\n" +
			statsMetricLabelStyle.Render("problems"),
	)
	card4 := statsMetricCardStyle.Render(
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("220")).Render(fmt.Sprintf("★ %d", m.overview.Mastered)) + "\n" +
			statsMetricLabelStyle.Render(fmt.Sprintf("mastered (%d%%)", masteredPct)),
	)

	return "  " + lipgloss.JoinHorizontal(lipgloss.Top, card1, "  ", card2, "  ", card3, "  ", card4)
}

// ── Shared bar helpers ────────────────────────────────────────────────────────

func renderBar(pct float64, width int, colorCode string) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}
	filled := int(pct * float64(width))
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return lipgloss.NewStyle().Foreground(lipgloss.Color(colorCode)).Render(bar)
}

func renderLabeledBar(label string, value, total int, colorCode string, width int) string {
	pct := float64(value) / safeDiv(total)
	bar := renderBar(pct, width, colorCode)
	suffix := statsDimStyle.Render(fmt.Sprintf("  %d / %d", value, total))
	return fmt.Sprintf("  %s  %s%s", label, bar, suffix)
}

func safeDiv(n int) float64 {
	if n == 0 {
		return 1
	}
	return float64(n)
}

func diffBarColor(d string) string {
	switch d {
	case "easy":
		return "10"
	case "medium":
		return "11"
	case "hard":
		return "9"
	default:
		return "241"
	}
}

func diffColor(d string) lipgloss.Color {
	switch d {
	case "easy":
		return lipgloss.Color("10")
	case "medium":
		return lipgloss.Color("11")
	case "hard":
		return lipgloss.Color("9")
	default:
		return lipgloss.Color("241")
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func timeAgo(ts string) string {
	t, err := time.Parse("2006-01-02 15:04:05", ts)
	if err != nil {
		// try date-only
		t, err = time.Parse("2006-01-02", ts[:10])
		if err != nil {
			return ts
		}
	}
	diff := time.Since(t)
	switch {
	case diff < 2*time.Minute:
		return "just now"
	case diff < time.Hour:
		return fmt.Sprintf("%dm ago", int(diff.Minutes()))
	case diff < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(diff.Hours()))
	case diff < 48*time.Hour:
		return "yesterday"
	case diff < 7*24*time.Hour:
		return fmt.Sprintf("%d days ago", int(diff.Hours()/24))
	default:
		return t.Format("Jan 2")
	}
}
