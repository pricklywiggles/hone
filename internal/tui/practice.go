package tui

import (
	"context"
	"fmt"
	"image/color"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jmoiron/sqlx"
	"github.com/pricklywiggles/hone/internal/config"
	"github.com/pricklywiggles/hone/internal/monitor"
	"github.com/pricklywiggles/hone/internal/srs"
	"github.com/pricklywiggles/hone/internal/store"
	"github.com/spf13/viper"
)

// ── State machine ─────────────────────────────────────────────────────────────

type practiceState int

const (
	practiceStart practiceState = iota
	practiceWaiting
	practiceDone
	practiceError
	practiceAllCaughtUp
)

// ── Messages ──────────────────────────────────────────────────────────────────

type practiceTickMsg struct{}
type practiceAnimTickMsg struct{}
type practiceResultMsg monitor.Result
type practiceSavedMsg struct {
	nextDate      string
	quality       int
	newlyMastered bool
	filterStats   store.TodayStats
	overallStats  store.TodayStats
	frozen        bool
}
type practiceNextMsg struct {
	problem  *store.Problem
	srsState *srs.ProblemSRS
	isDue    bool
}
type practiceNoNextMsg struct{}
type practiceSessionReadyMsg struct {
	session  *monitor.Session
	resultCh <-chan monitor.Result
}
type practiceSessionErrMsg struct{ err error }

// ── Model ─────────────────────────────────────────────────────────────────────

// PracticeModel is the Bubble Tea model for a practice session.
type PracticeModel struct {
	state      practiceState
	problem    *store.Problem
	srsState   *srs.ProblemSRS
	isDue      bool
	startedAt  time.Time
	result     *monitor.Result
	monitorErr error
	nextDate   string
	quality       int
	newlyMastered bool
	filterStats   store.TodayStats
	overallStats  store.TodayStats
	cancelFn      context.CancelFunc
	ctx        context.Context
	session    *monitor.Session
	resultCh   <-chan monitor.Result
	db         *sqlx.DB
	profileDir string
	filter     store.PracticeFilter
	filterName string
	queue      []store.QueueEntry
	dueCount   int
	help         help.Model
	width        int
	height       int
	freePractice    bool
	frozen          bool
	pendingEntry    *store.QueueEntry
	animFrame       int
	showDebug       bool
	debugScroll     int
}

