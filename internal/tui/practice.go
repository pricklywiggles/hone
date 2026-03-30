package tui

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/help"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jmoiron/sqlx"
	"github.com/pricklywiggles/hone/internal/config"
	"github.com/pricklywiggles/hone/internal/monitor"
	"github.com/pricklywiggles/hone/internal/srs"
	"github.com/pricklywiggles/hone/internal/store"
)

// ── State machine ─────────────────────────────────────────────────────────────

type practiceState int

const (
	practiceWaiting practiceState = iota
	practiceDone
)

// ── Messages ──────────────────────────────────────────────────────────────────

type practiceTickMsg struct{}
type practiceResultMsg monitor.Result
type practiceSavedMsg string // carries the next review date after DB write
type practiceNextMsg struct {
	problem  *store.Problem
	srsState *srs.ProblemSRS
	isDue    bool
}
type practiceNoNextMsg struct{}

// ── Model ─────────────────────────────────────────────────────────────────────

// PracticeModel is the Bubble Tea model for a practice session.
// startedAt and cancelFn are initialised in the constructor so they are
// available from the first tick / render.
type PracticeModel struct {
	state           practiceState
	problem         *store.Problem
	srsState        *srs.ProblemSRS
	isDue           bool
	startedAt       time.Time
	result          *monitor.Result
	nextDate        string
	cancelFn        context.CancelFunc
	ctx             context.Context
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
	url := platformURL(m.problem.Platform, m.problem.Slug)
	return tea.Batch(
		tickCmd(),
		waitForResult(m.ctx, m.problem.Platform, url, m.profileDir),
	)
}

func (m PracticeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.cancelFn()
			return m, tea.Quit
		case "q", "esc":
			m.cancelFn()
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

	case practiceResultMsg:
		r := monitor.Result(msg)
		m.result = &r
		m.state = practiceDone
		m.cancelFn()
		return m, m.saveAttempt(r)

	case practiceSavedMsg:
		m.nextDate = string(msg)

	case practiceNextMsg:
		ctx, cancel := context.WithCancel(context.Background())
		m.problem = msg.problem
		m.srsState = msg.srsState
		m.isDue = msg.isDue
		m.startedAt = time.Now()
		m.result = nil
		m.nextDate = ""
		m.state = practiceWaiting
		m.ctx = ctx
		m.cancelFn = cancel
		url := platformURL(m.problem.Platform, m.problem.Slug)
		return m, tea.Batch(tickCmd(), waitForResult(ctx, m.problem.Platform, url, m.profileDir))

	case practiceNoNextMsg:
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

		return practiceSavedMsg(newState.NextReviewDate)
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

func waitForResult(ctx context.Context, platform, problemURL, profileDir string) tea.Cmd {
	ch := monitor.Monitor(ctx, platform, problemURL, profileDir)
	return func() tea.Msg {
		r, ok := <-ch
		if !ok {
			return nil
		}
		return practiceResultMsg(r)
	}
}

// ── View ──────────────────────────────────────────────────────────────────────

var (
	prHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62"))
	prTitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	prDimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	prOKStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	prFailStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9"))
	prCardStyle   = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 3).
			Width(52)
	prDiffColors = map[string]lipgloss.Color{
		"easy":   lipgloss.Color("10"),
		"medium": lipgloss.Color("11"),
		"hard":   lipgloss.Color("9"),
	}
)

func (m PracticeModel) View() string {
	if m.state == practiceDone {
		return m.viewDone()
	}
	return m.viewWaiting()
}

func (m PracticeModel) viewWaiting() string {
	elapsed := time.Since(m.startedAt)
	diffStyle := lipgloss.NewStyle().Foreground(prDiffColors[m.problem.Difficulty])
	dueLabel := prDimStyle.Render("upcoming")
	if m.isDue {
		dueLabel = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render("due today")
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
	borderColor := lipgloss.Color("10")
	if !m.result.Success {
		verdict = prFailStyle.Render("✗ Wrong Answer")
		borderColor = lipgloss.Color("9")
	}

	nextLine := prDimStyle.Render("next review: " + m.nextDate)
	if m.nextDate == "" {
		nextLine = prDimStyle.Render("saving…")
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		verdict,
		"",
		prTitleStyle.Render(m.problem.Title),
		prDimStyle.Render("time  "+formatDuration(elapsed)),
		nextLine,
	)

	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1, 3).
		Width(52).
		Render(content)

	return "\n" + card + "\n\n  " + m.help.View(practiceDoneKeyMap{})
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

func platformURL(platform, slug string) string {
	switch platform {
	case "leetcode":
		return "https://leetcode.com/problems/" + slug + "/"
	case "neetcode":
		return "https://neetcode.io/problems/" + slug + "/question"
	default:
		return ""
	}
}
