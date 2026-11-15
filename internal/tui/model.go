// Package tui implements the Bubble Tea interface for browsing and managing secrets.
package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"pandora/internal/store"
)

type item struct {
	meta store.SecretMeta
}

func (i item) Title() string       { return i.meta.Title }
func (i item) Description() string { return fmt.Sprintf("%s · %s", i.meta.Category, i.meta.Created) }
func (i item) FilterValue() string { return i.meta.Title + " " + i.meta.Category }

type clearClipboardMsg struct{}

// mode is what the keyboard currently drives.
type mode int

const (
	modeBrowse mode = iota
	modeForm
	modeConfirmDelete
)

type model struct {
	list          list.Model
	width, height int
	statusMsg     string
	statusErr     bool
	showValue     bool
	mode          mode
	inputs        []textinput.Model
	focusIndex    int
	editing       string // title of the secret being edited; empty when adding
	deleting      string // title awaiting delete confirmation
}

// New returns the root model, populated from the stored metadata.
func New() tea.Model {
	delegate := list.NewDefaultDelegate()
	delegate.Styles.NormalTitle = delegate.Styles.NormalTitle.Foreground(text)
	delegate.Styles.NormalDesc = delegate.Styles.NormalDesc.Foreground(subtle)
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(accent).BorderLeftForeground(accent)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(subtle).BorderLeftForeground(accent)

	l := list.New(nil, delegate, 40, 14)
	l.Title = "Secrets"
	l.Styles.Title = lipgloss.NewStyle().Bold(true).Foreground(accent)
	l.SetShowHelp(false)

	inputs := make([]textinput.Model, 3)
	inputs[0] = textinput.New()
	inputs[0].Placeholder = "e.g. Stripe API Key"
	inputs[0].Focus()
	inputs[1] = textinput.New()
	inputs[1].Placeholder = "TOKENS / INFRA"
	inputs[2] = textinput.New()
	inputs[2].Placeholder = secretPlaceholder
	inputs[2].EchoMode = textinput.EchoPassword
	inputs[2].EchoCharacter = '•'

	m := model{list: l, inputs: inputs}
	if err := m.reload(""); err != nil {
		m.statusMsg, m.statusErr = fmt.Sprintf("✗ Could not load secrets: %v", err), true
	}
	return m
}

func (m model) Init() tea.Cmd { return nil }

const (
	secretPlaceholder     = "Secret value"
	secretKeepPlaceholder = "Leave empty to keep current"
)

// reload re-reads the metadata file into the list and selects the secret
// titled selectTitle, or keeps the current selection when it is empty or
// no longer exists. On error the list is left untouched.
func (m *model) reload(selectTitle string) error {
	if selectTitle == "" {
		if sel, ok := m.list.SelectedItem().(item); ok {
			selectTitle = sel.meta.Title
		}
	}

	metas, err := store.LoadMetadata()
	if err != nil {
		return err
	}
	items := make([]list.Item, len(metas))
	for i, meta := range metas {
		items[i] = item{meta: meta}
	}
	m.list.SetItems(items)

	for i, it := range m.list.VisibleItems() {
		if it.(item).meta.Title == selectTitle {
			m.list.Select(i)
			break
		}
	}
	return nil
}

// paneWidths splits the terminal width between the list and detail panes.
// Each pane spends 2 columns on its border; 2 more separate them.
func (m model) paneWidths() (left, right int) {
	left = max(30, (m.width-6)*45/100)
	right = m.width - 6 - left
	return left, right
}

// modalWidth is the outer width of the modal dialogs.
func (m model) modalWidth() int { return min(56, m.width-4) }