func NewPracticeModel(
	db *sqlx.DB,
	profileDir string,
	queue []store.QueueEntry,
	filter store.PracticeFilter,
	filterName string,
) PracticeModel {
	dueCount := 0
	for _, e := range queue {
		if e.IsDue {
			dueCount++
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	return PracticeModel{
		state:        practiceStart,
		db:           db,
		profileDir:   profileDir,
		queue:        queue,
		dueCount:     dueCount,
		freePractice: dueCount == 0,
		ctx:          ctx,
		cancelFn:     cancel,
		filter:       filter,
		filterName:   filterName,
		help:         newHelpModel(),
	}
}

func (m PracticeModel) Init() tea.Cmd {
	return tea.Batch(tea.RequestWindowSize, animTickCmd())
}

func (m PracticeModel) startSession() tea.Cmd {
	return func() tea.Msg {
		session, err := monitor.NewSession(m.profileDir)
		if err != nil {
			return practiceSessionErrMsg{err}
		}
		url := config.BuildURL(m.problem.Platform, m.problem.Slug)
		ch := session.Monitor(m.ctx, m.problem.Platform, url)
		return practiceSessionReadyMsg{session, ch}
	}
}

func (m PracticeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			m.cancelFn()
			if m.session != nil {
				m.session.Close()
			}
			return m, tea.Quit
		case "q", "esc":
			if m.state == practiceStart {
				m.cancelFn()
				return m, Pop()
			}
			if m.state == practiceAllCaughtUp {
				m.cancelFn()
				if m.session != nil {
					m.session.Close()
				}
				return m, Pop()
			}
			m.cancelFn()
			if m.session != nil {
				m.session.Close()
			}
			return m, Pop()
		case "n", "enter":
			if m.state == practiceStart {
				entry := m.queue[0]
				m.queue = m.queue[1:]
				m.problem = &entry.Problem
				m.srsState = &entry.SRS
				m.isDue = entry.IsDue
				m.startedAt = time.Now()
				m.state = practiceWaiting
				return m, tea.Batch(tickCmd(), m.startSession())
			}
			if m.state == practiceAllCaughtUp {
				entry := m.pendingEntry
				m.pendingEntry = nil
				m.freePractice = true
				return m, func() tea.Msg {
					return practiceNextMsg{problem: &entry.Problem, srsState: &entry.SRS, isDue: entry.IsDue}
				}
			}
			if m.state == practiceDone {
				if len(m.queue) == 0 {
					return m, popNext()
				}
				entry := m.queue[0]
				m.queue = m.queue[1:]
				if !entry.IsDue && !m.freePractice {
					m.pendingEntry = &entry
					m.state = practiceAllCaughtUp
					m.animFrame = 0
					return m, animTickCmd()
				}
				return m, func() tea.Msg {
					return practiceNextMsg{problem: &entry.Problem, srsState: &entry.SRS, isDue: entry.IsDue}
				}
			}
		case "p":
			if m.state == practiceWaiting {
				m.cancelFn()
				r := monitor.Result{Success: true, CompletedAt: time.Now()}
				m.result = &r
				m.state = practiceDone
				return m, tea.Batch(m.saveAttempt(r), focusTerminalCmd())
			}
		case "f":
			if m.state == practiceWaiting {
				m.cancelFn()
				r := monitor.Result{Success: false, CompletedAt: time.Now()}
				m.result = &r
				m.state = practiceDone
				return m, tea.Batch(m.saveAttempt(r), focusTerminalCmd())
			}
		case "d":
			if m.state == practiceDone {
				m.showDebug = !m.showDebug
				m.debugScroll = 0
			}
		case "j", "down":
			if m.showDebug && m.state == practiceDone {
				m.debugScroll++
			}
		case "k", "up":
			if m.showDebug && m.state == practiceDone && m.debugScroll > 0 {
				m.debugScroll--
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case practiceTickMsg:
		if m.state == practiceWaiting {
			return m, tickCmd()
		}

	case practiceAnimTickMsg:
		if m.state == practiceStart || m.state == practiceAllCaughtUp {
			m.animFrame++
			return m, animTickCmd()
		}

	case practiceSessionReadyMsg:
		m.session = msg.session
		m.resultCh = msg.resultCh
		return m, waitForResult(msg.resultCh)

	case practiceSessionErrMsg:
		m.monitorErr = msg.err
		m.state = practiceError
		return m, nil

	case practiceResultMsg:
		r := monitor.Result(msg)
		if r.Err != nil {
			m.monitorErr = r.Err
			m.state = practiceError
			return m, nil
		}
		m.result = &r
		m.state = practiceDone
		return m, tea.Batch(m.saveAttempt(r), focusTerminalCmd())

	case practiceSavedMsg:
		m.nextDate = msg.nextDate
		m.quality = msg.quality
		m.newlyMastered = msg.newlyMastered
		m.frozen = msg.frozen
		m.filterStats = msg.filterStats
		m.overallStats = msg.overallStats

	case practiceNextMsg:
		ctx, cancel := context.WithCancel(context.Background())
		if !msg.isDue && !m.freePractice {
			m.freePractice = true
		}
		m.problem = msg.problem
		m.srsState = msg.srsState
		m.isDue = msg.isDue
		m.startedAt = time.Now()
		m.result = nil
		m.nextDate = ""
		m.quality = 0
		m.newlyMastered = false
		m.frozen = false
		m.filterStats = store.TodayStats{}
		m.overallStats = store.TodayStats{}
		m.debugScroll = 0
		m.state = practiceWaiting
		m.ctx = ctx
		m.cancelFn = cancel
		url := config.BuildURL(m.problem.Platform, m.problem.Slug)
		ch := m.session.Monitor(ctx, m.problem.Platform, url)
		m.resultCh = ch
		return m, tea.Batch(tickCmd(), waitForResult(ch))

	case practiceNoNextMsg:
		if m.session != nil {
			m.session.Close()
		}
		return m, Pop()
	}

	return m, nil
}

// saveAttempt persists the attempt + SRS state and returns the next review date.
func (m PracticeModel) saveAttempt(r monitor.Result) tea.Cmd {
	return func() tea.Msg {
		elapsed := r.CompletedAt.Sub(m.startedAt)
		durationMin := int(elapsed.Minutes())
		durationSec := int(elapsed.Seconds())

		result := "fail"
		if r.Success {
			result = "success"
		}

		thresholds := config.ThresholdsFor(m.problem.Difficulty)
		var quality int
		if r.Success {
			quality = srs.QualityFromDuration(durationMin, thresholds)
		} else {
			quality = 1
		}

		_ = store.RecordAttempt(m.db, m.problem.ID, m.startedAt, r.CompletedAt, result, durationSec, quality)

		// Upcoming problems that are solved successfully don't advance SRS —
		// same-day re-solves prove working memory, not durable recall.
		// Failures still reset SRS since they're real evidence of forgetting.
		frozen := !m.isDue && r.Success
		if frozen {
			fStats, _ := store.GetTodayStatsFiltered(m.db, m.filter)
			oStats, _ := store.GetTodayStats(m.db)
			return practiceSavedMsg{
				nextDate:     m.srsState.NextReviewDate,
				quality:      quality,
				frozen:       true,
				filterStats:  fStats,
				overallStats: oStats,
			}
		}

		wasMastered := m.srsState.MasteredBefore == 1
		newState := srs.UpdateSRS(*m.srsState, r.Success, durationMin, thresholds, time.Now())
		_ = store.SaveSRSState(m.db, newState)

		fStats, _ := store.GetTodayStatsFiltered(m.db, m.filter)
		oStats, _ := store.GetTodayStats(m.db)
		return practiceSavedMsg{
			nextDate:      newState.NextReviewDate,
			quality:       quality,
			newlyMastered: !wasMastered && newState.MasteredBefore == 1,
			filterStats:   fStats,
			overallStats:  oStats,
		}
	}
}

func popNext() tea.Cmd {
	return func() tea.Msg {
		return practiceNoNextMsg{}
	}
}

// ── Commands ──────────────────────────────────────────────────────────────────

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return practiceTickMsg{} })
}

