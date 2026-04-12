package tui

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jmoiron/sqlx"
	"github.com/pricklywiggles/hone/internal/store"
)

// ── Messages ──────────────────────────────────────────────────────────────────

type statsLoadedMsg struct {
	overview      store.OverviewStats
	streak        int
	diff          []store.DiffStat
	topics        []store.TopicStat
	recent        []store.RecentAttempt
	practiceStats *store.PlaylistStats
}

type statsErrMsg struct{ err error }

// ── Styles ────────────────────────────────────────────────────────────────────

const statsIndent = "    "

var (
	statsSectionStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorAccent)

	statsMetricNumStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorBright)

	statsMetricLabelStyle = lipgloss.NewStyle().
				Foreground(colorDim)

	statsCardBase = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, 2)

	statsDimStyle  = lipgloss.NewStyle().Foreground(colorDim)
	statsOKStyle   = lipgloss.NewStyle().Foreground(colorSuccess).Bold(true)
	statsFailStyle = lipgloss.NewStyle().Foreground(colorDanger).Bold(true)

	barEmptyColor    = colorDimBg
	barMasteredColor = colorBarDone
	barAttemptColor  = colorBarWIP
)

// ── Model ─────────────────────────────────────────────────────────────────────

type StatsTabModel struct {
	loaded        bool
	loading       bool
	err           error
	overview      store.OverviewStats
	streak        int
	diff          []store.DiffStat
	topics        []store.TopicStat
	recent        []store.RecentAttempt
	practiceStats *store.PlaylistStats
	height        int
	viewport      viewport.Model
	cardsWidth    int
	db            *sqlx.DB
	filter        store.PracticeFilter
	help          help.Model
}

func NewStatsTabModel(db *sqlx.DB, filter store.PracticeFilter, height int) StatsTabModel {
	return StatsTabModel{db: db, filter: filter, height: height, help: newHelpModel()}
}

func (m StatsTabModel) withHeight(h int) StatsTabModel {
	m.height = h
	if m.loaded {
		m.syncViewport()
	}
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
		recent, err := store.GetRecentAttempts(m.db, 6)
		if err != nil {
			return statsErrMsg{err}
		}
		var ps *store.PlaylistStats
		if m.filter.PlaylistID != nil {
			s, err := store.GetPlaylistStats(m.db, *m.filter.PlaylistID)
			if err == nil {
				ps = &s
			}
		} else if m.filter.TopicID != nil {
			s, err := store.GetTopicStatsById(m.db, *m.filter.TopicID)
			if err == nil {
				ps = &s
			}
		}
		return statsLoadedMsg{overview: overview, streak: streak, diff: diff, topics: topics, recent: recent, practiceStats: ps}
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
		m.practiceStats = msg.practiceStats
		m.loaded = true
		m.loading = false
		m.err = nil
		m.syncViewport()
		return m, nil

	case statsErrMsg:
		m.err = msg.err
		m.loading = false
		return m, nil

	case tea.KeyPressMsg:
		if msg.String() == "r" {
			m.loading = true
			return m, m.loadCmd()
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m *StatsTabModel) syncViewport() {
	header, cardsWidth := m.renderFixedHeader()
	m.cardsWidth = cardsWidth
	headerH := lipgloss.Height(header)
	// 3 = box borders (2) + help line (1)
	vpH := m.height - headerH - 3
	if vpH < 1 {
		vpH = 1
	}
	// Content width: cardsWidth minus box padding (2*2)
	vpW := cardsWidth - 4
	if vpW < 1 {
		vpW = 1
	}
	m.viewport.SetHeight(vpH)
	m.viewport.SetWidth(vpW)
	m.viewport.SetContent(m.renderScrollableContent())
	m.viewport.GotoTop()
}

func (m StatsTabModel) View() tea.View {
	if !m.loaded {
		return tea.NewView("\n  " + statsDimStyle.Render("loading…"))
	}
	if m.err != nil {
		return tea.NewView("\n  " + statsFailStyle.Render("error: "+m.err.Error()))
	}

	fixedHeader, cardsWidth := m.renderFixedHeader()

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorDimBg).
		Padding(0, 2).
		Width(cardsWidth).
		Render(m.viewport.View())

	return tea.NewView(fixedHeader + lipgloss.NewStyle().MarginLeft(4).Render(box) + "\n" + statsIndent + m.help.View(statsKeyMap{}))
}

func (m StatsTabModel) renderFixedHeader() (string, int) {
	var b strings.Builder
	b.WriteString("\n")
	cards, cardsInnerWidth := m.renderMetricCards()
	b.WriteString(cards)

	if ps := m.practiceStats; ps != nil && ps.Total > 0 {
		b.WriteString(m.renderPracticeSection(ps, cardsInnerWidth))
	}
	b.WriteString("\n")
	return b.String(), cardsInnerWidth
}

