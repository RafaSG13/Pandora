package tui

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/RafaSG13/Pandora/internal/gen"
	"github.com/RafaSG13/Pandora/internal/secretclip"
	"github.com/RafaSG13/Pandora/internal/store"
)

// writeClipboard and readClipboard are the plain system clipboard accessors
// (reads and clears); variables so tests never touch the developer's real
// clipboard. writeSecretClipboard is the concealed copy for secret values
// (see internal/secretclip). scheduleClipboardClear starts the detached helper
// that clears a copied secret after its deadline even if the TUI has quit.
var (
	writeClipboard       = clipboard.WriteAll
	readClipboard        = clipboard.ReadAll
	writeSecretClipboard = secretclip.Write

	scheduleClipboardClear = secretclip.ClearLater
)

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.update(msg)
	nm, ok := next.(model)
	if !ok {
		return next, cmd
	}
	nm.syncReveal()
	if nm.auditErr != nil { // last, so it is not hidden by the action's own toast
		nm.notify(fmt.Sprintf("✗ Could not write the audit log: %v", nm.auditErr), true)
		nm.auditErr = nil
	}
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
		m.hideReveal()
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
			m.inputs[i].SetCursor(m.inputs[i].Position()) // re-scroll: the cursor may now be out of view
		}
		m.secret.SetWidth(inputW)
		m.secret.SetHeight(m.secretRows())
		// The editor keeps its scroll offset across a resize and only follows the
		// cursor in Update, from the lines of its last View: a shorter box could
		// otherwise show a blank row, with the cursor outside it, until a key.
		_ = m.secret.View()
		m.secret, _ = m.secret.Update(nil)
		authW := max(10, m.modalWidth()-8-15) // label, marker, brackets and cursor take 15 columns
		for _, in := range []*textinput.Model{&m.pwInput, &m.pwConfirm} {
			in.Width = authW
			in.SetCursor(in.Position())
		}
		return m, nil

	case tickMsg:
		return m.tick()

	case spinner.TickMsg:
		if !m.checking && !m.unlocking {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case orphanMsg:
		if m.locked() {
			return m, nil // the answer arrived after the vault was locked again
		}
		m.orphans, m.checking = msg.missing, false
		m.list.SetDelegate(m.delegate())
		return m, nil

	case unlockedMsg:
		return m.unlocked(msg)

	case tea.KeyMsg:
		m.lastActive = m.nowOr()
		if m.locked() {
			return m.updateAuth(msg)
		}
		m.statusMsg, m.statusErr = "", false

		switch m.mode {
		case modeForm:
			return m.updateForm(msg)
		case modeConfirmDelete:
			return m.updateConfirmDelete(msg)
		case modeHelp:
			return m.updateHelp(msg)
		case modeHistory:
			return m.updateHistory(msg)
		case modeView:
			return m.updateViewer(msg)
		case modePicker:
			return m.updatePicker(msg)
		}

		if m.list.FilterState() == list.Filtering {
			break
		}
		if s := msg.String(); len(s) == 1 && s[0] >= '0' && s[0] <= '9' {
			return m.pickCategory(int(s[0] - '0')), nil
		}

		sel, hasSel := m.list.SelectedItem().(item)
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if s := msg.String(); s == "pgup" || s == "pgdown" {
			// bubbles pages by its own page size, but the list is one page tall
			// (see fitList): move by what the pane shows instead.
			step := m.itemsInView()
			if s == "pgup" {
				step = -step
			}
			if n := len(m.list.VisibleItems()); n > 0 {
				m.list.Select(max(0, min(n-1, m.list.Index()+step)))
			}
			return m, nil
		}
		switch m.keys.action(msg.String()) {
		case "quit":
			return m, tea.Quit
		case "new":
			m.openForm("", store.SecretMeta{})
			return m, nil
		case "edit":
			if hasSel {
				m.openForm(sel.meta.Title, sel.meta)
			}
			return m, nil
		case "delete":
			if hasSel {
				m.mode, m.deleting = modeConfirmDelete, sel.meta.Title
			}
			return m, nil
		case "reload":
			if err := m.reload(""); err != nil {
				m.notify(fmt.Sprintf("✗ Could not reload: %v", err), true)
				return m, nil
			}
			m.notify(fmt.Sprintf("✓ Reloaded (%s)", count(m.total)), false)
			return m, m.startOrphanCheck()
		case "sort":
			m.sort = m.sort.next()
			if err := m.reload(""); err != nil {
				m.notify(fmt.Sprintf("✗ Could not reload: %v", err), true)
				return m, nil
			}
			m.notify("Sorted by "+m.sort.String(), false)
			return m, nil
		case "tag":
			return m.cycleTag(), nil
		case "filters":
			m.openPicker()
			return m, nil
		case "compact":
			m.compact = !m.compact
			m.list.SetDelegate(m.delegate())
			m.fitList(len(m.list.Items()))
			return m, nil
		case "history":
			if hasSel {
				m.openHistory(sel.meta.Title)
			}
			return m, nil
		case "reveal":
			if hasSel {
				m.toggleReveal(sel.meta.Title)
			}
			return m, nil
		case "view":
			if hasSel {
				m.openViewer(sel.meta.Title)
			}
			return m, nil
		case "lock":
			m.lock()
			return m, nil
		case "clear_clipboard":
			if m.clipUntil.IsZero() {
				m.notify("Nothing to clear", false)
				return m, nil
			}
			m.clearClipboard(true)
			return m, nil
		case "help":
			m.mode, m.helpScroll = modeHelp, 0
			return m, nil
		case "copy":
			if hasSel {
				m.copySelected(sel.meta.Title)
			}
			return m, nil
		case "copy_username":
			if hasSel {
				m.copyUsername(sel.meta)
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
	if m.hist.shown && !now.Before(m.hist.until) {
		m.hist.hideValue()
	}
	if m.revealTitle != "" && !now.Before(m.revealUntil) {
		m.hideReveal()
	}
	if m.mode == modeView && m.revealTitle == "" { // the viewer closes when the value hides
		m.mode, m.viewTop = modeBrowse, 0
	}
	if !m.clipUntil.IsZero() && !now.Before(m.clipUntil) {
		m.clearClipboard(false)
	}
	if m.statusMsg != "" && m.mode != modeForm && !now.Before(m.toastUntil) {
		m.statusMsg, m.statusErr = "", false
	}
	if m.lockAfter > 0 && !m.locked() && now.Sub(m.lastActive) >= m.lockAfter {
		m.lock()
	}
	if !m.locked() && !store.IsUnlocked() { // the store dropped the session during an operation
		m.lockForKeyChange()
	}
	cmds := []tea.Cmd{tickCmd()}
	if c := m.reloadIfChangedElsewhere(); c != nil {
		cmds = append(cmds, c)
	}
	return m, tea.Batch(cmds...)
}

// reloadIfChangedElsewhere refreshes the list when another process (the CLI, a
// second instance) has rewritten the vault since the last load. It only acts
// in browse mode: a form or a delete confirmation is never disturbed, and the
// change is picked up on the first tick after it closes. The selection is kept
// by title; own writes already reload, which refreshes the stamp, so they do
// not trigger this. A vault that cannot be stat-ed is ignored: the list keeps
// showing what was loaded.
func (m *model) reloadIfChangedElsewhere() tea.Cmd {
	if m.mode != modeBrowse || m.unlocking {
		return nil
	}
	s, err := store.VaultStamp()
	if err != nil || s == m.stamp {
		return nil
	}
	if err := m.reload(""); err != nil {
		if errors.Is(err, store.ErrKeyChanged) {
			m.lockForKeyChange()
			return nil
		}
		m.notify(fmt.Sprintf("✗ Could not reload: %v", err), true)
		return nil
	}
	m.notify(fmt.Sprintf("✓ Reloaded — changed elsewhere (%s)", count(m.total)), false)
	return m.startOrphanCheck()
}

// keyChangedMsg is shown on the unlock screen when the data key was rotated by
// another process. It is short enough for the 60-column unlock modal.
const keyChangedMsg = "Key changed elsewhere: unlock again"

// lockForKeyChange locks the app and tells the user why. lock resets
// unlockErr, so the message is set after it.
func (m *model) lockForKeyChange() {
	m.lock()
	m.unlockErr = keyChangedMsg
}

// startOrphanCheck begins the background check for entries whose keyring value
// is missing; it returns nil when there is nothing to check.
func (m *model) startOrphanCheck() tea.Cmd {
	m.checking = m.total > 0
	if !m.checking {
		return nil
	}
	return tea.Batch(m.spinner.Tick, checkOrphans(m.titles()))
}

// wipeClipboard ends the pending copy: the deadline and the remembered hash
// are dropped, and the clipboard is emptied only if it still holds what
// Pandora copied. When it cannot be read it is cleared anyway. It reports
// whether the clipboard was left alone because its content changed, and any
// error from clearing it.
func (m *model) wipeClipboard() (changed bool, err error) {
	want := m.clipHash
	m.clipUntil, m.clipHash = zeroTime, [sha256.Size]byte{}
	cleared, err := secretclip.ClearIfHoldsUsing(readClipboard, writeClipboard, want)
	return !cleared && err == nil, err
}

// clearClipboard clears the clipboard if it still holds the copied secret and
// forgets the pending deadline. manual (the x key) also reports when the
// clipboard was left alone; the automatic deadline stays silent about that.
func (m *model) clearClipboard(manual bool) {
	changed, err := m.wipeClipboard()
	switch {
	case err != nil:
		m.notify(fmt.Sprintf("✗ Could not clear clipboard: %v", err), true)
	case changed:
		if manual {
			m.notify("Clipboard changed since you copied — left untouched", false)
		}
	default:
		m.notify("✓ Clipboard cleared", false)
	}
}

// lock masks everything: the data key is dropped, the list, filters and any
// open form are emptied, the reveal and a pending clipboard are cleared, and
// the master-password screen is shown. Nothing but a correct password brings
// the secrets back; an unsaved form is discarded.
func (m *model) lock() {
	m.hideReveal()
	m.statusMsg = ""
	if !m.clipUntil.IsZero() {
		_, _ = m.wipeClipboard()
	}
	store.Lock()
	m.list.ResetFilter()
	m.list.SetItems(nil)
	m.orphans, m.checking = nil, false
	m.total, m.expiring, m.loadErr = 0, 0, ""
	m.viewTop, m.picker = 0, pickerState{}
	m.categories, m.category, m.scroll = nil, "", 0
	m.tags, m.tag = nil, ""
	m.versions, m.hist = nil, historyState{}
	m.editing, m.deleting = "", ""
	m.formInitial, m.discardKey = [numFields]string{}, ""
	for i := range m.inputs {
		m.inputs[i].SetValue("")
		m.inputs[i].SetSuggestions(nil)
	}
	m.secret.Reset()
	m.secretShown = false
	m.pwInput.SetValue("")
	m.pwConfirm.SetValue("")
	m.unlocking, m.unlockErr = false, ""
	m.mode = modeUnlock
	m.lastClickTitle, m.lastClickAt = "", time.Time{}
	m.setReveal(false)
	m.focusPassword(0)
}

// copySelected puts title's value on the clipboard (concealed) and arms the
// clipboard timer. Keyboard and double-click share it.
func (m *model) copySelected(title string) {
	val, err := store.GetSecretValue(title)
	if err != nil {
		m.notify(fmt.Sprintf("✗ Could not read secret: %v", err), true)
		return
	}
	if err := writeSecretClipboard(val); err != nil {
		m.notify(fmt.Sprintf("✗ Could not copy: %v", err), true)
		return
	}
	m.clipUntil = m.nowOr().Add(m.cfg.ClipboardTTL)
	m.clipHash = sha256.Sum256([]byte(val))
	// The helper outlives the TUI; its failure is harmless because the
	// in-process deadline still clears the clipboard while the TUI runs.
	_ = scheduleClipboardClear(val, m.cfg.ClipboardTTL)
	m.notify("✓ Copied to clipboard", false)
	m.audit("copy", title)
}

// copyUsername puts meta's username on the plain clipboard: it is metadata,
// not a secret value, so it is neither concealed, auto-cleared nor audited.
// A pending secret-clear is dropped because the secret is no longer on the
// clipboard.
func (m *model) copyUsername(meta store.SecretMeta) {
	if meta.Username == "" {
		m.notify("No username for this secret", false)
		return
	}
	if err := writeClipboard(meta.Username); err != nil {
		m.notify(fmt.Sprintf("✗ Could not copy: %v", err), true)
		return
	}
	m.clipUntil, m.clipHash = zeroTime, [sha256.Size]byte{}
	m.notify("✓ Username copied", false)
}

// unlockedMsg reports the result of an unlock or first-run setup, which runs
// off the UI thread because the key derivation takes a moment. warning is a
// non-fatal problem after a successful migration.
type unlockedMsg struct {
	err     error
	warning string
}

func unlockCmd(password string, setup, migrating bool) tea.Cmd {
	return func() tea.Msg {
		var err error
		switch {
		case !setup:
			err = store.Unlock(password)
		case migrating:
			err = store.MigrateLegacy(password)
		default:
			err = store.Create(password)
		}
		var cleanup *store.CleanupError
		if errors.As(err, &cleanup) {
			return unlockedMsg{warning: cleanup.Error()}
		}
		return unlockedMsg{err: err}
	}
}

// updateAuth handles keys on the unlock and setup screens.
func (m model) updateAuth(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "ctrl+c" {
		return m, tea.Quit
	}
	if m.unlocking {
		return m, nil
	}
	switch key {
	case "ctrl+r":
		m.setReveal(!m.pwShow)
		return m, nil
	case "esc":
		return m, nil
	case "tab", "shift+tab", "up", "down":
		if m.mode == modeSetup {
			m.focusPassword(1 - m.pwField)
		}
		return m, nil
	case "enter":
		if m.mode == modeSetup && m.pwField == 0 {
			m.focusPassword(1)
			return m, nil
		}
		return m.submitAuth()
	}
	m.unlockErr = ""
	if msg.Paste {
		// A password copied from a manager or file often ends in a newline.
		msg.Runes = []rune(strings.TrimRight(string(msg.Runes), "\r\n"))
	}
	var cmd tea.Cmd
	if m.pwField == 0 {
		m.pwInput, cmd = m.pwInput.Update(msg)
	} else {
		m.pwConfirm, cmd = m.pwConfirm.Update(msg)
	}
	return m, cmd
}

// submitAuth validates the entered password and starts the unlock.
func (m model) submitAuth() (tea.Model, tea.Cmd) {
	pw := m.pwInput.Value()
	setup := m.mode == modeSetup
	if pw == "" && !setup {
		return m, nil
	}
	if setup {
		switch {
		case utf8.RuneCountInString(pw) < store.MinPasswordLen:
			m.unlockErr = fmt.Sprintf("✗ Use at least %d characters. Type a new one", store.MinPasswordLen)
			m.resetPasswordFields()
			return m, nil
		case pw != m.pwConfirm.Value():
			m.unlockErr = fmt.Sprintf("✗ Passwords do not match: %d vs %d chars", utf8.RuneCountInString(pw), utf8.RuneCountInString(m.pwConfirm.Value()))
			m.resetPasswordFields()
			return m, nil
		}
	}
	m.unlocking, m.unlockErr = true, ""
	return m, tea.Batch(m.spinner.Tick, unlockCmd(pw, setup, setup && m.migrating))
}

// resetPasswordFields empties both password inputs and returns to the first,
// so a failed setup starts over instead of leaving half the answer behind.
func (m *model) resetPasswordFields() {
	m.pwInput.SetValue("")
	m.pwConfirm.SetValue("")
	m.focusPassword(0)
}

// unlocked applies the result of unlockCmd.
func (m model) unlocked(msg unlockedMsg) (tea.Model, tea.Cmd) {
	m.unlocking = false
	m.pwInput.SetValue("")
	m.pwConfirm.SetValue("")
	m.focusPassword(0)
	m.setReveal(false)
	if msg.err != nil {
		if errors.Is(msg.err, store.ErrWrongPassword) {
			m.unlockErr = "✗ Wrong password"
		} else {
			m.unlockErr = "✗ " + msg.err.Error()
		}
		return m, nil
	}
	m.mode, m.migrating = modeBrowse, false
	m.lastActive = m.nowOr()
	if err := m.reload(""); err != nil {
		m.notify(fmt.Sprintf("✗ Could not load secrets: %v", err), true)
	}
	if msg.warning != "" {
		m.notify("✗ Migrated, but "+msg.warning, true)
	} else if m.cfgWarn != "" && m.loadErr == "" {
		m.notify(m.cfgWarn, true)
	}
	m.cfgWarn = ""
	m.checking = m.total > 0
	if !m.checking {
		return m, nil
	}
	return m, tea.Batch(m.spinner.Tick, checkOrphans(m.titles()))
}

// updateHelp handles keys in the help overlay. When the overlay does not fit
// the terminal the scroll keys move it; every other key closes it.
func (m model) updateHelp(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "ctrl+c" {
		return m, tea.Quit
	}
	_, view := m.helpLayout()
	if total := len(m.helpLines()); total > view {
		page := max(1, view-1)
		switch key {
		case "up", "k":
			m.helpScroll--
		case "down", "j":
			m.helpScroll++
		case "pgup":
			m.helpScroll -= page
		case "pgdown":
			m.helpScroll += page
		case "home":
			m.helpScroll = 0
		case "end":
			m.helpScroll = total
		default:
			m.mode = modeBrowse
			return m, nil
		}
		m.helpScroll = clampHelpScroll(m.helpScroll, total, view)
		return m, nil
	}
	m.mode = modeBrowse
	return m, nil
}

// pickCategory applies the n-th category filter (1-based); 0 clears it and
// repeating the active one toggles it off.
func (m model) pickCategory(n int) model {
	newCategory := ""
	switch {
	case n == 0:
	case n > len(m.categories):
		m.notify(fmt.Sprintf("No category %d", n), false)
		return m
	case strings.EqualFold(m.categories[n-1], m.category):
	default:
		newCategory = m.categories[n-1]
	}
	return m.applyFilter(newCategory, m.tag)
}

// cycleTag moves the tag filter to the next tag in use: every tag in turn,
// then back to all secrets.
func (m model) cycleTag() model {
	if len(m.tags) == 0 {
		m.notify("No tags yet — add some in the form", false)
		return m
	}
	next := ""
	switch i := slices.Index(m.tags, m.tag); {
	case m.tag == "":
		next = m.tags[0]
	case i >= 0 && i+1 < len(m.tags):
		next = m.tags[i+1]
	}
	return m.applyFilter(m.category, next)
}

// applyFilter sets the category and tag filters, scrolls to the top, reloads and toasts what is shown.
func (m model) applyFilter(category, tag string) model {
	m.category, m.tag, m.scroll = category, tag, 0
	if err := m.reload(""); err != nil {
		m.notify(fmt.Sprintf("✗ Could not reload: %v", err), true)
		return m
	}
	m.notify("Showing "+describeFilter(category, tag), false)
	return m
}

// describeFilter names what the category and tag filters show.
func describeFilter(category, tag string) string {
	switch {
	case category != "" && tag != "":
		return "category " + category + ", #" + tag
	case category != "":
		return "category " + category
	case tag != "":
		return "#" + tag
	}
	return "all secrets"
}

// openForm shows the form. editTitle is the secret being edited, or "" to add;
// meta prefills the fields. The secret field always starts empty and masked.
func (m *model) openForm(editTitle string, meta store.SecretMeta) {
	m.mode, m.editing = modeForm, editTitle
	m.inputs[fieldName].SetValue(meta.Title)
	m.inputs[fieldCategory].SetValue(meta.Category)
	m.inputs[fieldUsername].SetValue(meta.Username)
	m.inputs[fieldCategory].SetSuggestions(m.categories)
	m.secret.Reset()
	m.secretShown = false
	m.secret.Placeholder = secretPlaceholder
	if editTitle != "" {
		m.secret.Placeholder = secretKeepPlaceholder
	}
	m.inputs[fieldExpires].SetValue(meta.Expires)
	m.inputs[fieldNotes].SetValue(meta.Notes)
	m.inputs[fieldTags].SetValue(strings.Join(meta.Tags, ", "))
	m.setFocus(fieldName)
	for i := range m.inputs {
		m.formInitial[i] = m.inputs[i].Value()
	}
	m.formInitial[fieldSecret], m.discardKey = "", ""
}

// formDirty reports whether the open form differs from what it opened with.
func (m model) formDirty() bool {
	for i := range m.inputs {
		if i != fieldSecret && m.inputs[i].Value() != m.formInitial[i] {
			return true
		}
	}
	return m.secret.Value() != ""
}

// updateForm handles key presses while the add/edit form is open. In the
// secret field enter inserts a newline and up/down move through its lines;
// tab and shift+tab move between fields, ctrl+s saves, ctrl+r shows or hides
// the text.
func (m model) updateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	last := len(m.inputs) - 1
	armed := m.discardKey
	m.discardKey = ""
	inSecret := m.focusIndex == fieldSecret

	switch msg.String() {
	case "ctrl+c":
		if m.formDirty() && armed != "ctrl+c" {
			m.discardKey = "ctrl+c"
			m.notify("Unsaved changes: ctrl+c again quits", true)
			return m, nil
		}
		return m, tea.Quit // like every other screen; esc is the way to cancel
	case "esc":
		if m.formDirty() && armed != "esc" {
			m.discardKey = "esc"
			m.notify("Unsaved changes: esc again discards", true)
			return m, nil
		}
		m.mode = modeBrowse
		return m, nil
	case "up":
		if inSecret && m.secret.Line() > 0 {
			break
		}
		m.setFocus((m.focusIndex - 1 + len(m.inputs)) % len(m.inputs))
		return m, nil
	case "shift+tab":
		m.setFocus((m.focusIndex - 1 + len(m.inputs)) % len(m.inputs))
		return m, nil
	case "down":
		if inSecret && m.secret.Line() < strings.Count(m.secret.Value(), "\n") {
			break
		}
		m.setFocus((m.focusIndex + 1) % len(m.inputs))
		return m, nil
	case "tab":
		m.setFocus((m.focusIndex + 1) % len(m.inputs))
		return m, nil
	case "enter":
		if inSecret {
			break // a newline in the value
		}
		if m.focusIndex < last {
			m.setFocus(m.focusIndex + 1)
			return m, nil
		}
		return m.saveForm()
	case "ctrl+s":
		return m.saveForm()
	case "ctrl+r":
		m.secretShown = !m.secretShown
		return m, nil
	case "ctrl+g":
		val, err := gen.Generate(gen.Charsets[m.genSet], gen.Lengths[m.genLen])
		if err != nil {
			m.notify(fmt.Sprintf("✗ %v", err), true)
			return m, nil
		}
		m.secret.SetValue(val)
		m.secretShown = true // show what was generated
		m.setFocus(fieldSecret)
		return m, nil
	case "ctrl+l":
		m.genLen = (m.genLen + 1) % len(gen.Lengths)
		return m, nil
	case "ctrl+t":
		m.genSet = (m.genSet + 1) % len(gen.Charsets)
		return m, nil
	}

	var cmd tea.Cmd
	if inSecret {
		m.secret, cmd = m.secret.Update(msg)
		// textarea scrolls to the cursor during Update using the lines its last
		// View rendered, so render once and update again to follow a paste at once.
		_ = m.secret.View()
		m.secret, _ = m.secret.Update(nil)
	} else {
		m.inputs[m.focusIndex], cmd = m.inputs[m.focusIndex].Update(msg)
	}
	return m, cmd
}

// saveForm validates the form and persists the new or edited secret.
// On failure the form stays open so nothing typed is lost.
func (m model) saveForm() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(m.inputs[fieldName].Value())
	val := m.secret.Value()
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
	tags, err := store.ParseTags(m.inputs[fieldTags].Value())
	if err != nil {
		m.notify(fmt.Sprintf("✗ Tags: %v", err), true)
		return m, nil
	}

	meta := store.SecretMeta{
		Title:    name,
		Category: strings.TrimSpace(m.inputs[fieldCategory].Value()),
		Username: strings.TrimSpace(m.inputs[fieldUsername].Value()),
		Created:  m.nowOr().Format(store.DateLayout),
		Expires:  expires,
		Notes:    strings.TrimSpace(m.inputs[fieldNotes].Value()),
		Tags:     tags,
	}

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
	if adding {
		m.audit("add", name)
	} else {
		m.audit("edit", name)
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

	m.hideReveal()
	if err := store.DeleteSecret(title); err != nil {
		m.notify(fmt.Sprintf("✗ Could not delete: %v", err), true)
		return m, nil
	}
	m.audit("rm", title)
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
		leftW, rightW := m.paneWidths()
		if m.clickValueBox(msg, leftW, rightW) {
			return m, nil
		}
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
		sel, ok := m.list.SelectedItem().(item)
		if !ok {
			return m, nil
		}
		now := m.nowOr()
		if title := sel.meta.Title; title == m.lastClickTitle && !m.lastClickAt.IsZero() && now.Sub(m.lastClickAt) <= doubleClickWindow {
			m.copySelected(title)
			m.lastClickTitle, m.lastClickAt = "", time.Time{} // a third click starts a new pair
		} else {
			m.lastClickTitle, m.lastClickAt = title, now
		}
	}
	return m, nil
}

