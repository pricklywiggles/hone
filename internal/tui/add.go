package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jmoiron/sqlx"
	"github.com/pricklywiggles/hone/internal/platform"
	"github.com/pricklywiggles/hone/internal/scraper"
	"github.com/pricklywiggles/hone/internal/store"
)

type addState int

const (
	stateInput    addState = iota
	stateScraping
	stateDone
	stateErr
)

type scrapeResultMsg struct {
	plat string
	meta platform.ProblemMeta
}

type scrapeErrMsg struct{ err error }

// AddModel is the Bubble Tea model for the add-problem flow.
// It runs standalone via tui.Run() or can be pushed onto the router stack.
type AddModel struct {
	state      addState
	input      textinput.Model
	spinner    spinner.Model
	result     *scrapeResultMsg
	err        error
	db         *sqlx.DB
	profileDir string
	help       help.Model
	standalone bool // true → tea.Quit to exit; false → Pop()
}

func NewAddModel(db *sqlx.DB, profileDir, initialURL string) AddModel {
	ti := textinput.New()
	ti.Placeholder = "https://neetcode.io/problems/…"
	ti.CharLimit = 300
	ti.Width = 60

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(colorAccent)

	m := AddModel{
		input:      ti,
		spinner:    sp,
		db:         db,
		profileDir: profileDir,
		help:       newHelpModel(),
		standalone: true,
	}
	if initialURL != "" {
		m.state = stateScraping
		m.input.SetValue(initialURL)
	} else {
		m.state = stateInput
		m.input.Focus()
	}
	return m
}

func (m AddModel) Init() tea.Cmd {
	if m.state == stateScraping {
		return tea.Batch(m.spinner.Tick, m.doScrape(m.input.Value()))
	}
	return textinput.Blink
}

func (m AddModel) doScrape(rawURL string) tea.Cmd {
	return func() tea.Msg {
		plat, slug, err := platform.ParseURL(rawURL)
		if err != nil {
			return scrapeErrMsg{fmt.Errorf("parsing URL: %w", err)}
		}
		browser, err := scraper.NewBrowser(m.profileDir)
		if err != nil {
			return scrapeErrMsg{fmt.Errorf("browser: %w", err)}
		}
		defer browser.Close()
		meta, err := scraper.Scrape(browser, plat, slug)
		if err != nil {
			return scrapeErrMsg{fmt.Errorf("scraping: %w", err)}
		}
		_, err = store.InsertProblem(m.db, plat, slug, meta.Title, meta.Difficulty, meta.Topics)
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE constraint failed") {
				return scrapeErrMsg{fmt.Errorf("problem already exists")}
			}
			return scrapeErrMsg{fmt.Errorf("saving: %w", err)}
		}
		return scrapeResultMsg{plat: plat, meta: meta}
	}
}

func (m AddModel) exit() tea.Cmd {
	if m.standalone {
		return tea.Quit
	}
	return Pop()
}

func (m AddModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEsc:
			if m.state == stateInput {
				return m, m.exit()
			}
		case tea.KeyEnter:
			if m.state == stateInput && strings.TrimSpace(m.input.Value()) != "" {
				m.state = stateScraping
				return m, tea.Batch(m.spinner.Tick, m.doScrape(m.input.Value()))
			}
		default:
			if m.state == stateDone || m.state == stateErr {
				if m.state == stateDone && !m.standalone {
					return m, tea.Batch(m.exit(), func() tea.Msg { return problemAddedMsg{} })
				}
				return m, m.exit()
			}
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case scrapeResultMsg:
		m.result = &msg
		m.state = stateDone
		return m, nil

	case scrapeErrMsg:
		m.err = msg.err
		m.state = stateErr
		return m, nil
	}

	if m.state == stateInput {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	return m, nil
}

// Styles
var (
	addHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	addLabelStyle  = lipgloss.NewStyle().Foreground(colorDim)
	addTitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(colorBright)
	addCardStyle   = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorAccent).
			Padding(1, 3).
			Width(52)
	addErrCardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorDanger).
			Padding(1, 3).
			Width(52)
	diffColors = map[string]lipgloss.Color{
		"easy":   colorSuccess,
		"medium": colorWarning,
		"hard":   colorDanger,
	}
)

func (m AddModel) View() string {
	switch m.state {
	case stateInput:
		return fmt.Sprintf(
			"\n  %s\n\n  %s\n\n  %s",
			addHeaderStyle.Render("Add Problem"),
			m.input.View(),
			m.help.View(addInputKeyMap{}),
		)

	case stateScraping:
		label := m.input.Value()
		if plat, slug, err := platform.ParseURL(label); err == nil {
			label = plat + " / " + slug
		}
		return fmt.Sprintf("\n  %s Scraping %s…", m.spinner.View(), addLabelStyle.Render(label))

	case stateDone:
		r := m.result
		diffStyle := lipgloss.NewStyle().Foreground(diffColors[r.meta.Difficulty])
		topicsStr := addLabelStyle.Render(strings.Join(r.meta.Topics, ", "))
		if len(r.meta.Topics) == 0 {
			topicsStr = addLabelStyle.Render("no topics")
		}
		content := lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Bold(true).Foreground(colorSuccess).Render("✓ Added"),
			"",
			addTitleStyle.Render(r.meta.Title),
			diffStyle.Render(r.meta.Difficulty)+"  "+addLabelStyle.Render(r.plat),
			topicsStr,
		)
		return "\n" + addCardStyle.Render(content) + "\n\n  " + m.help.View(addDoneKeyMap{})

	case stateErr:
		content := lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Bold(true).Foreground(colorDanger).Render("✗ Failed"),
			"",
			m.err.Error(),
		)
		return "\n" + addErrCardStyle.Render(content) + "\n\n  " + m.help.View(addDoneKeyMap{})
	}

	return ""
}