func animTickCmd() tea.Cmd {
	return tea.Tick(150*time.Millisecond, func(time.Time) tea.Msg { return practiceAnimTickMsg{} })
}

var gradientColors = []color.Color{
	lipgloss.Color("#7C8EF2"), lipgloss.Color("#56B6C2"), lipgloss.Color("#73D0A0"), lipgloss.Color("#E5C07B"),
	lipgloss.Color("#E0B464"), lipgloss.Color("#D19A66"), lipgloss.Color("#E06C75"), lipgloss.Color("#8A7FBD"),
}

func waitForResult(ch <-chan monitor.Result) tea.Cmd {
	return func() tea.Msg {
		r, ok := <-ch
		if !ok {
			return nil
		}
		return practiceResultMsg(r)
	}
}

var safeAppName = regexp.MustCompile(`^[a-zA-Z0-9_. -]+$`)

func focusTerminalCmd() tea.Cmd {
	if !viper.GetBool("auto_focus") {
		return nil
	}
	return func() tea.Msg {
		app := os.Getenv("TERM_PROGRAM")
		if app == "" || !safeAppName.MatchString(app) {
			return nil
		}
		exec.Command("osascript", "-e",
			fmt.Sprintf(`tell application "%s" to activate`, app)).Run()
		return nil
	}
}

// ── View ──────────────────────────────────────────────────────────────────────

const prCardWidth = 56

var (
	prHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	prTitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(colorBright)
	prDimStyle    = lipgloss.NewStyle().Foreground(colorDim)
	prOKStyle     = lipgloss.NewStyle().Bold(true).Foreground(colorSuccess)
	prFailStyle   = lipgloss.NewStyle().Bold(true).Foreground(colorDanger)
	prTimerStyle  = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	prDotStyle    = lipgloss.NewStyle().Foreground(colorDimBg)
	prCardBase    = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1, 3).
			Width(prCardWidth)
	prDiffColors = map[string]color.Color{
		"easy":   colorSuccess,
		"medium": colorWarning,
		"hard":   colorDanger,
	}
)

