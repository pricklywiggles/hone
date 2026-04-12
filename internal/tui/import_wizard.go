package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"charm.land/bubbles/v2/filepicker"
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jmoiron/sqlx"
	"github.com/pricklywiggles/hone/internal/backup"
	"github.com/pricklywiggles/hone/internal/db"
	"github.com/pricklywiggles/hone/internal/importer"
)

// ── wizard choice items ─────────────────────────────────────────────────────

type wizardItem struct {
	title string
	desc  string
}

func (i wizardItem) Title() string       { return i.title }
func (i wizardItem) Description() string { return i.desc }
func (i wizardItem) FilterValue() string { return i.title }

// ── wizard states ───────────────────────────────────────────────────────────

type importWizardState int

const (
	iwChooseType importWizardState = iota
	iwChooseFile
	iwInputURL
	iwRunning
	iwDone
)

// ── messages ────────────────────────────────────────────────────────────────

type backupRestoreMsg struct {
	problems, playlists, attempts int
	err                           error
}

// ── model ───────────────────────────────────────────────────────────────────

type ImportWizardModel struct {
	state      importWizardState
	importType int // 0=playlist, 1=backup, 2=url
	list       list.Model
	picker     filepicker.Model
	urlInput   textinput.Model
	spinner    spinner.Model
	help       help.Model

	// Embedded sub-model for playlist import execution.
	importModel *ImportModel

	resultMsg string
	errMsg    string

	db         *sqlx.DB
	dataDir    string
	profileDir string
	width      int
	height     int
}

func NewImportWizardModel(db *sqlx.DB, dataDir, profileDir string) ImportWizardModel {
	items := []list.Item{
		wizardItem{title: "Playlist file", desc: "Import problems from a text file with # playlist headers"},
		wizardItem{title: "Backup file", desc: "Restore from a JSON backup created by hone export --backup"},
		wizardItem{title: "Single URL", desc: "Add one problem by pasting its URL"},
	}
	l := newWizardList(items, "Import")

	ti := textinput.New()
	ti.Placeholder = "https://neetcode.io/problems/…"
	ti.CharLimit = 300
	ti.SetWidth(60)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(colorAccent)

	return ImportWizardModel{
		state:      iwChooseType,
		list:       l,
		urlInput:   ti,
		spinner:    sp,
		help:       newHelpModel(),
		db:         db,
		dataDir:    dataDir,
		profileDir: profileDir,
	}
}

func (m ImportWizardModel) Init() tea.Cmd { return nil }

