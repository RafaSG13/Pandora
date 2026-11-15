package tui

import (
	"errors"
	"fmt"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"pandora/internal/store"
)

// writeClipboard is the system clipboard writer; a variable so tests never
// touch the developer's real clipboard.
var writeClipboard = clipboard.WriteAll

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.update(msg)
	nm, ok := next.(model)
	if !ok {
		return next, cmd
	}
	nm.syncReveal()
	switch msg.(type) {
	case tea.MouseMsg, tickMsg, spinner.TickMsg, orphanMsg:
		nm.clampScroll(false) // the wheel scrolls independently of the selection
	default:
		nm.clampScroll(true)
	}
	return nm, cmd
}

// syncReveal hides the revealed value as soon as the selection moves away.
func (m *model) syncReveal() {
	if m.revealTitle == "" {
		return
	}
	if sel, ok := m.list.SelectedItem().(item); !ok || sel.meta.Title != m.revealTitle {
		m.revealTitle = ""
	}
}

func (m model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.fitList(len(m.list.Items()))
		inputW := max(10, m.modalWidth()-8-formLabelW-2)
		for i := range m.inputs {
			m.inputs[i].Width = inputW
		}
		return m, nil

	case tickMsg:
		return m.tick()

	case spinner.TickMsg:
		if !m.checking {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case orphanMsg:
		m.orphans, m.checking = msg.missing, false
		m.list.SetDelegate(m.delegate())
		return m, nil

	case tea.KeyMsg:
		m.lastActive = m.nowOr()
		if m.mode == modeLocked {
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			m.mode = m.prevMode
			return m, nil
		}
		m.statusMsg, m.statusErr = "", false

		switch m.mode {
		case modeForm:
			return m.updateForm(msg)
		case modeConfirmDelete:
			return m.updateConfirmDelete(msg)
		case modeHelp:
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			m.mode = modeBrowse
			return m, nil
		}

		if m.list.FilterState() == list.Filtering {
			break
		}
		if s := msg.String(); len(s) == 1 && s[0] >= '0' && s[0] <= '9' {
			return m.pickCategory(int(s[0] - '0')), nil
		}

		sel, hasSel := m.list.SelectedItem().(item)
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "n":
			m.openForm("", store.SecretMeta{})
			return m, nil
		case "e":
			if hasSel {
				m.openForm(sel.meta.Title, sel.meta)
			}
			return m, nil
		case "d":
			if hasSel {
				m.mode, m.deleting = modeConfirmDelete, sel.meta.Title
			}
			return m, nil
		case "r":
			if err := m.reload(""); err != nil {
				m.notify(fmt.Sprintf("✗ Could not reload: %v", err), true)
				return m, nil
			}
			m.notify(fmt.Sprintf("✓ Reloaded (%s)", count(m.total)), false)
			m.checking = m.total > 0
			if !m.checking {
				return m, nil
			}
			return m, tea.Batch(m.spinner.Tick, checkOrphans(m.titles()))
		case "s":
			m.sort = m.sort.next()
			if err := m.reload(""); err != nil {
				m.notify(fmt.Sprintf("✗ Could not reload: %v", err), true)
				return m, nil
			}
			m.notify("Sorted by "+m.sort.String(), false)
			return m, nil
		case "t":
			m.compact = !m.compact
			m.list.SetDelegate(m.delegate())
			m.fitList(len(m.list.Items()))
			return m, nil
		case "v":
			if !hasSel {
				return m, nil
			}
			if m.revealed(sel.meta.Title) {
				m.revealTitle = ""
			} else {
				m.revealTitle, m.revealUntil = sel.meta.Title, m.nowOr().Add(revealTTL)
			}
			return m, nil
		case "x":
			if m.clipUntil.IsZero() {
				m.notify("Nothing to clear", false)
				return m, nil
			}
			m.clearClipboard()
			return m, nil
		case "?":
			m.mode = modeHelp
			return m, nil
		case "enter", "c":
			if hasSel {
				val, err := store.GetSecretValue(sel.meta.Title)
				if err != nil {
					m.notify(fmt.Sprintf("✗ Could not read secret: %v", err), true)
					return m, nil
				}
				if err := writeClipboard(val); err != nil {
					m.notify(fmt.Sprintf("✗ Could not copy: %v", err), true)
					return m, nil
				}
				m.clipUntil = m.nowOr().Add(clipboardTTL)
				m.notify("✓ Copied to clipboard", false)
			}
			return m, nil
		}

	case tea.MouseMsg:
		m.lastActive = m.nowOr()
		if m.mode != modeBrowse || m.list.FilterState() == list.Filtering {
			return m, nil // modals and the filter prompt own the input
		}
		return m.updateMouse(msg)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// tick runs once a second: it expires the reveal, the clipboard and the toast,
// and locks the app after the idle timeout.
func (m model) tick() (tea.Model, tea.Cmd) {
	now := m.nowOr()
	if m.revealTitle != "" && !now.Before(m.revealUntil) {
		m.revealTitle = ""
	}
	if !m.clipUntil.IsZero() && !now.Before(m.clipUntil) {
		m.clearClipboard()
	}
	if m.statusMsg != "" && m.mode != modeForm && !now.Before(m.toastUntil) {
		m.statusMsg, m.statusErr = "", false
	}
	if m.lockAfter > 0 && m.mode != modeLocked && now.Sub(m.lastActive) >= m.lockAfter {
		m.lock()
	}
	return m, tickCmd()
}

// clearClipboard empties the system clipboard and forgets the pending deadline.
func (m *model) clearClipboard() {
	m.clipUntil = zeroTime
	if err := writeClipboard(""); err != nil {
		m.notify(fmt.Sprintf("✗ Could not clear clipboard: %v", err), true)
		return
	}
	m.notify("✓ Clipboard cleared", false)
}

// lock masks everything: the reveal is dropped, a pending clipboard is
// cleared, and the screen shows the lock until a key is pressed.
func (m *model) lock() {
	m.prevMode, m.mode = m.mode, modeLocked
	m.revealTitle = ""
	m.statusMsg = ""
	if !m.clipUntil.IsZero() {
		m.clipUntil = zeroTime
		_ = writeClipboard("")
	}
}

// pickCategory applies the n-th category filter (1-based); 0 clears it and
// repeating the active one toggles it off.
func (m model) pickCategory(n int) model {
	switch {
	case n == 0:
		m.category = ""
	case n > len(m.categories):
		m.notify(fmt.Sprintf("No category %d", n), false)
		return m
	case strings.EqualFold(m.categories[n-1], m.category):
		m.category = ""
	default:
		m.category = m.categories[n-1]
	}
	m.scroll = 0
	if err := m.reload(""); err != nil {
		m.notify(fmt.Sprintf("✗ Could not reload: %v", err), true)
		return m
	}
	if m.category == "" {
		m.notify("All categories", false)
	} else {
		m.notify("Category: "+m.category, false)
	}
	return m
}

// openForm shows the form. editTitle is the secret being edited, or "" to add;
// meta prefills the fields. The secret field always starts empty.
func (m *model) openForm(editTitle string, meta store.SecretMeta) {
	m.mode, m.editing = modeForm, editTitle
	m.inputs[fieldName].SetValue(meta.Title)
	m.inputs[fieldCategory].SetValue(meta.Category)
	m.inputs[fieldCategory].SetSuggestions(m.categories)
	m.inputs[fieldSecret].SetValue("")
	m.inputs[fieldSecret].EchoMode = textinput.EchoPassword
	m.inputs[fieldSecret].Placeholder = secretPlaceholder
	if editTitle != "" {
		m.inputs[fieldSecret].Placeholder = secretKeepPlaceholder
	}
	m.inputs[fieldExpires].SetValue(meta.Expires)
	m.inputs[fieldNotes].SetValue(meta.Notes)
	m.setFocus(fieldName)
}

// updateForm handles key presses while the add/edit form is open.
func (m model) updateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	last := len(m.inputs) - 1

	switch msg.String() {
	case "esc":
		m.mode = modeBrowse
		return m, nil
	case "up", "shift+tab":
		m.setFocus((m.focusIndex - 1 + len(m.inputs)) % len(m.inputs))
		return m, nil
	case "down", "tab":
		m.setFocus((m.focusIndex + 1) % len(m.inputs))
		return m, nil
	case "enter":
		if m.focusIndex < last {
			m.setFocus(m.focusIndex + 1)
			return m, nil
		}
		return m.saveForm()
	case "ctrl+s":
		return m.saveForm()
	case "ctrl+g":
		val, err := generateSecret(genCharsets[m.genSet], genLengths[m.genLen])
		if err != nil {
			m.notify(fmt.Sprintf("✗ %v", err), true)
			return m, nil
		}
		m.inputs[fieldSecret].SetValue(val)
		m.inputs[fieldSecret].EchoMode = textinput.EchoNormal // show what was generated
		m.setFocus(fieldSecret)
		return m, nil
	case "ctrl+l":
		m.genLen = (m.genLen + 1) % len(genLengths)
		return m, nil
	case "ctrl+t":
		m.genSet = (m.genSet + 1) % len(genCharsets)
		return m, nil
	}

	var cmd tea.Cmd
	m.inputs[m.focusIndex], cmd = m.inputs[m.focusIndex].Update(msg)
	return m, cmd
}

