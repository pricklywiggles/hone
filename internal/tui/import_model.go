package tui

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jmoiron/sqlx"
	"github.com/pricklywiggles/hone/internal/config"
	"github.com/pricklywiggles/hone/internal/importer"
	"github.com/pricklywiggles/hone/internal/platform"
	"github.com/pricklywiggles/hone/internal/scraper"
	"github.com/pricklywiggles/hone/internal/store"
)

// ── item state ────────────────────────────────────────────────────────────────

type importItemState int

const (
	importPending importItemState = iota
	importWorking
	importDone
	importSkipped // problem already existed in library
	importFailed
)

type importFlatItem struct {
	url      string
	playlist string
	state    importItemState
	label    string // "platform/slug" once URL is parsed
	err      error
}

// ── messages ──────────────────────────────────────────────────────────────────

type importResultMsg struct {
	index       int
	label       string
	playlist    string
	playlistID  int
	playlistNew bool
	existed     bool
}

type importErrMsg struct {
	index int
	label string
	err   error
}

type importBrowserReadyMsg struct {
	browser *scraper.Browser
	err     error
}

// ── model ─────────────────────────────────────────────────────────────────────

type ImportModel struct {
	header        string
	items         []importFlatItem
	current       int
	spinner       spinner.Model
	progress      progress.Model
	added         int
	skipped       int
	failed        int
	playlistsNew  int
	db            *sqlx.DB
	profileDir    string
	playlistCache map[string]int // name → id
	browser       *scraper.Browser
}

func NewImportModel(db *sqlx.DB, profileDir string, groups []importer.ImportGroup) ImportModel {
	var items []importFlatItem
	playlistNames := make(map[string]struct{})
	for _, g := range groups {
		if g.Playlist != "" {
			playlistNames[g.Playlist] = struct{}{}
		}
		for _, u := range g.URLs {
			items = append(items, importFlatItem{url: u, playlist: g.Playlist})
		}
	}
	if len(items) > 0 {
		items[0].state = importWorking
	}

	header := fmt.Sprintf("Importing %d problems", len(items))
	if n := len(playlistNames); n > 0 {
		header += fmt.Sprintf(" across %d playlist(s)", n)
	}

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(colorAccent)

	prog := progress.New(progress.WithDefaultGradient())
	prog.Width = 40

	return ImportModel{
		header:        header,
		items:         items,
		spinner:       sp,
		progress:      prog,
		db:            db,
		profileDir:    profileDir,
		playlistCache: make(map[string]int),
	}
}

func (m ImportModel) Init() tea.Cmd {
	if len(m.items) == 0 {
		return tea.Quit
	}
	return tea.Batch(m.spinner.Tick, func() tea.Msg {
		b, err := scraper.NewBrowser(m.profileDir)
		return importBrowserReadyMsg{browser: b, err: err}
	})
}

func (m ImportModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			if m.browser != nil {
				m.browser.Close()
			}
			return m, tea.Quit
		}
		if m.allDone() {
			if m.browser != nil {
				m.browser.Close()
			}
			return m, tea.Quit
		}

	case importBrowserReadyMsg:
		if msg.err != nil {
			for i := range m.items {
				m.items[i].state = importFailed
				m.items[i].err = msg.err
			}
			m.failed = len(m.items)
			return m, nil
		}
		m.browser = msg.browser
		return m, m.processItem(0)

	case spinner.TickMsg:
		if m.allDone() {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case progress.FrameMsg:
		pm, cmd := m.progress.Update(msg)
		m.progress = pm.(progress.Model)
		return m, cmd

	case importResultMsg:
		item := &m.items[msg.index]
		item.label = msg.label
		if msg.existed {
			item.state = importSkipped
			m.skipped++
		} else {
			item.state = importDone
			m.added++
		}
		if msg.playlist != "" {
			m.playlistCache[msg.playlist] = msg.playlistID
			if msg.playlistNew {
				m.playlistsNew++
			}
		}
		return m, m.advanceProgress(msg.index)

	case importErrMsg:
		item := &m.items[msg.index]
		if msg.label != "" {
			item.label = msg.label
		}
		item.state = importFailed
		item.err = msg.err
		m.failed++
		config.AppendFailedURL(item.url)
		return m, m.advanceProgress(msg.index)
	}

	return m, nil
}

func (m *ImportModel) advanceProgress(completed int) tea.Cmd {
	done := m.added + m.skipped + m.failed
	progressCmd := m.progress.SetPercent(float64(done) / float64(len(m.items)))

	next := completed + 1
	if next >= len(m.items) {
		return progressCmd
	}
	m.items[next].state = importWorking
	m.current = next
	return tea.Batch(progressCmd, m.processItem(next))
}

func (m ImportModel) allDone() bool {
	for _, it := range m.items {
		if it.state == importPending || it.state == importWorking {
			return false
		}
	}
	return true
}

