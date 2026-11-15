package tui

import (
	"hash/fnv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// categoryPalette holds one entry per category colour. Light is dark enough for
// white badge text on any terminal; Dark is the lighter variant used for text
// and bars on dark backgrounds.
var categoryPalette = []struct{ light, dark string }{
	{"#2563EB", "#60A5FA"}, // blue
	{"#0D9488", "#2DD4BF"}, // teal
	{"#B45309", "#FBBF24"}, // amber
	{"#BE123C", "#FB7185"}, // rose
	{"#15803D", "#4ADE80"}, // green
	{"#0E7490", "#22D3EE"}, // cyan
	{"#BE185D", "#F472B6"}, // pink
	{"#7C3AED", "#A78BFA"}, // violet
}

// categoryIndex maps a category to a stable palette slot (case-insensitive).
func categoryIndex(category string) int {
	h := fnv.New32a()
	h.Write([]byte(strings.ToUpper(strings.TrimSpace(category))))
	return int(h.Sum32() % uint32(len(categoryPalette)))
}

// categoryColor is the foreground colour for a category; uncategorised secrets are muted.
func categoryColor(category string) lipgloss.TerminalColor {
	if strings.TrimSpace(category) == "" {
		return subtle
	}
	c := categoryPalette[categoryIndex(category)]
	return lipgloss.AdaptiveColor{Light: c.light, Dark: c.dark}
}

// categoryBadge renders the category as a coloured pill.
func categoryBadge(category string) string {
	if strings.TrimSpace(category) == "" {
		return subtleStyle.Render("No category")
	}
	c := categoryPalette[categoryIndex(category)]
	return badgeStyle.Background(lipgloss.Color(c.light)).Render(category)
}
