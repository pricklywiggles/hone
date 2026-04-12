package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jmoiron/sqlx"
	"github.com/pricklywiggles/hone/internal/config"
	"github.com/pricklywiggles/hone/internal/platform"
	"github.com/pricklywiggles/hone/internal/scraper"
	"github.com/pricklywiggles/hone/internal/store"
)

type batchItemState int

const (
	batchPending  batchItemState = iota
	batchScraping
	batchDone
	batchFailed
)

type batchItem struct {
	url   string
	state batchItemState
	plat  string
	meta  platform.ProblemMeta
	err   error
}

type batchResultMsg struct {
	index int
	plat  string
	meta  platform.ProblemMeta
}

type batchErrMsg struct {
	index int
	err   error
}

type batchBrowserReadyMsg struct {
	browser *scraper.Browser
	err     error
}

type BatchAddModel struct {
	items      []batchItem
	current    int
	spinner    spinner.Model
	progress   progress.Model
	added      int
	skipped    int
	failed     int
	db         *sqlx.DB
	profileDir string
	browser    *scraper.Browser
}

func NewBatchAddModel(db *sqlx.DB, profileDir string, urls []string) BatchAddModel {
	items := make([]batchItem, len(urls))
	for i, u := range urls {
		items[i] = batchItem{url: u, state: batchPending}
	}
	if len(items) > 0 {
		items[0].state = batchScraping
	}

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(colorAccent)

	prog := progress.New(progress.WithDefaultBlend())
	prog.SetWidth(40)

	return BatchAddModel{
		items:      items,
		spinner:    sp,
		progress:   prog,
		db:         db,
		profileDir: profileDir,
	}
}

func (m BatchAddModel) Init() tea.Cmd {
	if len(m.items) == 0 {
		return tea.Quit
	}
	return tea.Batch(m.spinner.Tick, func() tea.Msg {
		b, err := scraper.NewBrowser(m.profileDir)
		return batchBrowserReadyMsg{browser: b, err: err}
	})
}

func (m BatchAddModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.browser != nil {
				m.browser.Close()
			}
			return m, tea.Quit
		default:
			if m.allDone() {
				if m.browser != nil {
					m.browser.Close()
				}
				return m, tea.Quit
			}
		}

	case batchBrowserReadyMsg:
		if msg.err != nil {
			for i := range m.items {
				m.items[i].state = batchFailed
				m.items[i].err = msg.err
			}
			m.failed = len(m.items)
			return m, nil
		}
		m.browser = msg.browser
		return m, m.scrapeItem(0)

	case spinner.TickMsg:
		if m.allDone() {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case progress.FrameMsg:
		var cmd tea.Cmd
		m.progress, cmd = m.progress.Update(msg)
		return m, cmd

	case batchResultMsg:
		m.items[msg.index].state = batchDone
		m.items[msg.index].plat = msg.plat
		m.items[msg.index].meta = msg.meta
		m.added++
		return m, m.startNext(msg.index)

	case batchErrMsg:
		m.items[msg.index].state = batchFailed
		m.items[msg.index].err = msg.err
		if msg.err != nil && msg.err.Error() == "already exists" {
			m.skipped++
		} else {
			m.failed++
			config.AppendFailedURL(m.items[msg.index].url)
		}
		return m, m.startNext(msg.index)
	}

	return m, nil
}

func (m *BatchAddModel) startNext(completed int) tea.Cmd {
	done := m.added + m.skipped + m.failed
	total := len(m.items)
	pct := float64(done) / float64(total)
	progressCmd := m.progress.SetPercent(pct)

	next := completed + 1
	if next >= total {
		return progressCmd
	}
	m.items[next].state = batchScraping
	m.current = next
	return tea.Batch(progressCmd, m.scrapeItem(next))
}

func (m BatchAddModel) allDone() bool {
	for _, it := range m.items {
		if it.state == batchPending || it.state == batchScraping {
			return false
		}
	}
	return true
}

func (m BatchAddModel) scrapeItem(index int) tea.Cmd {
	return func() tea.Msg {
		rawURL := m.items[index].url
		plat, slug, err := platform.ParseURL(rawURL)
		if err != nil {
			return batchErrMsg{index: index, err: fmt.Errorf("invalid URL: %w", err)}
		}
		meta, err := scraper.Scrape(m.browser, plat, slug)
		if err != nil {
			return batchErrMsg{index: index, err: err}
		}
		_, err = store.InsertProblem(m.db, plat, slug, meta.Title, meta.Difficulty, meta.Topics)
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE constraint failed") {
				return batchErrMsg{index: index, err: fmt.Errorf("already exists")}
			}
			return batchErrMsg{index: index, err: fmt.Errorf("save failed: %w", err)}
		}
		return batchResultMsg{index: index, plat: plat, meta: meta}
	}
}

func (m BatchAddModel) FailedURLs() []string {
	var urls []string
	for _, it := range m.items {
		if it.state == batchFailed {
			urls = append(urls, it.url)
		}
	}
	return urls
}

var (
	batchOKStyle   = lipgloss.NewStyle().Foreground(colorSuccess).Bold(true)
	batchSkipStyle = lipgloss.NewStyle().Foreground(colorWarning)
	batchFailStyle = lipgloss.NewStyle().Foreground(colorDanger).Bold(true)
	batchDimStyle  = lipgloss.NewStyle().Foreground(colorDim)
)

func (m BatchAddModel) View() tea.View {
	var b strings.Builder
	total := len(m.items)

	b.WriteString("\n  ")
	b.WriteString(addHeaderStyle.Render(fmt.Sprintf("Adding %d problems", total)))
	b.WriteString("\n\n  ")
	b.WriteString(m.progress.View())
	b.WriteString("  ")
	b.WriteString(batchDimStyle.Render(fmt.Sprintf("%d / %d", m.added+m.skipped+m.failed, total)))
	b.WriteString("\n\n  ")

	if m.allDone() {
		b.WriteString(batchDimStyle.Render("done"))
	} else {
		label := m.items[m.current].url
		if plat, slug, err := platform.ParseURL(label); err == nil {
			label = plat + " / " + slug
		}
		b.WriteString(m.spinner.View() + " " + batchDimStyle.Render(label))
	}

	b.WriteString("\n\n  ")
	b.WriteString(batchOKStyle.Render(fmt.Sprintf("✓ %d added", m.added)))
	b.WriteString("  ")
	b.WriteString(batchSkipStyle.Render(fmt.Sprintf("– %d skipped", m.skipped)))
	b.WriteString("  ")
	b.WriteString(batchFailStyle.Render(fmt.Sprintf("✗ %d failed", m.failed)))

	if m.allDone() {
		b.WriteString("\n\n  ")
		b.WriteString(batchDimStyle.Render("press any key to exit"))
	}

	b.WriteString("\n")
	return tea.NewView(b.String())
}
