package tui

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jmoiron/sqlx"
	"github.com/pricklywiggles/hone/internal/backup"
	"github.com/pricklywiggles/hone/internal/store"
)

// ── wizard states ───────────────────────────────────────────────────────────

type exportWizardState int

const (
	ewChooseType exportWizardState = iota
	ewChoosePlaylist
	ewChooseDest
	ewInputPath
	ewRunning
	ewDone
)

// ── messages ────────────────────────────────────────────────────────────────

type exportResultMsg struct {
	content  string
	err      error
	wrote    string // file path if written to file
}

// ── model ───────────────────────────────────────────────────────────────────

type ExportWizardModel struct {
	state      exportWizardState
	exportType int    // 0=backup, 1=playlist
	playlist   string // "" = all, otherwise specific name
	toFile     bool
	filePath   string

	typeList     list.Model
	playlistList list.Model
	destList     list.Model
	pathInput    textinput.Model
	spinner      spinner.Model
	help         help.Model

	resultMsg string
	errMsg    string

	db     *sqlx.DB
	width  int
	height int
}

func NewExportWizardModel(db *sqlx.DB) ExportWizardModel {
	typeItems := []list.Item{
		wizardItem{title: "Backup (JSON)", desc: "Full backup of all data — problems, SRS state, attempts, playlists"},
		wizardItem{title: "Playlist (text)", desc: "Export playlists as a text file compatible with hone import --playlist"},
	}
	typeList := newWizardList(typeItems, "Export")

	destItems := []list.Item{
		wizardItem{title: "Print to terminal", desc: "Output to stdout"},
		wizardItem{title: "Save to file", desc: "Write to a file on disk"},
	}
	destList := newWizardList(destItems, "Destination")

	ti := textinput.New()
	ti.Placeholder = "output.json"
	ti.CharLimit = 200
	ti.Width = 50

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(colorAccent)

	return ExportWizardModel{
		state:    ewChooseType,
		typeList: typeList,
		destList: destList,
		pathInput: ti,
		spinner:  sp,
		help:     newHelpModel(),
		db:       db,
	}
}

func (m ExportWizardModel) Init() tea.Cmd { return nil }

