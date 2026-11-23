package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/RafaSG13/Pandora/internal/store"
)

// historyState is the previous-values screen (modeHistory) for one secret.
// The revealed value is read once and dropped by hideValue, like the detail
// card's reveal; lock() clears the whole state.
type historyState struct {
	title  string
	infos  []store.VersionInfo // newest first: infos[0] is "back 1"
	cursor int

	value string
	shown bool
	err   error
	until time.Time
}

// hideValue drops the revealed previous value.
func (h *historyState) hideValue() { h.value, h.shown, h.err = "", false, nil }

// historyRows is how many versions the screen lists at once. The dialog is
// the rows plus 11 lines of chrome (see renderHistory), which must fit the
// terminal so the view stays exactly as high as the window.
func (m model) historyRows() int { return min(8, max(1, m.height-11)) }

// openHistory shows the previous values of title, or says there are none.
func (m *model) openHistory(title string) {
	infos, err := store.Versions(title)
	switch {
	case err != nil:
		m.notify(fmt.Sprintf("✗ Could not read the history: %v", err), true)
	case len(infos) == 0:
		m.notify("No previous values", false)
	default:
		m.hideReveal()
		m.hist = historyState{title: title, infos: infos}
		m.mode = modeHistory
	}
}

// closeHistory returns to the list and forgets everything the screen held.
func (m *model) closeHistory() {
	m.hist = historyState{}
	m.mode = modeBrowse
}

// updateHistory handles the keys of the previous-values screen: ↑/↓ choose a
// version, r reveals it, enter makes it current, esc closes.
func (m model) updateHistory(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	h := &m.hist
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc", "q", "H":
		m.closeHistory()
	case "up", "k":
		if h.cursor > 0 {
			h.cursor--
			h.hideValue()
		}
	case "down", "j":
		if h.cursor < len(h.infos)-1 {
			h.cursor++
			h.hideValue()
		}
	case "r":
		if h.shown {
			h.hideValue()
			break
		}
		h.value, h.err = store.GetVersion(h.title, h.cursor+1)
		h.shown, h.until = true, m.nowOr().Add(m.cfg.RevealTTL)
		if h.err == nil {
			m.audit("reveal", h.title)
		}
	case "enter":
		title, back, replaced := h.title, h.cursor+1, h.infos[h.cursor].Replaced
		m.closeHistory()
		if err := store.RestoreVersion(title, back); err != nil {
			m.notify(fmt.Sprintf("✗ Could not restore: %v", err), true)
			return m, nil
		}
		m.audit("restore", title)
		if err := m.reload(title); err != nil {
			m.notify(fmt.Sprintf("✗ Restored, but could not reload: %v", err), true)
			return m, nil
		}
		m.notify(fmt.Sprintf("✓ Restored the value replaced on %s", replaced), false)
		return m, m.startOrphanCheck()
	}
	return m, nil
}

// renderHistory is the previous-values dialog: the title, one row per
// version, the chosen version's value (hidden until r), and a reminder that
// old values stay in the keyring. Its height is historyRows()+7 plus the
// border and padding, which placeModal completes with the key hints.
func (m model) renderHistory() string {
	h := m.hist
	inner := m.modalWidth() - 8
	rows := m.historyRows()
	top := max(0, min(h.cursor-rows/2, len(h.infos)-rows))

	lines := []string{
		lipgloss.NewStyle().Bold(true).Foreground(accent).Render(truncate(fmt.Sprintf("History of %q", h.title), inner)),
		"",
	}
	for i := top; i < min(len(h.infos), top+rows); i++ {
		text := fmt.Sprintf("%3d  replaced %s", -(i + 1), h.infos[i].Replaced)
		if i == h.cursor {
			lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(accent).Render(g.marker+" "+truncate(text, inner-2)))
		} else {
			lines = append(lines, textStyle.Render("  "+truncate(text, inner-2)))
		}
	}

	value := subtleStyle.Render("Value hidden")
	switch {
	case h.shown && h.err != nil:
		value = lipgloss.NewStyle().Foreground(danger).Render(truncate(fmt.Sprintf("✗ %v", h.err), inner))
	case h.shown:
		first, _, multi := strings.Cut(h.value, "\n")
		if multi {
			first += g.ellipsis
		}
		value = textStyle.Render(truncate(first, inner))
	}
	lines = append(lines, "", value, subtleStyle.Render(truncate("Previous values stay encrypted in the keyring.", inner)))
	return modalStyle.Width(m.modalWidth()).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

// deleteNote is the line under the delete confirmation: what is removed from the keyring.
func deleteNote(previous int) string {
	if previous == 0 {
		return "Its value is removed from the OS keyring. This cannot be undone."
	}
	return fmt.Sprintf("Its value and its %d previous values are removed from the OS keyring. This cannot be undone.", previous)
}