// Detail card position on screen, in 0-based coordinates: header (0), rule (1),
// pane border (2) and padding (3) come first; horizontally the left pane's
// outer width (leftW+2), the 2-column gap, then the right pane's border and
// 2 columns of padding. Mirrors renderBody.
const (
	cardTopRow    = 4
	cardLeftExtra = 2 + 2 + 1 + 2 // added to the left pane's inner width
)

// clickValueBox toggles the reveal when msg lands on the detail card's value
// box, and reports whether it did.
func (m *model) clickValueBox(msg tea.MouseMsg, leftW, rightW int) bool {
	sel, ok := m.list.SelectedItem().(item)
	if !ok || m.total == 0 {
		return false
	}
	cardW := rightW - 4
	_, box := m.detailCard(cardW, m.paneOuter()-4)
	left := leftW + cardLeftExtra
	if box.height == 0 || msg.X < left || msg.X >= left+cardW {
		return false
	}
	top := cardTopRow + box.top
	if msg.Y < top || msg.Y >= top+box.height {
		return false
	}
	m.toggleReveal(sel.meta.Title)
	return true
}

// setFocus focuses input i and blurs the rest. The secret is a textarea, not
// one of the inputs.
func (m *model) setFocus(i int) {
	m.focusIndex = i
	for j := range m.inputs {
		if j == fieldSecret {
			continue
		}
		if j == i {
			m.inputs[j].Focus()
			m.inputs[j].TextStyle = textStyle
			m.inputs[j].CursorEnd() // type after what is there
		} else {
			m.inputs[j].Blur()
			m.inputs[j].TextStyle = subtleStyle
			m.inputs[j].CursorStart() // a long value shows its beginning, not the tail the cursor left it on
		}
	}
	if i == fieldSecret {
		m.secret.Focus()
	} else {
		m.secret.Blur()
	}
}

func count(n int) string {
	if n == 1 {
		return "1 secret"
	}
	return fmt.Sprintf("%d secrets", n)
}
