// Package tui implements the atob interactive TUI using Bubble Tea v2.
package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/atotto/clipboard"

	"github.com/wingitman/atob/internal/config"
	"github.com/wingitman/atob/internal/convert"
	appupdate "github.com/wingitman/atob/internal/update"
	"github.com/wingitman/atob/internal/version"
)

// focusPane identifies which pane is active.
type focusPane int

const (
	focusList focusPane = iota
	focusInput
	focusOutput
)

// Preload carries optional pre-loaded content for the TUI startup.
type Preload struct {
	FilePath string // if set, load this file into the input pane
	Text     string // if set, pre-populate input with this text
}

var PreloadNone = Preload{}

func PreloadFile(p string) Preload { return Preload{FilePath: p} }
func PreloadText(t string) Preload { return Preload{Text: t} }

// ── tea messages ─────────────────────────────────────────────────────────────

type conversionResultMsg struct {
	output string
	err    error
}

type debounceMsg struct{ id int }

type statusMsg struct {
	text  string
	isErr bool
}

type clearStatusMsg struct{}

type configReloadedMsg struct {
	cfg config.Config
}

// ── model ─────────────────────────────────────────────────────────────────────

type model struct {
	cfg    config.Config
	keys   resolvedKeys
	width  int
	height int

	focus  focusPane
	list   listState
	input  textarea.Model
	output viewport.Model

	selected          *pickerEntry
	outputContent     string
	outputErr         string
	preloadedFilePath string // non-empty when a binary file was pre-loaded via CLI arg

	debounceID int

	statusText  string
	statusIsErr bool

	savePopup savePopupState

	version string

	updateMode     updateViewMode
	updateInfo     appupdate.Info
	updateChecking bool
	updateCursor   int
	updateExpanded map[string]bool

	themeMode   bool
	themeCursor int
	themeNames  []string
}

// buildVersion is set via SetVersion(), called from cmd.SetVersion().
var buildVersion = "dev"

// SetVersion injects the version string stamped at build time.
func SetVersion(v string) { buildVersion = v }

// Start launches the TUI. Returns when the user quits.
func Start(cfg config.Config, preload Preload) error {
	m := newModel(cfg, preload)
	p := tea.NewProgram(m)
	_, err := p.Run()
	return err
}

func newModel(cfg config.Config, preload Preload) model {
	applyTheme(cfg)
	items := buildPickerList()
	ls := newListState(items)

	ta := newTextarea()
	var preloadedFilePath string

	if preload.FilePath != "" {
		absPath, err := filepath.Abs(preload.FilePath)
		if err != nil {
			absPath = preload.FilePath
		}
		data, err := os.ReadFile(absPath)
		if err == nil {
			if looksLikeBinary(data) {
				// Binary file: put the path in the textarea so binary converters
				// can read it. Pre-filter the list to ONLY the Binary category items
				// (exact category match, not substring search so "text → binary" etc.
				// don't leak in).
				ta.SetValue(absPath)
				preloadedFilePath = absPath
				ls.searchQuery = "binary"
				ls.applyFilter()
			} else {
				// Text file: put the contents in the textarea.
				ta.SetValue(string(data))
			}
		}
	} else if preload.Text != "" {
		ta.SetValue(preload.Text)
	}

	vp := newViewport(80, 20)

	themeNames, _ := config.ThemeNames(cfg)

	m := model{
		cfg:               cfg,
		keys:              resolveKeys(cfg.Keybinds),
		focus:             focusList,
		list:              ls,
		input:             ta,
		output:            vp,
		preloadedFilePath: preloadedFilePath,
		version:           buildVersion,
		updateExpanded:    map[string]bool{},
		themeNames:        themeNames,
	}

	// Auto-select the first converter so navigation and live preview work
	// from the moment the TUI opens, even before the user presses Enter.
	if e := m.list.selected(); e != nil {
		entry := *e
		entry.Category = categoryFor(entry)
		m.selected = &entry
		m.input.Placeholder = placeholderFor(m.selected)
	}

	return m
}

// looksLikeBinary returns true if data contains a null byte in the first 512
// bytes — the same heuristic used by git and the Unix 'file' command.
func looksLikeBinary(data []byte) bool {
	n := len(data)
	if n > 512 {
		n = 512
	}
	for _, b := range data[:n] {
		if b == 0 {
			return true
		}
	}
	return false
}

// ── Init ──────────────────────────────────────────────────────────────────────

func applyTheme(cfg config.Config) {
	theme := config.ResolveTheme(cfg)
	ConfigureTheme(theme.Colors, theme.Terminal)
}

