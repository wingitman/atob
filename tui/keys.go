package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/wingitman/atob/internal/config"
)

// resolvedKeys holds all key bindings resolved from config at startup,
// or after a live config reload. All key checks in Update use matchKey().
type resolvedKeys struct {
	up, down            string
	pageUp, pageDown    string
	halfUp, halfDown    string
	top, bottom         string
	nextPane, prevPane  string
	search, clearSearch string
	sel                 string
	run                 string
	copyOutput          string
	saveOutput          string
	openConfig          string
	showUpdates         string
	clearInput          string
	quit, quitAlt       string
	theme               string
}

// resolveKeys copies every binding from the config struct into resolvedKeys.
func resolveKeys(k config.Keybinds) resolvedKeys {
	return resolvedKeys{
		up:          k.Up,
		down:        k.Down,
		pageUp:      k.PageUp,
		pageDown:    k.PageDown,
		halfUp:      k.HalfUp,
		halfDown:    k.HalfDown,
		top:         k.Top,
		bottom:      k.Bottom,
		nextPane:    k.NextPane,
		prevPane:    k.PrevPane,
		search:      k.Search,
		clearSearch: k.ClearSearch,
		sel:         k.Select,
		run:         k.Run,
		copyOutput:  k.CopyOutput,
		saveOutput:  k.SaveOutput,
		openConfig:  k.OpenConfig,
		showUpdates: k.ShowUpdates,
		clearInput:  k.ClearInput,
		quit:        k.Quit,
		quitAlt:     k.QuitAlt,
		theme:       k.Theme,
	}
}

// matchKey returns true if the pressed key string matches the binding.
func matchKey(pressed, binding string) bool {
	return binding != "" && pressed == binding
}

// sep is the visual separator between hint segments.
const sep = "  "

// hint builds a single "key: action" segment.
func hint(key, action string) string { return key + ": " + action }

// helpLine returns a compact contextual footer string for the current state.
// All key names come from m.keys so changes to the config are reflected live.
// The result is truncated to maxWidth visible characters to prevent wrapping.
func helpLine(keys resolvedKeys, focus focusPane, saveOpen bool,
	selected *pickerEntry, hasOutput bool, searching bool, maxWidth int) string {

	var parts []string

	if saveOpen {
		parts = []string{"←/→ format", "enter save", "esc cancel"}
		return truncate(strings.Join(parts, sep), maxWidth)
	}

	// Global keys always shown at the right side.
	global := []string{
		hint(keys.nextPane, "next"),
		hint(keys.showUpdates, "updates"),
		hint(keys.quit, "quit"),
	}

	switch focus {
	case focusList:
		if searching {
			parts = []string{
				"type to filter",
				hint(keys.sel, "select"),
				hint(keys.clearSearch, "clear"),
			}
		} else {
			parts = []string{
				hint(keys.up+"/"+keys.down, "navigate"),
				hint(keys.search, "search"),
				hint(keys.sel, "select"),
				hint(keys.top+"/"+keys.bottom, "top/end"),
				hint(keys.openConfig, "config"),
			}
		}

	case focusInput:
		if selected == nil {
			parts = []string{"select a converter first"}
		} else {
			parts = []string{
				hint(keys.run, "run"),
				"ctrl+v paste",
				hint(keys.clearInput, "clear"),
			}
		}

	case focusOutput:
		if !hasOutput {
			parts = []string{"run a conversion to see output"}
		} else {
			parts = []string{
				hint(keys.up+"/"+keys.down, "scroll"),
				hint(keys.halfUp+"/"+keys.halfDown, "½pg"),
				hint(keys.copyOutput, "copy"),
				hint(keys.saveOutput, "save"),
			}
		}
	}

	all := append(parts, global...)
	return truncate(strings.Join(all, sep), maxWidth)
}

// truncate clips s to maxWidth visible characters, appending "…" if clipped.
// Uses lipgloss.Width so ANSI escape sequences are not counted.
func truncate(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return s
	}
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	// Clip rune by rune until it fits with the ellipsis.
	runes := []rune(s)
	for len(runes) > 0 {
		candidate := string(runes) + "…"
		if lipgloss.Width(candidate) <= maxWidth {
			return candidate
		}
		runes = runes[:len(runes)-1]
	}
	return "…"
}
