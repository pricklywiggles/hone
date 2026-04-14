package tui

import (
	"fmt"
	"image/color"
	"math"
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
	playlists     []store.TopicStat
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
	playlists     []store.TopicStat
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
		playlists, err := store.GetPlaylistPerfStats(m.db)
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
		return statsLoadedMsg{overview: overview, streak: streak, diff: diff, playlists: playlists, topics: topics, recent: recent, practiceStats: ps}
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
		m.playlists = msg.playlists
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
	vpW := cardsWidth - 8 // borders(2) + padding(4) + scrollbar(2)
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

	contentW := cardsWidth - 6 // borders(2) + padding(4)
	inner := m.viewport.View()
	if m.viewport.TotalLineCount() > m.viewport.VisibleLineCount() {
		inner = m.composeScrollbar(inner, contentW)
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorDimBg).
		Padding(0, 2).
		Width(cardsWidth).
		Render(inner)

	return tea.NewView(fixedHeader + lipgloss.NewStyle().MarginLeft(4).Render(box) + "\n" + statsIndent + m.help.View(statsKeyMap{}))
}

func (m StatsTabModel) composeScrollbar(content string, contentW int) string {
	lines := strings.Split(content, "\n")
	sb := m.renderScrollbar()
	sbLines := strings.Split(sb, "\n")
	vpW := contentW - 2
	var b strings.Builder
	for i, line := range lines {
		pad := vpW - lipgloss.Width(line)
		if pad > 0 {
			line += strings.Repeat(" ", pad)
		}
		if i < len(sbLines) {
			line += " " + sbLines[i]
		}
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(line)
	}
	return b.String()
}

