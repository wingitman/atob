package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// pickerEntry mirrors the picker data for the TUI.
type pickerEntry struct {
	From        string
	To          string
	Label       string
	Description string
	FileBased   bool
	Category    string
}

// categoryFor returns the display category name for a converter entry.
func categoryFor(e pickerEntry) string {
	switch {
	case e.From == "file" || e.To == "inspect" || e.To == "hexdump" || e.To == "strings" || e.To == "decompile":
		return "Binary"
	case e.From == "any" || e.To == "camel" || e.To == "pascal" ||
		e.To == "snake" || e.To == "kebab" ||
		e.To == "screaming-snake" || e.To == "screaming-kebab":
		return "Case"
	case e.From == "json" || e.To == "json" || e.From == "yaml" ||
		e.To == "yaml" || e.From == "toml" || e.To == "toml" ||
		e.From == "xml" || e.To == "xml" || e.From == "csv" ||
		e.To == "csv" || e.From == "xlsx" || e.To == "xlsx":
		return "Formats"
	case e.To == "base64" || e.From == "base64" || e.To == "hex" ||
		e.From == "hex" || e.To == "url" || e.From == "url" ||
		e.To == "html" || e.From == "html":
		return "Encoding"
	case e.To == "md5" || e.To == "sha1" || e.To == "sha256" || e.To == "sha512":
		return "Hashing"
	case e.To == "gzip" || e.From == "gzip" || e.To == "zlib" || e.From == "zlib":
		return "Compression"
	case e.To == "binary" || e.From == "binary" || e.To == "octal" ||
		e.From == "octal" || e.To == "decimal" || e.From == "decimal":
		return "Numbers"
	case e.To == "uuid" || e.To == "epoch" || e.From == "epoch":
		return "Identity"
	default:
		return "Other"
	}
}

// ── Flat rows ─────────────────────────────────────────────────────────────────
//
// To achieve pixel-perfect scroll synchronisation between navigation and
// rendering we represent the list as a flat slice of rows where each row is
// either a category header or a converter item.  cursor and offset index into
// this flat slice.  Navigation skips over header rows automatically.

type listRow struct {
	isHeader bool
	category string // non-empty when isHeader == true
	itemIdx  int    // index into listState.filtered when isHeader == false
}

// buildFlatRows converts the filtered item slice into a flat display row slice,
// inserting one header row whenever the category changes.
func buildFlatRows(filtered []pickerEntry) []listRow {
	var rows []listRow
	prev := ""
	for i, e := range filtered {
		if e.Category != prev {
			rows = append(rows, listRow{isHeader: true, category: e.Category})
			prev = e.Category
		}
		rows = append(rows, listRow{itemIdx: i})
	}
	return rows
}

// ── List state ────────────────────────────────────────────────────────────────

type listState struct {
	allItems    []pickerEntry
	filtered    []pickerEntry
	rows        []listRow // flat display rows for current filtered set
	cursor      int       // row index into rows (always points at a non-header row)
	offset      int       // first visible row index
	searching   bool
	searchQuery string
}

func newListState(items []pickerEntry) listState {
	rows := buildFlatRows(items)
	// Start cursor on the first non-header row.
	start := firstItemRow(rows, 0)
	return listState{
		allItems: items,
		filtered: items,
		rows:     rows,
		cursor:   start,
	}
}

// firstItemRow returns the index of the first non-header row at or after start.
func firstItemRow(rows []listRow, start int) int {
	for i := start; i < len(rows); i++ {
		if !rows[i].isHeader {
			return i
		}
	}
	return start
}

// lastItemRow returns the index of the last non-header row at or before end.
func lastItemRow(rows []listRow, end int) int {
	for i := end; i >= 0; i-- {
		if !rows[i].isHeader {
			return i
		}
	}
	return end
}