func (m model) applySelectedTheme() (tea.Model, tea.Cmd) {
	if m.themeCursor < 0 || m.themeCursor >= len(m.themeNames) {
		m.themeMode = false
		return m, nil
	}
	name := m.themeNames[m.themeCursor]
	if err := config.SetThemeName(name); err != nil {
		m.statusText = "Could not save theme: " + err.Error()
		m.statusIsErr = true
		m.themeMode = false
		return m, nil
	}
	m.cfg.Themes.ThemeName = name
	applyTheme(m.cfg)
	m.themeMode = false
	m.statusText = "theme: " + name
	m.statusIsErr = false
	return m, tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		return clearStatusMsg{}
	})
}

func (m *model) clampThemeCursor() {
	if len(m.themeNames) == 0 {
		m.themeCursor = 0
		return
	}
	if m.themeCursor < 0 {
		m.themeCursor = 0
	}
	if m.themeCursor >= len(m.themeNames) {
		m.themeCursor = len(m.themeNames) - 1
	}
}

func (m model) updateTheme(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc", m.keys.quit, m.keys.quitAlt:
		m.themeMode = false
		return m, nil
	case m.keys.up:
		m.themeCursor--
		m.clampThemeCursor()
		return m, nil
	case m.keys.down:
		m.themeCursor++
		m.clampThemeCursor()
		return m, nil
	case m.keys.sel:
		return m.applySelectedTheme()
	}
	return m, nil
}

func (m model) Init() tea.Cmd {
	if !m.cfg.Updates.DisableChecks {
		return checkUpdatesCmd(m.cfg, version.Commit)
	}
	return nil
}

