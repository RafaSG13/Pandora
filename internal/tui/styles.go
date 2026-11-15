package tui

import (
	"fmt"
	"hash/fnv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	accent  = lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#A855F7"}
	subtle  = lipgloss.AdaptiveColor{Light: "#64748B", Dark: "#94A3B8"}
	success = lipgloss.AdaptiveColor{Light: "#059669", Dark: "#10B981"}
	danger  = lipgloss.AdaptiveColor{Light: "#DC2626", Dark: "#F87171"}
	warning = lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FBBF24"}
	text    = lipgloss.AdaptiveColor{Light: "#1E293B", Dark: "#F8FAFC"}
)

var (
	subtleStyle = lipgloss.NewStyle().Foreground(subtle)
	textStyle   = lipgloss.NewStyle().Foreground(text)
	labelStyle  = lipgloss.NewStyle().Bold(true).Foreground(subtle).Width(10)
	keyStyle    = lipgloss.NewStyle().Bold(true).Foreground(accent)
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(accent)

	// selectedBg is the full-row background of the selected list item.
	selectedBg = lipgloss.AdaptiveColor{Light: "#EDE9FE", Dark: "#2E1F4D"}

	// paneStyle is the base for both panes; callers set width, height and border colour.
	paneStyle = lipgloss.NewStyle().
			Border(g.border).
			Padding(1, 2)

	modalStyle = lipgloss.NewStyle().
			Border(g.border).
			BorderForeground(accent).
			Padding(1, 3)
)

// categoryPalette holds mid-tone colours that stay readable on light and dark
// terminals and carry white badge text. The accent purple is left out so
// categories never look like the selection.
var categoryPalette = []lipgloss.AdaptiveColor{
	{Light: "#2563EB", Dark: "#3B82F6"}, // blue
	{Light: "#0891B2", Dark: "#06B6D4"}, // cyan
	{Light: "#059669", Dark: "#10B981"}, // emerald
	{Light: "#D97706", Dark: "#F59E0B"}, // amber
	{Light: "#EA580C", Dark: "#F97316"}, // orange
	{Light: "#E11D48", Dark: "#F43F5E"}, // rose
	{Light: "#DB2777", Dark: "#EC4899"}, // pink
	{Light: "#4F46E5", Dark: "#6366F1"}, // indigo
}

// categoryColor returns a stable colour for a category: the same name (ignoring
// case and surrounding space) always maps to the same palette entry. An empty
// category is muted.
func categoryColor(category string) lipgloss.AdaptiveColor {
	key := strings.ToUpper(strings.TrimSpace(category))
	if key == "" {
		return subtle
	}
	h := fnv.New32a()
	h.Write([]byte(key))
	return categoryPalette[h.Sum32()%uint32(len(categoryPalette))]
}

func categoryBadge(category string) string {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(categoryColor(category)).
		Padding(0, 1).
		Render(category)
}

// gradient colours the header logo from the accent to a cyan.
var gradientStops = [2][2]string{
	{"#7C3AED", "#0891B2"}, // light terminals: from, to
	{"#A855F7", "#22D3EE"}, // dark terminals
}

// gradientText renders s with a left-to-right colour gradient.
func gradientText(s string) string {
	runes := []rune(s)
	stops := gradientStops[1]
	if !lipgloss.HasDarkBackground() {
		stops = gradientStops[0]
	}
	var from, to [3]int
	fmt.Sscanf(stops[0], "#%02x%02x%02x", &from[0], &from[1], &from[2])
	fmt.Sscanf(stops[1], "#%02x%02x%02x", &to[0], &to[1], &to[2])

	var b strings.Builder
	for i, r := range runes {
		t := 0.0
		if len(runes) > 1 {
			t = float64(i) / float64(len(runes)-1)
		}
		var c [3]int
		for k := range c {
			c[k] = from[k] + int(float64(to[k]-from[k])*t)
		}
		hex := fmt.Sprintf("#%02X%02X%02X", c[0], c[1], c[2])
		b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(hex)).Render(string(r)))
	}
	return b.String()
}

// tagTopBorder replaces the first line of a bordered block with one carrying a
// title tag, e.g. "╭─ Secrets ───╮", drawn in style.
func tagTopBorder(block, title string, style lipgloss.Style) string {
	lines := strings.SplitN(block, "\n", 2)
	w := lipgloss.Width(lines[0])
	tag := " " + title + " "
	fill := w - 2 - 1 - lipgloss.Width(tag)
	if fill < 0 {
		return block
	}
	top := style.Render(g.border.TopLeft+g.border.Top) + style.Bold(true).Render(tag) +
		style.Render(strings.Repeat(g.border.Top, fill)+g.border.TopRight)
	if len(lines) == 1 {
		return top
	}
	return top + "\n" + lines[1]
}