// saveForm validates the form and persists the new or edited secret.
// On failure the form stays open so nothing typed is lost.
func (m model) saveForm() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(m.inputs[fieldName].Value())
	val := m.inputs[fieldSecret].Value()
	adding := m.editing == ""
	if name == "" || (adding && val == "") {
		m.notify("✗ Name and secret are required", true)
		if !adding {
			m.statusMsg = "✗ Name is required"
		}
		return m, nil
	}
	expires := strings.TrimSpace(m.inputs[fieldExpires].Value())
	if _, err := store.ParseDate(expires); err != nil {
		m.notify("✗ Expires must be a date like 2026-12-31", true)
		return m, nil
	}

	meta := store.SecretMeta{
		Title:    name,
		Category: strings.TrimSpace(m.inputs[fieldCategory].Value()),
		Created:  m.nowOr().Format(store.DateLayout),
		Expires:  expires,
		Notes:    strings.TrimSpace(m.inputs[fieldNotes].Value()),
	}

	var err error
	if adding {
		err = store.SaveSecret(meta, val)
	} else {
		var newValue *string
		if val != "" {
			newValue = &val
		}
		err = store.UpdateSecret(m.editing, meta, newValue)
	}
	if err != nil {
		switch {
		case errors.Is(err, store.ErrExists):
			m.notify(fmt.Sprintf("✗ %q already exists", name), true)
		case adding:
			m.notify(fmt.Sprintf("✗ Could not save: %v", err), true)
		default:
			m.notify(fmt.Sprintf("✗ Could not update: %v", err), true)
		}
		return m, nil
	}

	m.mode = modeBrowse
	delete(m.orphans, name) // a stored value means it is no longer an orphan
	if err := m.reload(name); err != nil {
		m.notify(fmt.Sprintf("✗ Saved, but could not reload: %v", err), true)
		return m, nil
	}
	m.notify("✓ Saved to OS keyring", false)
	if !adding {
		m.statusMsg = "✓ Updated"
	}
	return m, nil
}