func (m StatsTabModel) renderScrollbar() string {
	vpH := m.viewport.Height()
	totalLines := m.viewport.TotalLineCount()
	thumbSize := max(1, vpH*vpH/totalLines)
	scrollMax := totalLines - vpH
	thumbPos := 0
	if scrollMax > 0 {
		thumbPos = m.viewport.YOffset() * (vpH - thumbSize) / scrollMax
	}

	track := lipgloss.NewStyle().Foreground(colorDimBg)
	thumb := lipgloss.NewStyle().Foreground(colorDim)

	var b strings.Builder
	for i := range vpH {
		if i > 0 {
			b.WriteByte('\n')
		}
		if i >= thumbPos && i < thumbPos+thumbSize {
			b.WriteString(thumb.Render("█"))
		} else {
			b.WriteString(track.Render("│"))
		}
	}
	return b.String()
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
		b.WriteString("\n")
		b.WriteString(renderLegendDot(barMasteredColor) + " " + statsDimStyle.Render("mastered"))
		b.WriteString("   ")
		b.WriteString(renderLegendDot(barAttemptColor) + " " + statsDimStyle.Render("attempted"))
		b.WriteString("   ")
		b.WriteString(renderLegendDot(barEmptyColor) + " " + statsDimStyle.Render("remaining"))
		b.WriteString("\n")
	}

	// ── Worst performance by playlist ────────────────────────────────────────
	var attemptedPlaylists []store.TopicStat
	for _, p := range m.playlists {
		if p.Successes+p.Failures > 0 {
			attemptedPlaylists = append(attemptedPlaylists, p)
		}
	}
	if len(attemptedPlaylists) > 0 {
		b.WriteString("\n\n")
		b.WriteString(statsSectionStyle.Render("Worst Performance By Playlist"))
		b.WriteString("\n\n")
		limit := 6
		for i, p := range attemptedPlaylists {
			if i >= limit {
				break
			}
			name := fmt.Sprintf("%-20s", truncate(p.Name, 20))

			bar := renderSegmentedBar(p.Total, 28,
				barSegment{value: p.Successes, color: colorSuccess},
				barSegment{value: p.Failures, color: colorBarFail},
			)

			ratio := statsDimStyle.Render(fmt.Sprintf("  %3d/%-3d", p.Successes, p.Total))

			dueStr := "          "
			if p.DueToday > 0 {
				dueStr = "  " + lipgloss.NewStyle().Foreground(colorWarning).Render(fmt.Sprintf("%d due", p.DueToday))
			}

			b.WriteString(name + "  " + bar + ratio + dueStr + "\n")
		}
		b.WriteString("\n")
		b.WriteString(renderLegendDot(colorSuccess) + " " + statsDimStyle.Render("succeeded"))
		b.WriteString("   ")
		b.WriteString(renderLegendDot(colorBarFail) + " " + statsDimStyle.Render("failed"))
		b.WriteString("   ")
		b.WriteString(renderLegendDot(barEmptyColor) + " " + statsDimStyle.Render("unattempted"))
		b.WriteString("\n")
	}

	// ── Worst performance by topic ───────────────────────────────────────────
	var attempted []store.TopicStat
	for _, t := range m.topics {
		if t.Successes+t.Failures > 0 {
			attempted = append(attempted, t)
		}
	}
	if len(attempted) > 0 {
		b.WriteString("\n\n")
		b.WriteString(statsSectionStyle.Render("Worst Performance By Topic"))
		b.WriteString("\n\n")
		limit := 6
		for i, t := range attempted {
			if i >= limit {
				break
			}
			name := fmt.Sprintf("%-20s", truncate(t.Name, 20))

			bar := renderSegmentedBar(t.Total, 28,
				barSegment{value: t.Successes, color: colorSuccess},
				barSegment{value: t.Failures, color: colorBarFail},
			)

			ratio := statsDimStyle.Render(fmt.Sprintf("  %3d/%-3d", t.Successes, t.Total))

			dueStr := "          "
			if t.DueToday > 0 {
				dueStr = "  " + lipgloss.NewStyle().Foreground(colorWarning).Render(fmt.Sprintf("%d due", t.DueToday))
			}

			b.WriteString(name + "  " + bar + ratio + dueStr + "\n")
		}
		b.WriteString("\n")
		b.WriteString(renderLegendDot(colorSuccess) + " " + statsDimStyle.Render("succeeded"))
		b.WriteString("   ")
		b.WriteString(renderLegendDot(colorBarFail) + " " + statsDimStyle.Render("failed"))
		b.WriteString("   ")
		b.WriteString(renderLegendDot(barEmptyColor) + " " + statsDimStyle.Render("unattempted"))
		b.WriteString("\n")
	}

	// ── Recent attempts ───────────────────────────────────────────────────────
	if len(m.recent) > 0 {
		b.WriteString("\n\n")
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

	joined := lipgloss.JoinHorizontal(lipgloss.Top, card1, "  ", card2, "  ", card3, "  ", card4)
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

	inner.WriteString("\n\n")
	legend := renderLegendDot(barMasteredColor) + " " + statsDimStyle.Render(fmt.Sprintf("mastered %d", ps.Mastered))
	legend += "   "
	legend += renderLegendDot(barAttemptColor) + " " + statsDimStyle.Render(fmt.Sprintf("in progress %d", attempted))
	legend += "   "
	legend += renderLegendDot(barEmptyColor) + " " + statsDimStyle.Render(fmt.Sprintf("unseen %d", remaining))
	if ps.DueToday > 0 {
		dueStr := lipgloss.NewStyle().Foreground(colorWarning).Render(fmt.Sprintf("%d due", ps.DueToday))
		pad := barW - lipgloss.Width(legend) - lipgloss.Width(dueStr)
		if pad < 1 {
			pad = 1
		}
		legend += strings.Repeat(" ", pad) + dueStr
	}
	inner.WriteString(legend)

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
// Non-zero segments always get at least 1 cell, and the filled portion
// always sums exactly to the correct total width when segments cover all of total.
func renderSegmentedBar(total, width int, segments ...barSegment) string {
	if total == 0 {
		return lipgloss.NewStyle().Background(barEmptyColor).Render(strings.Repeat(" ", width))
	}

	segSum := 0
	for _, seg := range segments {
		segSum += seg.value
	}
	filledWidth := width
	if segSum < total {
		filledWidth = int(math.Round(float64(segSum) / float64(total) * float64(width)))
	}

	widths := make([]int, len(segments))
	allocated := 0
	for i, seg := range segments {
		if seg.value > 0 {
			w := int(math.Round(float64(seg.value) / float64(segSum) * float64(filledWidth)))
			if w < 1 {
				w = 1
			}
			widths[i] = w
			allocated += w
		}
	}
	// Redistribute any overshoot/undershoot to the largest segment.
	if allocated != filledWidth {
		largest := -1
		for i, seg := range segments {
			if seg.value > 0 && (largest < 0 || widths[i] > widths[largest]) {
				largest = i
			}
		}
		if largest >= 0 {
			widths[largest] += filledWidth - allocated
			if widths[largest] < 1 {
				widths[largest] = 1
			}
		}
	}

	var b strings.Builder
	used := 0
	for i, seg := range segments {
		if widths[i] > 0 {
			b.WriteString(lipgloss.NewStyle().Background(seg.color).Render(strings.Repeat(" ", widths[i])))
			used += widths[i]
		}
		_ = seg
	}
	if used < width {
		b.WriteString(lipgloss.NewStyle().Background(barEmptyColor).Render(strings.Repeat(" ", width-used)))
	}
	return b.String()
}

func renderLegendDot(c color.Color) string {
	return lipgloss.NewStyle().Background(c).Render("  ")
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
