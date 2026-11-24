package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// The viewer (modeView) shows the whole revealed value of one secret, wrapped
// and scrollable, because the detail card keeps only its first lines. The
// value lives in model.revealValue like any reveal; hideReveal (expiry, lock,
// delete, selection change) drops it and tick then closes the viewer.

// openViewer reveals title's value (reading it once) and opens the viewer.
func (m *model) openViewer(title string) {
	if !m.revealed(title) || m.revealErr != nil {
		m.revealSelected(title)
	}
	if m.revealErr != nil {
		m.notify(fmt.Sprintf("✗ Could not read secret: %v", m.revealErr), true)
		m.hideReveal()
		return
	}
	m.mode, m.viewTop = modeView, 0
	m.revealUntil = m.nowOr().Add(m.cfg.RevealTTL)
}

// viewerWidth is the viewer's outer width: wider than the other dialogs so
// keys and tokens wrap less.
func (m model) viewerWidth() int { return m.width - 4 }

// viewerRows is how many value lines the viewer shows: its border, padding,
// title, status line and the blank lines around them take the other 10 rows
// (placeModal adds a blank line and the key hints).
func (m model) viewerRows() int { return max(1, m.height-10) }

// viewerLines is the value wrapped to the viewer's text width, the same way
// the detail card wraps it.
func (m model) viewerLines() []string {
	return strings.Split(textStyle.Width(max(1, m.viewerWidth()-8)).Render(m.revealValue), "\n")
}

// updateViewer handles the keys of the viewer. Every key renews the reveal
// deadline, so reading keeps the value open; it closes reveal_seconds after
// the last key.
func (m model) updateViewer(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.revealUntil = m.nowOr().Add(m.cfg.RevealTTL)
	key := msg.String()
	if key == "ctrl+c" {
		return m, tea.Quit
	}
	if key == "esc" || key == "q" || m.keys.action(key) == "view" {
		m.mode, m.viewTop = modeBrowse, 0
		return m, nil
	}

	rows := m.viewerRows()
	last := max(0, len(m.viewerLines())-rows)
	switch key {
	case "up", "k":
		m.viewTop--
	case "down", "j":
		m.viewTop++
	case "pgup":
		m.viewTop -= rows
	case "pgdown":
		m.viewTop += rows
	case "home", "g":
		m.viewTop = 0
	case "end", "G":
		m.viewTop = last
	case "c":
		m.copySelected(m.revealTitle)
	}
	m.viewTop = max(0, min(m.viewTop, last))
	return m, nil
}

// renderViewer is the viewer dialog: the title, viewerRows() value lines from
// viewTop, and a status line with the position and the time left. Its height
// is viewerRows()+4 plus the modal's border and padding.
func (m model) renderViewer() string {
	inner := m.viewerWidth() - 8
	rows := m.viewerRows()
	all := m.viewerLines()
	top := max(0, min(m.viewTop, len(all)-rows))

	lines := []string{
		lipgloss.NewStyle().Bold(true).Foreground(accent).Render(truncate(m.revealTitle, inner)),
		"",
	}
	shown := all[top:min(len(all), top+rows)]
	lines = append(lines, shown...)
	for i := len(shown); i < rows; i++ {
		lines = append(lines, "")
	}
	left := max(0, int(m.revealUntil.Sub(m.nowOr()).Round(time.Second)/time.Second))
	status := fmt.Sprintf("%d–%d of %d lines %s hides in %ds", top+1, top+len(shown), len(all), g.sep, left)
	lines = append(lines, "", subtleStyle.Render(truncate(status, inner)))
	return modalStyle.Width(m.viewerWidth()).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}