// updateConfirmDelete handles the delete confirmation: y deletes, anything else cancels.
func (m model) updateConfirmDelete(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	title := m.deleting
	m.mode, m.deleting = modeBrowse, ""
	if msg.String() != "y" {
		return m, nil
	}

	if err := store.DeleteSecret(title); err != nil {
		m.notify(fmt.Sprintf("✗ Could not delete: %v", err), true)
		return m, nil
	}
	if err := m.reload(""); err != nil {
		m.notify(fmt.Sprintf("✗ Deleted, but could not reload: %v", err), true)
		return m, nil
	}
	m.notify(fmt.Sprintf("✓ Deleted %q", title), false)
	return m, nil
}

// updateMouse handles the wheel (scrolls the list, leaving the selection) and
// left clicks (select the item under the pointer) while browsing.
func (m model) updateMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Action != tea.MouseActionPress {
		return m, nil
	}
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		m.scroll--
	case tea.MouseButtonWheelDown:
		m.scroll++
	case tea.MouseButtonLeft:
		leftW, _ := m.paneWidths()
		if msg.X < 1 || msg.X > leftW || msg.Y < listTopRow {
			return m, nil
		}
		rows := m.rowsPer()
		row, offset := (msg.Y-listTopRow)/rows, (msg.Y-listTopRow)%rows
		if offset >= m.textLines() { // separator line between items
			return m, nil
		}
		idx := m.scroll + row
		if row >= m.itemsInView() || idx >= len(m.list.VisibleItems()) {
			return m, nil
		}
		m.list.Select(idx)
	}
	return m, nil
}

// setFocus focuses input i and blurs the rest.
func (m *model) setFocus(i int) {
	m.focusIndex = i
	for j := range m.inputs {
		if j == i {
			m.inputs[j].Focus()
			m.inputs[j].TextStyle = textStyle
		} else {
			m.inputs[j].Blur()
			m.inputs[j].TextStyle = subtleStyle
		}
	}
}

func count(n int) string {
	if n == 1 {
		return "1 secret"
	}
	return fmt.Sprintf("%d secrets", n)
}