func (m ExportWizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.typeList.SetSize(msg.Width-4, msg.Height-6)
		m.destList.SetSize(msg.Width-4, msg.Height-6)
		m.playlistList.SetSize(msg.Width-4, msg.Height-6)
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if msg.String() == "q" && m.state == ewChooseType {
			return m, tea.Quit
		}
		if msg.String() == "esc" {
			return m.handleBack()
		}
	}

	switch m.state {
	case ewChooseType:
		return m.updateChooseType(msg)
	case ewChoosePlaylist:
		return m.updateChoosePlaylist(msg)
	case ewChooseDest:
		return m.updateChooseDest(msg)
	case ewInputPath:
		return m.updateInputPath(msg)
	case ewRunning:
		return m.updateRunning(msg)
	case ewDone:
		if _, ok := msg.(tea.KeyMsg); ok {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m ExportWizardModel) handleBack() (tea.Model, tea.Cmd) {
	switch m.state {
	case ewChoosePlaylist:
		m.state = ewChooseType
	case ewChooseDest:
		if m.exportType == 1 {
			m.state = ewChoosePlaylist
		} else {
			m.state = ewChooseType
		}
	case ewInputPath:
		m.state = ewChooseDest
	default:
		return m, nil
	}
	return m, nil
}

func (m ExportWizardModel) updateChooseType(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok && msg.String() == "enter" {
		i, ok := m.typeList.SelectedItem().(wizardItem)
		if !ok {
			return m, nil
		}
		switch i.title {
		case "Backup (JSON)":
			m.exportType = 0
			m.state = ewChooseDest
			m.pathInput.SetValue("")
			m.pathInput.Placeholder = "backup.json"
		case "Playlist (text)":
			m.exportType = 1
			m.state = ewChoosePlaylist
			return m, m.loadPlaylists()
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.typeList, cmd = m.typeList.Update(msg)
	return m, cmd
}

type exportPlaylistsLoadedMsg struct {
	playlists []store.Playlist
	err       error
}

func (m ExportWizardModel) loadPlaylists() tea.Cmd {
	db := m.db
	return func() tea.Msg {
		playlists, err := store.ListPlaylists(db)
		return exportPlaylistsLoadedMsg{playlists: playlists, err: err}
	}
}

func (m ExportWizardModel) updateChoosePlaylist(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case exportPlaylistsLoadedMsg:
		if msg.err != nil {
			m.state = ewDone
			m.errMsg = msg.err.Error()
			return m, nil
		}
		items := []list.Item{
			wizardItem{title: "All playlists", desc: "Export every playlist in one file"},
		}
		for _, p := range msg.playlists {
			items = append(items, wizardItem{
				title: p.Name,
				desc:  fmt.Sprintf("%d problems", p.ProblemCount),
			})
		}
		m.playlistList = newWizardList(items, "Choose playlist")
		m.playlistList.SetSize(m.width-4, m.height-6)
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "enter" {
			i, ok := m.playlistList.SelectedItem().(wizardItem)
			if !ok {
				return m, nil
			}
			if i.title == "All playlists" {
				m.playlist = ""
			} else {
				m.playlist = i.title
			}
			m.state = ewChooseDest
			m.pathInput.SetValue("")
			m.pathInput.Placeholder = "playlists.txt"
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.playlistList, cmd = m.playlistList.Update(msg)
	return m, cmd
}

func (m ExportWizardModel) updateChooseDest(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok && msg.String() == "enter" {
		i, ok := m.destList.SelectedItem().(wizardItem)
		if !ok {
			return m, nil
		}
		switch i.title {
		case "Print to terminal":
			m.toFile = false
			m.state = ewRunning
			return m, tea.Batch(m.spinner.Tick, m.runExport())
		case "Save to file":
			m.toFile = true
			m.state = ewInputPath
			m.pathInput.Focus()
			return m, textinput.Blink
		}
	}
	var cmd tea.Cmd
	m.destList, cmd = m.destList.Update(msg)
	return m, cmd
}

func (m ExportWizardModel) updateInputPath(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok && msg.String() == "enter" {
		path := m.pathInput.Value()
		if path == "" {
			return m, nil
		}
		m.filePath = path
		m.state = ewRunning
		return m, tea.Batch(m.spinner.Tick, m.runExport())
	}
	var cmd tea.Cmd
	m.pathInput, cmd = m.pathInput.Update(msg)
	return m, cmd
}

func (m ExportWizardModel) updateRunning(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case exportResultMsg:
		m.state = ewDone
		if msg.err != nil {
			m.errMsg = msg.err.Error()
		} else if msg.wrote != "" {
			m.resultMsg = fmt.Sprintf("Wrote %s", msg.wrote)
		} else {
			m.resultMsg = msg.content
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m ExportWizardModel) runExport() tea.Cmd {
	db := m.db
	toFile := m.toFile
	filePath := m.filePath
	exportType := m.exportType
	playlist := m.playlist

	return func() tea.Msg {
		var content string
		var err error

		if exportType == 0 {
			data, e := backup.ExportFullBackup(db)
			if e != nil {
				return exportResultMsg{err: e}
			}
			b, e := json.MarshalIndent(data, "", "  ")
			if e != nil {
				return exportResultMsg{err: e}
			}
			content = string(b) + "\n"
		} else {
			if playlist == "" {
				content, err = backup.ExportPlaylistFormat(db)
			} else {
				content, err = backup.ExportSinglePlaylistFormat(db, playlist)
			}
			if err != nil {
				return exportResultMsg{err: err}
			}
		}

		if toFile {
			if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
				return exportResultMsg{err: err}
			}
			return exportResultMsg{wrote: filePath}
		}
		return exportResultMsg{content: content}
	}
}

// ── view ────────────────────────────────────────────────────────────────────

func (m ExportWizardModel) View() string {
	switch m.state {
	case ewChooseType:
		return m.typeList.View()

	case ewChoosePlaylist:
		if len(m.playlistList.Items()) == 0 {
			return "\n  " + m.spinner.View() + " Loading playlists…\n"
		}
		return m.playlistList.View()

	case ewChooseDest:
		return m.destList.View()

	case ewInputPath:
		return "\n  " +
			lipgloss.NewStyle().Bold(true).Foreground(colorAccent).Render("Save to") +
			"\n\n  " + m.pathInput.View() + "\n\n  " +
			m.help.View(wizardInputKeyMap{})

	case ewRunning:
		return "\n  " + m.spinner.View() + " Exporting…\n"

	case ewDone:
		if m.errMsg != "" {
			return "\n  " + lipgloss.NewStyle().Foreground(colorDanger).Render("Error: "+m.errMsg) +
				"\n\n  " + statsDimStyle.Render("press any key to exit") + "\n"
		}
		if m.resultMsg != "" && m.toFile {
			return "\n  " + lipgloss.NewStyle().Foreground(colorSuccess).Render("✓ "+m.resultMsg) +
				"\n\n  " + statsDimStyle.Render("press any key to exit") + "\n"
		}
		// Printed to terminal: show content then exit hint.
		return "\n" + m.resultMsg + "\n  " + statsDimStyle.Render("press any key to exit") + "\n"
	}
	return ""
}