// processItem runs one item: parses URL, checks DB, scrapes if new,
// looks up/creates playlist, adds problem to playlist. All blocking DB work
// is captured in a closure to run as an async tea.Cmd.
func (m ImportModel) processItem(index int) tea.Cmd {
	item := m.items[index]
	db := m.db
	browser := m.browser

	// Snapshot the cache so the closure has a consistent view.
	cacheCopy := make(map[string]int, len(m.playlistCache))
	for k, v := range m.playlistCache {
		cacheCopy[k] = v
	}

	return func() tea.Msg {
		plat, slug, err := platform.ParseURL(item.url)
		if err != nil {
			return importErrMsg{index: index, label: item.url, err: err}
		}
		label := plat + "/" + slug

		// Check if the problem already exists.
		existing, err := store.GetProblemBySlug(db, plat, slug)
		if err != nil {
			return importErrMsg{index: index, label: label, err: err}
		}

		var problemID int
		existed := existing != nil

		if existed {
			problemID = existing.ID
		} else {
			meta, err := scraper.Scrape(browser, plat, slug)
			if err != nil {
				return importErrMsg{index: index, label: label, err: fmt.Errorf("scrape: %w", err)}
			}
			id, err := store.InsertProblem(db, plat, slug, meta.Title, meta.Difficulty, meta.Topics)
			if err != nil {
				return importErrMsg{index: index, label: label, err: fmt.Errorf("save: %w", err)}
			}
			problemID = int(id)
		}

		// Resolve playlist.
		playlistID := 0
		playlistNew := false
		if item.playlist != "" {
			if id, ok := cacheCopy[item.playlist]; ok {
				playlistID = id
			} else {
				pl, err := store.GetPlaylistByName(db, item.playlist)
				if errors.Is(err, sql.ErrNoRows) {
					id, err := store.CreatePlaylist(db, item.playlist)
					if err != nil {
						return importErrMsg{index: index, label: label, err: fmt.Errorf("create playlist: %w", err)}
					}
					playlistID = int(id)
					playlistNew = true
				} else if err != nil {
					return importErrMsg{index: index, label: label, err: fmt.Errorf("find playlist: %w", err)}
				} else {
					playlistID = pl.ID
				}
			}
			if err := store.AddProblemToPlaylist(db, playlistID, problemID); err != nil {
				return importErrMsg{index: index, label: label, err: fmt.Errorf("add to playlist: %w", err)}
			}
		}

		return importResultMsg{
			index:       index,
			label:       label,
			playlist:    item.playlist,
			playlistID:  playlistID,
			playlistNew: playlistNew,
			existed:     existed,
		}
	}
}

func (m ImportModel) FailedURLs() []string {
	var urls []string
	for _, it := range m.items {
		if it.state == importFailed {
			urls = append(urls, it.url)
		}
	}
	return urls
}

// ── view ──────────────────────────────────────────────────────────────────────

func (m ImportModel) View() string {
	var b strings.Builder

	b.WriteString("\n  ")
	b.WriteString(addHeaderStyle.Render(m.header))
	b.WriteString("\n\n")

	// Settled items — render each as a result line.
	for _, item := range m.items {
		switch item.state {
		case importDone:
			label := item.label
			if item.playlist != "" {
				label += batchDimStyle.Render("  [" + item.playlist + "]")
			}
			b.WriteString("  " + batchOKStyle.Render("✓") + "  " + label + "\n")
		case importSkipped:
			b.WriteString("  " + batchSkipStyle.Render("–") + "  " + batchDimStyle.Render(item.label+"  already in library") + "\n")
		case importFailed:
			errStr := ""
			if item.err != nil {
				errStr = "  " + batchFailStyle.Render(item.err.Error())
			}
			b.WriteString("  " + batchFailStyle.Render("✗") + "  " + item.label + errStr + "\n")
		}
	}

	b.WriteString("\n  ")
	b.WriteString(m.progress.View())
	done := m.added + m.skipped + m.failed
	b.WriteString("  ")
	b.WriteString(batchDimStyle.Render(fmt.Sprintf("%d / %d", done, len(m.items))))
	b.WriteString("\n  ")

	if m.allDone() {
		b.WriteString(batchDimStyle.Render("done"))
	} else {
		label := m.items[m.current].url
		if m.items[m.current].label != "" {
			label = m.items[m.current].label
		} else if plat, slug, err := platform.ParseURL(label); err == nil {
			label = plat + "/" + slug
		}
		b.WriteString(m.spinner.View() + " " + batchDimStyle.Render(label))
	}

	b.WriteString("\n\n  ")
	b.WriteString(batchOKStyle.Render(fmt.Sprintf("✓ %d added", m.added)))
	b.WriteString("  ")
	b.WriteString(batchSkipStyle.Render(fmt.Sprintf("– %d skipped", m.skipped)))
	b.WriteString("  ")
	b.WriteString(batchFailStyle.Render(fmt.Sprintf("✗ %d failed", m.failed)))
	if m.playlistsNew > 0 {
		b.WriteString("  ")
		b.WriteString(batchDimStyle.Render(fmt.Sprintf("%d playlist(s) created", m.playlistsNew)))
	}

	if m.allDone() {
		b.WriteString("\n\n  ")
		b.WriteString(batchDimStyle.Render("press any key to exit"))
	}

	b.WriteString("\n")
	return b.String()
}
