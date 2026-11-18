// Package tui implements the Bubble Tea interface for browsing and managing secrets.
package tui

import (
	"crypto/sha256"
	"fmt"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/RafaSG13/Pandora/internal/store"
)

type item struct {
	meta store.SecretMeta
}

// FilterValue is what the "/" filter searches: title first (rows highlight
// matches inside the title only), then category, tags and notes. The list
// holds metadata only, so the secret value is never searched.
func (i item) FilterValue() string {
	parts := []string{i.meta.Title, i.meta.Category}
	parts = append(parts, i.meta.Tags...)
	parts = append(parts, i.meta.Notes)
	return strings.Join(parts, " ")
}

// mode is what the keyboard currently drives.
type mode int

const (
	modeBrowse mode = iota
	modeForm
	modeConfirmDelete
	modeHelp
	modeUnlock // asks for the master password; nothing is loaded
	modeSetup  // first run: choose the master password (or migrate an old store)
)

// Form fields, in tab order.
const (
	fieldName = iota
	fieldCategory
	fieldSecret
	fieldExpires
	fieldNotes
	fieldTags
	numFields
)

// sortMode orders the list; s cycles through them.
type sortMode int

const (
	sortCreated sortMode = iota
	sortName
	sortCategory
)

func (s sortMode) next() sortMode { return (s + 1) % 3 }

func (s sortMode) String() string {
	return [...]string{"date added", "name", "category"}[s]
}

// Timings. The lock timeout can be changed with PANDORA_LOCK_MINUTES (0 disables).
const (
	clipboardTTL   = 15 * time.Second
	revealTTL      = 10 * time.Second
	toastTTL       = 3 * time.Second
	defaultLockTTL = 5 * time.Minute
)

type model struct {
	list          list.Model
	width, height int
	mode          mode
	now           func() time.Time

	// Master-password screens (modeUnlock, modeSetup).
	pwInput, pwConfirm textinput.Model
	pwField            int  // 0 = password, 1 = confirmation (setup only)
	pwShow             bool // typed text is visible (ctrl+r)
	unlocking          bool
	unlockErr          string
	migrating          bool // setup converts an older plain-text store
	legacyN            int  // secrets in that store, for the heading

	statusMsg  string
	statusErr  bool
	toastUntil time.Time

	// Per-item reveal: only revealTitle's value is shown, until revealUntil.
	// revealValue is read once when revealing and dropped on hide, so a long
	// value is not fetched from the keyring on every frame.
	revealTitle string
	revealValue string
	revealErr   error
	revealUntil time.Time

	// Clipboard auto-clear deadline; zero when nothing is pending. clipHash is
	// the SHA-256 of what was copied, so the clear can tell whether the
	// clipboard still holds it; the value itself is never kept.
	clipUntil time.Time
	clipHash  [sha256.Size]byte

	// Idle lock.
	lockAfter  time.Duration
	lastActive time.Time

	// List view state.
	scroll     int // index of the first list item shown; decoupled from the selection
	sort       sortMode
	category   string   // active category filter; empty shows all
	categories []string // distinct categories across all secrets
	tag        string   // active tag filter; empty shows all
	tags       []string // distinct tags across all secrets, sorted
	compact    bool
	total      int // number of secrets, regardless of the category filter
	expiring   int // secrets expired or expiring soon
	loadErr    string
	stamp      store.Stamp // vault version the list was last loaded from

	// Orphan detection (metadata entries whose keyring value is missing).
	orphans  map[string]bool
	checking bool
	spinner  spinner.Model

	// Form. inputs[fieldSecret] is unused: the secret is a multi-line textarea
	// (secret), masked unless secretShown, because textinput cannot hold newlines.
	inputs      []textinput.Model
	secret      textarea.Model
	secretShown bool
	focusIndex  int
	editing     string // title of the secret being edited; empty when adding
	genLen      int    // index into genLengths
	genSet      int    // index into genCharsets

	deleting string // title awaiting delete confirmation

	helpScroll int // first help line shown when the overlay does not fit
}

