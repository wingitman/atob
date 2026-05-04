package tui

import (
	"image/color"
	"os"

	"charm.land/lipgloss/v2"
)

var isDark = lipgloss.HasDarkBackground(os.Stdin, os.Stdout)

// col returns the correct colour for dark/light backgrounds.
var lightDark = lipgloss.LightDark(isDark)

func col(dark, light string) color.Color {
	return lightDark(lipgloss.Color(light), lipgloss.Color(dark))
}

var (
	accentColor = col("#7C6AF7", "#0D7A6C")
	dimColor    = col("#555555", "#AAAAAA")
	errorColor  = col("#FF5F5F", "#CC0000")
	okColor     = col("#5FFF87", "#007700")
	titleColor  = col("#FFFFFF", "#000000")
	subtleColor = col("#888888", "#666666")
	boldColor   = col("#E0E0E0", "#111111")
)

// Border styles — active pane has accented border, inactive is dim.
var (
	activeBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(accentColor)

	inactiveBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(dimColor)
)

// Header / footer bar styles.
var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(titleColor).
			Padding(0, 1)

	footerStyle = lipgloss.NewStyle().
			Foreground(subtleColor).
			Padding(0, 1)

	versionStyle = lipgloss.NewStyle().
			Foreground(dimColor)
)

// Pane title styles.
var (
	paneTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(accentColor).
			Padding(0, 1)

	paneSubtitleStyle = lipgloss.NewStyle().
				Foreground(subtleColor).
				Italic(true).
				Padding(0, 1)
)

// List item styles.
var (
	selectedItemStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(accentColor)

	normalItemStyle = lipgloss.NewStyle().
			Foreground(boldColor)

	itemDescStyle = lipgloss.NewStyle().
			Foreground(dimColor)

	categoryStyle = lipgloss.NewStyle().
			Foreground(accentColor).
			Bold(true).
			Padding(0, 1)

	searchPromptStyle = lipgloss.NewStyle().
				Foreground(accentColor).
				Bold(true)
)

// Output pane styles.
var (
	outputTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(titleColor).
				Padding(0, 1)

	outputLabelStyle = lipgloss.NewStyle().
				Foreground(accentColor)

	errorStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Padding(0, 1)

	statusOkStyle = lipgloss.NewStyle().
			Foreground(okColor)

	statusErrStyle = lipgloss.NewStyle().
			Foreground(errorColor)
)

// Save popup styles.
var (
	popupStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accentColor).
			Padding(1, 2)

	popupTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(titleColor)

	activeFormatStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(titleColor).
				Background(accentColor).
				Padding(0, 1)

	inactiveFormatStyle = lipgloss.NewStyle().
				Foreground(dimColor).
				Padding(0, 1)

	popupPathStyle = lipgloss.NewStyle().
			Foreground(subtleColor).
			Italic(true)
)