func (m StatsTabModel) renderScrollableContent() string {
	var b strings.Builder

	// ── Overall progress ─────────────────────────────────────────────────────
	b.WriteString("\n")
	b.WriteString(statsSectionStyle.Render("Overall Progress"))
	b.WriteString("\n\n")
	if m.overview.Total > 0 {
		attempted := m.overview.AttemptedOnce - m.overview.Mastered
		if attempted < 0 {
			attempted = 0
		}
		remaining := m.overview.Total - m.overview.Mastered - attempted
		if remaining < 0 {
			remaining = 0
		}
		bar := renderSegmentedBar(m.overview.Total, 40,
			barSegment{value: m.overview.Mastered, color: barMasteredColor},
			barSegment{value: attempted, color: barAttemptColor},
		)
		b.WriteString(bar + "\n\n")
		b.WriteString(renderLegendDot(barMasteredColor) + " " + statsDimStyle.Render(fmt.Sprintf("mastered %d", m.overview.Mastered)))
		b.WriteString("   ")
		b.WriteString(renderLegendDot(barAttemptColor) + " " + statsDimStyle.Render(fmt.Sprintf("attempted %d", attempted)))
		b.WriteString("   ")
		b.WriteString(renderLegendDot(barEmptyColor) + " " + statsDimStyle.Render(fmt.Sprintf("remaining %d", remaining)))
		b.WriteString("\n")
	}

	// ── By difficulty ─────────────────────────────────────────────────────────
	if len(m.diff) > 0 {
		b.WriteString("\n\n")
		b.WriteString(statsSectionStyle.Render("By Difficulty"))
		b.WriteString("\n\n")
		for _, d := range m.diff {
			label := fmt.Sprintf("%-8s", d.Difficulty)
			notMastered := d.Attempted - d.Mastered
			if notMastered < 0 {
				notMastered = 0
			}
			bar := renderSegmentedBar(d.Total, 26,
				barSegment{value: d.Mastered, color: barMasteredColor},
				barSegment{value: notMastered, color: barAttemptColor},
			)
			ratio := fmt.Sprintf("%d/%d", d.Attempted, d.Total)
			mastered := fmt.Sprintf("mastered %d", d.Mastered)
			b.WriteString(label + bar + "  " + statsDimStyle.Render(fmt.Sprintf("%-7s %s", ratio, mastered)))
			b.WriteString("\n")
		}
	}

	// ── Weakest topics ────────────────────────────────────────────────────────
	if len(m.topics) > 0 {
		b.WriteString("\n")
		b.WriteString(statsSectionStyle.Render("Weakest Topics"))
		b.WriteString("\n\n")
		limit := 6
		for i, t := range m.topics {
			if i >= limit {
				break
			}
			name := fmt.Sprintf("%-20s", truncate(t.Name, 20))

			notMastered := t.Attempted - t.Mastered
			if notMastered < 0 {
				notMastered = 0
			}
			bar := renderSegmentedBar(t.Total, 28,
				barSegment{value: t.Mastered, color: barMasteredColor},
				barSegment{value: notMastered, color: barAttemptColor},
			)

			ratio := statsDimStyle.Render(fmt.Sprintf("  %3d/%-3d", t.Mastered, t.Total))

			dueStr := "          "
			if t.DueToday > 0 {
				dueStr = "  " + lipgloss.NewStyle().Foreground(colorWarning).Render(fmt.Sprintf("%d due", t.DueToday))
			}

			b.WriteString(name + "  " + bar + ratio + dueStr + "\n")
		}
	}

	// ── Recent attempts ───────────────────────────────────────────────────────
	if len(m.recent) > 0 {
		b.WriteString("\n")
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
			title := fmt.Sprintf("%-28s", truncate(a.Title, 28))
			b.WriteString(fmt.Sprintf("%s  %s  %s  %s  %s\n",
				icon,
				title,
				diffStyle.Render(fmt.Sprintf("%-6s", a.Difficulty)),
				statsDimStyle.Render(fmt.Sprintf("%5s", dur)),
				statsDimStyle.Render(when),
			))
		}
	}

	return b.String()
}

func (m StatsTabModel) renderMetricCards() (string, int) {
	// Width sets content+padding width; borders add 2 more columns.
	// Height must be uniform so JoinHorizontal doesn't pad outside the border.
	cardW := 18
	cardH := 3
	card := func(borderColor color.Color) lipgloss.Style {
		return statsCardBase.
			Width(cardW).
			Height(cardH).
			BorderForeground(borderColor)
	}

	streakNum := lipgloss.NewStyle().Bold(true).Foreground(colorStreak).
		Render(fmt.Sprintf("🔥 %d", m.streak))
	dueNum := lipgloss.NewStyle().Bold(true).Foreground(colorWarning).
		Render(fmt.Sprintf("📅 %d", m.overview.DueToday))
	totalNum := statsMetricNumStyle.Render(fmt.Sprintf("%d", m.overview.Total))
	masteredNum := lipgloss.NewStyle().Bold(true).Foreground(colorMastered).
		Render(fmt.Sprintf("★ %d", m.overview.Mastered))

	card1 := card(colorStreak).Render(
		streakNum + "\n" + statsMetricLabelStyle.Render("streak days"),
	)
	card2 := card(colorWarning).Render(
		dueNum + "\n" + statsMetricLabelStyle.Render("due today"),
	)
	card3 := card(colorAccent).Render(
		totalNum + "\n" + statsMetricLabelStyle.Render("problems"),
	)
	card4 := card(colorMastered).Render(
		masteredNum + "\n" + statsMetricLabelStyle.Render("mastered"),
	)

	joined := lipgloss.JoinHorizontal(lipgloss.Top, card1, " ", card2, " ", card3, " ", card4)
	innerWidth := lipgloss.Width(joined)
	return lipgloss.NewStyle().MarginLeft(4).Render(joined), innerWidth
}

