package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/RafaSG13/Pandora/internal/store"
)

// layoutSizes are the terminals every screen must fit: the 60×18 floor, the
// sizes just above it (where the panes are shortest), and roomy ones.
var layoutSizes = [][2]int{{60, 18}, {60, 22}, {62, 19}, {70, 20}, {80, 24}, {100, 30}, {160, 50}}

// fitsWindow resizes m to each size and fails when its view is not exactly as
// tall as the window or is wider than it: a taller view scrolls the header off
// the screen. It stops at the first bad size of a label.
func fitsWindow(t *testing.T, label string, m tea.Model) {
	t.Helper()
	for _, s := range layoutSizes {
		w, h := s[0], s[1]
		sized, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
		out := sized.View()
		if got := lipgloss.Height(out); got != h {
			t.Errorf("%s at %d×%d: view is %d lines tall, want %d\n%s", label, w, h, got, h, ansi.Strip(out))
			return
		}
		if got := lipgloss.Width(out); got > w {
			t.Errorf("%s at %d×%d: view is %d columns wide\n%s", label, w, h, got, ansi.Strip(out))
			return
		}
	}
}

const (
	longTitle    = "a-very-long-secret-title-that-keeps-going-and-going-until-it-is-way-wider-than-any-pane"
	longCategory = "A-LONG-CATEGORY-NAME-THAT-OVERFLOWS"
)

func TestEmptyScreensFitTheWindow(t *testing.T) {
	fitsWindow(t, "welcome", newTestModel(t))

	broken := newTestModel(t).(model)
	broken.loadErr = "open /home/someone/.config/pandora/vault.json: permission denied while reading the encrypted vault"
	fitsWindow(t, "load error", broken)

	small, _ := newTestModel(t).Update(tea.WindowSizeMsg{Width: 60, Height: 18})
	if out := small.View(); !strings.Contains(out, "to add your first secret") {
		t.Errorf("the welcome lost its call to action at 60×18:\n%s", out)
	}
	brokenSmall, _ := broken.Update(tea.WindowSizeMsg{Width: 60, Height: 18})
	if out := brokenSmall.View(); !strings.Contains(out, "permission denied") {
		t.Errorf("the load error is not shown at 60×18:\n%s", out)
	}
}

// busiestCard is a secret with everything the detail card can show: a long
// title and category, an expiry, tags, notes, an update date and a history row.
func busiestCard(t *testing.T) tea.Model {
	t.Helper()
	t.Cleanup(func() { store.SetHistoryDepth(0) })
	store.SetHistoryDepth(3)
	soon := time.Now().AddDate(0, 0, 3).Format(store.DateLayout)
	notes := strings.Repeat("line of notes with some words ", 12)

	m := newTestModel(t)
	m = press(m, "n", longTitle, "enter", longCategory, "enter", "v1", "tab", soon, "enter", notes, "enter", "alpha, beta, gamma, delta", "enter")
	return editValue(m, "v2")
}

// The ASCII glyph set has a three-column ellipsis; a line cut to fit must
// count it, or the pane wraps the line and grows.
func TestLayoutHoldsWithASCIIGlyphs(t *testing.T) {
	restoreGlyphs(t)
	useGlyphs(true)
	fitsWindow(t, "welcome", newTestModel(t))
	m := busiestCard(t)
	fitsWindow(t, "busiest card", m)
	fitsWindow(t, "busiest card, revealed", press(m, "v"))
	fitsWindow(t, "filter applied", press(filterBy(addThree(newTestModel(t)), "bra"), "enter"))
}

func TestDetailCardFitsItsPaneWhateverTheSecretHolds(t *testing.T) {
	m := busiestCard(t)

	fitsWindow(t, "masked", m)
	fitsWindow(t, "revealed", press(m, "v"))
	orphan := m.(model)
	orphan.orphans = map[string]bool{longTitle: true}
	fitsWindow(t, "missing value", orphan)

	small, _ := press(m, "v").Update(tea.WindowSizeMsg{Width: 60, Height: 18})
	if out := small.View(); !strings.Contains(out, "v2") {
		t.Errorf("the revealed value is cut off at 60×18:\n%s", out)
	}
	masked, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 18})
	if out := masked.View(); !strings.Contains(out, maskedValue) {
		t.Errorf("the value box is cut off at 60×18:\n%s", out)
	}
}

