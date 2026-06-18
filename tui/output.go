package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
)

// saveFormat represents the format choice in the save popup.
type saveFormat int

const (
	saveFormatRaw saveFormat = iota
	saveFormatPretty
)

// savePopupState holds the state of the save-output popup.
type savePopupState struct {
	open          bool
	format        saveFormat
	previewPath   string
	statusMessage string
}

// newViewport creates a configured viewport for the output pane.
func newViewport(width, height int) viewport.Model {
	vp := viewport.New(viewport.WithWidth(width), viewport.WithHeight(height))
	return vp
}

// outputExtFor returns the file extension appropriate for the converter's output.
func outputExtFor(entry *pickerEntry) string {
	if entry == nil {
		return ".txt"
	}
	switch entry.To {
	case "yaml":
		return ".yaml"
	case "toml":
		return ".toml"
	case "xml":
		return ".xml"
	case "csv":
		return ".csv"
	case "json", "json-pretty", "inspect":
		return ".json"
	case "decompile":
		return ".md"
	case "hexdump":
		return ".txt"
	case "strings":
		return ".txt"
	case "base64":
		return ".b64"
	default:
		return ".txt"
	}
}

// saveFileName generates a timestamped filename for the saved output.
func saveFileName(entry *pickerEntry, format saveFormat, ext string) string {
	label := "output"
	if entry != nil {
		label = strings.ReplaceAll(entry.To, " ", "-")
	}
	stamp := time.Now().Format("20060102-150405")
	return fmt.Sprintf("atob-%s-%s%s", label, stamp, ext)
}

// saveOutput writes content to a file in the save directory.
// Returns the path it was saved to or an error.
func saveOutput(content string, entry *pickerEntry, format saveFormat, saveDir string) (string, error) {
	if saveDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		saveDir = filepath.Join(home, "Downloads")
	}
	if err := os.MkdirAll(saveDir, 0o755); err != nil {
		return "", fmt.Errorf("cannot create save directory: %w", err)
	}

	ext := outputExtFor(entry)
	name := saveFileName(entry, format, ext)
	path := filepath.Join(saveDir, name)

	finalContent := content
	if format == saveFormatPretty && (ext == ".json") {
		// Pretty-print JSON if it isn't already
		// (RunToString already returns trimmed output, attempt re-indent)
		trimmed := strings.TrimSpace(content)
		if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
			// Use the json-pretty converter if possible
			if out, err := prettyJSON(trimmed); err == nil {
				finalContent = out
			}
		}
	}

	if err := os.WriteFile(path, []byte(finalContent), 0o644); err != nil {
		return "", fmt.Errorf("write failed: %w", err)
	}
	return path, nil
}

// prettyJSON re-formats a JSON string with indentation.
func prettyJSON(s string) (string, error) {
	// Import encoding/json at the call site to avoid circular imports.
	// We do a simple check and delegate to the formatter in cmd.
	import_json := func(s string) (string, error) {
		var buf strings.Builder
		indent := 0
		inString := false
		escape := false
		for i := 0; i < len(s); i++ {
			c := s[i]
			if escape {
				buf.WriteByte(c)
				escape = false
				continue
			}
			if c == '\\' && inString {
				buf.WriteByte(c)
				escape = true
				continue
			}
			if c == '"' {
				inString = !inString
				buf.WriteByte(c)
				continue
			}
			if inString {
				buf.WriteByte(c)
				continue
			}
			switch c {
			case '{', '[':
				buf.WriteByte(c)
				indent++
				buf.WriteByte('\n')
				buf.WriteString(strings.Repeat("  ", indent))
			case '}', ']':
				indent--
				buf.WriteByte('\n')
				buf.WriteString(strings.Repeat("  ", indent))
				buf.WriteByte(c)
			case ',':
				buf.WriteByte(c)
				buf.WriteByte('\n')
				buf.WriteString(strings.Repeat("  ", indent))
			case ':':
				buf.WriteByte(c)
				buf.WriteByte(' ')
			case ' ', '\t', '\n', '\r':
				// skip existing whitespace
			default:
				buf.WriteByte(c)
			}
		}
		return buf.String(), nil
	}
	return import_json(s)
}

// renderOutput renders the right pane. Returns the rendered string.
func renderOutput(vp viewport.Model, entry *pickerEntry, errMsg string,
	statusMsg string, savePopup savePopupState, width, height int, focused bool) string {

	var sb strings.Builder

	// Title bar
	label := ""
	if entry != nil {
		label = "  ·  " + outputLabelStyle.Render(entry.Label)
	}
	title := outputTitleStyle.Render("OUTPUT") + label
	sb.WriteString(title + "\n")

	// Viewport content area
	vpHeight := height - 3 // title + status bar + divider
	if vpHeight < 1 {
		vpHeight = 1
	}

	if errMsg != "" {
		// Show error in the content area
		lines := strings.Split(errMsg, "\n")
		for i, l := range lines {
			if i >= vpHeight {
				break
			}
			sb.WriteString(errorStyle.Width(width - 2).Render(l) + "\n")
		}
		for i := len(lines); i < vpHeight; i++ {
			sb.WriteString(strings.Repeat(" ", width-2) + "\n")
		}
	} else {
		vp.SetWidth(width - 2)
		vp.SetHeight(vpHeight)
		sb.WriteString(vp.View() + "\n")
	}

	// Divider + status bar
	divider := lipgloss.NewStyle().Foreground(dimColor).Render(strings.Repeat("─", width-2))
	sb.WriteString(divider + "\n")

	if statusMsg != "" {
		sb.WriteString(lipgloss.NewStyle().Width(width - 2).Render(statusMsg))
	} else if entry != nil && errMsg == "" {
		pct := fmt.Sprintf("%3.f%%", vp.ScrollPercent()*100)
		sb.WriteString(lipgloss.NewStyle().Foreground(dimColor).Width(width-2).Render(pct))
	} else {
		sb.WriteString(strings.Repeat(" ", width-2))
	}

	return sb.String()
}

// renderSavePopup renders the save format selection popup as a centered overlay.
func renderSavePopup(popup savePopupState, entry *pickerEntry, termWidth, termHeight int) string {
	ext := outputExtFor(entry)

	var raw, pretty string
	if popup.format == saveFormatRaw {
		raw = activeFormatStyle.Render("Raw")
		pretty = inactiveFormatStyle.Render("Pretty")
	} else {
		raw = inactiveFormatStyle.Render("Raw")
		pretty = activeFormatStyle.Render("Pretty")
	}

	// Only show Pretty option for JSON/YAML outputs
	showPretty := ext == ".json" || ext == ".yaml"

	var content strings.Builder
	content.WriteString(popupTitleStyle.Render("Save output") + "\n\n")
	if showPretty {
		content.WriteString(raw + "  " + pretty + "\n\n")
	} else {
		content.WriteString(raw + "\n\n")
	}
	if popup.previewPath != "" {
		content.WriteString(popupPathStyle.Render("→ "+popup.previewPath) + "\n\n")
	}
	content.WriteString(lipgloss.NewStyle().Foreground(dimColor).Render("enter: save  esc: cancel"))

	box := popupStyle.Render(content.String())
	boxW := lipgloss.Width(box)
	boxH := lipgloss.Height(box)

	row := (termHeight - boxH) / 2
	col := (termWidth - boxW) / 2
	if row < 0 {
		row = 0
	}
	if col < 0 {
		col = 0
	}

	return lipgloss.Place(termWidth, termHeight, lipgloss.Left, lipgloss.Top,
		lipgloss.NewStyle().MarginLeft(col).MarginTop(row).Render(box))
}