func (m PracticeModel) renderLayout(card string, keys help.KeyMap) string {
	helpBar := "    " + m.help.View(keys)

	if m.width == 0 {
		return "\n" + card + "\n\n" + helpBar
	}

	if m.showDebug {
		debugPanel := m.renderDebugPanel()
		body := card + "\n\n" + debugPanel
		availH := m.height - 2
		centered := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, body)
		lines := strings.Split(centered, "\n")
		if len(lines) < availH {
			centered += strings.Repeat("\n", availH-len(lines))
		}
		return centered + "\n" + helpBar
	}

	availH := m.height - 2
	centered := lipgloss.Place(m.width, availH, lipgloss.Center, lipgloss.Center, card)
	return centered + "\n" + helpBar
}

func (m PracticeModel) renderDebugPanel() string {
	total := 1 + len(m.queue) // current problem + remaining
	header := prDimStyle.Render(fmt.Sprintf("── queue (%d) ──", total))

	type entry struct {
		title, difficulty, date string
		current                 bool
	}
	entries := make([]entry, 0, total)
	if m.problem != nil {
		entries = append(entries, entry{
			title: m.problem.Title, difficulty: m.problem.Difficulty,
			date: m.srsState.NextReviewDate, current: true,
		})
	}
	for _, e := range m.queue {
		entries = append(entries, entry{
			title: e.Problem.Title, difficulty: e.Problem.Difficulty,
			date: e.SRS.NextReviewDate,
		})
	}

	if len(entries) == 0 {
		return prDimStyle.Render("no candidates")
	}

	maxVisible := 15
	if m.height > 0 {
		cardH := lipgloss.Height(prCardBase.Render(""))
		maxVisible = m.height - cardH - 8
		if maxVisible < 5 {
			maxVisible = 5
		}
	}
	if m.debugScroll > len(entries)-maxVisible {
		m.debugScroll = len(entries) - maxVisible
	}
	if m.debugScroll < 0 {
		m.debugScroll = 0
	}

	end := m.debugScroll + maxVisible
	if end > len(entries) {
		end = len(entries)
	}

	var lines []string
	lines = append(lines, header)
	for i := m.debugScroll; i < end; i++ {
		e := entries[i]
		diffColor := prDiffColors[e.difficulty]
		marker := " "
		if e.current {
			marker = prTimerStyle.Render("▸")
		}
		diff := lipgloss.NewStyle().Foreground(diffColor).Width(7).Render(e.difficulty)
		title := prDimStyle.Render(e.title)
		date := lipgloss.NewStyle().Foreground(colorDimBg).Render(e.date)
		lines = append(lines, fmt.Sprintf(" %s %s %s  %s", marker, diff, title, date))
	}
	if end < len(entries) {
		lines = append(lines, prDimStyle.Render(fmt.Sprintf("   … %d more (j/k to scroll)", len(entries)-end)))
	}

	return strings.Join(lines, "\n")
}

func (m PracticeModel) View() tea.View {
	switch m.state {
	case practiceStart:
		return tea.NewView(m.viewStart())
	case practiceDone:
		return tea.NewView(m.viewDone())
	case practiceError:
		return tea.NewView(m.viewError())
	case practiceAllCaughtUp:
		return tea.NewView(m.viewAllCaughtUp())
	default:
		return tea.NewView(m.viewWaiting())
	}
}

func (m PracticeModel) viewStart() string {
	n := len(gradientColors)
	borderColor := gradientColors[m.animFrame%n]

	starColors := make([]color.Color, 5)
	for i := range starColors {
		starColors[i] = gradientColors[(m.animFrame+i)%n]
	}
	var stars strings.Builder
	for _, c := range starColors {
		stars.WriteString(lipgloss.NewStyle().Foreground(c).Render("★ "))
	}
	starLine := stars.String()

	bright := lipgloss.NewStyle().Foreground(colorBright)
	innerW := prCardWidth - 2 - 6
	wrap := lipgloss.NewStyle().Width(innerW).Align(lipgloss.Center)

	scope := "Your library"
	if m.filterName != "" {
		scope = m.filterName
	}
	scopeLine := bright.Render(scope)

	var title, dueLine, queueLine string
	var extraLines []string

	if m.dueCount > 0 {
		title = lipgloss.NewStyle().Bold(true).Foreground(colorAccent).Render("Ready to Practice")
		dueLine = lipgloss.NewStyle().Foreground(colorWarning).Render(
			fmt.Sprintf("%d problems due today", m.dueCount))
		queueLine = prDimStyle.Render(fmt.Sprintf("%d total in queue", len(m.queue)))
	} else {
		title = lipgloss.NewStyle().Bold(true).Foreground(colorMastered).Render("All Caught Up!")
		dueLine = prDimStyle.Render("0 problems due today")
		queueLine = prDimStyle.Render(fmt.Sprintf("%d upcoming in queue", len(m.queue)))
		extraLines = append(extraLines,
			"",
			wrap.Render(prDimStyle.Render("SRS paused for successful solves. Failures will still reset progress.")),
		)
	}

	prompt := bright.Render("Press enter to start, q to quit")

	lines := []string{
		starLine,
		"",
		title,
		"",
		scopeLine,
		dueLine,
		queueLine,
	}
	lines = append(lines, extraLines...)
	lines = append(lines,
		"",
		prompt,
		"",
		starLine,
	)

	content := lipgloss.JoinVertical(lipgloss.Center, lines...)
	card := prCardBase.BorderForeground(borderColor).Render(content)

	if m.width == 0 {
		return "\n" + card
	}
	availH := m.height - 2
	return lipgloss.Place(m.width, availH, lipgloss.Center, lipgloss.Center, card)
}

