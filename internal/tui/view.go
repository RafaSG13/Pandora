package tui

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/RafaSG13/Pandora/internal/gen"
	"github.com/RafaSG13/Pandora/internal/store"
	"github.com/RafaSG13/Pandora/internal/version"
)

var maskedValue = strings.Repeat(g.mask, 12)

// asciiReplacer maps the non-ASCII characters that appear in literal strings
// to single-column ASCII, so PANDORA_ASCII never needs a wider layout.
var asciiReplacer = strings.NewReplacer(
	"✓", "+", "✗", "x", "—", "-", "…", ".", "↑", "^", "↓", "v", "→", ">", "×", "x",
	"·", "-", "•", "*", "●", "*", "◆", "#", "▌", ">", "█", "#", "░", "-", "┃", "#", "┆", "|",
)

func (m model) View() string {
	if g.ascii {
		return asciiReplacer.Replace(m.view())
	}
	return m.view()
}

func (m model) view() string {
	if m.width == 0 {
		return "Loading…"
	}
	if m.width < minWidth || m.height < minHeight {
		msg := fmt.Sprintf("Terminal too small\n%d×%d now, %d×%d needed", m.width, m.height, minWidth, minHeight)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, subtleStyle.Render(msg))
	}

	switch m.mode {
	case modeUnlock, modeSetup:
		return m.renderAuth()
	case modeForm:
		return m.placeModal(m.renderForm())
	case modeConfirmDelete:
		return m.placeModal(m.renderConfirmDelete())
	case modeHelp:
		return m.placeModal(m.renderHelpOverlay())
	}

	rule := subtleStyle.Render(strings.Repeat(g.rule, m.width))
	body := m.renderBody()
	lines := []string{m.renderHeader(), rule}
	lines = append(lines, strings.Split(body, "\n")...)
	paneBottom := len(lines) - 1
	// Keep the footer pinned to the bottom when the panes fit their content.
	for len(lines) < m.height-2 {
		lines = append(lines, "")
	}
	lines = append(lines, m.renderCountdowns(), m.renderHelp())

	if m.statusMsg != "" {
		lines = overlayToast(lines, paneBottom-1, m.renderToast(), m.width)
	}
	return strings.Join(lines, "\n")
}

// backendName is the OS keyring Pandora talks to.
func backendName() string {
	switch runtime.GOOS {
	case "darwin":
		return "Keychain"
	case "windows":
		return "Credential Manager"
	default:
		return "Secret Service"
	}
}

func (m model) renderHeader() string {
	left := subtleStyle.Render(g.logo) + " " + gradientText("PANDORA") + subtleStyle.Render(" "+g.sep+" "+backendName())

	chip := lipgloss.NewStyle().Foreground(success).Render(g.dot + " unlocked")
	// Right-hand pieces in priority order; those that do not fit are dropped.
	pieces := []string{subtleStyle.Render(count(m.total)), chip}
	if m.expiring > 0 {
		pieces = append(pieces, lipgloss.NewStyle().Foreground(warning).Bold(true).Render(fmt.Sprintf("%s %d expiring", g.clock, m.expiring)))
	}
	if m.checking {
		pieces = append(pieces, m.spinner.View()+subtleStyle.Render(" checking keyring"))
	}

	right := pieces[0]
	for _, p := range pieces[1:] {
		next := p + "  " + right
		if lipgloss.Width(left)+1+lipgloss.Width(next) > m.width {
			break
		}
		right = next
	}
	gap := max(1, m.width-lipgloss.Width(left)-lipgloss.Width(right))
	return left + strings.Repeat(" ", gap) + right
}

// renderBody draws the panes, or the welcome / error screen when there are no
// secrets to show.
func (m model) renderBody() string {
	if m.total == 0 {
		return m.renderEmpty()
	}
	leftW, rightW := m.paneWidths()
	h := m.paneOuter() - 2 // lipgloss Height excludes the border

	left := paneStyle.BorderForeground(accent).Width(leftW).Height(h).Render(m.listView())
	right := paneStyle.BorderForeground(subtle).Width(rightW).Height(h).Render(m.renderDetail(rightW-4, h-2))
	left = tagTopBorder(left, "Secrets", lipgloss.NewStyle().Foreground(accent))
	right = tagTopBorder(right, "Details", lipgloss.NewStyle().Foreground(subtle))
	return lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right)
}

