package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"pandora/internal/store"
)

const maskedValue = "••••••••••••"

func (m model) View() string {
	if m.width == 0 {
		return "Loading…"
	}

	switch m.mode {
	case modeForm:
		return m.placeModal(m.renderForm())
	case modeConfirmDelete:
		return m.placeModal(m.renderConfirmDelete())
	}

	rule := subtleStyle.Render(strings.Repeat("─", m.width))
	return strings.Join([]string{m.renderHeader(), rule, m.renderPanes(), m.renderStatus(), m.renderHelp()}, "\n")
}

func (m model) renderHeader() string {
	left := titleStyle.Render("▌PANDORA") + subtleStyle.Render(" · OS keychain vault")

	right := subtleStyle.Render(count(len(m.list.Items())))

	gap := max(1, m.width-lipgloss.Width(left)-lipgloss.Width(right))
	return left + strings.Repeat(" ", gap) + right
}

func (m model) renderPanes() string {
	leftW, rightW := m.paneWidths()
	h := m.height - 6

	left := paneStyle.BorderForeground(accent).Width(leftW).Height(h).Render(m.list.View())
	right := paneStyle.BorderForeground(subtle).Width(rightW).Height(h).Render(m.renderDetail(rightW-4, h-2))
	return lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right)
}

// renderDetail draws the selected secret inside a w×h content area.
func (m model) renderDetail(w, h int) string {
	sel, ok := m.list.SelectedItem().(item)
	if !ok {
		if len(m.list.Items()) > 0 {
			return ""
		}
		empty := subtleStyle.Width(w).Align(lipgloss.Center).Render("No secrets yet — press n to add one.")
		return lipgloss.PlaceVertical(h, lipgloss.Center, empty)
	}

	valueW := max(1, w-lipgloss.Width(labelStyle.Render("")))
	value := subtleStyle.Render(maskedValue)
	if m.showValue {
		if real, err := store.GetSecretValue(sel.meta.Title); err == nil {
			value = textStyle.Render(real)
		}
	}
	category := subtleStyle.Render("—")
	if sel.meta.Category != "" {
		category = badgeStyle.Render(sel.meta.Category)
	}

	row := func(label, content string) string {
		return lipgloss.JoinHorizontal(lipgloss.Top,
			labelStyle.Render(label),
			lipgloss.NewStyle().Width(valueW).Render(content))
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		textStyle.Bold(true).Width(w).Render(sel.meta.Title),
		"",
		row("Category", category),
		row("Created", textStyle.Render(sel.meta.Created)),
		row("Value", value),
	)
}

// placeModal centres a dialog with the contextual help line beneath it.
func (m model) placeModal(dialog string) string {
	body := lipgloss.JoinVertical(lipgloss.Center, dialog, "", m.renderHelp())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, body)
}

func (m model) renderForm() string {
	inner := m.modalWidth() - 6
	labels := []string{"Name", "Category", "Secret"}

	title := "New secret"
	if m.editing != "" {
		title = "Edit secret"
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render(title))
	for i, in := range m.inputs {
		b.WriteString("\n\n")
		b.WriteString(subtleStyle.Bold(true).Render(labels[i]))
		b.WriteString("\n")
		b.WriteString(in.View())
	}
	b.WriteString("\n\n")
	b.WriteString(m.renderStatus())

	return modalStyle.Width(m.modalWidth()).Render(lipgloss.NewStyle().MaxWidth(inner).Render(b.String()))
}

func (m model) renderConfirmDelete() string {
	inner := m.modalWidth() - 6
	body := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Bold(true).Foreground(danger).Render("Delete secret"),
		"",
		textStyle.Width(inner).Render(`Delete "`+m.deleting+`"?`),
		"",
		subtleStyle.Width(inner).Render("Its value is removed from the OS keyring. This cannot be undone."),
	)
	return modalStyle.BorderForeground(danger).Width(m.modalWidth()).Render(body)
}

func (m model) renderStatus() string {
	if m.statusMsg == "" {
		return ""
	}
	color := success
	if m.statusErr {
		color = danger
	}
	return lipgloss.NewStyle().Foreground(color).Bold(true).MaxWidth(m.width).Render(m.statusMsg)
}

func (m model) renderHelp() string {
	keys := [][2]string{
		{"↑/↓", "move"}, {"/", "filter"}, {"enter/c", "copy"}, {"v", "reveal"},
		{"n", "new"}, {"e", "edit"}, {"d", "delete"}, {"r", "reload"}, {"q", "quit"},
	}
	switch m.mode {
	case modeForm:
		keys = [][2]string{{"tab", "next"}, {"shift+tab", "prev"}, {"enter", "save"}, {"esc", "cancel"}}
	case modeConfirmDelete:
		keys = [][2]string{{"y", "delete"}, {"any other key", "cancel"}}
	}

	// On narrow terminals drop the least essential hints until the line fits.
	line := renderKeys(keys)
	for _, drop := range []string{"move", "filter", "reveal", "reload"} {
		if lipgloss.Width(line) <= m.width {
			break
		}
		for i, k := range keys {
			if k[1] == drop {
				keys = append(keys[:i:i], keys[i+1:]...)
				break
			}
		}
		line = renderKeys(keys)
	}
	return lipgloss.NewStyle().MaxWidth(m.width).Render(line)
}

func renderKeys(keys [][2]string) string {
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = keyStyle.Render(k[0]) + " " + subtleStyle.Render(k[1])
	}
	return strings.Join(parts, subtleStyle.Render(" · "))
}