func (m PracticeModel) viewAllCaughtUp() string {
	n := len(gradientColors)
	borderColor := gradientColors[m.animFrame%n]

	starColors := make([]color.Color, 5)
	for i := range starColors {
		starColors[i] = gradientColors[(m.animFrame+i)%n]
	}

	var stars strings.Builder
	for _, c := range starColors {
		stars.WriteString(lipgloss.NewStyle().Foreground(c).Render("★ "))
	}
	starLine := stars.String()

	title := lipgloss.NewStyle().Bold(true).Foreground(colorMastered).Render("All caught up!")

	scope := "your library"
	if m.filterName != "" {
		scope = m.filterName
	}
	bright := lipgloss.NewStyle().Foreground(colorBright)
	innerW := prCardWidth - 2 - 6
	wrap := lipgloss.NewStyle().Width(innerW).Align(lipgloss.Center)

	fs := m.filterStats
	statsLine := bright.Render(fmt.Sprintf("%d solved", fs.Succeeded)) +
		prDimStyle.Render(" / ") +
		bright.Render(fmt.Sprintf("%d failed", fs.Attempted-fs.Succeeded))
	remaining := len(m.queue) + 1
	doneLine := wrap.Render(prDimStyle.Render(fmt.Sprintf(
		"You've finished all due problems in %s!", scope)))
	continueLine := wrap.Render(prDimStyle.Render(fmt.Sprintf(
		"You can continue with %d upcoming problems "+
			"(success won't change due dates, failures "+
			"will) or switch to another playlist.",
		remaining)))
	prompt := bright.Render("Press enter to continue, q to quit")

	content := lipgloss.JoinVertical(lipgloss.Center,
		starLine,
		"",
		title,
		"",
		doneLine,
		statsLine,
		"",
		continueLine,
		"",
		prompt,
		"",
		starLine,
	)

	card := prCardBase.BorderForeground(borderColor).Render(content)
	if m.width == 0 {
		return "\n" + card
	}
	availH := m.height - 2
	return lipgloss.Place(m.width, availH, lipgloss.Center, lipgloss.Center, card)
}

func (m PracticeModel) viewWaiting() string {
	elapsed := time.Since(m.startedAt)
	diffStyle := lipgloss.NewStyle().Foreground(prDiffColors[m.problem.Difficulty])
	dot := prDotStyle.Render(" · ")

	dueLabel := prDimStyle.Render("upcoming")
	if m.isDue {
		dueLabel = lipgloss.NewStyle().Foreground(colorWarning).Render("due today")
	}

	innerW := prCardWidth - 2 - 6
	timer := lipgloss.PlaceHorizontal(innerW, lipgloss.Center, prTimerStyle.Render(formatDuration(elapsed)))

	header := prHeaderStyle.Render("Practice")
	if m.freePractice {
		header = prHeaderStyle.Render("Free Practice")
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		prTitleStyle.Render(m.problem.Title),
		diffStyle.Render(m.problem.Difficulty)+dot+prDimStyle.Render(m.problem.Platform)+dot+dueLabel,
		"",
		timer,
	)

	borderColor := colorAccent
	var banner string
	if m.freePractice {
		banner = prDimStyle.Render("All caught up! SRS paused for successful solves.")
		borderColor = colorDimBg
	}

	card := prCardBase.BorderForeground(borderColor).Render(content)
	if banner != "" {
		card = banner + "\n\n" + card
	}
	return m.renderLayout(card, practiceWaitingKeyMap{})
}

