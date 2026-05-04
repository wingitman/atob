package tui

import (
	"os"
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/lipgloss/v2"
)

// newTextarea creates a configured textarea for the input pane.
func newTextarea() textarea.Model {
	ta := textarea.New()
	ta.Placeholder = "Enter input here…"
	ta.ShowLineNumbers = false
	ta.SetWidth(40)
	ta.SetHeight(10)
	ta.CharLimit = 0 // unlimited

	s := ta.Styles()
	s.Focused.Base = lipgloss.NewStyle()
	s.Blurred.Base = lipgloss.NewStyle()
	ta.SetStyles(s)

	return ta
}

// placeholderFor returns the input placeholder for a given converter.
func placeholderFor(e *pickerEntry) string {
	if e == nil {
		return "Select a converter from the list…"
	}
	if e.FileBased || isBinaryTarget(e.To) {
		return "Enter file path (e.g. /usr/bin/ls or ./myfile.bin)…"
	}
	switch e.From {
	case "json":
		return `Enter JSON (e.g. {"key": "value"})…`
	case "yaml":
		return "Enter YAML…"
	case "toml":
		return "Enter TOML…"
	case "xml":
		return "Enter XML…"
	case "csv":
		return "Enter CSV (first row = headers)…"
	case "base64":
		return "Enter Base64-encoded string…"
	case "epoch":
		return "Enter Unix timestamp (e.g. 1741000000)…"
	case "any":
		return "Enter text to convert case…"
	}
	return "Enter input…"
}

// isBinaryTarget returns true for targets that expect file-path input.
func isBinaryTarget(to string) bool {
	switch to {
	case "inspect", "hexdump", "strings":
		return true
	}
	return false
}

// subtitleFor returns the dim subtitle shown under the textarea.
func subtitleFor(e *pickerEntry) string {
	if e == nil {
		return ""
	}
	from := e.From
	if from == "any" {
		from = "text"
	}
	return from + " → " + e.To
}

// isFilePath returns true if the content looks like a readable file path.
func isFilePath(content string) bool {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return false
	}
	// Must look path-like (starts with / ~ . or a letter on Windows)
	if !strings.HasPrefix(trimmed, "/") &&
		!strings.HasPrefix(trimmed, "~") &&
		!strings.HasPrefix(trimmed, "./") &&
		!strings.HasPrefix(trimmed, "../") {
		return false
	}
	expanded := expandPath(trimmed)
	info, err := os.Stat(expanded)
	return err == nil && !info.IsDir()
}

// expandPath resolves ~ to the home directory.
func expandPath(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return home + p[1:]
		}
	}
	return p
}

// renderInput renders the middle pane. Returns the rendered string.
func renderInput(ta textarea.Model, entry *pickerEntry, width, height int, focused bool) string {
	var sb strings.Builder

	// Title
	title := paneTitleStyle.Render("INPUT")
	sb.WriteString(title + "\n")

	// Textarea (takes up most of the height)
	taHeight := height - 3 // title + subtitle + divider
	if taHeight < 1 {
		taHeight = 1
	}
	ta.SetWidth(width - 2)
	ta.SetHeight(taHeight)
	sb.WriteString(ta.View() + "\n")

	// Divider + subtitle
	divider := lipgloss.NewStyle().Foreground(dimColor).Render(strings.Repeat("─", width-2))
	sb.WriteString(divider + "\n")

	subtitle := subtitleFor(entry)
	if subtitle != "" {
		sb.WriteString(paneSubtitleStyle.Width(width - 2).Render(subtitle))
	} else {
		sb.WriteString(strings.Repeat(" ", width-2))
	}

	return sb.String()
}