// selected returns the currently highlighted pickerEntry, or nil.
func (l *listState) selected() *pickerEntry {
	if len(l.rows) == 0 || l.cursor < 0 || l.cursor >= len(l.rows) {
		return nil
	}
	row := l.rows[l.cursor]
	if row.isHeader {
		return nil
	}
	if row.itemIdx < 0 || row.itemIdx >= len(l.filtered) {
		return nil
	}
	e := l.filtered[row.itemIdx]
	return &e
}

// ── Navigation ────────────────────────────────────────────────────────────────
//
// listH is the number of *display lines* available for the scrollable area
// (after subtracting title, divider, and search bar).  Because the selected
// item takes 2 lines (label + description), we must ensure both fit.

// linesBefore counts how many display lines rows[0..rowIdx-1] occupy when
// cursor is at cursorRow (to account for the 2-line selected item).
func linesBefore(rows []listRow, fromRow, toRow, cursorRow int) int {
	count := 0
	for i := fromRow; i < toRow && i < len(rows); i++ {
		if rows[i].isHeader {
			count++
		} else if i == cursorRow {
			count += 2 // label + description for selected item
		} else {
			count++
		}
	}
	return count
}

// scrollIntoView adjusts l.offset so that the cursor row is fully visible
// within listH display lines.
func (l *listState) scrollIntoView(listH int) {
	// Scroll up if cursor is above the viewport.
	if l.cursor < l.offset {
		l.offset = l.cursor
		// Back up further if a header row sits just above.
		for l.offset > 0 && l.rows[l.offset-1].isHeader {
			l.offset--
		}
		return
	}

	// Scroll down until the cursor (+ its description line) fits.
	for {
		used := linesBefore(l.rows, l.offset, l.cursor+1, l.cursor)
		// Add 1 for the description line of the selected item.
		if used <= listH {
			break
		}
		l.offset++
	}
}

func (l *listState) moveUp(listH int) {
	// Find the previous non-header row.
	for i := l.cursor - 1; i >= 0; i-- {
		if !l.rows[i].isHeader {
			l.cursor = i
			l.scrollIntoView(listH)
			return
		}
	}
}

func (l *listState) moveDown(listH int) {
	// Find the next non-header row.
	for i := l.cursor + 1; i < len(l.rows); i++ {
		if !l.rows[i].isHeader {
			l.cursor = i
			l.scrollIntoView(listH)
			return
		}
	}
}

func (l *listState) pageUp(listH int) {
	// Move cursor up by approximately listH item rows.
	moved := 0
	for i := l.cursor - 1; i >= 0 && moved < listH; i-- {
		if !l.rows[i].isHeader {
			l.cursor = i
			moved++
		}
	}
	l.scrollIntoView(listH)
}

func (l *listState) pageDown(listH int) {
	moved := 0
	for i := l.cursor + 1; i < len(l.rows) && moved < listH; i++ {
		if !l.rows[i].isHeader {
			l.cursor = i
			moved++
		}
	}
	l.scrollIntoView(listH)
}

func (l *listState) jumpTop() {
	l.cursor = firstItemRow(l.rows, 0)
	l.offset = 0
}

func (l *listState) jumpBottom(listH int) {
	l.cursor = lastItemRow(l.rows, len(l.rows)-1)
	l.scrollIntoView(listH)
}

func (l *listState) rowAtDisplayLine(line, listH int) int {
	if line < 0 || line >= listH {
		return -1
	}
	used := 0
	for ri := l.offset; ri < len(l.rows) && used < listH; ri++ {
		row := l.rows[ri]
		if row.isHeader {
			if used == line {
				return -1
			}
			used++
			continue
		}
		if used == line {
			return ri
		}
		used++
		if ri == l.cursor {
			if used == line {
				return ri
			}
			used++
		}
	}
	return -1
}

// knownCategories is the set of category names used by the converter list.
// When a search query exactly matches one of these (case-insensitive), the
// filter restricts to that category only — so typing "binary" shows only the
// Binary category without matching converter labels like "text → binary".
var knownCategories = map[string]bool{
	"formats":     true,
	"encoding":    true,
	"hashing":     true,
	"compression": true,
	"numbers":     true,
	"case":        true,
	"identity":    true,
	"binary":      true,
	"other":       true,
}