// vaultArt is a small vault drawn with the active border glyphs.
func vaultArt() string {
	b := g.border
	row := func(l, fill, r string, n int) string { return l + strings.Repeat(fill, n) + r }
	return strings.Join([]string{
		row(b.TopLeft, b.Top, b.TopRight, 13),
		b.Left + "   " + row(b.TopLeft, b.Top, b.TopRight, 5) + "   " + b.Right,
		b.Left + "   " + b.Left + "  " + g.dot + "  " + b.Right + "   " + b.Right,
		b.Left + "   " + row(b.BottomLeft, b.Bottom, b.BottomRight, 5) + "   " + b.Right,
		row(b.BottomLeft, b.Bottom, b.BottomRight, 13),
	}, "\n")
}

// renderEmpty is the first-run welcome, or the error state when the metadata
// could not be read.
func (m model) renderEmpty() string {
	inner := min(m.width-8, 60)
	var body string
	if m.loadErr != "" {
		body = lipgloss.JoinVertical(lipgloss.Center,
			lipgloss.NewStyle().Bold(true).Foreground(danger).Render("Could not load your secrets"),
			"",
			subtleStyle.Width(inner).Align(lipgloss.Center).Render(m.loadErr),
			"",
			subtleStyle.Width(inner).Align(lipgloss.Center).Render("Your metadata file is left untouched. Fix the problem and reload."),
		)
	} else {
		body = lipgloss.JoinVertical(lipgloss.Center,
			lipgloss.NewStyle().Bold(true).Foreground(accent).Render("Welcome to Pandora"),
			"",
			subtleStyle.Width(inner).Align(lipgloss.Center).Render(
				"Secret values live in your operating system's keyring ("+backendName()+"), encrypted with your master password. "+
					"Names, categories and notes are kept encrypted in a local vault file; Pandora never uses the network."),
			"",
			textStyle.Bold(true).Render("Press "+keyStyle.Render("n")+" to add your first secret"),
		)
	}
	art := lipgloss.NewStyle().Foreground(accent).Render(vaultArt())
	card := lipgloss.JoinVertical(lipgloss.Center, art, "", body)

	h := m.paneOuter() - 2
	return paneStyle.BorderForeground(accent).Width(m.width - 2).Height(h).
		Render(lipgloss.Place(m.width-6, h-2, lipgloss.Center, lipgloss.Center, card))
}

// listView renders the list scrolled to m.scroll: the title and status bars
// followed by a window of whole items cropped from the single-page list, with
// a scrollbar when not everything fits.
func (m model) listView() string {
	lines := strings.Split(m.list.View(), "\n")
	if len(lines) <= listChrome {
		return strings.Join(lines, "\n")
	}
	n, view, rows := len(m.list.VisibleItems()), m.itemsInView(), m.rowsPer()

	if m.list.FilterState() == list.Unfiltered && n > 0 { // position indicator replaces "N items"
		pos := fmt.Sprintf("%d/%d %s by %s", m.list.Index()+1, n, g.sep, m.sort)
		lines[2] = subtleStyle.Render("  " + pos)
	}

	start := min(listChrome+m.scroll*rows, len(lines))
	end := min(start+view*rows-(rows-m.textLines()), len(lines))
	items := lines[start:end]

	if n > view {
		listW := m.list.Width()
		thumbH := max(1, len(items)*view/n)
		thumbTop := 0
		if n > view {
			thumbTop = (len(items) - thumbH) * m.scroll / (n - view)
		}
		for i, l := range items {
			bar := subtleStyle.Render(g.track)
			if i >= thumbTop && i < thumbTop+thumbH {
				bar = lipgloss.NewStyle().Foreground(accent).Render(g.thumb)
			}
			items[i] = l + strings.Repeat(" ", max(0, listW-lipgloss.Width(l))) + " " + bar
		}
	}
	return strings.Join(append(lines[:listChrome:listChrome], items...), "\n")
}

