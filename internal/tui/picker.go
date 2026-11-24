package tui

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/RafaSG13/Pandora/internal/store"
)

// The picker (modePicker) lists every category and tag with its secret count,
// so categories past the ninth (which have no digit key) can be reached. It
// holds names only; lock() clears it.

type pickKind int

const (
	pickAll pickKind = iota
	pickCategory
	pickTag
)

type pickEntry struct {
	kind  pickKind
	name  string
	count int
}

type pickerState struct {
	entries []pickEntry
	cursor  int
}

// openPicker builds the entries from the stored metadata and shows the
// picker, or says there is nothing to pick from.
func (m *model) openPicker() {
	metas, err := store.LoadMetadata()
	if err != nil {
		m.notify(fmt.Sprintf("✗ Could not load: %v", err), true)
		return
	}
	categories, tags := distinctCategories(metas), distinctTags(metas)
	if len(categories) == 0 && len(tags) == 0 {
		m.notify("No categories or tags yet", false)
		return
	}
	entries := []pickEntry{{pickAll, "", len(metas)}}
	cursor := 0
	for _, c := range categories {
		n := 0
		for _, meta := range metas {
			if strings.EqualFold(strings.TrimSpace(meta.Category), c) {
				n++
			}
		}
		if strings.EqualFold(c, m.category) {
			cursor = len(entries)
		}
		entries = append(entries, pickEntry{pickCategory, c, n})
	}
	for _, t := range tags {
		n := 0
		for _, meta := range metas {
			if slices.Contains(meta.Tags, t) {
				n++
			}
		}
		if m.category == "" && t == m.tag {
			cursor = len(entries)
		}
		entries = append(entries, pickEntry{pickTag, t, n})
	}
	m.picker, m.mode = pickerState{entries: entries, cursor: cursor}, modePicker
}

// pickerRows is how many entries the picker lists at once; the dialog is the
// rows plus 3 lines of text, 4 of border and padding, and 2 under it.
func (m model) pickerRows() int { return min(12, max(1, m.height-9)) }

// toggle returns "" when name is the current filter value (picking it again
// clears it), otherwise name.
func toggle(cur, name string) string {
	if strings.EqualFold(cur, name) {
		return ""
	}
	return name
}

func (m model) updatePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "ctrl+c" {
		return m, tea.Quit
	}
	if key == "esc" || key == "q" || m.keys.action(key) == "filters" {
		m.picker, m.mode = pickerState{}, modeBrowse
		return m, nil
	}

	p := &m.picker
	rows := m.pickerRows()
	last := len(p.entries) - 1
	switch key {
	case "up", "k":
		p.cursor--
	case "down", "j":
		p.cursor++
	case "pgup":
		p.cursor -= rows
	case "pgdown":
		p.cursor += rows
	case "home", "g":
		p.cursor = 0
	case "end", "G":
		p.cursor = last
	case "enter":
		if p.cursor < 0 || p.cursor > last {
			return m, nil
		}
		e := p.entries[p.cursor]
		m.picker, m.mode = pickerState{}, modeBrowse
		switch e.kind {
		case pickCategory:
			return m.applyFilter(toggle(m.category, e.name), m.tag), nil
		case pickTag:
			return m.applyFilter(m.category, toggle(m.tag, e.name)), nil
		}
		return m.applyFilter("", ""), nil
	}
	p.cursor = max(0, min(p.cursor, last))
	return m, nil
}

func (m model) renderPicker() string {
	inner := m.modalWidth() - 8
	p := m.picker
	rows := m.pickerRows()
	top := max(0, min(p.cursor-rows/2, len(p.entries)-rows))

	lines := []string{lipgloss.NewStyle().Bold(true).Foreground(accent).Render("Filter by category or tag"), ""}
	for i := top; i < min(len(p.entries), top+rows); i++ {
		e := p.entries[i]
		prefix := "  "
		if i == p.cursor {
			prefix = lipgloss.NewStyle().Bold(true).Foreground(accent).Render(g.marker) + " "
		}
		suffix := "  " + subtleStyle.Render(strconv.Itoa(e.count))
		active := false
		lead, label := "", "All secrets" // lead is the styled colour dot of a category
		switch e.kind {
		case pickCategory:
			active = strings.EqualFold(e.name, m.category)
			lead, label = lipgloss.NewStyle().Foreground(categoryColor(e.name)).Render(g.dot)+" ", e.name
		case pickTag:
			active = e.name == m.tag
			label = "#" + e.name
		default:
			active = m.category == "" && m.tag == ""
		}
		if active {
			suffix += "  " + lipgloss.NewStyle().Foreground(accent).Render("(active)")
		}
		room := max(1, inner-lipgloss.Width(prefix)-lipgloss.Width(lead)-lipgloss.Width(suffix))
		style := textStyle
		if i == p.cursor {
			style = style.Bold(true).Foreground(accent)
		}
		lines = append(lines, prefix+lead+style.Render(truncate(label, room))+suffix)
	}
	if len(p.entries) > rows {
		lines = append(lines, subtleStyle.Render(truncate(fmt.Sprintf("%d–%d of %d", top+1, min(len(p.entries), top+rows), len(p.entries)), inner)))
	}
	return modalStyle.Width(m.modalWidth()).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}