// applyFilter rebuilds the filtered slice and flat rows, resetting the cursor.
// If the search query exactly matches a known category name (case-insensitive),
// only items in that category are shown — preventing label substring collisions
// like "text → binary" appearing in a "binary" search.
// For all other queries, the normal substring search across label, description,
// and category applies.
func (l *listState) applyFilter() {
	if l.searchQuery == "" {
		l.filtered = l.allItems
	} else {
		q := strings.ToLower(l.searchQuery)
		var out []pickerEntry
		if knownCategories[q] {
			// Exact category filter — match only by category name.
			for _, e := range l.allItems {
				if strings.ToLower(e.Category) == q {
					out = append(out, e)
				}
			}
		} else {
			// General substring search across label, description, and category.
			for _, e := range l.allItems {
				if strings.Contains(strings.ToLower(e.Label), q) ||
					strings.Contains(strings.ToLower(e.Description), q) ||
					strings.Contains(strings.ToLower(e.Category), q) {
					out = append(out, e)
				}
			}
		}
		l.filtered = out
	}
	l.rows = buildFlatRows(l.filtered)
	l.cursor = firstItemRow(l.rows, 0)
	l.offset = 0
}

// ── Rendering ─────────────────────────────────────────────────────────────────

// renderList renders the left pane. height is the total inner pane height
// (borders already stripped by caller). Reserves 3 lines at the bottom:
// title(1) + divider(1) + search(1).
func renderList(l listState, width, height int) string {
	const reserved = 3
	listH := height - reserved
	if listH < 1 {
		listH = 1
	}

	var sb strings.Builder

	// Title
	sb.WriteString(paneTitleStyle.Render("CONVERTERS") + "\n")

	// Items — walk flat rows from offset, stop when listH lines consumed.
	lineCount := 0
	for ri := l.offset; ri < len(l.rows) && lineCount < listH; ri++ {
		row := l.rows[ri]

		if row.isHeader {
			sb.WriteString(categoryStyle.Width(width).Render(row.category) + "\n")
			lineCount++
			continue
		}

		isCursor := ri == l.cursor
		e := l.filtered[row.itemIdx]

		labelText := e.Label
		if e.FileBased {
			labelText += " [file]"
		}
		if isCursor {
			sb.WriteString(selectedItemStyle.Width(width).Render("▶ "+labelText) + "\n")
		} else {
			sb.WriteString(normalItemStyle.Width(width).Render("  "+labelText) + "\n")
		}
		lineCount++

		// Description only for the selected item, only if there is still room.
		if isCursor && lineCount < listH {
			sb.WriteString(itemDescStyle.Width(width).Render("  "+e.Description) + "\n")
			lineCount++
		}
	}

	// Pad remaining lines so the pane height is stable.
	for lineCount < listH {
		sb.WriteString(strings.Repeat(" ", width) + "\n")
		lineCount++
	}

	// Divider + search bar
	sb.WriteString(lipgloss.NewStyle().Foreground(dimColor).Render(strings.Repeat("─", width)) + "\n")

	if l.searching {
		// Active search mode: show query with blinking cursor block.
		q := l.searchQuery + "█"
		bar := searchPromptStyle.Render("/") + " " + q
		for lipgloss.Width(bar) > width && len(q) > 1 {
			q = q[1:]
			bar = searchPromptStyle.Render("/") + " " + q
		}
		sb.WriteString(lipgloss.NewStyle().Width(width).Render(bar))
	} else if l.searchQuery != "" {
		// A filter is active but the user is not typing — show the query
		// so they know the list is filtered. Esc or clearing input removes it.
		bar := searchPromptStyle.Render("/") + " " + l.searchQuery +
			itemDescStyle.Render("  esc to clear")
		sb.WriteString(lipgloss.NewStyle().Width(width).Render(bar))
	} else {
		sb.WriteString(itemDescStyle.Width(width).Render("/ search  ↑↓ navigate"))
	}

	return sb.String()
}
