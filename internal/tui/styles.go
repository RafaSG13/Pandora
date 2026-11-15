package tui

import "github.com/charmbracelet/lipgloss"

var (
	accent  = lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#A855F7"}
	subtle  = lipgloss.AdaptiveColor{Light: "#64748B", Dark: "#94A3B8"}
	success = lipgloss.AdaptiveColor{Light: "#059669", Dark: "#10B981"}
	danger  = lipgloss.AdaptiveColor{Light: "#DC2626", Dark: "#F87171"}
	// selectedBg tints the selected list row.
	selectedBg = lipgloss.AdaptiveColor{Light: "#EDE9FE", Dark: "#2E1065"}
	text       = lipgloss.AdaptiveColor{Light: "#1E293B", Dark: "#F8FAFC"}
)

var (
	subtleStyle = lipgloss.NewStyle().Foreground(subtle)
	textStyle   = lipgloss.NewStyle().Foreground(text)
	labelStyle  = lipgloss.NewStyle().Bold(true).Foreground(subtle).Width(10)
	keyStyle    = lipgloss.NewStyle().Bold(true).Foreground(accent)
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(accent)

	badgeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(accent).
			Padding(0, 1)

	// paneStyle is the base for both panes; callers set width, height and border colour.
	paneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1, 2)

	modalStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accent).
			Padding(1, 3)
)