func TestListPaneKeepsItsHeightWhileFiltering(t *testing.T) {
	fresh := func() tea.Model { return addThree(newTestModel(t)) }
	fitsWindow(t, "typing", filterBy(fresh(), "a"))
	fitsWindow(t, "applied", press(filterBy(fresh(), "bra"), "enter"))
	fitsWindow(t, "typing with no match", filterBy(fresh(), "zzz"))
	fitsWindow(t, "applied after its last match was deleted", press(press(filterBy(fresh(), "bra"), "enter"), "d", "y"))
	fitsWindow(t, "long search term", press(filterBy(fresh(), "alphabetagammadeltaepsilon"), "enter"))

	long := press(add(newTestModel(t), "a", longCategory, "v"), "1")
	fitsWindow(t, "category with a long name", long)
}

func TestNoMatchIsSaidInTheDetailPane(t *testing.T) {
	typing := filterBy(addThree(newTestModel(t)), "zzz")
	deleted := press(press(filterBy(addThree(newTestModel(t)), "bra"), "enter"), "d", "y") // the only match is gone
	for name, m := range map[string]tea.Model{"typing": typing, "applied": deleted} {
		if out := m.View(); !strings.Contains(out, "No secret matches") {
			t.Errorf("%s: an empty filter result says nothing:\n%s", name, out)
		}
	}
}

func TestFooterFollowsTheFilterState(t *testing.T) {
	typing := filterBy(addThree(newTestModel(t)), "b")
	if out := typing.View(); !strings.Contains(out, "enter apply") || strings.Contains(out, "n new") {
		t.Errorf("while typing a filter the footer should offer enter/esc only:\n%s", out)
	}
	applied := press(typing, "enter")
	narrow, _ := applied.Update(tea.WindowSizeMsg{Width: 60, Height: 18})
	for name, m := range map[string]tea.Model{"applied": applied, "applied at 60×18": narrow} {
		if out := m.View(); !strings.Contains(out, "esc clear filter") {
			t.Errorf("%s: the footer does not say how to clear the filter:\n%s", name, out)
		}
	}
}

func TestReloadKeepsAnActiveFilter(t *testing.T) {
	m := press(filterBy(addThree(newTestModel(t)), "bra"), "enter")
	want := []string{"bravo"}
	if got := visibleTitles(m); !sameStrings(got, want) {
		t.Fatalf("filter shows %v, want %v", got, want)
	}

	m = press(m, "e", "enter", "enter", "", "tab", "", "enter", "", "enter", "", "enter") // edit and save
	if got := visibleTitles(m); !sameStrings(got, want) {
		t.Errorf("after editing, the filtered list shows %v, want %v", got, want)
	}
	m = press(m, "r")
	if got := visibleTitles(m); !sameStrings(got, want) {
		t.Errorf("after reloading, the filtered list shows %v, want %v", got, want)
	}
	m = add(m, "brazil", "X", "v")
	if got := visibleTitles(m); !sameStrings(got, []string{"bravo", "brazil"}) {
		t.Errorf("a new secret matching the filter is missing: %v", got)
	}
}

func TestPageKeysMoveByAPage(t *testing.T) {
	m := addMany(newTestModel(t), 9)
	page := m.(model).itemsInView()
	if page >= 9 {
		t.Fatalf("the test needs more secrets than one page (%d)", page)
	}
	key := func(m tea.Model, k tea.KeyType) tea.Model {
		m, _ = m.Update(tea.KeyMsg{Type: k})
		return m
	}
	m = press(m, "g")
	m = key(m, tea.KeyPgDown)
	if got := m.(model).list.Index(); got != page {
		t.Errorf("pgdn moved to row %d, want %d", got, page)
	}
	m = key(m, tea.KeyPgDown)
	if got := m.(model).list.Index(); got != 8 {
		t.Errorf("pgdn at the end moved to row %d, want the last (8)", got)
	}
	m = key(m, tea.KeyPgUp)
	if got := m.(model).list.Index(); got != 8-page {
		t.Errorf("pgup moved to row %d, want %d", got, 8-page)
	}
	if sel := selected(m); !strings.Contains(ansi.Strip(m.View()), sel) {
		t.Errorf("the selected secret %q scrolled out of view", sel)
	}
}

