package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jmoiron/sqlx"
	"github.com/pricklywiggles/hone/internal/store"
)

// ── Messages ──────────────────────────────────────────────────────────────────

type problemsLoadedMsg struct{ rows []store.ProblemRow }
type problemsErrMsg struct{ err error }

// launchProblemMsg triggers navigation to a practice session for a specific problem.
type launchProblemMsg struct {
	problem  *store.Problem
	srsState *store.ProblemRow
}

// ── Sort modes ────────────────────────────────────────────────────────────────

type sortMode int

const (
	sortNextReview sortMode = iota
	sortTitle
	sortDifficulty
	sortAttempts
)

func (s sortMode) label() string {
	switch s {
	case sortTitle:
		return "title"
	case sortDifficulty:
		return "difficulty"
	case sortAttempts:
		return "attempts"
	default:
		return "due date"
	}
}

// ── Model ─────────────────────────────────────────────────────────────────────

type ProblemsTabModel struct {
	table       table.Model
	allRows     []store.ProblemRow
	filtered    []store.ProblemRow
	filterInput textinput.Model
	filtering   bool
	sort        sortMode
	loaded      bool
	height      int
	db          *sqlx.DB
	profileDir  string
	activePlaylistID *int
	help        help.Model
}

func NewProblemsTabModel(db *sqlx.DB, profileDir string, activePlaylistID *int, height int) ProblemsTabModel {
	ti := textinput.New()
	ti.Placeholder = "filter by title…"
	ti.CharLimit = 80
	ti.Width = 40

	t := newProblemsTable(nil, tableBodyHeight(height))

	return ProblemsTabModel{
		table:            t,
		filterInput:      ti,
		height:           height,
		db:               db,
		profileDir:       profileDir,
		activePlaylistID: activePlaylistID,
		help:             newHelpModel(),
	}
}

func (m ProblemsTabModel) withHeight(h int) ProblemsTabModel {
	m.height = h
	m.table.SetHeight(tableBodyHeight(h))
	return m
}

func tableBodyHeight(h int) int {
	reserved := 6 // filter bar + help + padding
	if h-reserved < 5 {
		return 5
	}
	return h - reserved
}

func (m ProblemsTabModel) loadCmd() tea.Cmd {
	return func() tea.Msg {
		rows, err := store.GetAllProblems(m.db)
		if err != nil {
			return problemsErrMsg{err}
		}
		return problemsLoadedMsg{rows}
	}
}

func (m ProblemsTabModel) activated() (ProblemsTabModel, tea.Cmd) {
	return m, m.loadCmd()
}

func (m ProblemsTabModel) Init() tea.Cmd { return nil }

func (m ProblemsTabModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case problemsLoadedMsg:
		m.allRows = msg.rows
		m.applyFilterAndSort()
		m.loaded = true
		return m, nil

	case problemsErrMsg:
		m.loaded = true
		return m, nil

	case tea.KeyMsg:
		if m.filtering {
			return m.updateFilter(msg)
		}
		return m.updateNormal(msg)
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m ProblemsTabModel) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "/":
		m.filtering = true
		m.filterInput.SetValue("")
		m.filterInput.Focus()
		return m, textinput.Blink

	case "s":
		m.sort = (m.sort + 1) % 4
		m.applyFilterAndSort()
		return m, nil

	case "r":
		return m, m.loadCmd()

	case "enter":
		if len(m.filtered) == 0 {
			return m, nil
		}
		idx := m.table.Cursor()
		if idx < 0 || idx >= len(m.filtered) {
			return m, nil
		}
		row := m.filtered[idx]
		problem := &store.Problem{
			ID:         row.ID,
			Platform:   row.Platform,
			Slug:       row.Slug,
			Title:      row.Title,
			Difficulty: row.Difficulty,
		}
		return m, func() tea.Msg {
			srsState, err := store.GetSRSState(m.db, row.ID)
			if err != nil {
				return nil
			}
			isDue := !row.IsOverdue && isToday(row.NextReview)
			if row.IsOverdue {
				isDue = true
			}
			practiceModel := NewPracticeModel(m.db, m.profileDir, problem, srsState, isDue, m.activePlaylistID)
			return PushMsg{Model: practiceModel}
		}
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m ProblemsTabModel) updateFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc, tea.KeyEnter:
		m.filtering = false
		m.filterInput.Blur()
		return m, nil
	}
	var cmd tea.Cmd
	m.filterInput, cmd = m.filterInput.Update(msg)
	m.applyFilterAndSort()
	return m, cmd
}

