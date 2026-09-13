package tui

import "charm.land/lipgloss/v2"

// Colors follow Letterboxd's palette.
var (
	orange = lipgloss.Color("#FF8000")
	green  = lipgloss.Color("#00E054")
	blue   = lipgloss.Color("#40BCF4")
	red    = lipgloss.Color("#FF5C5C")
	ink    = lipgloss.Color("#14181C")
	text   = lipgloss.Color("#DDE6ED")
	muted  = lipgloss.Color("#678899")
)

var (
	logoStyle     = lipgloss.NewStyle().Bold(true).Foreground(ink).Background(green).Padding(0, 1)
	textStyle     = lipgloss.NewStyle().Foreground(text)
	mutedStyle    = lipgloss.NewStyle().Foreground(muted)
	boldStyle     = lipgloss.NewStyle().Bold(true).Foreground(text)
	doneStyle     = lipgloss.NewStyle().Foreground(green)
	activeStyle   = lipgloss.NewStyle().Foreground(orange)
	errorStyle    = lipgloss.NewStyle().Foreground(red)
	accentStyle   = lipgloss.NewStyle().Bold(true).Foreground(blue)
	starStyle     = lipgloss.NewStyle().Bold(true).Foreground(green)
	barStyle      = lipgloss.NewStyle().Foreground(green)
	selectedBar   = lipgloss.NewStyle().Foreground(orange)
	countStyle    = lipgloss.NewStyle().Bold(true).Foreground(text)
	posterBorder  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(muted)
	dividerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#2C3440"))
	keyStyle      = lipgloss.NewStyle().Foreground(text)
	helpTextStyle = lipgloss.NewStyle().Foreground(muted)
)
