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

// Brand and theme-picker styles.
var (
	BrandDelby = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF"))

	BrandSoft = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#5865F2"))

	Selector = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF"))
)

// ConfigureTheme applies a complete semantic palette. Terminal mode omits
// explicit colors so the terminal's normal foreground and background inherit.
func ConfigureTheme(colors map[string]string, terminal bool) {
	accentColor = themedColor(colors, terminal, "accent", "#7C6AF7")
	dimColor = themedColor(colors, terminal, "muted", "#888888")
	errorColor = themedColor(colors, terminal, "error", "#FF5F5F")
	okColor = themedColor(colors, terminal, "success", "#5FFF87")
	titleColor = themedColor(colors, terminal, "foreground", "#FFFFFF")
	subtleColor = themedColor(colors, terminal, "muted", "#888888")
	boldColor = themedColor(colors, terminal, "foreground", "#E0E0E0")

	activeBorderStyle = themedBorder(lipgloss.NewStyle().Border(lipgloss.RoundedBorder()), colors, terminal, "accent", "#7C6AF7")
	inactiveBorderStyle = themedBorder(lipgloss.NewStyle().Border(lipgloss.RoundedBorder()), colors, terminal, "muted", "#888888")
	headerStyle = themedStyle(lipgloss.NewStyle().Bold(true).Padding(0, 1), colors, terminal, "foreground", "#FFFFFF")
	footerStyle = themedStyle(lipgloss.NewStyle().Padding(0, 1), colors, terminal, "muted", "#888888")
	versionStyle = themedStyle(lipgloss.NewStyle(), colors, terminal, "muted", "#888888")
	paneTitleStyle = themedStyle(lipgloss.NewStyle().Bold(true).Padding(0, 1), colors, terminal, "accent", "#7C6AF7")
	paneSubtitleStyle = themedStyle(lipgloss.NewStyle().Italic(true).Padding(0, 1), colors, terminal, "muted", "#888888")
	selectedItemStyle = themedStyle(lipgloss.NewStyle().Bold(true), colors, terminal, "accent", "#7C6AF7")
	normalItemStyle = themedStyle(lipgloss.NewStyle(), colors, terminal, "foreground", "#E0E0E0")
	itemDescStyle = themedStyle(lipgloss.NewStyle(), colors, terminal, "muted", "#888888")
	categoryStyle = themedStyle(lipgloss.NewStyle().Bold(true).Padding(0, 1), colors, terminal, "accent", "#7C6AF7")
	searchPromptStyle = themedStyle(lipgloss.NewStyle().Bold(true), colors, terminal, "accent", "#7C6AF7")
	outputTitleStyle = themedStyle(lipgloss.NewStyle().Bold(true).Padding(0, 1), colors, terminal, "foreground", "#FFFFFF")
	outputLabelStyle = themedStyle(lipgloss.NewStyle(), colors, terminal, "accent", "#7C6AF7")
	errorStyle = themedStyle(lipgloss.NewStyle().Padding(0, 1), colors, terminal, "error", "#FF5F5F")
	statusOkStyle = themedStyle(lipgloss.NewStyle(), colors, terminal, "success", "#5FFF87")
	statusErrStyle = themedStyle(lipgloss.NewStyle(), colors, terminal, "error", "#FF5F5F")
	popupStyle = themedBorder(lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2), colors, terminal, "accent", "#7C6AF7")
	popupTitleStyle = themedStyle(lipgloss.NewStyle().Bold(true), colors, terminal, "foreground", "#FFFFFF")
	activeFormatStyle = themedStyle(themedBackground(lipgloss.NewStyle().Bold(true).Padding(0, 1), colors, terminal, "accent", "#7C6AF7"), colors, terminal, "foreground", "#FFFFFF")
	inactiveFormatStyle = themedStyle(lipgloss.NewStyle().Padding(0, 1), colors, terminal, "muted", "#888888")
	popupPathStyle = themedStyle(lipgloss.NewStyle().Italic(true), colors, terminal, "muted", "#888888")

	BrandDelby = themedStyle(lipgloss.NewStyle().Bold(true), colors, terminal, "brand_primary", "#FFFFFF")
	BrandSoft = themedStyle(lipgloss.NewStyle().Bold(true), colors, terminal, "brand_secondary", "#5865F2")
	Selector = themedStyle(lipgloss.NewStyle().Bold(true), colors, terminal, "selector", "#FFFFFF")
}

func themedColor(colors map[string]string, terminal bool, key, fallback string) color.Color {
	if value, ok := themedValue(colors, terminal, key, fallback); ok {
		return lipgloss.Color(value)
	}
	return nil
}

func themedStyle(style lipgloss.Style, colors map[string]string, terminal bool, key, fallback string) lipgloss.Style {
	if value, ok := themedValue(colors, terminal, key, fallback); ok {
		return style.Foreground(lipgloss.Color(value))
	}
	return style
}

func themedBackground(style lipgloss.Style, colors map[string]string, terminal bool, key, fallback string) lipgloss.Style {
	if value, ok := themedValue(colors, terminal, key, fallback); ok {
		return style.Background(lipgloss.Color(value))
	}
	return style
}

func themedBorder(style lipgloss.Style, colors map[string]string, terminal bool, key, fallback string) lipgloss.Style {
	if value, ok := themedValue(colors, terminal, key, fallback); ok {
		return style.BorderForeground(lipgloss.Color(value))
	}
	return style
}

func themedValue(colors map[string]string, terminal bool, key, fallback string) (string, bool) {
	if value := colors[key]; value != "" {
		return value, true
	}
	if terminal {
		return "", false
	}
	return fallback, fallback != ""
}