// New returns the root model, populated from the stored metadata.
func New() tea.Model {
	applyColorMode(detectNoColor(os.Getenv)) // before any style can resolve a colour

	l := list.New(nil, rowDelegate{}, 40, 14)
	l.Title = "Secrets"
	l.Styles.Title = lipgloss.NewStyle().Bold(true).Foreground(accent)
	l.SetShowHelp(false)
	l.SetShowPagination(false) // the list is scrolled by model.scroll, not paginated

	inputs := make([]textinput.Model, numFields)
	for i := range inputs {
		inputs[i] = textinput.New()
		inputs[i].Prompt = ""
	}
	inputs[fieldName].Placeholder = "e.g. Stripe API Key"
	inputs[fieldCategory].Placeholder = "TOKENS / INFRA"
	inputs[fieldCategory].ShowSuggestions = true
	// tab moves between fields, so accept a category suggestion with →.
	inputs[fieldCategory].KeyMap.AcceptSuggestion = key.NewBinding(key.WithKeys("right"))
	// inputs[fieldSecret] stays a bare textinput that is never shown.
	inputs[fieldExpires].Placeholder = "YYYY-MM-DD (optional)"
	inputs[fieldExpires].CharLimit = 10
	inputs[fieldNotes].Placeholder = "URL, account… (optional)"
	inputs[fieldTags].Placeholder = "comma-separated (optional)"

	sp := spinner.New()
	sp.Spinner = g.spinner
	sp.Style = lipgloss.NewStyle().Foreground(accent)

	m := model{
		list:       l,
		inputs:     inputs,
		secret:     newSecretArea(),
		now:        time.Now,
		lockAfter:  lockTimeout(os.Getenv("PANDORA_LOCK_MINUTES")),
		spinner:    sp,
		genLen:     1,
		lastActive: time.Now(),
	}
	m.pwInput, m.pwConfirm = newPasswordInput(), newPasswordInput()
	m.setFocus(fieldName)
	switch {
	case store.IsUnlocked():
		if err := m.reload(""); err != nil {
			m.notify(fmt.Sprintf("✗ Could not load secrets: %v", err), true)
		}
		m.checking = m.total > 0
	case store.Exists():
		m.mode = modeUnlock
		m.focusPassword(0)
	default:
		m.mode = modeSetup
		m.migrating = store.LegacyExists()
		m.legacyN = store.LegacyCount()
		m.focusPassword(0)
	}
	return m
}

// newPasswordInput is a masked single-line input for the master password.
func newPasswordInput() textinput.Model {
	in := textinput.New()
	in.Prompt = ""
	in.EchoMode = textinput.EchoPassword
	in.EchoCharacter = []rune(g.mask)[0]
	return in
}

// locked reports whether a master-password screen is showing.
func (m model) locked() bool { return m.mode == modeUnlock || m.mode == modeSetup }

// focusPassword focuses the password (0) or confirmation (1) input and sets
// the hints shown inside the empty boxes.
func (m *model) focusPassword(i int) {
	m.pwField = i
	m.pwInput.Placeholder = "your master password"
	m.pwConfirm.Placeholder = "type it again"
	if m.mode == modeSetup {
		m.pwInput.Placeholder = "choose a password"
	}
	m.pwInput.Blur()
	m.pwConfirm.Blur()
	if i == 0 {
		m.pwInput.Focus()
	} else {
		m.pwConfirm.Focus()
	}
}

// setReveal shows (true) or masks (false) what is typed in the password boxes.
func (m *model) setReveal(show bool) {
	m.pwShow = show
	mode := textinput.EchoPassword
	if show {
		mode = textinput.EchoNormal
	}
	m.pwInput.EchoMode, m.pwConfirm.EchoMode = mode, mode
}

// lockTimeout parses PANDORA_LOCK_MINUTES; empty or invalid means the default.
func lockTimeout(v string) time.Duration {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n < 0 {
		return defaultLockTTL
	}
	return time.Duration(n) * time.Minute
}

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{tickCmd()}
	if m.checking {
		cmds = append(cmds, m.spinner.Tick, checkOrphans(m.titles()))
	}
	return tea.Batch(cmds...)
}

var zeroTime time.Time

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// orphanMsg carries the titles whose keyring value is missing.
type orphanMsg struct{ missing map[string]bool }

