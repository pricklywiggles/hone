package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"time"

	"github.com/charmbracelet/bubbles/help"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	practiceWaiting practiceState = iota
	practiceDone
	practiceError
)

// ── Messages ──────────────────────────────────────────────────────────────────

type practiceTickMsg struct{}
type practiceResultMsg monitor.Result
type practiceSavedMsg struct {
	nextDate   string
	todayStats store.TodayStats
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
	todayStats store.TodayStats
	cancelFn   context.CancelFunc
	ctx        context.Context
	session    *monitor.Session
	resultCh   <-chan monitor.Result
	db         *sqlx.DB
	profileDir string
	filter     store.PracticeFilter
	help       help.Model
}

func NewPracticeModel(
	db *sqlx.DB,
	profileDir string,
	problem *store.Problem,
	srsState *srs.ProblemSRS,
	isDue bool,
	filter store.PracticeFilter,
) PracticeModel {
	ctx, cancel := context.WithCancel(context.Background())
	return PracticeModel{
		db:         db,
		profileDir: profileDir,
		problem:    problem,
		srsState:   srsState,
		isDue:      isDue,
		startedAt:  time.Now(),
		ctx:        ctx,
		cancelFn:   cancel,
		filter:     filter,
		help:       newHelpModel(),
	}
}

func (m PracticeModel) Init() tea.Cmd {
	return tea.Batch(tickCmd(), m.startSession())
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
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.cancelFn()
			if m.session != nil {
				m.session.Close()
			}
			return m, tea.Quit
		case "q", "esc":
			m.cancelFn()
			if m.session != nil {
				m.session.Close()
			}
			return m, Pop()
		case "n", "enter":
			if m.state == practiceDone {
				return m, m.fetchNext()
			}
		}

	case practiceTickMsg:
		if m.state == practiceWaiting {
			return m, tickCmd()
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
		m.todayStats = msg.todayStats

	case practiceNextMsg:
		ctx, cancel := context.WithCancel(context.Background())
		m.problem = msg.problem
		m.srsState = msg.srsState
		m.isDue = msg.isDue
		m.startedAt = time.Now()
		m.result = nil
		m.nextDate = ""
		m.todayStats = store.TodayStats{}
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

		newState := srs.UpdateSRS(*m.srsState, r.Success, durationMin, thresholds, time.Now())
		_ = store.RecordAttempt(m.db, m.problem.ID, m.startedAt, r.CompletedAt, result, durationSec, quality)
		_ = store.SaveSRSState(m.db, newState)

		today, _ := store.GetTodayStats(m.db)
		return practiceSavedMsg{nextDate: newState.NextReviewDate, todayStats: today}
	}
}

func (m PracticeModel) fetchNext() tea.Cmd {
	return func() tea.Msg {
		problem, srsState, isDue, err := store.PickNext(m.db, m.filter)
		if err != nil || problem == nil {
			return practiceNoNextMsg{}
		}
		return practiceNextMsg{problem: problem, srsState: srsState, isDue: isDue}
	}
}

// ── Commands ──────────────────────────────────────────────────────────────────

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return practiceTickMsg{} })
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

var (
	prHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	prTitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(colorBright)
	prDimStyle    = lipgloss.NewStyle().Foreground(colorDim)
	prOKStyle     = lipgloss.NewStyle().Bold(true).Foreground(colorSuccess)
	prFailStyle   = lipgloss.NewStyle().Bold(true).Foreground(colorDanger)
	prCardStyle   = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorAccent).
			Padding(1, 3).
			Width(52)
	prDiffColors = map[string]lipgloss.Color{
		"easy":   colorSuccess,
		"medium": colorWarning,
		"hard":   colorDanger,
	}
)

func (m PracticeModel) View() string {
	switch m.state {
	case practiceDone:
		return m.viewDone()
	case practiceError:
		return m.viewError()
	default:
		return m.viewWaiting()
	}
}

func (m PracticeModel) viewWaiting() string {
	elapsed := time.Since(m.startedAt)
	diffStyle := lipgloss.NewStyle().Foreground(prDiffColors[m.problem.Difficulty])
	dueLabel := prDimStyle.Render("upcoming")
	if m.isDue {
		dueLabel = lipgloss.NewStyle().Foreground(colorWarning).Render("due today")
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		prHeaderStyle.Render("Practice"),
		"",
		prTitleStyle.Render(m.problem.Title),
		diffStyle.Render(m.problem.Difficulty)+"  "+prDimStyle.Render(m.problem.Platform)+"  "+dueLabel,
		"",
		prDimStyle.Render("time  ")+lipgloss.NewStyle().Bold(true).Render(formatDuration(elapsed)),
	)

	return "\n" + prCardStyle.Render(content) +
		"\n\n  " + m.help.View(practiceWaitingKeyMap{})
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

	nextLine := prDimStyle.Render("next review: " + m.nextDate)
	todayLine := prDimStyle.Render(fmt.Sprintf("today  %d/%d solved",
		m.todayStats.Succeeded, m.todayStats.Attempted))
	if m.nextDate == "" {
		nextLine = prDimStyle.Render("saving…")
		todayLine = ""
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		verdict,
		"",
		prTitleStyle.Render(m.problem.Title),
		prDimStyle.Render("time  "+formatDuration(elapsed)),
		nextLine,
		todayLine,
	)

	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1, 3).
		Width(52).
		Render(content)

	return "\n" + card + "\n\n  " + m.help.View(practiceDoneKeyMap{})
}

func (m PracticeModel) viewError() string {
	content := lipgloss.JoinVertical(lipgloss.Left,
		prFailStyle.Render("Monitor Error"),
		"",
		prDimStyle.Render(m.monitorErr.Error()),
		"",
		prDimStyle.Render("Press q to go back."),
	)

	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorDanger).
		Padding(1, 3).
		Width(52).
		Render(content)

	return "\n" + card
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
