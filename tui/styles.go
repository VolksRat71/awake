package tui

import "github.com/charmbracelet/lipgloss"

var (
	green  = lipgloss.Color("#00FF87")
	yellow = lipgloss.Color("#FFDB58")
	red    = lipgloss.Color("#FF6B6B")
	dim    = lipgloss.Color("#666666")
	white  = lipgloss.Color("#FFFFFF")
	cyan   = lipgloss.Color("#00D7FF")

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(cyan).
			Align(lipgloss.Center)

	statusActiveStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(green)

	statusWarningStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(yellow)

	statusIdleStyle = lipgloss.NewStyle().
			Foreground(dim)

	labelStyle = lipgloss.NewStyle().
			Foreground(dim)

	valueStyle = lipgloss.NewStyle().
			Foreground(white)

	countdownStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(white)

	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(cyan)

	normalStyle = lipgloss.NewStyle().
			Foreground(white)

	hotkeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(cyan)

	footerStyle = lipgloss.NewStyle().
			Foreground(dim)

	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(dim).
			Padding(0, 1)

	activeBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(green).
				Padding(0, 1)

	warningBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(yellow).
				Padding(0, 1)
)
