package tui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"pandora/internal/store"
)

const (
	clipboardSeconds = 15
	revealTTL        = 10 * time.Second
)

// writeClipboard is a variable so tests can replace the system clipboard.
var writeClipboard = clipboard.WriteAll

// clipboardTickMsg fires once a second while a copied value is on the clipboard.
// gen identifies the copy it belongs to; ticks from superseded copies are ignored.
type clipboardTickMsg struct{ gen int }

// hideRevealMsg hides a revealed value after revealTTL, unless gen is stale.
type hideRevealMsg struct{ gen int }

func tickClipboard(gen int) tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return clipboardTickMsg{gen} })
}

// Update applies msg, then hides the revealed value if the selection moved off it.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.update(msg)
	nm := next.(model)
	if nm.revealed != "" {
		if sel, ok := nm.list.SelectedItem().(item); !ok || sel.meta.Title != nm.revealed {
			nm.revealed = ""
		}
	}
	return nm, cmd
}

func (m model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		left, _ := m.paneWidths()
		m.list.SetSize(left-4, max(3, m.height-8))
		inputW := max(10, m.modalWidth()-6-3)
		for i := range m.inputs {
			m.inputs[i].Width = inputW
		}
		return m, nil

	case tea.KeyMsg:
		m.statusMsg, m.statusErr = "", false

		switch m.mode {
		case modeForm:
			return m.updateForm(msg)
		case modeConfirmDelete:
			return m.updateConfirmDelete(msg)
		}

		if m.list.FilterState() == list.Filtering {
			break
		}

		sel, hasSel := m.list.SelectedItem().(item)
		switch msg.String() {
		case "ctrl+c", "q":
			if m.clipLeft > 0 {
				_ = writeClipboard("") // the countdown dies with the process
			}
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
				m.statusMsg, m.statusErr = fmt.Sprintf("✗ Could not reload: %v", err), true
			} else {
				m.statusMsg = fmt.Sprintf("✓ Reloaded (%s)", count(len(m.list.Items())))
			}
			return m, nil
		case "v":
			if !hasSel {
				return m, nil
			}
			m.revealGen++
			if m.revealed == sel.meta.Title {
				m.revealed = ""
				return m, nil
			}
			m.revealed = sel.meta.Title
			gen := m.revealGen
			return m, tea.Tick(revealTTL, func(time.Time) tea.Msg { return hideRevealMsg{gen} })
		case "x":
			if m.clipLeft > 0 {
				m.copyGen++ // cancels the pending countdown
				m.clipLeft = 0
				m.statusMsg = "✓ Clipboard cleared"
				if err := writeClipboard(""); err != nil {
					m.statusMsg, m.statusErr = fmt.Sprintf("✗ Could not clear clipboard: %v", err), true
				}
			}
			return m, nil
		case "enter", "c":
			if hasSel {
				val, err := store.GetSecretValue(sel.meta.Title)
				if err != nil {
					m.statusMsg, m.statusErr = fmt.Sprintf("✗ Could not read secret: %v", err), true
					return m, nil
				}
				if err := writeClipboard(val); err != nil {
					m.statusMsg, m.statusErr = fmt.Sprintf("✗ Could not copy: %v", err), true
					return m, nil
				}
				m.copyGen++ // a newer copy supersedes any pending countdown
				m.clipLeft = clipboardSeconds
				return m, tickClipboard(m.copyGen)
			}
		}

	case clipboardTickMsg:
		if msg.gen != m.copyGen || m.clipLeft == 0 {
			return m, nil // superseded by a newer copy, or already cleared
		}
		m.clipLeft--
		if m.clipLeft > 0 {
			return m, tickClipboard(m.copyGen)
		}
		m.statusMsg, m.statusErr = "✓ Clipboard cleared", false
		if err := writeClipboard(""); err != nil {
			m.statusMsg, m.statusErr = fmt.Sprintf("✗ Could not clear clipboard: %v", err), true
		}
		return m, nil

	case hideRevealMsg:
		if msg.gen == m.revealGen {
			m.revealed = ""
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// openForm shows the form. editTitle is the secret being edited, or "" to add;
// meta prefills name and category. The secret field always starts empty.
func (m *model) openForm(editTitle string, meta store.SecretMeta) {
	m.mode, m.editing = modeForm, editTitle
	m.inputs[0].SetValue(meta.Title)
	m.inputs[1].SetValue(meta.Category)
	m.inputs[2].SetValue("")
	m.inputs[2].Placeholder = secretPlaceholder
	if editTitle != "" {
		m.inputs[2].Placeholder = secretKeepPlaceholder
	}
	m.setFocus(0)
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
	}

	var cmd tea.Cmd
	m.inputs[m.focusIndex], cmd = m.inputs[m.focusIndex].Update(msg)
	return m, cmd
}

// saveForm validates the form and persists the new or edited secret.
// On failure the form stays open so nothing typed is lost.
func (m model) saveForm() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(m.inputs[0].Value())
	val := m.inputs[2].Value()
	adding := m.editing == ""
	if name == "" || (adding && val == "") {
		m.statusMsg, m.statusErr = "✗ Name and secret are required", true
		if !adding {
			m.statusMsg = "✗ Name is required"
		}
		return m, nil
	}

	meta := store.SecretMeta{
		Title:    name,
		Category: strings.TrimSpace(m.inputs[1].Value()),
		Created:  time.Now().Format("2006-01-02"),
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
		m.statusErr = true
		switch {
		case errors.Is(err, store.ErrExists):
			m.statusMsg = fmt.Sprintf("✗ %q already exists", name)
		case adding:
			m.statusMsg = fmt.Sprintf("✗ Could not save: %v", err)
		default:
			m.statusMsg = fmt.Sprintf("✗ Could not update: %v", err)
		}
		return m, nil
	}

	m.mode = modeBrowse
	if err := m.reload(name); err != nil {
		m.statusMsg, m.statusErr = fmt.Sprintf("✗ Saved, but could not reload: %v", err), true
		return m, nil
	}
	m.statusMsg = "✓ Saved to OS keyring"
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
		m.statusMsg, m.statusErr = fmt.Sprintf("✗ Could not delete: %v", err), true
		return m, nil
	}
	if err := m.reload(""); err != nil {
		m.statusMsg, m.statusErr = fmt.Sprintf("✗ Deleted, but could not reload: %v", err), true
		return m, nil
	}
	m.statusMsg = fmt.Sprintf("✓ Deleted %q", title)
	return m, nil
}

// setFocus focuses input i and blurs the rest.
func (m *model) setFocus(i int) {
	m.focusIndex = i
	for j := range m.inputs {
		if j == i {
			m.inputs[j].Focus()
		} else {
			m.inputs[j].Blur()
		}
	}
}

func count(n int) string {
	if n == 1 {
		return "1 secret"
	}
	return fmt.Sprintf("%d secrets", n)
}