// ── Update ────────────────────────────────────────────────────────────────────

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizePanes()

	case conversionResultMsg:
		if msg.err != nil {
			m.outputErr = msg.err.Error()
			m.outputContent = ""
		} else {
			m.outputErr = ""
			m.outputContent = msg.output
			m.output.SetContent(msg.output)
			m.output.GotoTop()
		}

	case debounceMsg:
		// Only fire if the ID matches the current debounce generation.
		// debounceID is incremented directly on the local m in Update (never
		// inside a value-receiver method), so this check is always accurate.
		if msg.id == m.debounceID {
			cmds = append(cmds, m.runConversionCmd())
		}

	case statusMsg:
		m.statusText = msg.text
		m.statusIsErr = msg.isErr
		cmds = append(cmds, tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
			return clearStatusMsg{}
		}))

	case clearStatusMsg:
		m.statusText = ""
		m.statusIsErr = false

	case configReloadedMsg:
		m.cfg = msg.cfg
		m.keys = resolveKeys(msg.cfg.Keybinds)
		applyTheme(msg.cfg)
		m.themeNames, _ = config.ThemeNames(msg.cfg)
		m.statusText = "Config reloaded."
		m.statusIsErr = false
		cmds = append(cmds, tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
			return clearStatusMsg{}
		}))

	case updateCheckMsg:
		m.updateChecking = false
		m.updateInfo = msg.info
		if msg.info.CheckError == "" && len(msg.info.Available) > 0 {
			m.updateCursor = 0
			m.updateMode = updateViewPrompt
		}

	case updateLaunchMsg:
		if msg.err != "" {
			m.statusText = "Update failed: " + msg.err
			m.statusIsErr = true
			m.updateMode = updateViewHistory
			cmds = append(cmds, tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
				return clearStatusMsg{}
			}))
			break
		}
		return m, tea.Quit

	case tea.MouseWheelMsg:
		cmds = append(cmds, m.handleMouseWheel(msg))

	case tea.MouseClickMsg:
		cmds = append(cmds, m.handleMouseClick(msg))

	case tea.KeyPressMsg:
		key := msg.String()

		if m.themeMode {
			return m.updateTheme(key)
		}

		if m.updateMode == updateViewPrompt {
			switch key {
			case "y", "Y":
				return m, m.launchUpdate(true, "")
			case "enter":
				m.toggleSelectedUpdateDetails()
				return m, nil
			case "esc", "n", "N":
				m.updateMode = updateViewNone
				return m, nil
			}
			if matchKey(key, m.keys.up) {
				m.updateCursor--
				m.clampUpdateCursor()
				return m, nil
			}
			if matchKey(key, m.keys.down) {
				m.updateCursor++
				m.clampUpdateCursor()
				return m, nil
			}
			return m, nil
		}

		if m.updateMode == updateViewHistory {
			switch {
			case key == "esc" || matchKey(key, m.keys.quit) || matchKey(key, m.keys.quitAlt):
				m.updateMode = updateViewNone
				return m, nil
			case matchKey(key, m.keys.up):
				m.updateCursor--
				m.clampUpdateCursor()
				return m, nil
			case matchKey(key, m.keys.down):
				m.updateCursor++
				m.clampUpdateCursor()
				return m, nil
			case key == "enter":
				m.toggleSelectedUpdateDetails()
				return m, nil
			case matchKey(key, m.keys.run):
				m.updateChecking = true
				return m, checkUpdatesCmd(m.cfg, version.Commit)
			case key == "i" || key == "I":
				if c := m.selectedUpdateCommit(); c != nil {
					return m, m.launchUpdate(false, c.Hash)
				}
				return m, nil
			case key == "y" || key == "Y":
				return m, m.launchUpdate(true, "")
			}
			return m, nil
		}

		// ── Save popup ──────────────────────────────────────────────────────
		if m.savePopup.open {
			switch key {
			case "left":
				m.savePopup.format = saveFormatRaw
				m.savePopup.previewPath = m.previewSavePath(saveFormatRaw)
			case "right":
				m.savePopup.format = saveFormatPretty
				m.savePopup.previewPath = m.previewSavePath(saveFormatPretty)
			case "enter":
				path, err := saveOutput(m.outputContent, m.selected,
					m.savePopup.format, m.cfg.Output.SaveDir)
				m.savePopup.open = false
				if err != nil {
					e := err.Error()
					cmds = append(cmds, tea.Cmd(func() tea.Msg {
						return statusMsg{text: "Save failed: " + e, isErr: true}
					}))
				} else {
					saved := path
					cmds = append(cmds, tea.Cmd(func() tea.Msg {
						return statusMsg{text: "Saved → " + saved}
					}))
				}
			case "esc":
				m.savePopup.open = false
			}
			return m, tea.Batch(cmds...)
		}

		// ── Global: quit ────────────────────────────────────────────────────
		if matchKey(key, m.keys.quit) {
			return m, tea.Quit
		}
		if matchKey(key, m.keys.quitAlt) && m.focus != focusInput {
			return m, tea.Quit
		}

		// ── Global: open config (not in input pane) ──────────────────────────
		if matchKey(key, m.keys.openConfig) && m.focus != focusInput && m.list.searching != true {
			return m, openConfigCmd()
		}

		if matchKey(key, m.keys.showUpdates) && m.focus != focusInput && !m.list.searching {
			m.updateMode = updateViewHistory
			m.updateCursor = 0
			m.updateChecking = true
			return m, checkUpdatesCmd(m.cfg, version.Commit)
		}

		if matchKey(key, m.keys.theme) && m.focus != focusInput && !m.list.searching {
			m.themeNames, _ = config.ThemeNames(m.cfg)
			m.themeCursor = 0
			for i, name := range m.themeNames {
				if name == m.cfg.Themes.ThemeName {
					m.themeCursor = i
					break
				}
			}
			m.themeMode = true
			return m, nil
		}

		// ── Global: cycle panes ─────────────────────────────────────────────
		if matchKey(key, m.keys.nextPane) {
			m.input.Blur()
			m.focus = (m.focus + 1) % 3
			if m.focus == focusInput {
				cmds = append(cmds, m.input.Focus())
			}
			return m, tea.Batch(cmds...)
		}
		if matchKey(key, m.keys.prevPane) {
			m.input.Blur()
			m.focus = (m.focus + 2) % 3
			if m.focus == focusInput {
				cmds = append(cmds, m.input.Focus())
			}
			return m, tea.Batch(cmds...)
		}

		// ── List pane ───────────────────────────────────────────────────────
		if m.focus == focusList {
			listH := m.listPaneHeight()

			if m.list.searching {
				switch key {
				case "esc":
					m.list.searching = false
					m.list.searchQuery = ""
					m.list.applyFilter()

				case "enter":
					// Select the highlighted item and move to input pane.
					// IMPORTANT: return immediately so the Enter key is NOT
					// also delivered to the input pane (which would insert a newline).
					m.list.searching = false
					m.applySelection(&m, &cmds)
					return m, tea.Batch(cmds...)

				case "backspace":
					if len(m.list.searchQuery) > 0 {
						m.list.searchQuery = m.list.searchQuery[:len(m.list.searchQuery)-1]
						m.list.applyFilter()
					}

				default:
					if len(key) == 1 {
						m.list.searchQuery += key
						m.list.applyFilter()
					}
				}

			} else {
				switch {
				case matchKey(key, m.keys.search):
					m.list.searching = true
					m.list.searchQuery = ""

				case matchKey(key, m.keys.sel) || key == "enter":
					// Select and switch to input — return immediately to prevent
					// the Enter key leaking into the input pane handler below.
					m.applySelection(&m, &cmds)
					return m, tea.Batch(cmds...)

				case matchKey(key, m.keys.up):
					m.list.moveUp(listH)
					updateHoverSelected(&m, &cmds)
				case matchKey(key, m.keys.down):
					m.list.moveDown(listH)
					updateHoverSelected(&m, &cmds)
				case matchKey(key, m.keys.pageUp):
					m.list.pageUp(listH)
					updateHoverSelected(&m, &cmds)
				case matchKey(key, m.keys.pageDown):
					m.list.pageDown(listH)
					updateHoverSelected(&m, &cmds)
				case matchKey(key, m.keys.top):
					m.list.jumpTop()
					updateHoverSelected(&m, &cmds)
				case matchKey(key, m.keys.bottom):
					m.list.jumpBottom(listH)
					updateHoverSelected(&m, &cmds)
				}
			}
			return m, tea.Batch(cmds...)
		}

		// ── Input pane ──────────────────────────────────────────────────────
		if m.focus == focusInput {
			switch {
			case matchKey(key, m.keys.clearInput):
				// Clear the input textarea and reset all preload state.
				m.input.Reset()
				m.clearBinaryPreload()
				// Clear the output too — stale results from the old input
				// would be confusing after a clear.
				m.outputContent = ""
				m.outputErr = ""
				m.output.SetContent("")

			case matchKey(key, m.keys.run):
				cmds = append(cmds, m.runConversionCmd())

			case key == "ctrl+v":
				text, err := clipboard.ReadAll()
				if err == nil && text != "" {
					m.input.InsertString(text)
					m.clearBinaryPreload()
					m.debounceID++
					id := m.debounceID
					delay := time.Duration(m.cfg.TUI.DebounceMs) * time.Millisecond
					cmds = append(cmds, tea.Tick(delay, func(t time.Time) tea.Msg {
						return debounceMsg{id: id}
					}))
				}

			default:
				var taCmd tea.Cmd
				m.input, taCmd = m.input.Update(msg)
				cmds = append(cmds, taCmd)
				m.clearBinaryPreload()
				if m.cfg.TUI.LivePreview {
					m.debounceID++
					id := m.debounceID
					delay := time.Duration(m.cfg.TUI.DebounceMs) * time.Millisecond
					cmds = append(cmds, tea.Tick(delay, func(t time.Time) tea.Msg {
						return debounceMsg{id: id}
					}))
				}
			}
		}

		// ── Output pane ─────────────────────────────────────────────────────
		if m.focus == focusOutput {
			switch {
			case matchKey(key, m.keys.copyOutput):
				if m.outputContent != "" {
					if err := clipboard.WriteAll(m.outputContent); err == nil {
						n := len(strings.Split(m.outputContent, "\n"))
						cmds = append(cmds, tea.Cmd(func() tea.Msg {
							return statusMsg{text: fmt.Sprintf("Copied %d lines to clipboard", n)}
						}))
					} else {
						e := err.Error()
						cmds = append(cmds, tea.Cmd(func() tea.Msg {
							return statusMsg{text: "Clipboard error: " + e, isErr: true}
						}))
					}
				}
			case matchKey(key, m.keys.saveOutput):
				if m.outputContent != "" {
					m.savePopup.open = true
					m.savePopup.format = saveFormatRaw
					m.savePopup.previewPath = m.previewSavePath(saveFormatRaw)
				}
			case matchKey(key, m.keys.up):
				m.output.ScrollUp(1)
			case matchKey(key, m.keys.down):
				m.output.ScrollDown(1)
			case matchKey(key, m.keys.pageUp):
				m.output.ScrollUp(m.output.VisibleLineCount() / 2)
			case matchKey(key, m.keys.pageDown):
				m.output.ScrollDown(m.output.VisibleLineCount() / 2)
			case matchKey(key, m.keys.halfUp):
				m.output.ScrollUp(m.output.VisibleLineCount() / 2)
			case matchKey(key, m.keys.halfDown):
				m.output.ScrollDown(m.output.VisibleLineCount() / 2)
			case matchKey(key, m.keys.top):
				m.output.GotoTop()
			case matchKey(key, m.keys.bottom):
				m.output.GotoBottom()
			default:
				var vpCmd tea.Cmd
				m.output, vpCmd = m.output.Update(msg)
				cmds = append(cmds, vpCmd)
			}
		}
	}

	return m, tea.Batch(cmds...)
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m model) renderThemeScreen() string {
	var b strings.Builder
	b.WriteString(headerStyle.Render("Themes"))
	b.WriteString("\n")
	b.WriteString(footerStyle.Render("Choose a theme and press Enter. Esc cancels."))
	b.WriteString("\n\n")
	for i, name := range m.themeNames {
		line := "  " + name
		if name == m.cfg.Themes.ThemeName {
			line += footerStyle.Render("  (current)")
		}
		if i == m.themeCursor {
			line = Selector.Render("▶ ") + line[2:]
			if lipgloss.Width(line) < m.width {
				line += strings.Repeat(" ", m.width-lipgloss.Width(line))
			}
			line = activeFormatStyle.Render(line)
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

func (m model) View() tea.View {
	if m.width == 0 {
		v := tea.NewView("Loading…")
		v.MouseMode = tea.MouseModeCellMotion
		return v
	}
	if m.themeMode {
		v := tea.NewView(m.renderThemeScreen())
		v.AltScreen = true
		v.MouseMode = tea.MouseModeCellMotion
		return v
	}
	if m.updateMode != updateViewNone {
		return m.renderUpdateView()
	}

	if m.savePopup.open {
		v := tea.NewView(renderSavePopup(m.savePopup, m.selected, m.width, m.height))
		v.AltScreen = true
		v.MouseMode = tea.MouseModeCellMotion
		return v
	}

	leftW, midW, rightW := m.paneWidths()
	// Inner height: total minus header(1) + footer(1) + border top+bottom(2) = 4
	innerH := m.height - 4
	if innerH < 4 {
		innerH = 4
	}

	// ── Header ──────────────────────────────────────────────────────────────
	delby := BrandDelby.Render("delby")
	soft := BrandSoft.Render("soft")
	app := lipgloss.NewStyle().Foreground(dimColor).Render(" / atob")
	brand := " " + delby + soft + app + " "

	var verStr string
	if m.version != "" && m.version != "dev" {
		verStr = versionStyle.Render(" v" + strings.TrimPrefix(m.version, "v"))
	}

	rightHints := footerStyle.Render(m.keys.openConfig + ": config  " + m.keys.quit + ": quit")
	brandW := lipgloss.Width(brand) + lipgloss.Width(verStr)
	rightW2 := lipgloss.Width(rightHints)
	gap := m.width - brandW - rightW2
	if gap < 1 {
		gap = 1
	}
	header := brand + verStr + strings.Repeat(" ", gap) + rightHints

	// ── Pane content ────────────────────────────────────────────────────────
	listContent := renderList(m.list, leftW-2, innerH)
	inputContent := renderInput(m.input, m.selected, midW-2, innerH, m.focus == focusInput)

	statusLine := m.statusText
	if m.statusText != "" {
		if m.statusIsErr {
			statusLine = statusErrStyle.Render(m.statusText)
		} else {
			statusLine = statusOkStyle.Render(m.statusText)
		}
	}
	outputContent := renderOutput(m.output, m.selected, m.outputErr,
		statusLine, m.savePopup, rightW-2, innerH, m.focus == focusOutput)

	// ── Borders ─────────────────────────────────────────────────────────────
	// MaxHeight(innerH) clips the fully-bordered box (content + 2 border rows)
	// to exactly innerH lines, preventing content overflow from pushing the
	// footer off-screen. Without this, tall content (many list items + category
	// headers) makes the bordered row exceed its allocated height.
	leftBorder := m.borderStyle(focusList).Width(leftW - 2).Height(innerH).MaxHeight(innerH)
	midBorder := m.borderStyle(focusInput).Width(midW - 2).Height(innerH).MaxHeight(innerH)
	rightBorder := m.borderStyle(focusOutput).Width(rightW - 2).Height(innerH).MaxHeight(innerH)

	row := lipgloss.JoinHorizontal(lipgloss.Top,
		leftBorder.Render(listContent),
		midBorder.Render(inputContent),
		rightBorder.Render(outputContent),
	)

	// ── Footer (contextual hints) ────────────────────────────────────────────
	// footerStyle has Padding(0,1) so visible content area is m.width-2.
	footerContentW := m.width - 2
	if footerContentW < 1 {
		footerContentW = 1
	}
	footer := footerStyle.Width(m.width).Render(helpLine(
		m.keys,
		m.focus,
		m.savePopup.open,
		m.selected,
		m.outputContent != "" || m.outputErr != "",
		m.list.searching,
		footerContentW,
	))

	full := lipgloss.JoinVertical(lipgloss.Left, header, row, footer)

	v := tea.NewView(full)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (m *model) resizePanes() {
	_, midW, rightW := m.paneWidths()
	innerH := m.height - 4
	if innerH < 4 {
		innerH = 4
	}
	m.input.SetWidth(midW - 4)
	m.input.SetHeight(innerH - 3)
	m.output.SetWidth(rightW - 4)
	m.output.SetHeight(innerH - 3)
}

func (m model) paneWidths() (left, mid, right int) {
	total := m.width
	left = total * 25 / 100
	mid = total * 35 / 100
	right = total - left - mid
	if left < 18 {
		left = 18
	}
	if mid < 20 {
		mid = 20
	}
	if right < 20 {
		right = 20
	}
	return
}

// listPaneHeight returns the number of item rows available in the list pane.
// Matches the `listH` value computed inside renderList (height - 3 reserved lines).
func (m model) listPaneHeight() int {
	innerH := m.height - 4
	if innerH < 4 {
		innerH = 4
	}
	listH := innerH - 3 // title(1) + divider(1) + search(1)
	if listH < 1 {
		listH = 1
	}
	return listH
}

func (m *model) handleMouseWheel(msg tea.MouseWheelMsg) tea.Cmd {
	if m.savePopup.open {
		return nil
	}
	e := msg.Mouse()
	m.focusPaneAt(e.X)
	if m.focus == focusList {
		listH := m.listPaneHeight()
		if e.Button == tea.MouseWheelUp {
			m.list.moveUp(listH)
		} else if e.Button == tea.MouseWheelDown {
			m.list.moveDown(listH)
		}
		var cmds []tea.Cmd
		updateHoverSelected(m, &cmds)
		return tea.Batch(cmds...)
	}
	if m.focus == focusOutput {
		if e.Button == tea.MouseWheelUp {
			m.output.ScrollUp(1)
		} else if e.Button == tea.MouseWheelDown {
			m.output.ScrollDown(1)
		}
	}
	return nil
}

func (m *model) handleMouseClick(msg tea.MouseClickMsg) tea.Cmd {
	if m.savePopup.open || msg.Button != tea.MouseLeft {
		return nil
	}
	e := msg.Mouse()
	m.focusPaneAt(e.X)
	if m.focus != focusList {
		if m.focus != focusInput {
			m.input.Blur()
		}
		if m.focus == focusInput {
			return m.input.Focus()
		}
		return nil
	}
	m.input.Blur()
	listH := m.listPaneHeight()
	row := m.list.rowAtDisplayLine(e.Y-3, listH)
	if row < 0 {
		return nil
	}
	if row == m.list.cursor {
		var cmds []tea.Cmd
		m.applySelection(m, &cmds)
		return tea.Batch(cmds...)
	}
	m.list.cursor = row
	m.list.scrollIntoView(listH)
	var cmds []tea.Cmd
	updateHoverSelected(m, &cmds)
	return tea.Batch(cmds...)
}

func (m *model) focusPaneAt(x int) {
	left, mid, _ := m.paneWidths()
	m.input.Blur()
	switch {
	case x < left:
		m.focus = focusList
	case x < left+mid:
		m.focus = focusInput
	default:
		m.focus = focusOutput
	}
}

func (m model) borderStyle(pane focusPane) lipgloss.Style {
	if m.focus == pane {
		return activeBorderStyle
	}
	return inactiveBorderStyle
}

// clearBinaryPreload clears the preloaded file path state and, if the list
// was filtered to Binary converters for that preload, restores all converters.
// Call this whenever the user manually edits the input textarea.
func (m *model) clearBinaryPreload() {
	if m.preloadedFilePath == "" {
		return // nothing to clear
	}
	m.preloadedFilePath = ""
	// The list was filtered to "binary" for this preload. Restore all items.
	m.list.searchQuery = ""
	m.list.applyFilter()
	// Update hover selection to the new first item after restoring the list.
	if e := m.list.selected(); e != nil {
		entry := *e
		entry.Category = categoryFor(entry)
		m.selected = &entry
		m.input.Placeholder = placeholderFor(m.selected)
	}
}

// updateHoverSelected updates m.selected to whichever item the list cursor
// currently points at, WITHOUT switching pane focus. If the input has content
// and live preview is on, it also schedules a debounce tick so the output pane
// reflects the newly highlighted converter.
//
// Call this after every list navigation key (up, down, pageUp, pageDown, g, G).
func updateHoverSelected(m *model, cmds *[]tea.Cmd) {
	e := m.list.selected()
	if e == nil {
		return
	}
	entry := *e
	entry.Category = categoryFor(entry)
	m.selected = &entry
	m.input.Placeholder = placeholderFor(m.selected)

	// Trigger live preview if the input already has content.
	if m.input.Value() != "" && m.cfg.TUI.LivePreview {
		m.debounceID++
		id := m.debounceID
		delay := time.Duration(m.cfg.TUI.DebounceMs) * time.Millisecond
		*cmds = append(*cmds, tea.Tick(delay, func(t time.Time) tea.Msg {
			return debounceMsg{id: id}
		}))
	}
}

// applySelection sets m.selected, switches focus to the input pane, and
// — if there is already content in the input — schedules a debounce tick so
// the live preview fires without waiting for a keypress.
//
// It modifies m and appends to cmds in-place; the caller must return
// (m, tea.Batch(*cmds)) immediately after calling this to prevent the
// triggering key (usually Enter) from leaking into the input pane handler.
func (m *model) applySelection(mm *model, cmds *[]tea.Cmd) {
	e := mm.list.selected()
	if e == nil {
		return
	}
	entry := *e
	entry.Category = categoryFor(entry)
	mm.selected = &entry
	mm.input.Placeholder = placeholderFor(mm.selected)

	// Switch focus to input pane and focus the textarea.
	mm.input.Blur()
	mm.focus = focusInput
	*cmds = append(*cmds, mm.input.Focus())

	// If there is already text in the input, schedule a live-preview tick.
	if mm.input.Value() != "" && mm.cfg.TUI.LivePreview {
		mm.debounceID++
		id := mm.debounceID
		delay := time.Duration(mm.cfg.TUI.DebounceMs) * time.Millisecond
		*cmds = append(*cmds, tea.Tick(delay, func(t time.Time) tea.Msg {
			return debounceMsg{id: id}
		}))
	}
}

func (m model) runConversionCmd() tea.Cmd {
	if m.selected == nil {
		return nil
	}
	entry := *m.selected
	inputText := strings.TrimSpace(m.input.Value())
	if inputText == "" {
		return nil
	}

	preloadedPath := m.preloadedFilePath

	return func() tea.Msg {
		var output string
		var err error

		if isBinaryTarget(entry.To) {
			// Binary converters: treat input as a file path
			expanded := expandPath(inputText)
			data, readErr := os.ReadFile(expanded)
			if readErr != nil {
				return conversionResultMsg{err: fmt.Errorf("cannot read file %q: %w", expanded, readErr)}
			}
			output, err = convert.RunBinaryToString(data, entry.To)
		} else if preloadedPath != "" {
			// A binary file was pre-loaded and a text converter was selected.
			return conversionResultMsg{err: fmt.Errorf(
				"binary file loaded — select inspect, hexdump, strings, or decompile\n\n" +
					"press esc in the list to clear the filter and see all converters",
			)}
		} else {
			from := entry.From
			if from == "any" {
				from = "text"
			}
			output, err = convert.RunToString(inputText, from, entry.To, nil)
		}

		return conversionResultMsg{output: output, err: err}
	}
}

func (m model) previewSavePath(format saveFormat) string {
	saveDir := m.cfg.Output.SaveDirResolved()
	ext := outputExtFor(m.selected)
	name := saveFileName(m.selected, format, ext)
	full := filepath.Join(saveDir, name)
	if home, err := os.UserHomeDir(); err == nil {
		if strings.HasPrefix(full, home) {
			full = "~" + full[len(home):]
		}
	}
	return full
}

// ── Config editor ─────────────────────────────────────────────────────────────

// openConfigCmd suspends the TUI, opens atob.toml in $EDITOR, then resumes.
// When the editor closes, config is reloaded so keybind changes apply live.
func openConfigCmd() tea.Cmd {
	path := config.ConfigPath()
	editor := resolveEditor()
	return tea.ExecProcess(exec.Command(editor, path), func(err error) tea.Msg {
		cfg, _ := config.Load()
		return configReloadedMsg{cfg: cfg}
	})
}

// resolveEditor returns the user's preferred editor.
func resolveEditor() string {
	if e := os.Getenv("EDITOR"); e != "" {
		return e
	}
	if e := os.Getenv("VISUAL"); e != "" {
		return e
	}
	if runtime.GOOS == "windows" {
		return "notepad"
	}
	return "nano"
}

// ── Converter list data ───────────────────────────────────────────────────────

func buildPickerList() []pickerEntry {
	raw := []struct{ from, to, label, desc string }{
		// Formats
		{"json", "yaml", "json → yaml", "Convert JSON to YAML"},
		{"json", "toml", "json → toml", "Convert JSON to TOML"},
		{"json", "xml", "json → xml", "Convert JSON to XML"},
		{"json", "csv", "json → csv", "Convert JSON array to CSV"},
		{"json", "json", "json → json", "Pretty-print JSON"},
		{"yaml", "json", "yaml → json", "Convert YAML to JSON"},
		{"toml", "json", "toml → json", "Convert TOML to JSON"},
		{"xml", "json", "xml → json", "Convert XML to JSON"},
		{"csv", "json", "csv → json", "Convert CSV to JSON"},
		{"csv", "xlsx", "csv → xlsx", "Convert CSV file to XLSX"},
		{"xlsx", "csv", "xlsx → csv", "Convert XLSX file to CSV"},
		// Encoding
		{"text", "base64", "text → base64", "Encode text to Base64"},
		{"base64", "text", "base64 → text", "Decode Base64 to text"},
		{"text", "hex", "text → hex", "Hex-encode text"},
		{"hex", "text", "hex → text", "Hex-decode to text"},
		{"text", "url", "text → url", "URL-encode text"},
		{"url", "text", "url → text", "URL-decode text"},
		{"text", "html", "text → html", "HTML-encode special characters"},
		{"html", "text", "html → text", "HTML-decode entities"},
		{"text", "morsecode", "text → morse", "Morse encode entities"},
		{"morsecode", "text", "morse → text", "Morse encode entities"},
		// Hashing
		{"text", "md5", "text → md5", "Hash text with MD5"},
		{"text", "sha1", "text → sha1", "Hash text with SHA-1"},
		{"text", "sha256", "text → sha256", "Hash text with SHA-256"},
		{"text", "sha512", "text → sha512", "Hash text with SHA-512"},
		// Compression
		{"text", "gzip", "text → gzip", "Gzip-compress (base64 output)"},
		{"gzip", "text", "gzip → text", "Gzip-decompress (base64 input)"},
		{"text", "zlib", "text → zlib", "Zlib-compress (base64 output)"},
		{"zlib", "text", "zlib → text", "Zlib-decompress (base64 input)"},
		// Numbers
		{"text", "binary", "text → binary", "Convert decimal to binary"},
		{"binary", "text", "binary → text", "Convert binary to decimal"},
		{"text", "octal", "text → octal", "Convert decimal to octal"},
		{"octal", "text", "octal → text", "Convert octal to decimal"},
		{"decimal", "hex", "decimal → hex", "Convert decimal to hex"},
		{"hex", "decimal", "hex → decimal", "Convert hex to decimal"},
		// Identity
		{"epoch", "text", "epoch → text", "Unix epoch → human datetime"},
		{"text", "epoch", "text → epoch", "Datetime string → Unix epoch"},
		{"text", "uuid", "text → uuid", "Generate a new UUID v4"},
		// Binary
		{"file", "inspect", "file → inspect", "Auto-detect binary format, return JSON metadata"},
		{"file", "hexdump", "file → hexdump", "Hex dump with offsets and ASCII panel"},
		{"file", "strings", "file → strings", "Extract printable strings"},
		{"file", "decompile", "file → decompile", "Decompile / unpack AWS Lambda or generic ZIP archive"},
		// Case
		{"any", "camel", "any → camel", "Convert text to camelCase"},
		{"any", "pascal", "any → pascal", "Convert text to PascalCase"},
		{"any", "snake", "any → snake", "Convert text to snake_case"},
		{"any", "kebab", "any → kebab", "Convert text to kebab-case"},
		{"any", "screaming-snake", "any → screaming-snake", "Convert text to SCREAMING_SNAKE_CASE"},
		{"any", "screaming-kebab", "any → screaming-kebab", "Convert text to SCREAMING-KEBAB-CASE"},
	}

	items := make([]pickerEntry, len(raw))
	for i, r := range raw {
		fb := r.to == "xlsx" || r.from == "xlsx"
		items[i] = pickerEntry{
			From:        r.from,
			To:          r.to,
			Label:       r.label,
			Description: r.desc,
			FileBased:   fb,
		}
		items[i].Category = categoryFor(items[i])
	}
	return items
}
