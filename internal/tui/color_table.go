package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ctColumn struct {
	title    string
	width    int
	padRight int // extra right padding after this column
}

type colorTable struct {
	cols    []ctColumn
	rows    [][]string
	cursor  int
	offset  int
	height  int
	focused bool
}

func newColorTable(cols []ctColumn, height int) colorTable {
	return colorTable{cols: cols, height: height, focused: true}
}

func (t *colorTable) SetRows(rows [][]string) {
	t.rows = rows
	if t.cursor >= len(rows) {
		t.cursor = max(0, len(rows)-1)
	}
	t.clampOffset()
}

func (t *colorTable) SetHeight(h int) { t.height = h; t.clampOffset() }
func (t colorTable) Cursor() int      { return t.cursor }

func (t colorTable) Update(msg tea.Msg) (colorTable, tea.Cmd) {
	if !t.focused {
		return t, nil
	}
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
			if t.cursor > 0 {
				t.cursor--
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
			if t.cursor < len(t.rows)-1 {
				t.cursor++
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("pgup"))):
			t.cursor -= t.height
			if t.cursor < 0 {
				t.cursor = 0
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("pgdown"))):
			t.cursor += t.height
			if t.cursor >= len(t.rows) {
				t.cursor = len(t.rows) - 1
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("home"))):
			t.cursor = 0
		case key.Matches(msg, key.NewBinding(key.WithKeys("end"))):
			t.cursor = len(t.rows) - 1
		}
		t.clampOffset()
	}
	return t, nil
}

func (t *colorTable) clampOffset() {
	if t.cursor < t.offset {
		t.offset = t.cursor
	}
	if t.cursor >= t.offset+t.height {
		t.offset = t.cursor - t.height + 1
	}
	if t.offset < 0 {
		t.offset = 0
	}
}

var (
	ctHeaderStyle = lipgloss.NewStyle().
			Foreground(colorDim).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(colorDimBg).
			BorderBottom(true)
	ctSelectedStyle = lipgloss.NewStyle().
			Foreground(colorBright).
			Background(colorAccent).
			Bold(true)
)

func (t colorTable) renderCell(col ctColumn, val string) string {
	padL := 1
	padR := 1 + col.padRight
	contentWidth := col.width - padL - padR
	if contentWidth < 1 {
		contentWidth = 1
	}
	if lipgloss.Width(val) > contentWidth {
		val = truncate(val, contentWidth)
	}
	cell := lipgloss.NewStyle().
		Width(col.width).
		MaxWidth(col.width).
		PaddingLeft(padL).
		PaddingRight(padR).
		Inline(true).
		Render(val)
	return cell
}

func (t colorTable) View() string {
	var b strings.Builder

	// Header
	var headerCells []string
	for _, col := range t.cols {
		headerCells = append(headerCells, t.renderCell(col, col.title))
	}
	b.WriteString(ctHeaderStyle.Render(lipgloss.JoinHorizontal(lipgloss.Top, headerCells...)))
	b.WriteString("\n")

	// Rows
	end := t.offset + t.height
	if end > len(t.rows) {
		end = len(t.rows)
	}
	for i := t.offset; i < end; i++ {
		row := t.rows[i]
		var cells []string
		for j, col := range t.cols {
			val := ""
			if j < len(row) {
				val = row[j]
			}
			cells = append(cells, t.renderCell(col, val))
		}
		line := lipgloss.JoinHorizontal(lipgloss.Top, cells...)
		if i == t.cursor {
			line = ctSelectedStyle.Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	// Fill remaining height with blank lines
	for i := end - t.offset; i < t.height; i++ {
		b.WriteString("\n")
	}

	return b.String()
}