// renderDetail draws the selected secret as a card inside a w×h content area:
// title with its category badge, a metadata block, a bordered value box and
// the notes.
func (m model) renderDetail(w, h int) string {
	sel, ok := m.list.SelectedItem().(item)
	if !ok {
		if len(m.list.Items()) > 0 {
			return ""
		}
		empty := subtleStyle.Width(w).Align(lipgloss.Center).Render("No secrets yet — press n to add one.")
		return lipgloss.PlaceVertical(h, lipgloss.Center, empty)
	}
	meta := sel.meta
	now := m.nowOr()

	header := textStyle.Bold(true).Render(truncate(meta.Title, w))
	if meta.Category != "" {
		badge := categoryBadge(meta.Category)
		const gap = 2
		header = textStyle.Bold(true).Render(truncate(meta.Title, max(1, w-lipgloss.Width(badge)-gap))) +
			strings.Repeat(" ", gap) + badge
	}

	row := func(label, content string) string {
		return lipgloss.JoinHorizontal(lipgloss.Top, labelStyle.Render(label), content)
	}
	rows := []string{row("Created", textStyle.Render(meta.Created))}
	if meta.Updated != "" {
		rows = append(rows, row("Updated", textStyle.Render(meta.Updated)))
	}
	if label, level := expiryInfo(meta, now, m.expiryDays); level != expiryNone {
		style := subtleStyle
		switch level {
		case expirySoon:
			style = lipgloss.NewStyle().Foreground(warning).Bold(true)
		case expiryExpired:
			style = lipgloss.NewStyle().Foreground(danger).Bold(true)
		}
		rows = append(rows, row("Expires", textStyle.Render(meta.Expires)+"  "+style.Render(g.clock+" "+label)))
	}
	if len(meta.Tags) > 0 {
		rows = append(rows, row("Tags", textStyle.Render(truncate("#"+strings.Join(meta.Tags, " #"), max(1, w-10)))))
	}

	// The notes decide how many lines the value box may use.
	var tail []string
	if meta.Notes != "" {
		wrapped := strings.Split(textStyle.Width(w).Render(meta.Notes), "\n")
		if len(wrapped) > 3 {
			wrapped = append(wrapped[:2], truncate(strings.TrimSpace(ansi.Strip(wrapped[2])), w-1)+g.ellipsis)
		}
		tail = append([]string{"", subtleStyle.Bold(true).Render("Notes")}, wrapped...)
	}
	// header, blank, rows, blank, label, the box's two borders, then the notes.
	valueRows := max(2, h-(2+len(rows)+2+2+len(tail)))

	icon, hint, borderColor := g.lock, "v to reveal", subtle
	value := subtleStyle.Render(maskedValue)
	switch {
	case m.orphans[meta.Title]:
		icon, borderColor = g.orphan, danger
		value = lipgloss.NewStyle().Foreground(danger).Render("Value missing from the keyring — press e to set it")
		hint = ""
	case m.revealed(meta.Title):
		if m.revealErr != nil {
			value = lipgloss.NewStyle().Foreground(danger).Render(fmt.Sprintf("✗ %v", m.revealErr))
			break
		}
		left := int(m.revealUntil.Sub(now).Round(time.Second) / time.Second)
		icon, hint, borderColor = g.unlock, fmt.Sprintf("v to hide %s %ds", g.sep, max(0, left)), accent
		value = revealBlock(m.revealValue, w-4, valueRows)
	}
	box := lipgloss.NewStyle().
		Border(g.border).
		BorderForeground(borderColor).
		Padding(0, 1).
		Width(max(1, w-2)).
		Render(value)

	label := subtleStyle.Bold(true).Render(icon + " Value")
	if hint != "" && lipgloss.Width(label)+2+lipgloss.Width(hint) <= w {
		label += "  " + subtleStyle.Render(hint)
	}

	parts := []string{header, ""}
	parts = append(parts, rows...)
	parts = append(parts, "", label, box)
	parts = append(parts, tail...)

	out := strings.Split(lipgloss.JoinVertical(lipgloss.Left, parts...), "\n")
	if len(out) > h {
		out = out[:h]
	}
	return strings.Join(out, "\n")
}

// renderToast is the bordered pill showing the latest status message.
func (m model) renderToast() string {
	color := success
	if m.statusErr {
		color = danger
	}
	_, rightW := m.paneWidths()
	msg := truncate(m.statusMsg, max(8, rightW-8)) // stays inside the detail pane
	return lipgloss.NewStyle().
		Border(g.border).
		BorderForeground(color).
		Foreground(color).
		Bold(true).
		Padding(0, 1).
		Render(msg)
}