func (m ImportWizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width-4, msg.Height-6)
		return m, nil

	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if msg.String() == "q" && m.state == iwChooseType {
			return m, tea.Quit
		}
		if msg.String() == "esc" && m.state != iwChooseType && m.state != iwRunning {
			m.state = iwChooseType
			return m, nil
		}
	}

	switch m.state {
	case iwChooseType:
		return m.updateChooseType(msg)
	case iwChooseFile:
		return m.updateChooseFile(msg)
	case iwInputURL:
		return m.updateInputURL(msg)
	case iwRunning:
		return m.updateRunning(msg)
	case iwDone:
		if _, ok := msg.(tea.KeyMsg); ok {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m ImportWizardModel) updateChooseType(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok && msg.String() == "enter" {
		i, ok := m.list.SelectedItem().(wizardItem)
		if !ok {
			return m, nil
		}
		switch i.title {
		case "Playlist file":
			m.importType = 0
			m.state = iwChooseFile
			fp := filepicker.New()
			fp.CurrentDirectory, _ = os.Getwd()
			fp.AutoHeight = false
			fp.SetHeight(m.height - 8)
			m.picker = fp
			return m, m.picker.Init()
		case "Backup file":
			m.importType = 1
			m.state = iwChooseFile
			fp := filepicker.New()
			fp.CurrentDirectory, _ = os.Getwd()
			fp.AllowedTypes = []string{".json"}
			fp.AutoHeight = false
			fp.SetHeight(m.height - 8)
			m.picker = fp
			return m, m.picker.Init()
		case "Single URL":
			m.importType = 2
			m.state = iwInputURL
			m.urlInput.Focus()
			return m, textinput.Blink
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m ImportWizardModel) updateChooseFile(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.picker, cmd = m.picker.Update(msg)

	if didSelect, path := m.picker.DidSelectFile(msg); didSelect {
		m.state = iwRunning
		if m.importType == 0 {
			return m, m.startPlaylistImport(path)
		}
		return m, m.startBackupRestore(path)
	}

	return m, cmd
}

func (m ImportWizardModel) updateInputURL(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok && msg.String() == "enter" {
		url := m.urlInput.Value()
		if url == "" {
			return m, nil
		}
		addModel := NewAddModel(m.db, m.profileDir, url)
		return addModel, addModel.Init()
	}
	var cmd tea.Cmd
	m.urlInput, cmd = m.urlInput.Update(msg)
	return m, cmd
}

func (m ImportWizardModel) updateRunning(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case playlistImportReadyMsg:
		if msg.err != nil {
			m.state = iwDone
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.importModel = &msg.model
		return m, m.importModel.Init()

	case backupRestoreMsg:
		m.state = iwDone
		if msg.err != nil {
			m.errMsg = msg.err.Error()
		} else {
			m.resultMsg = fmt.Sprintf("Restored %d problem(s), %d playlist(s), %d attempt(s)",
				msg.problems, msg.playlists, msg.attempts)
		}
		return m, nil
	}

	if m.importModel != nil {
		if _, ok := msg.(tea.KeyMsg); ok && m.importModel.Done() {
			added, skipped, failed := m.importModel.Stats()
			m.state = iwDone
			m.resultMsg = fmt.Sprintf("%d added, %d skipped, %d failed", added, skipped, failed)
			if failed > 0 {
				m.resultMsg += fmt.Sprintf("\n  %d URL(s) failed", failed)
			}
			return m, nil
		}
		model, cmd := m.importModel.Update(msg)
		im := model.(ImportModel)
		m.importModel = &im
		return m, cmd
	}

	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

// ── commands ────────────────────────────────────────────────────────────────

type playlistImportReadyMsg struct {
	model ImportModel
	err   error
}

func (m ImportWizardModel) startPlaylistImport(path string) tea.Cmd {
	return func() tea.Msg {
		groups, err := importer.ParseImportFile(path)
		if err != nil {
			return playlistImportReadyMsg{err: err}
		}
		total := 0
		for _, g := range groups {
			total += len(g.URLs)
		}
		if total == 0 {
			return playlistImportReadyMsg{err: fmt.Errorf("no URLs found in file")}
		}
		model := NewImportModel(m.db, m.profileDir, groups)
		return playlistImportReadyMsg{model: model}
	}
}

func (m ImportWizardModel) startBackupRestore(path string) tea.Cmd {
	dataDir := m.dataDir
	callerDB := m.db
	return func() tea.Msg {
		dbPath := filepath.Join(dataDir, "data.db")
		// Close the DB that PersistentPreRunE opened so we can replace it.
		if callerDB != nil {
			callerDB.Close()
		}
		os.Remove(dbPath)

		raw, err := os.ReadFile(path)
		if err != nil {
			return backupRestoreMsg{err: fmt.Errorf("read backup: %w", err)}
		}

		var data backup.BackupData
		if err := json.Unmarshal(raw, &data); err != nil {
			return backupRestoreMsg{err: fmt.Errorf("parse backup: %w", err)}
		}

		newDB, err := db.Open(dataDir)
		if err != nil {
			return backupRestoreMsg{err: fmt.Errorf("create database: %w", err)}
		}
		defer newDB.Close()

		if err := backup.RestoreFromBackup(newDB, data); err != nil {
			newDB.Close()
			os.Remove(dbPath)
			return backupRestoreMsg{err: fmt.Errorf("restore: %w", err)}
		}

		return backupRestoreMsg{
			problems:  len(data.Problems),
			playlists: len(data.Playlists),
			attempts:  len(data.Attempts),
		}
	}
}

// ── view ────────────────────────────────────────────────────────────────────

func (m ImportWizardModel) View() tea.View {
	switch m.state {
	case iwChooseType:
		return tea.NewView(m.list.View())

	case iwChooseFile:
		header := "  Select a file"
		if m.importType == 1 {
			header = "  Select a JSON backup file"
		}
		return tea.NewView(lipgloss.NewStyle().Bold(true).Foreground(colorAccent).Render(header) + "\n\n" +
			m.picker.View() + "\n\n  " +
			m.help.View(wizardFileKeyMap{}))

	case iwInputURL:
		return tea.NewView("\n  " +
			lipgloss.NewStyle().Bold(true).Foreground(colorAccent).Render("Enter a problem URL") +
			"\n\n  " + m.urlInput.View() + "\n\n  " +
			m.help.View(wizardInputKeyMap{}))

	case iwRunning:
		if m.importModel != nil {
			return m.importModel.View()
		}
		return tea.NewView("\n  " + m.spinner.View() + " Restoring from backup…\n")

	case iwDone:
		if m.errMsg != "" {
			return tea.NewView("\n  " + lipgloss.NewStyle().Foreground(colorDanger).Render("Error: "+m.errMsg) +
				"\n\n  " + statsDimStyle.Render("press any key to exit") + "\n")
		}
		return tea.NewView("\n  " + lipgloss.NewStyle().Foreground(colorSuccess).Render("✓ "+m.resultMsg) +
			"\n\n  " + statsDimStyle.Render("press any key to exit") + "\n")
	}
	return tea.NewView("")
}

// ── shared wizard helpers ───────────────────────────────────────────────────

func newWizardList(items []list.Item, title string) list.Model {
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(colorAccent).
		BorderLeftForeground(colorAccent)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(colorDim).
		BorderLeftForeground(colorAccent)
	l := list.New(items, delegate, 60, 14)
	l.Title = title
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = lipgloss.NewStyle().Bold(true).Foreground(colorAccent).Padding(0, 1)
	return l
}
