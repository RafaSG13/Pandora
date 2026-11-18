package tui

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// rowDelegate draws list rows. Roomy rows are three lines: a category-coloured
// dot and the title, the category and creation date (plus an expiry badge when
// one is due), and a thin separator. Compact rows are one line with the
// category on the right. The selected row gets a full-width background.
//
// Height and Spacing must stay in sync with roomyRows / compactRows (model.go).
type rowDelegate struct {
	compact  bool
	orphans  map[string]bool // titles whose keyring value is missing
	now      func() time.Time
	soonDays int // expiry_warn_days
}

func (d rowDelegate) Height() int {
	if d.compact {
		return compactRows
	}
	return roomyRows
}

func (rowDelegate) Spacing() int                            { return 0 }
func (rowDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

const (
	gutter = 2 // aligns rows under the list title
	lead   = 4 // gutter plus the dot and its space
)

func (d rowDelegate) Render(w io.Writer, m list.Model, index int, it list.Item) {
	i, ok := it.(item)
	width := m.Width()
	if !ok || width <= 0 {
		return
	}
	now := time.Now()
	if d.now != nil {
		now = d.now()
	}

	var (
		filtering = m.FilterState() == list.Filtering
		selected  = index == m.Index() && !filtering
		dimmed    = filtering && m.FilterValue() == ""
		matched   []int
	)
	if filtering || m.FilterState() == list.FilterApplied {
		matched = m.MatchesForItem(index)
	}

	base := lipgloss.NewStyle()
	if selected {
		base = base.Background(selectedBg)
	}

	rowTitle := base.Foreground(text)
	catColor := categoryColor(i.meta.Category)
	switch {
	case dimmed:
		rowTitle, catColor = base.Foreground(subtle), subtle
	case selected:
		rowTitle = base.Bold(true).Foreground(accent)
	}

	dot := base.Foreground(catColor).Render(g.dot + " ")
	if d.orphans[i.meta.Title] {
		dot = base.Bold(true).Foreground(danger).Render(g.orphan + " ")
	}
	indent := base.Render(strings.Repeat(" ", gutter))
	if selected && noColor {
		// Without colour the highlight is invisible, so mark the row instead.
		indent = g.marker + strings.Repeat(" ", gutter-lipgloss.Width(g.marker))
	}

	expLabel, level := expiryInfo(i.meta, now, d.soonDays)
	var expStyle lipgloss.Style
	switch {
	case dimmed || level < expirySoon:
		expLabel = ""
	case level == expiryExpired:
		expStyle = base.Foreground(danger)
	default:
		expStyle = base.Foreground(warning)
	}

	// The compact row shows only a glyph for an expiry; without colour an
	// expired secret gets a "!" so it differs from one expiring soon.
	expMark := g.clock
	if noColor && level == expiryExpired {
		expMark += "!"
	}
	var lines []string
	if d.compact {
		lines = []string{d.compactLine(i, indent, dot, rowTitle, base, catColor, expLabel, expStyle, expMark, matched, width)}
	} else {
		textW := width - lead
		title := lipgloss.StyleRunes(truncate(i.meta.Title, textW), matched, rowTitle.Underline(true), rowTitle)

		category := truncate(i.meta.Category, textW)
		rest := ""
		if left := textW - lipgloss.Width(category); left > 0 {
			sep := " " + g.sep + " "
			if category == "" {
				sep = ""
			}
			rest = truncate(sep+i.meta.Created, left)
		}
		line2 := base.Render(strings.Repeat(" ", lead)) + base.Foreground(catColor).Render(category) + base.Foreground(subtle).Render(rest)
		if expLabel != "" {
			badge := "  " + g.clock + " " + expLabel
			if lipgloss.Width(line2)+lipgloss.Width(badge) <= width {
				line2 += expStyle.Render(badge)
			}
		}
		lines = []string{indent + dot + title, line2}
	}

	if selected { // extend the background to the full row
		for k := range lines {
			lines[k] = padRow(lines[k], width, base)
		}
	}
	if !d.compact && index < len(m.VisibleItems())-1 {
		lines = append(lines, subtleStyle.Faint(true).Render(strings.Repeat(" ", gutter)+strings.Repeat(g.rule, max(0, width-2*gutter))))
	}
	fmt.Fprint(w, strings.Join(lines, "\n"))
}

// compactLine is "● title ........ CATEGORY ◷" on one line.
func (d rowDelegate) compactLine(i item, indent, dot string, rowTitle, base lipgloss.Style, catColor lipgloss.TerminalColor, expLabel string, expStyle lipgloss.Style, expMark string, matched []int, width int) string {
	right := ""
	rightW := 0
	if i.meta.Category != "" {
		c := truncate(i.meta.Category, max(1, width/3))
		right += base.Foreground(catColor).Render(c)
		rightW += lipgloss.Width(c)
	}
	if expLabel != "" {
		right += expStyle.Render(" " + expMark)
		rightW += 1 + lipgloss.Width(expMark)
	}
	if right != "" {
		right = base.Render(" ") + right
		rightW++
	}
	titleW := max(1, width-lead-rightW-1)
	title := lipgloss.StyleRunes(truncate(i.meta.Title, titleW), matched, rowTitle.Underline(true), rowTitle)
	pad := max(1, width-lead-lipgloss.Width(title)-rightW)
	return indent + dot + title + base.Render(strings.Repeat(" ", pad)) + right
}

func padRow(line string, width int, base lipgloss.Style) string {
	if pad := width - lipgloss.Width(line); pad > 0 {
		return line + base.Render(strings.Repeat(" ", pad))
	}
	return line
}

// truncate shortens s to at most w columns, ending in an ellipsis when it cuts.
func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	ell := g.ellipsis
	if lipgloss.Width(ell) >= w {
		ell = ""
	}
	r := []rune(s)
	for len(r) > 0 && lipgloss.Width(string(r))+lipgloss.Width(ell) > w {
		r = r[:len(r)-1]
	}
	return string(r) + ell
}
