package tui

import "charm.land/lipgloss/v2"

var (
	colorAccent   = lipgloss.Color("#7C8EF2") // soft blue-purple — headers, borders, primary accent
	colorSuccess  = lipgloss.Color("#73D0A0") // soft green — easy, success, checkmarks
	colorWarning  = lipgloss.Color("#E0B464") // warm amber — medium, due today
	colorDanger   = lipgloss.Color("#E06C75") // soft coral — hard, errors, failures
	colorStreak   = lipgloss.Color("#D19A66") // soft orange — streak, overdue
	colorMastered = lipgloss.Color("#E5C07B") // soft gold — mastered, stars
	colorDim      = lipgloss.Color("#5C6370") // medium gray — labels, muted text
	colorDimBg    = lipgloss.Color("#3E4451") // dark gray — empty bars, backgrounds
	colorBright   = lipgloss.Color("#ABB2BF") // light gray — prominent text
	colorBarDone  = lipgloss.Color("#56B6C2") // muted teal — mastered segment in progress bars
	colorBarWIP   = lipgloss.Color("#8A7FBD") // dusty lavender — attempted segment in progress bars
	colorBarFail  = lipgloss.Color("#A8848A") // warm mauve — failure segment in weakest topics bars
)