// checkOrphans reads each value from the keyring off the UI thread (some
// backends are slow or prompt) and reports the ones that are not there.
func checkOrphans(titles []string) tea.Cmd {
	return func() tea.Msg {
		missing := map[string]bool{}
		for _, t := range titles {
			if _, err := store.GetSecretValue(t); store.IsValueMissing(err) {
				missing[t] = true
			}
		}
		return orphanMsg{missing}
	}
}

// titles returns every secret's title, regardless of the category filter.
func (m model) titles() []string {
	metas, err := store.LoadMetadata()
	if err != nil {
		return nil
	}
	out := make([]string, len(metas))
	for i, meta := range metas {
		out[i] = meta.Title
	}
	return out
}

// notify shows msg as a toast (browse mode) or inline status (forms).
func (m *model) notify(msg string, isErr bool) {
	m.statusMsg, m.statusErr = msg, isErr
	m.toastUntil = m.nowOr().Add(toastTTL)
}

func (m model) nowOr() time.Time {
	if m.now == nil {
		return time.Now()
	}
	return m.now()
}

const (
	secretPlaceholder     = "Secret value"
	secretKeepPlaceholder = "Leave empty to keep current"
)

// Layout and list geometry, in 0-based screen coordinates. bubbles' list does
// not expose its chrome, so these mirror what View renders.
const (
	// listTopRow is the screen row of the first list item: header (0), rule (1),
	// pane border (2), pane padding (3), list title bar (4–5: title plus one
	// padding line, from list.DefaultStyles) and status bar (6–7, same shape).
	listTopRow = 8
	// listChrome is the rows the list spends above its items (title bar plus
	// status bar, 2 each).
	listChrome = 4
	// roomyRows / compactRows are the rows one item occupies: two lines of
	// text plus a separator line, or a single line.
	roomyRows   = 3
	compactRows = 1
	// minPaneOuter is the shortest pane (border and padding included).
	minPaneOuter = 18
	// minWidth and minHeight are the smallest supported terminal.
	minWidth, minHeight = 60, 18
)

// rowsPer is the rows one list item occupies; textLines how many of them are
// text (the rest is the separator, which the last item omits).
func (m model) rowsPer() int {
	if m.compact {
		return compactRows
	}
	return roomyRows
}

func (m model) textLines() int {
	if m.compact {
		return 1
	}
	return 2
}

func (m model) delegate() rowDelegate {
	return rowDelegate{compact: m.compact, orphans: m.orphans, now: m.nowOr}
}

// reload re-reads the metadata file into the list, applying the category
// filter and sort, and selects the secret titled selectTitle, or keeps the
// current selection when it is empty or no longer exists. On error the list
// is left untouched.
func (m *model) reload(selectTitle string) error {
	if selectTitle == "" {
		if sel, ok := m.list.SelectedItem().(item); ok {
			selectTitle = sel.meta.Title
		}
	}

	m.stamp, _ = store.VaultStamp() // before the read: a write in between is seen on the next tick
	metas, err := store.LoadMetadata()
	if err != nil {
		m.loadErr = err.Error()
		return err
	}
	m.loadErr = ""
	m.total = len(metas)
	m.categories = distinctCategories(metas)
	if m.category != "" && !containsFold(m.categories, m.category) {
		m.category = "" // the last secret of that category is gone
	}
	m.tags = distinctTags(metas)
	if m.tag != "" && !slices.Contains(m.tags, m.tag) {
		m.tag = "" // the last secret with that tag is gone
	}

	m.expiring = 0
	now := m.nowOr()
	shown := make([]store.SecretMeta, 0, len(metas))
	for _, meta := range metas {
		if _, level := expiryInfo(meta, now); level >= expirySoon {
			m.expiring++
		}
		if (m.category == "" || strings.EqualFold(strings.TrimSpace(meta.Category), m.category)) &&
			(m.tag == "" || slices.Contains(meta.Tags, m.tag)) {
			shown = append(shown, meta)
		}
	}
	sortMetas(shown, m.sort)

	items := make([]list.Item, len(shown))
	for i, meta := range shown {
		items[i] = item{meta: meta}
	}
	m.list.Title = "All categories"
	switch {
	case m.category != "":
		m.list.Title = "Category " + g.sep + " " + m.category
		if m.tag != "" {
			m.list.Title += " " + g.sep + " #" + m.tag
		}
	case m.tag != "":
		m.list.Title = "Tag " + g.sep + " #" + m.tag
	}
	m.list.SetDelegate(m.delegate())
	m.fitList(len(items))
	m.list.SetItems(items)

	found := false
	for i, it := range m.list.VisibleItems() {
		if it.(item).meta.Title == selectTitle {
			m.list.Select(i)
			found = true
			break
		}
	}
	if n := len(items); !found && n > 0 && m.list.Index() >= n {
		m.list.Select(n - 1) // the selected row was the last one and is gone
	}
	return nil
}