// overlayToast draws box over the right edge of lines, starting at row top,
// two columns in from the terminal edge so the pane border stays visible.
func overlayToast(lines []string, top int, box string, width int) []string {
	const margin = 3
	boxLines := strings.Split(box, "\n")
	bw := lipgloss.Width(boxLines[0])
	top = max(2, top-len(boxLines)+1)
	for k, bl := range boxLines {
		row := top + k
		if row >= len(lines) {
			break
		}
		keep := max(0, width-bw-margin)
		head := ansi.Truncate(lines[row], keep, "")
		head += strings.Repeat(" ", max(0, keep-lipgloss.Width(head)))
		lines[row] = head + bl + ansi.TruncateLeft(lines[row], width-margin+1, "")
	}
	return lines
}

// renderCountdowns shows draining bars for the pending clipboard clear and
// the auto-hide of a revealed value; empty when neither is active.
func (m model) renderCountdowns() string {
	now := m.nowOr()
	var parts []string
	if !m.clipUntil.IsZero() {
		parts = append(parts, countdown("Copied "+g.sep+" clears in", m.clipUntil.Sub(now), m.clipTTL, success))
	}
	if m.revealTitle != "" && now.Before(m.revealUntil) {
		parts = append(parts, countdown("Value hides in", m.revealUntil.Sub(now), m.revealFor, accent))
	}
	return lipgloss.NewStyle().MaxWidth(m.width).Render(strings.Join(parts, "   "))
}

func countdown(label string, remaining, total time.Duration, color lipgloss.AdaptiveColor) string {
	const barW = 10
	secs := int((remaining + time.Second - 1) / time.Second)
	filled := min(barW, max(0, int((remaining*barW+total-1)/total)))
	bar := lipgloss.NewStyle().Foreground(color).Render(strings.Repeat(g.barFull, filled)) +
		subtleStyle.Render(strings.Repeat(g.barEmpty, barW-filled))
	return subtleStyle.Render(fmt.Sprintf("%s %ds ", label, max(0, secs))) + bar
}

// placeModal centres a dialog with the contextual help line beneath it.
func (m model) placeModal(dialog string) string {
	body := lipgloss.JoinVertical(lipgloss.Center, dialog, "", m.renderHelp())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, body)
}

// renderAuth is the master-password screen: unlock, or first-run setup (which
// also migrates an older plain-text store).
func (m model) renderAuth() string {
	inner := m.modalWidth() - 8
	// Each input is drawn as a [ box ] with a hint inside, so it reads as a
	// text field and not as a button.
	field := func(i int, label string, in textinput.Model) string {
		l := lipgloss.NewStyle().Width(formLabelW)
		box := lipgloss.NewStyle().Width(in.Width + 1).Render(in.View())
		if m.pwField == i {
			br := lipgloss.NewStyle().Foreground(accent).Bold(true)
			return br.Render(g.marker) + " " + l.Bold(true).Foreground(accent).Render(label) + br.Render("[") + box + br.Render("]")
		}
		return "  " + l.Foreground(subtle).Render(label) + subtleStyle.Render("[") + box + subtleStyle.Render("]")
	}

	var b strings.Builder
	if m.mode == modeSetup {
		heading := "Create a master password"
		if m.migrating {
			heading = fmt.Sprintf("Migrate %s to an encrypted vault", count(m.legacyN))
		}
		b.WriteString(titleStyle.Render(heading) + "\n\n")
		b.WriteString(subtleStyle.Render(fmt.Sprintf("At least %d characters. It cannot be recovered.", store.MinPasswordLen)) + "\n\n")
		b.WriteString(field(0, "Password", m.pwInput) + "\n")
		b.WriteString(field(1, "Repeat", m.pwConfirm))
	} else {
		b.WriteString(lipgloss.NewStyle().Foreground(warning).Bold(true).Render(g.lock+" Locked") + "\n\n")
		b.WriteString(subtleStyle.Render("Enter your master password.") + "\n\n")
		b.WriteString(field(0, "Password", m.pwInput))
	}
	b.WriteString("\n\n")
	switch {
	case m.unlocking:
		b.WriteString(m.spinner.View() + " " + subtleStyle.Render("Working"+g.ellipsis))
	case m.unlockErr != "":
		b.WriteString(lipgloss.NewStyle().Foreground(danger).Bold(true).Render(m.unlockErr))
	}
	return m.placeModal(modalStyle.Width(m.modalWidth()).Render(lipgloss.NewStyle().MaxWidth(inner).Render(b.String())))
}

// formLabelW is the width of the form's label column.
const formLabelW = 10