func (m StatsTabModel) renderPracticeSection(ps *store.PlaylistStats, cardsWidth int) string {
	attempted := ps.Attempted - ps.Mastered
	if attempted < 0 {
		attempted = 0
	}
	remaining := ps.Total - ps.Mastered - attempted
	if remaining < 0 {
		remaining = 0
	}

	contentW := cardsWidth - 4
	barW := contentW - 2

	var inner strings.Builder
	inner.WriteString(statsSectionStyle.Render("Currently practicing "))
	inner.WriteString(lipgloss.NewStyle().Bold(true).Foreground(colorBright).Render(ps.Name))
	inner.WriteString("\n\n")
	inner.WriteString(renderSegmentedBar(ps.Total, barW,
		barSegment{value: ps.Mastered, color: barMasteredColor},
		barSegment{value: attempted, color: barAttemptColor},
	))

	summary := fmt.Sprintf("  %d/%d/%d", ps.Mastered, attempted, remaining)
	if ps.DueToday > 0 {
		summary += fmt.Sprintf("  %d due", ps.DueToday)
	}
	inner.WriteString(statsDimStyle.Render(summary))

	// Width includes padding but not borders; subtract 2 for left+right border.
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorAccent).
		Padding(1, 2).
		Width(cardsWidth).
		Render(inner.String())

	return "\n" + lipgloss.NewStyle().MarginLeft(4).Render(box)
}

// ── Bar rendering ─────────────────────────────────────────────────────────────

type barSegment struct {
	value int
	color color.Color
}

// renderSegmentedBar draws a single bar with multiple colored segments.
// Segments are drawn left to right; remaining space uses barEmptyColor.
func renderSegmentedBar(total, width int, segments ...barSegment) string {
	if total == 0 {
		return lipgloss.NewStyle().Background(barEmptyColor).Render(strings.Repeat(" ", width))
	}
	var b strings.Builder
	used := 0
	for _, seg := range segments {
		w := int(float64(seg.value) / float64(total) * float64(width))
		if w+used > width {
			w = width - used
		}
		if w > 0 {
			b.WriteString(lipgloss.NewStyle().Background(seg.color).Render(strings.Repeat(" ", w)))
		}
		used += w
	}
	if used < width {
		b.WriteString(lipgloss.NewStyle().Background(barEmptyColor).Render(strings.Repeat(" ", width-used)))
	}
	return b.String()
}

func renderLegendDot(c color.Color) string {
	return lipgloss.NewStyle().Background(c).Render("  ")
}

func renderBar(pct float64, width int, c color.Color) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}
	filled := int(pct * float64(width))
	if filled > width {
		filled = width
	}
	empty := width - filled

	filledStyle := lipgloss.NewStyle().Background(c)
	emptyStyle := lipgloss.NewStyle().Background(barEmptyColor)

	return filledStyle.Render(strings.Repeat(" ", filled)) +
		emptyStyle.Render(strings.Repeat(" ", empty))
}

func renderLabeledBar(label string, value, total int, c color.Color, width int) string {
	pct := float64(value) / safeDiv(total)
	bar := renderBar(pct, width, c)
	suffix := statsDimStyle.Render(fmt.Sprintf("  %d / %d", value, total))
	return fmt.Sprintf(statsIndent+"%s  %s%s", label, bar, suffix)
}

func safeDiv(n int) float64 {
	if n == 0 {
		return 1
	}
	return float64(n)
}

func diffColor(d string) color.Color {
	switch d {
	case "easy":
		return colorSuccess
	case "medium":
		return colorWarning
	case "hard":
		return colorDanger
	default:
		return colorDim
	}
}

func truncate(s string, n int) string {
	if lipgloss.Width(s) <= n {
		return s
	}
	if n <= 1 {
		return "…"
	}
	runes := []rune(s)
	w := 0
	for i, r := range runes {
		rw := lipgloss.Width(string(r))
		if w+rw > n-1 {
			return string(runes[:i]) + "…"
		}
		w += rw
	}
	return s
}

func timeAgo(ts string) string {
	t, err := time.Parse("2006-01-02 15:04:05", ts)
	if err != nil {
		if len(ts) >= 10 {
			t, err = time.Parse("2006-01-02", ts[:10])
		}
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
		return fmt.Sprintf("%dd ago", int(diff.Hours()/24))
	default:
		return t.Format("Jan 2")
	}
}