func sortMetas(metas []store.SecretMeta, mode sortMode) {
	sort.SliceStable(metas, func(i, j int) bool {
		a, b := metas[i], metas[j]
		switch mode {
		case sortName:
			return strings.ToLower(a.Title) < strings.ToLower(b.Title)
		case sortCategory:
			ca, cb := strings.ToLower(a.Category), strings.ToLower(b.Category)
			if ca != cb {
				return ca < cb
			}
			return strings.ToLower(a.Title) < strings.ToLower(b.Title)
		default:
			return a.Created < b.Created
		}
	})
}

// distinctCategories returns the non-empty categories, de-duplicated ignoring
// case and sorted.
func distinctCategories(metas []store.SecretMeta) []string {
	var out []string
	for _, meta := range metas {
		c := strings.TrimSpace(meta.Category)
		if c != "" && !containsFold(out, c) {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i]) < strings.ToLower(out[j]) })
	return out
}

// distinctTags returns every tag in use, sorted. Stored tags are already
// normalised, so no case folding is needed.
func distinctTags(metas []store.SecretMeta) []string {
	var out []string
	for _, meta := range metas {
		for _, t := range meta.Tags {
			if !slices.Contains(out, t) {
				out = append(out, t)
			}
		}
	}
	slices.Sort(out)
	return out
}

func containsFold(list []string, s string) bool {
	for _, v := range list {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}

// paneOuterFor is the outer height of the panes for n secrets: they fit their
// content, but never shrink below minPaneOuter or outgrow the terminal.
func (m model) paneOuterFor(n int) int {
	want := listChrome + n*m.rowsPer() + 4
	return max(6, min(m.height-4, max(minPaneOuter, want)))
}

func (m model) paneOuter() int { return m.paneOuterFor(len(m.list.Items())) }

// fitList sizes the list so that all n items fit on a single page. The model
// scrolls by cropping the rendered list (see listView), which lets the wheel
// move the viewport without touching the selection; bubbles' list has no
// scroll offset of its own. Two columns on the right are left for the scrollbar.
func (m *model) fitList(n int) {
	left, _ := m.paneWidths()
	inner := m.paneOuterFor(n) - 4
	m.list.SetSize(left-4-2, max(inner, listChrome+m.rowsPer()*n+1))
}

// itemsInView is how many whole items fit in the list pane.
func (m model) itemsInView() int {
	areaH := m.paneOuter() - 4 - listChrome
	return max(1, (areaH+m.rowsPer()-m.textLines())/m.rowsPer())
}

// clampScroll keeps scroll within the list. With follow it also scrolls the
// viewport just enough to keep the selected item visible.
func (m *model) clampScroll(follow bool) {
	n, view := len(m.list.VisibleItems()), m.itemsInView()
	if follow {
		if idx := m.list.Index(); idx < m.scroll {
			m.scroll = idx
		} else if idx >= m.scroll+view {
			m.scroll = idx - view + 1
		}
	}
	m.scroll = max(0, min(m.scroll, n-view))
}

// revealed reports whether title's value is currently shown.
func (m model) revealed(title string) bool {
	return m.revealTitle == title && m.nowOr().Before(m.revealUntil)
}

// paneWidths splits the terminal width between the list and detail panes.
// Each pane spends 2 columns on its border; 2 more separate them.
func (m model) paneWidths() (left, right int) {
	left = max(30, (m.width-6)*45/100)
	right = m.width - 6 - left
	return left, right
}

// modalWidth is the outer width of the modal dialogs.
func (m model) modalWidth() int { return min(64, m.width-4) }