func (m model) renderForm() string {
	inner := m.modalWidth() - 8
	labels := [numFields]string{"Name", "Category", "Secret", "Expires", "Notes", "Tags"}

	title := "New secret"
	if m.editing != "" {
		title = "Edit secret"
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n")
	for i, in := range m.inputs {
		b.WriteString("\n")
		label := lipgloss.NewStyle().Width(formLabelW)
		if i == m.focusIndex {
			b.WriteString(lipgloss.NewStyle().Foreground(accent).Render(g.marker) + " ")
			b.WriteString(label.Bold(true).Foreground(accent).Render(labels[i]))
		} else {
			b.WriteString("  ")
			b.WriteString(label.Foreground(subtle).Render(labels[i]))
		}
		if i == fieldSecret {
			rows := m.secretLines()
			b.WriteString(rows[0])
			indent := strings.Repeat(" ", 2+formLabelW)
			for _, r := range rows[1:] {
				b.WriteString("\n" + indent + r)
			}
			continue
		}
		b.WriteString(in.View())
	}
	b.WriteString("\n\n")
	b.WriteString(subtleStyle.Render(keyStyle.Render("ctrl+g") + " generate " + g.sep + " " + keyStyle.Render("ctrl+r") + " show/hide"))
	b.WriteString("\n")
	b.WriteString(subtleStyle.Render(fmt.Sprintf("%s %d chars %s %s %s", keyStyle.Render("ctrl+l"), gen.Lengths[m.genLen], g.sep, keyStyle.Render("ctrl+t"), gen.Charsets[m.genSet].Name)))
	b.WriteString("\n") // status sits right under the hints: the Tags row took the spare line at 60×18
	b.WriteString(m.renderStatus())

	return modalStyle.Width(m.modalWidth()).Render(lipgloss.NewStyle().MaxWidth(inner).Render(b.String()))
}

func (m model) renderConfirmDelete() string {
	inner := m.modalWidth() - 8
	body := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Bold(true).Foreground(danger).Render("Delete secret"),
		"",
		textStyle.Width(inner).Render(`Delete "`+m.deleting+`"?`),
		"",
		subtleStyle.Width(inner).Render("Its value is removed from the OS keyring. This cannot be undone."),
	)
	return modalStyle.BorderForeground(danger).Width(m.modalWidth()).Render(body)
}

// helpGroup is one titled block of the help overlay.
type helpGroup struct {
	title string
	rows  [][2]string
}

// helpColumns is the full key reference, laid out in columns by renderHelpOverlay.
var helpColumns = [][]helpGroup{
	{
		{"Browse", [][2]string{
			{"↑/↓ j/k", "move"},
			{"g/G", "first / last"},
			{"pgup/pgdn", "page"},
			{"/", "filter"},
			{"esc", "clear filter"},
			{"1-9 / 0", "category filter"},
			{"T", "cycle tag filter"},
			{"s", "cycle sort"},
			{"t", "compact rows"},
			{"enter/c", "copy"},
			{"x", "clear clipboard"},
			{"v", "reveal / hide"},
			{"n / e / d", "new / edit / delete"},
			{"r", "reload"},
			{"?", "this help"},
			{"q", "quit"},
		}},
	},
	{
		{"Form", [][2]string{
			{"tab/↓", "next field"},
			{"shift+tab/↑", "previous field"},
			{"enter", "next / save (new line in Secret)"},
			{"ctrl+s", "save"},
			{"ctrl+r", "show / hide the secret"},
			{"→", "accept category"},
			{"ctrl+g", "generate secret"},
			{"ctrl+l", "secret length"},
			{"ctrl+t", "secret charset"},
			{"esc", "cancel"},
		}},
		{"Mouse", [][2]string{
			{"click", "select secret"},
			{"wheel", "scroll list"},
		}},
	},
}

// helpChrome is the rows around the help overlay's text: its border and
// padding (4), plus the blank line and key hints placed beneath it (2).
const helpChrome = 6

// helpLayout is the overlay's outer width and how many text rows fit above
// the chrome at the current terminal height.
func (m model) helpLayout() (width, view int) {
	return min(70, m.width-4), max(1, m.height-helpChrome)
}