func TestResizeKeepsFormEditorsInView(t *testing.T) {
	// A long multi-line secret with the cursor on its last line: a shorter
	// editor must still show that line, not an empty row.
	pem := "-----BEGIN KEY-----\n" + strings.Repeat("QUJDREVGR0hJSktMTU5PUFFSU1RVVldYWVo=\n", 20) + "-----END KEY-----"
	m := pasteText(openSecretField(newTestModel(t), "x"), pem)
	m = ctrl(m, tea.KeyCtrlR) // show it
	m.View()
	m, _ = m.Update(tea.WindowSizeMsg{Width: 60, Height: 18})
	if out := m.View(); !strings.Contains(out, "-----END KEY-----") {
		t.Errorf("after shrinking, the secret editor does not show the cursor line:\n%s", out)
	}

	// A name longer than the narrower field: the end, where the cursor is, shows.
	m = press(add(newTestModel(t), longTitle, "X", "v"), "e")
	m, _ = m.Update(tea.WindowSizeMsg{Width: 60, Height: 18})
	if out := m.View(); !strings.Contains(out, "than-any-pane") {
		t.Errorf("after shrinking, the name field does not show its cursor end:\n%s", out)
	}
}

func TestLongFormStatusAndDeleteTitleAreCutWithAnEllipsis(t *testing.T) {
	m := press(addThree(newTestModel(t)), "n")
	mm := m.(model)
	mm.notify("✗ Could not save: a really long keyring error message that goes on and on and on and on", true)
	small, _ := mm.Update(tea.WindowSizeMsg{Width: 60, Height: 18})
	if out := ansi.Strip(small.View()); !strings.Contains(out, g.ellipsis+" ") && !strings.Contains(out, g.ellipsis+"\n") {
		t.Errorf("a long status line is cut without an ellipsis:\n%s", out)
	}

	del := press(add(newTestModel(t), longTitle+longTitle, "X", "v"), "d")
	small, _ = del.Update(tea.WindowSizeMsg{Width: 60, Height: 18})
	out := ansi.Strip(small.View())
	if !strings.Contains(out, `Delete "a-very-long`) || !strings.Contains(out, g.ellipsis+`"?`) {
		t.Errorf("the delete question should keep the title on one line, cut with an ellipsis:\n%s", out)
	}
}

func TestCtrlCQuitsFromTheForm(t *testing.T) {
	m := press(addThree(newTestModel(t)), "n")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl+c did nothing in the form")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("ctrl+c in the form returned %T, want tea.QuitMsg", cmd())
	}
}

func TestBlurredFormFieldsShowTheirBeginning(t *testing.T) {
	m := press(newTestModel(t), "n", "b", "enter", "X", "enter", "v", "tab", "", "enter", "BEGINNING "+strings.Repeat("0123456789 ", 8))
	m = press(m, "tab") // Notes loses focus
	m, _ = m.Update(tea.WindowSizeMsg{Width: 60, Height: 18})
	if out := m.View(); !strings.Contains(out, "BEGINNING") {
		t.Errorf("a long note should show from its beginning once the field is left:\n%s", out)
	}
}

// Every mode, at every size in layoutSizes, fills the window exactly.
func TestEveryModeFitsTheWindow(t *testing.T) {
	t.Cleanup(func() { store.SetHistoryDepth(0) })
	withHistory := withVersions(t, 3) // "k" with two previous values
	fitsWindow(t, "browse", withHistory)
	fitsWindow(t, "history", press(withHistory, "H"))
	fitsWindow(t, "add form", press(withHistory, "n"))
	fitsWindow(t, "edit form", press(withHistory, "e"))
	fitsWindow(t, "delete confirmation", press(withHistory, "d"))
	fitsWindow(t, "help", press(withHistory, "?"))

	fitsWindow(t, "setup", newModelFor(t, false, 100, 30))
	migrating := newModelFor(t, false, 100, 30).(model)
	migrating.migrating, migrating.legacyN = true, 12
	fitsWindow(t, "migration", migrating)
	locked := newModelFor(t, true, 100, 30).(model)
	locked.lock()
	fitsWindow(t, "unlock", locked)
	locked.unlockErr = "✗ Wrong password"
	fitsWindow(t, "unlock with an error", locked)
}
