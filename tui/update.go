package tui

import (
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/wingitman/atob/internal/config"
	appupdate "github.com/wingitman/atob/internal/update"
)

type updateViewMode int

const (
	updateViewNone updateViewMode = iota
	updateViewPrompt
	updateViewHistory
)

type updateCheckMsg struct{ info appupdate.Info }

type updateLaunchMsg struct{ err string }

func checkUpdatesCmd(cfg config.Config, currentCommit string) tea.Cmd {
	return func() tea.Msg {
		return updateCheckMsg{info: appupdate.Check(cfg, currentCommit, 24)}
	}
}

func (m model) launchUpdate(latest bool, target string) tea.Cmd {
	req := appupdate.InstallRequest{
		RepoPath:       m.updateInfo.RepoPath,
		TargetCommit:   target,
		Latest:         latest,
		Terminal:       m.cfg.Updates.Terminal,
		RecorderBinary: currentExecutable(),
	}
	return func() tea.Msg {
		if err := appupdate.LaunchDetached(req); err != nil {
			return updateLaunchMsg{err: err.Error()}
		}
		return updateLaunchMsg{}
	}
}

func currentExecutable() string {
	path, err := os.Executable()
	if err != nil {
		return ""
	}
	return path
}

func (m *model) clampUpdateCursor() {
	commits := m.updateCommits()
	if len(commits) == 0 {
		m.updateCursor = 0
		return
	}
	if m.updateCursor < 0 {
		m.updateCursor = 0
	}
	if m.updateCursor >= len(commits) {
		m.updateCursor = len(commits) - 1
	}
}

func (m model) updateCommits() []appupdate.Commit {
	if len(m.updateInfo.Available) > 0 {
		return m.updateInfo.Available
	}
	return m.updateInfo.History
}

func (m model) selectedUpdateCommit() *appupdate.Commit {
	commits := m.updateCommits()
	if len(commits) == 0 || m.updateCursor < 0 || m.updateCursor >= len(commits) {
		return nil
	}
	return &commits[m.updateCursor]
}

func (m *model) toggleSelectedUpdateDetails() {
	c := m.selectedUpdateCommit()
	if c == nil {
		return
	}
	m.updateExpanded[c.Hash] = !m.updateExpanded[c.Hash]
}

