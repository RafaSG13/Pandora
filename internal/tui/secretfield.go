package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/lipgloss"

	"github.com/RafaSG13/Pandora/internal/store"
)

// formBaseHeight is the terminal height at which the form, with a one-row
// secret box, fills the window exactly (the 60×18 floor). Every extra terminal
// row lets the secret box grow by one, up to maxSecretRows.
const (
	formBaseHeight = 18
	maxSecretRows  = 8
)

// newSecretArea is the multi-line, unlimited-width secret editor. Its own
// MaxHeight and MaxWidth (99 rows, 500 columns) would silently cut long keys
// and tokens, so both are lifted; store.MaxValueBytes is the real limit.
func newSecretArea() textarea.Model {
	ta := textarea.New()
	ta.Prompt = ""
	ta.ShowLineNumbers = false
	ta.Placeholder = secretPlaceholder
	ta.CharLimit = store.MaxValueBytes
	ta.MaxHeight, ta.MaxWidth = 0, 0
	for _, s := range []*textarea.Style{&ta.FocusedStyle, &ta.BlurredStyle} {
		s.CursorLine = lipgloss.NewStyle()
		s.Placeholder = subtleStyle
	}
	ta.FocusedStyle.Text = textStyle
	ta.BlurredStyle.Text = subtleStyle
	return ta
}

// secretRows is the height of the secret box in the form.
func (m model) secretRows() int {
	return min(maxSecretRows, max(1, m.height-formBaseHeight+1))
}

// secretLines renders the secret box as exactly secretRows lines: the editor
// while the text is shown (or empty), otherwise one row of bullets per line of
// the value, so the text is never on screen unless asked for with ctrl+r.
func (m model) secretLines() []string {
	rows := m.secretRows()
	var lines []string
	if m.secretShown || m.secret.Value() == "" {
		lines = strings.Split(m.secret.View(), "\n")
	} else {
		lines = maskedLines(m.secret.Value(), m.secret.Width(), rows)
	}
	for len(lines) < rows {
		lines = append(lines, "")
	}
	return lines[:rows]
}

// maskedLines draws value as bullets, rows lines of at most w columns. A value
// with several lines in a single row shows its line count instead; one that
// does not fit ends with "+N more lines".
func maskedLines(value string, w, rows int) []string {
	lines := strings.Split(value, "\n")
	bullets := func(s string, limit int) string {
		n := min(utf8.RuneCountInString(s), max(0, limit))
		return subtleStyle.Render(strings.Repeat(g.mask, n))
	}
	out := make([]string, rows)
	switch {
	case len(lines) == 1:
		out[0] = bullets(lines[0], w)
	case rows == 1:
		tag := fmt.Sprintf(" (%d lines)", len(lines))
		out[0] = bullets(lines[0], w-len(tag)) + subtleStyle.Render(tag)
	default:
		for i := range rows {
			switch {
			case i == rows-1 && len(lines) > rows:
				out[i] = subtleStyle.Render(fmt.Sprintf("%s +%d more lines", g.ellipsis, len(lines)-i))
			case i < len(lines):
				out[i] = bullets(lines[i], w)
			}
		}
	}
	return out
}

// hideReveal drops the revealed value from the model.
func (m *model) hideReveal() {
	m.revealTitle, m.revealValue, m.revealErr = "", "", nil
}

// revealSelected shows title's value for revealTTL. The value is read once.
func (m *model) revealSelected(title string) {
	m.revealTitle, m.revealUntil = title, m.nowOr().Add(revealTTL)
	m.revealValue, m.revealErr = store.GetSecretValue(title)
}

// revealBlock wraps a revealed value to width columns and shows at most rows
// lines; when more remain the last line says how many (c still copies it all).
func revealBlock(value string, width, rows int) string {
	lines := strings.Split(textStyle.Width(max(1, width)).Render(value), "\n")
	if len(lines) > rows {
		keep := lines[:rows-1]
		note := fmt.Sprintf("%s +%d lines", g.ellipsis, len(lines)-len(keep))
		if width >= 34 {
			note = fmt.Sprintf("%s +%d more lines %s c copies all", g.ellipsis, len(lines)-len(keep), g.sep)
		}
		lines = append(keep, subtleStyle.Render(truncate(note, width)))
	}
	return strings.Join(lines, "\n")
}