func (m PracticeModel) viewDone() string {
	if m.result == nil {
		return ""
	}
	elapsed := m.result.CompletedAt.Sub(m.startedAt)
	verdict := prOKStyle.Render("✓ Accepted")
	borderColor := colorSuccess
	if !m.result.Success {
		verdict = prFailStyle.Render("✗ Wrong Answer")
		borderColor = colorDanger
	}

	nextLine := prDimStyle.Render("next review  ") + lipgloss.NewStyle().Foreground(colorBright).Render(m.nextDate)
	if m.frozen {
		nextLine += prDimStyle.Render("  (SRS unchanged)")
	}
	qualityLine := prDimStyle.Render("quality  ") + lipgloss.NewStyle().Foreground(colorBright).Render(fmt.Sprintf("%d/5", m.quality))

	bright := lipgloss.NewStyle().Foreground(colorBright)
	statsHeader := ""
	filterLine := ""
	todayLine := ""

	filterLabel := "session"
	if m.filterName != "" {
		filterLabel = m.filterName
	}
	fs := m.filterStats
	fRemaining := fs.DueRemaining
	os := m.overallStats
	oRemaining := os.DueRemaining

	statsHeader = lipgloss.NewStyle().Bold(true).Foreground(colorAccent).Render("Today's Stats")
	filterLine = prDimStyle.Render(filterLabel) + "\n" +
		bright.Render(fmt.Sprintf("%d solved", fs.Succeeded)) +
		prDimStyle.Render(" / ") +
		bright.Render(fmt.Sprintf("%d failed", fs.Attempted-fs.Succeeded)) +
		prDimStyle.Render(" / ") +
		bright.Render(fmt.Sprintf("%d remaining", fRemaining))
	todayLine = prDimStyle.Render("All problems") + "\n" +
		bright.Render(fmt.Sprintf("%d solved", os.Succeeded)) +
		prDimStyle.Render(" / ") +
		bright.Render(fmt.Sprintf("%d failed", os.Attempted-os.Succeeded)) +
		prDimStyle.Render(" / ") +
		bright.Render(fmt.Sprintf("%d remaining", oRemaining))

	masteredLine := ""
	if m.newlyMastered {
		masteredLine = lipgloss.NewStyle().Bold(true).Foreground(colorMastered).Render("★ Mastered!")
	}
	if m.nextDate == "" {
		nextLine = prDimStyle.Render("saving…")
		qualityLine = ""
		statsHeader = ""
		filterLine = ""
		todayLine = ""
		masteredLine = ""
	}

	lines := []string{
		verdict,
		"",
		prTitleStyle.Render(m.problem.Title),
		prDimStyle.Render("time  ") + bright.Render(formatDuration(elapsed)),
	}
	if qualityLine != "" {
		lines = append(lines, qualityLine)
	}
	lines = append(lines, nextLine)
	if masteredLine != "" {
		lines = append(lines, "", masteredLine)
	}
	if statsHeader != "" {
		lines = append(lines, "", statsHeader, "")
	}
	if filterLine != "" {
		lines = append(lines, filterLine)
	}
	if todayLine != "" {
		lines = append(lines, "", todayLine)
	}
	content := lipgloss.JoinVertical(lipgloss.Left, lines...)

	card := prCardBase.BorderForeground(borderColor).Render(content)
	return m.renderLayout(card, practiceDoneKeyMap{})
}

func (m PracticeModel) viewError() string {
	errMsg := m.monitorErr.Error()
	maxW := prCardWidth - 2 - 6
	if len(errMsg) > maxW*3 {
		errMsg = errMsg[:maxW*3] + "…"
	}
	wrapped := lipgloss.NewStyle().Width(maxW).Render(prDimStyle.Render(errMsg))

	lines := []string{
		prFailStyle.Render("Monitor Error"),
		"",
		wrapped,
	}
	content := strings.Join(lines, "\n")

	card := prCardBase.BorderForeground(colorDanger).Render(content)
	return m.renderLayout(card, practiceErrorKeyMap{})
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}
