package tui

import (
	_ "embed"
	"fmt"
	"math"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

//go:embed splash.txt
var splashArt string

const splashAutoAdvanceTicks = 80 // ~4 seconds at 50ms/tick

type splashTickMsg struct{}

// SplashModel wraps an inner model, displaying an animated splash screen first.
// On keypress or after ~4s it transitions seamlessly to the inner model.
type SplashModel struct {
	inner  tea.Model
	done   bool
	frame  int
	lines  []string
	artW   int
	width  int
	height int
}

func NewSplashModel(inner tea.Model) SplashModel {
	raw := strings.TrimRight(splashArt, "\n")
	lines := strings.Split(raw, "\n")
	artW := 0
	for _, l := range lines {
		if w := len([]rune(l)); w > artW {
			artW = w
		}
	}
	return SplashModel{inner: inner, lines: lines, artW: artW}
}

func (m SplashModel) Init() tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(time.Time) tea.Msg { return splashTickMsg{} })
}

func (m SplashModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.done {
		newInner, cmd := m.inner.Update(msg)
		m.inner = newInner
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		// Forward so inner is correctly sized when it activates.
		newInner, _ := m.inner.Update(msg)
		m.inner = newInner
		return m, nil

	case splashTickMsg:
		m.frame++
		if m.frame >= splashAutoAdvanceTicks {
			m.done = true
			return m, m.inner.Init()
		}
		return m, tea.Tick(50*time.Millisecond, func(time.Time) tea.Msg { return splashTickMsg{} })

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		m.done = true
		return m, m.inner.Init()
	}

	return m, nil
}

func (m SplashModel) View() string {
	if m.done {
		return m.inner.View()
	}
	return m.renderSplash()
}

func (m SplashModel) renderSplash() string {
	var b strings.Builder

	totalH := len(m.lines) + 3 // art + blank line + hint
	topPad := 0
	if m.height > totalH {
		topPad = (m.height - totalH) / 2
	}
	leftPad := 0
	if m.width > m.artW {
		leftPad = (m.width - m.artW) / 2
	}
	pad := strings.Repeat(" ", leftPad)

	for range topPad {
		b.WriteByte('\n')
	}

	for _, line := range m.lines {
		b.WriteString(pad)
		col := 0
		for _, ch := range line {
			if ch == ' ' {
				b.WriteByte(' ')
				col++
				continue
			}
			hue := math.Mod(float64(col)*1.5+float64(m.frame)*3.0, 360.0)
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(hslHex(hue, 0.85, 0.65))).Render(string(ch)))
			col++
		}
		b.WriteByte('\n')
	}

	hint := lipgloss.NewStyle().Foreground(colorDim).Render("press any key")
	// Fill remaining lines so the hint lands on the last row.
	artBottom := topPad + len(m.lines)
	blankLines := m.height - artBottom - 1
	if blankLines < 1 {
		blankLines = 1
	}
	for range blankLines {
		b.WriteByte('\n')
	}
	b.WriteString(pad)
	b.WriteString(hint)

	return b.String()
}

// hslHex converts HSL (h in [0,360], s and l in [0,1]) to a "#rrggbb" string.
func hslHex(h, s, l float64) string {
	c := (1 - math.Abs(2*l-1)) * s
	x := c * (1 - math.Abs(math.Mod(h/60.0, 2)-1))
	m := l - c/2
	var r, g, b float64
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	return fmt.Sprintf("#%02x%02x%02x",
		int(math.Round((r+m)*255)),
		int(math.Round((g+m)*255)),
		int(math.Round((b+m)*255)),
	)
}