// helpLines is the overlay's full text, one entry per row: the title, the key
// reference (two columns when there is room) and the build version.
func (m model) helpLines() []string {
	keyCol := lipgloss.NewStyle().Width(12)
	render := func(groups []helpGroup) string {
		var b strings.Builder
		for gi, grp := range groups {
			if gi > 0 {
				b.WriteString("\n\n")
			}
			b.WriteString(subtleStyle.Bold(true).Render(grp.title))
			for _, r := range grp.rows {
				b.WriteString("\n")
				b.WriteString(keyStyle.Inherit(keyCol).Render(r[0]) + subtleStyle.Render(r[1]))
			}
		}
		return b.String()
	}

	width, _ := m.helpLayout()
	cols := []string{render(helpColumns[0]), render(helpColumns[1])}
	var body string
	if width >= 70 { // two columns when there is room
		body = lipgloss.JoinHorizontal(lipgloss.Top, cols[0], "    ", cols[1])
	} else {
		body = cols[0] + "\n\n" + cols[1]
	}
	content := titleStyle.Render("Keyboard & mouse") + "\n\n" + body + "\n\n" +
		subtleStyle.Render("pandora "+version.Get().Version)
	return strings.Split(lipgloss.NewStyle().MaxWidth(width-8).Render(content), "\n")
}

// helpScrollable reports whether the overlay is taller than the terminal
// allows, in which case it scrolls and spends its last row on a position hint.
func (m model) helpScrollable() bool {
	_, view := m.helpLayout()
	return len(m.helpLines()) > view
}

// clampHelpScroll keeps the first shown help row within the text. When the
// text does not fit in view rows, the last of them is the position hint.
func clampHelpScroll(scroll, total, view int) int {
	if total <= view {
		return 0
	}
	return max(0, min(scroll, total-(view-1)))
}

func (m model) renderHelpOverlay() string {
	width, view := m.helpLayout()
	lines := m.helpLines()
	if total := len(lines); total > view {
		rows := view - 1
		off := clampHelpScroll(m.helpScroll, total, view)
		shown := make([]string, 0, view)
		shown = append(shown, lines[off:off+rows]...)
		lines = append(shown, subtleStyle.Render(fmt.Sprintf("%d-%d/%d  ↑/↓ scroll", off+1, off+rows, total)))
	}
	return modalStyle.Width(width).Render(strings.Join(lines, "\n"))
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
	var keys [][2]string
	switch m.mode {
	case modeForm:
		keys = [][2]string{{"tab", "next"}, {"shift+tab", "prev"}, {"enter", "save"}, {"esc", "cancel"}}
		if m.focusIndex == fieldSecret {
			keys = [][2]string{{"enter", "new line"}, {"tab", "next"}, {"ctrl+s", "save"}, {"esc", "cancel"}}
		}
	case modeConfirmDelete:
		keys = [][2]string{{"y", "delete"}, {"any other key", "cancel"}}
	case modeHelp:
		keys = [][2]string{{"any key", "close"}}
		if m.helpScrollable() {
			keys = [][2]string{{"↑/↓ pgup/pgdn", "scroll"}, {"esc", "close"}}
		}
	case modeUnlock:
		keys = [][2]string{{"enter", "unlock"}, {"ctrl+r", revealLabel(m.pwShow)}, {"ctrl+c", "quit"}}
	case modeSetup:
		keys = [][2]string{{"tab", "next field"}, {"enter", "next"}, {"ctrl+r", revealLabel(m.pwShow)}, {"ctrl+c", "quit"}}
		if m.pwField == 1 {
			keys[1][1] = "create"
			if m.migrating {
				keys[1][1] = "migrate"
			}
		}
	default:
		if len(m.list.Items()) == 0 {
			keys = [][2]string{{"n", "new"}, {"?", "help"}, {"q", "quit"}}
			if m.loadErr != "" {
				keys = [][2]string{{"r", "retry"}, {"n", "new"}, {"q", "quit"}}
			}
			break
		}
		keys = [][2]string{
			{"↑/↓", "move"}, {"/", "filter"}, {"enter/c", "copy"}, {"v", "reveal"},
			{"n", "new"}, {"e", "edit"}, {"d", "delete"}, {"s", "sort"}, {"?", "help"}, {"q", "quit"},
		}
		if !m.clipUntil.IsZero() {
			keys = append([][2]string{{"x", "clear clipboard"}}, keys...)
		}
	}

	// On narrow terminals drop the least essential hints until the line fits.
	line := renderKeys(keys)
	for _, drop := range []string{"sort", "move", "filter", "reveal", "clear clipboard"} {
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
	return strings.Join(parts, subtleStyle.Render(" "+g.sep+" "))
}

// revealLabel is the footer text of the show/hide-password key.
func revealLabel(shown bool) string {
	if shown {
		return "hide"
	}
	return "show"
}