func (m *ProblemsTabModel) applyFilterAndSort() {
	query := strings.ToLower(m.filterInput.Value())

	var filtered []store.ProblemRow
	for _, r := range m.allRows {
		if query == "" || strings.Contains(strings.ToLower(r.Title), query) {
			filtered = append(filtered, r)
		}
	}

	// Sort
	switch m.sort {
	case sortTitle:
		sortByTitle(filtered)
	case sortDifficulty:
		sortByDifficulty(filtered)
	case sortAttempts:
		sortByAttempts(filtered)
	// sortNextReview: already sorted by query default
	}

	m.filtered = filtered
	m.table.SetRows(buildTableRows(filtered))
}

func (m ProblemsTabModel) View() string {
	if !m.loaded {
		return "\n  " + statsDimStyle.Render("loading…")
	}

	var b strings.Builder

	// Header bar
	b.WriteString("\n  ")
	count := fmt.Sprintf("%d problems", len(m.filtered))
	if len(m.filtered) != len(m.allRows) {
		count = fmt.Sprintf("%d / %d problems", len(m.filtered), len(m.allRows))
	}
	b.WriteString(statsSectionStyle.Render(count))
	b.WriteString("  ")

	sortLabel := statsDimStyle.Render(fmt.Sprintf("sorted by %s", m.sort.label()))
	b.WriteString(sortLabel)
	b.WriteString("\n\n")

	// Filter input
	if m.filtering {
		b.WriteString("  / ")
		b.WriteString(m.filterInput.View())
		b.WriteString("\n\n")
	}

	// Table
	b.WriteString(m.table.View())
	b.WriteString("\n\n  ")

	b.WriteString(m.help.View(problemsKeyMap{filtering: m.filtering}))

	return b.String()
}

// ── Table helpers ─────────────────────────────────────────────────────────────

func newProblemsTable(rows []table.Row, height int) table.Model {
	cols := []table.Column{
		{Title: "★", Width: 2},
		{Title: "Title", Width: 36},
		{Title: "Difficulty", Width: 8},
		{Title: "W/L", Width: 7},
		{Title: "Next Review", Width: 13},
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

func buildTableRows(rows []store.ProblemRow) []table.Row {
	out := make([]table.Row, len(rows))
	for i, r := range rows {
		star := " "
		if r.Mastered {
			star = lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Render("★")
		}

		diff := lipgloss.NewStyle().Foreground(diffColor(r.Difficulty)).Render(r.Difficulty)

		wl := fmt.Sprintf("%d/%d", r.Successes, r.AttemptCount)
		if r.AttemptCount == 0 {
			wl = statsDimStyle.Render("—")
		}

		next := formatNextReview(r.NextReview, r.IsOverdue)

		out[i] = table.Row{star, r.Title, diff, wl, next}
	}
	return out
}

func formatNextReview(date string, overdue bool) string {
	if overdue {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true).Render("overdue")
	}
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	diff := t.Truncate(24 * time.Hour).Sub(today)
	switch {
	case diff <= 0:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render("today")
	case diff < 24*time.Hour:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render("tomorrow")
	case diff < 7*24*time.Hour:
		return statsDimStyle.Render(fmt.Sprintf("in %d days", int(diff.Hours()/24)))
	default:
		return statsDimStyle.Render(t.Format("Jan 2"))
	}
}

func isToday(date string) bool {
	today := time.Now().UTC().Format("2006-01-02")
	return date == today
}

// ── Simple sorts ──────────────────────────────────────────────────────────────

func sortByTitle(rows []store.ProblemRow) {
	for i := 1; i < len(rows); i++ {
		for j := i; j > 0 && rows[j].Title < rows[j-1].Title; j-- {
			rows[j], rows[j-1] = rows[j-1], rows[j]
		}
	}
}

func sortByDifficulty(rows []store.ProblemRow) {
	order := map[string]int{"easy": 0, "medium": 1, "hard": 2}
	for i := 1; i < len(rows); i++ {
		for j := i; j > 0 && order[rows[j].Difficulty] < order[rows[j-1].Difficulty]; j-- {
			rows[j], rows[j-1] = rows[j-1], rows[j]
		}
	}
}

func sortByAttempts(rows []store.ProblemRow) {
	for i := 1; i < len(rows); i++ {
		for j := i; j > 0 && rows[j].AttemptCount > rows[j-1].AttemptCount; j-- {
			rows[j], rows[j-1] = rows[j-1], rows[j]
		}
	}
}
