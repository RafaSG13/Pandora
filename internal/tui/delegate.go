package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

// delegate draws each secret as two rows: name, then category · date. A bar in
// the category's colour marks every row; the selected row gets a full-width
// background.
type delegate struct{}

func (delegate) Height() int                         { return 2 }
func (delegate) Spacing() int                        { return 1 }
func (delegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (delegate) Render(w io.Writer, m list.Model, index int, li list.Item) {
	it, ok := li.(item)
	if !ok {
		return
	}

	width := m.Width()
	selected := index == m.Index()
	base := lipgloss.NewStyle()
	if selected {
		base = base.Background(selectedBg)
	}

	catColor := categoryColor(it.meta.Category)
	bar := base.Foreground(catColor).Render("▌ ")
	textW := max(1, width-lipgloss.Width(bar))

	nameStyle := base.Foreground(text)
	if selected {
		nameStyle = nameStyle.Bold(true).Foreground(accent)
	}
	name := runewidth.Truncate(it.meta.Title, textW, "…") // plain text: truncate before styling
	if m.FilterState() != list.Unfiltered {
		// Matches index into FilterValue, which starts with the title.
		name = lipgloss.StyleRunes(name, m.MatchesForItem(index), nameStyle.Underline(true), nameStyle)
	} else {
		name = nameStyle.Render(name)
	}

	category := strings.TrimSpace(it.meta.Category)
	if category == "" {
		category = "No category"
	}
	desc := base.Foreground(catColor).Render(category) + base.Foreground(subtle).Render(" · "+it.meta.Created)
	desc = lipgloss.NewStyle().MaxWidth(textW).Render(desc) // ANSI-aware truncation

	fmt.Fprintf(w, "%s\n%s", pad(bar+name, width, base), pad(bar+desc, width, base))
}

// pad extends s to width columns using style's background.
func pad(s string, width int, style lipgloss.Style) string {
	if gap := width - lipgloss.Width(s); gap > 0 {
		return s + style.Render(strings.Repeat(" ", gap))
	}
	return s
}