func (m model) renderUpdateView() tea.View {
	var content string
	if m.updateMode == updateViewPrompt {
		content = m.renderUpdatePrompt()
	} else {
		content = m.renderUpdatesScreen()
	}
	footer := footerStyle.Width(m.width).Render(m.updateHelpLine())
	full := lipgloss.JoinVertical(lipgloss.Left, content, footer)
	v := tea.NewView(full)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (m model) renderUpdatePrompt() string {
	commits := m.updateInfo.Available
	rows := len(commits)
	if rows > 5 {
		rows = 5
	}
	start := m.updateCursor - rows/2
	if start < 0 {
		start = 0
	}
	if start+rows > len(commits) {
		start = len(commits) - rows
		if start < 0 {
			start = 0
		}
	}
	var b strings.Builder
	b.WriteString(statusOkStyle.Render("Update available"))
	b.WriteString("\n\n")
	b.WriteString("Current: " + shortCommit(m.updateInfo.CurrentCommit) + "\n")
	b.WriteString("Latest:  " + shortCommit(m.updateInfo.LatestCommit) + "\n")
	if m.updateInfo.Branch != "" {
		b.WriteString("Branch:  " + m.updateInfo.Branch + " -> " + m.updateInfo.Upstream + "\n")
	}
	b.WriteString("\nRecent changes:\n")
	for i := start; i < start+rows && i < len(commits); i++ {
		c := commits[i]
		prefix := "  "
		if i == m.updateCursor {
			prefix = "> "
		}
		line := fmt.Sprintf("%s%s %s", prefix, c.Short, c.Subject)
		if i == m.updateCursor {
			line = selectedItemStyle.Render(line)
		}
		b.WriteString(line + "\n")
		if m.updateExpanded[c.Hash] && c.Body != "" {
			b.WriteString(itemDescStyle.Render(indentLines(c.Body, "    ")) + "\n")
		}
	}
	b.WriteString("\n")
	b.WriteString(itemDescStyle.Render("y install in new terminal and exit · enter show/hide details · n/esc skip"))
	return centerBox(m.width, m.height-1, b.String())
}

func (m model) renderUpdatesScreen() string {
	var b strings.Builder
	b.WriteString(popupTitleStyle.Render("Updates"))
	b.WriteString("\n\n")
	if m.statusText != "" {
		if m.statusIsErr {
			b.WriteString(statusErrStyle.Render(m.statusText) + "\n\n")
		} else {
			b.WriteString(statusOkStyle.Render(m.statusText) + "\n\n")
		}
	}
	if m.updateChecking {
		b.WriteString(itemDescStyle.Render("Checking for updates..."))
		return centerBox(m.width, m.height-1, b.String())
	}
	if m.updateInfo.CheckError != "" {
		b.WriteString(statusErrStyle.Render("Check failed: ") + m.updateInfo.CheckError)
		return centerBox(m.width, m.height-1, b.String())
	}
	if m.updateInfo.RepoPath == "" {
		b.WriteString(itemDescStyle.Render("No update information loaded."))
		return centerBox(m.width, m.height-1, b.String())
	}
	b.WriteString(itemDescStyle.Render("Repo: ") + m.updateInfo.RepoPath + "\n")
	b.WriteString(itemDescStyle.Render("Branch: ") + m.updateInfo.Branch + " -> " + m.updateInfo.Upstream + "\n")
	b.WriteString(itemDescStyle.Render("Current: ") + shortCommit(m.updateInfo.CurrentCommit) + "\n")
	b.WriteString(itemDescStyle.Render("Latest: ") + shortCommit(m.updateInfo.LatestCommit) + "\n\n")

	commits := m.updateCommits()
	if len(commits) == 0 {
		b.WriteString(statusOkStyle.Render("No newer commits found."))
		return centerBox(m.width, m.height-1, b.String())
	}
	if len(m.updateInfo.Available) > 0 {
		b.WriteString(statusOkStyle.Render(fmt.Sprintf("%d update(s) available", len(m.updateInfo.Available))) + "\n")
	} else {
		b.WriteString(itemDescStyle.Render("Recent history") + "\n")
	}

	rows := m.height - 12
	if rows < 4 {
		rows = 4
	}
	if rows > len(commits) {
		rows = len(commits)
	}
	start := m.updateCursor - rows/2
	if start < 0 {
		start = 0
	}
	if start+rows > len(commits) {
		start = len(commits) - rows
		if start < 0 {
			start = 0
		}
	}
	for i := start; i < start+rows && i < len(commits); i++ {
		c := commits[i]
		line := fmt.Sprintf("  %s  %s  %s", c.Short, c.Date, c.Subject)
		if i == m.updateCursor {
			line = selectedItemStyle.Render(line)
		}
		b.WriteString(line + "\n")
		if m.updateExpanded[c.Hash] && c.Body != "" {
			b.WriteString(itemDescStyle.Render(indentLines(c.Body, "    ")) + "\n")
		}
	}
	return centerBox(m.width, m.height-1, b.String())
}

func (m model) updateHelpLine() string {
	if m.updateMode == updateViewPrompt {
		return truncate("y: install latest  enter: details  n/esc: skip  up/down: navigate", m.width-2)
	}
	return truncate("up/down: navigate  enter: details  i: install selected  y: install latest  ctrl+r: refresh  esc: back", m.width-2)
}

func centerBox(width, height int, body string) string {
	boxW := width - 8
	if boxW < 40 {
		boxW = width - 2
	}
	if boxW < 20 {
		boxW = 20
	}
	box := popupStyle.Width(boxW).Render(body)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}

func shortCommit(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	if s == "" {
		return "unknown"
	}
	return s
}

func indentLines(s, prefix string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}
