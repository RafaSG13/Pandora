package tui

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/zalando/go-keyring"

	"github.com/RafaSG13/Pandora/internal/store"
)

// testPassword is the master password of the vault newTestModel creates.
const testPassword = "correct horse battery"

// clipboardLog records what the app wrote to the (stubbed) system clipboard.
var clipboardLog []string

// newTestModel isolates keyring, config dir and clipboard, creates an
// unlocked vault and returns a sized model.
func newTestModel(t *testing.T) tea.Model {
	t.Helper()
	return newModelFor(t, true, 100, 30)
}

// newModelFor is newTestModel with a choice of vault (none means the first-run
// setup screen) and terminal size.
func newModelFor(t *testing.T, createVault bool, w, h int) tea.Model {
	t.Helper()
	keyring.MockInit()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	t.Setenv("AppData", dir)
	store.FastKDFForTests()
	store.Lock()
	t.Cleanup(store.Lock)
	if createVault {
		if err := store.Create(testPassword); err != nil {
			t.Fatal(err)
		}
	}

	clipboardLog = nil
	origWrite, origSecret, origRead := writeClipboard, writeSecretClipboard, readClipboard
	writeClipboard = func(s string) error { clipboardLog = append(clipboardLog, s); return nil }
	writeSecretClipboard = writeClipboard
	// The stub clipboard holds whatever was written last, like a real one.
	readClipboard = func() (string, error) {
		if len(clipboardLog) == 0 {
			return "", nil
		}
		return clipboardLog[len(clipboardLog)-1], nil
	}
	t.Cleanup(func() { writeClipboard, writeSecretClipboard, readClipboard = origWrite, origSecret, origRead })

	// Keep the developer's NO_COLOR / TERM out of the tests; New applies them.
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "xterm-256color")
	origNoColor, origProfile := noColor, lipgloss.ColorProfile()
	t.Cleanup(func() { noColor = origNoColor; lipgloss.SetColorProfile(origProfile) })

	m, _ := New().Update(tea.WindowSizeMsg{Width: w, Height: h})
	mm := m.(model)
	mm.lockAfter = 0 // idle lock is opt-in per test
	return mm
}

// press feeds keys ("enter", "esc", "tab" or literal text) to the model.
func press(m tea.Model, keys ...string) tea.Model {
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "tab":
			msg = tea.KeyMsg{Type: tea.KeyTab}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		m, _ = m.Update(msg)
	}
	return m
}

// add fills name, category and secret, skips Expires and Notes, and saves.
func add(m tea.Model, name, category, secret string) tea.Model {
	return press(m, "n", name, "enter", category, "enter", secret, "tab", "enter", "enter")
}

func titles(m tea.Model) []string {
	var out []string
	for _, it := range m.(model).list.Items() {
		out = append(out, it.(item).meta.Title)
	}
	return out
}

func TestAddSelectsNewSecretAndStoresValue(t *testing.T) {
	m := add(newTestModel(t), "a", "X", "s3cret")
	m = add(m, "b", "Y", "other")

	if got := titles(m); len(got) != 2 {
		t.Fatalf("titles = %v", got)
	}
	if sel := m.(model).list.SelectedItem().(item); sel.meta.Title != "b" {
		t.Errorf("selected %q, want newly added b", sel.meta.Title)
	}
	if v, _ := store.GetSecretValue("b"); v != "other" {
		t.Errorf("value = %q", v)
	}
}

func TestAddDuplicateKeepsFormOpenWithError(t *testing.T) {
	m := add(newTestModel(t), "a", "X", "one")
	m = add(m, "a", "Y", "two")

	mm := m.(model)
	if mm.mode != modeForm || !mm.statusErr || !strings.Contains(mm.statusMsg, "already exists") {
		t.Errorf("mode=%v status=%q err=%v, want open form with 'already exists'", mm.mode, mm.statusMsg, mm.statusErr)
	}
	if v, _ := store.GetSecretValue("a"); v != "one" {
		t.Errorf("original value overwritten: %q", v)
	}
}

func TestAddRequiresNameAndSecret(t *testing.T) {
	m := add(newTestModel(t), "a", "X", "")
	mm := m.(model)
	if mm.mode != modeForm || !mm.statusErr {
		t.Errorf("empty secret accepted: mode=%v status=%q", mm.mode, mm.statusMsg)
	}
	if len(titles(m)) != 0 {
		t.Error("secret saved despite validation error")
	}
}

func TestEditPrefillsAndEmptySecretKeepsValue(t *testing.T) {
	m := add(newTestModel(t), "a", "X", "keep-me")
	m = press(m, "e")

	mm := m.(model)
	if mm.mode != modeForm || mm.inputs[0].Value() != "a" || mm.inputs[1].Value() != "X" || mm.inputs[2].Value() != "" {
		t.Fatalf("form not prefilled: mode=%v name=%q cat=%q", mm.mode, mm.inputs[0].Value(), mm.inputs[1].Value())
	}

	// Replace the one-character name, change category; leave secret empty.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = press(m, "renamed", "enter", "Z", "enter", "tab", "enter", "enter")

	if got := titles(m); len(got) != 1 || got[0] != "renamed" {
		t.Fatalf("titles = %v, want [renamed]", got)
	}
	if v, _ := store.GetSecretValue("renamed"); v != "keep-me" {
		t.Errorf("value after rename = %q, want keep-me", v)
	}
	if _, err := store.GetSecretValue("a"); !errors.Is(err, keyring.ErrNotFound) {
		t.Errorf("old entry still in keyring: %v", err)
	}
}

func TestOpeningAddFormAfterEditStartsClean(t *testing.T) {
	m := add(newTestModel(t), "a", "X", "v")
	m = press(m, "e", "esc", "n")

	mm := m.(model)
	if mm.editing != "" || mm.inputs[0].Value() != "" || mm.inputs[1].Value() != "" || mm.secret.Placeholder != secretPlaceholder {
		t.Errorf("stale edit state in add form: editing=%q name=%q", mm.editing, mm.inputs[0].Value())
	}
}

func TestDeleteRequiresConfirmation(t *testing.T) {
	m := add(newTestModel(t), "a", "X", "v")

	m = press(m, "d")
	if m.(model).mode != modeConfirmDelete {
		t.Fatal("d did not open confirmation")
	}
	m = press(m, "n")
	if len(titles(m)) != 1 {
		t.Fatal("secret deleted without confirmation")
	}

	m = press(m, "d", "y")
	if len(titles(m)) != 0 {
		t.Errorf("titles = %v after confirmed delete", titles(m))
	}
	if _, err := store.GetSecretValue("a"); !errors.Is(err, keyring.ErrNotFound) {
		t.Errorf("value still in keyring: %v", err)
	}
}

func TestEditAndDeleteAreNoOpsWhenEmpty(t *testing.T) {
	m := press(newTestModel(t), "e", "d")
	if m.(model).mode != modeBrowse {
		t.Errorf("mode = %v, want browse", m.(model).mode)
	}
}

func TestReloadPicksUpExternalChangesAndKeepsSelection(t *testing.T) {
	m := add(newTestModel(t), "a", "X", "1")
	m = add(m, "b", "X", "2") // b selected

	// Another Pandora instance adds a secret behind this model's back.
	if err := store.SaveSecret(store.SecretMeta{Title: "c", Category: "Z", Created: "2026-01-01"}, "3"); err != nil {
		t.Fatal(err)
	}

	m = press(m, "r")
	got := map[string]bool{}
	for _, title := range titles(m) {
		got[title] = true
	}
	if len(got) != 3 || !got["a"] || !got["b"] || !got["c"] {
		t.Errorf("titles = %v, want a, b and c", titles(m))
	}
	if sel := m.(model).list.SelectedItem().(item); sel.meta.Title != "b" {
		t.Errorf("selection moved to %q, want b", sel.meta.Title)
	}
	if !strings.Contains(m.(model).statusMsg, "Reloaded") {
		t.Errorf("status = %q", m.(model).statusMsg)
	}
}

func click(m tea.Model, x, y int) tea.Model {
	m, _ = m.Update(tea.MouseMsg{X: x, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	return m
}

func wheel(m tea.Model, b tea.MouseButton) tea.Model {
	m, _ = m.Update(tea.MouseMsg{Button: b, Action: tea.MouseActionPress})
	return m
}

func selected(m tea.Model) string {
	return m.(model).list.SelectedItem().(item).meta.Title
}

func addThree(m tea.Model) tea.Model {
	m = add(m, "alpha", "X", "1")
	m = add(m, "bravo", "X", "2")
	return add(m, "charlie", "X", "3") // charlie selected
}

func TestHelpOverlayOpensAndAnyKeyCloses(t *testing.T) {
	m := add(newTestModel(t), "a", "X", "v")

	m = press(m, "?")
	if m.(model).mode != modeHelp {
		t.Fatalf("mode = %v, want help", m.(model).mode)
	}
	if !strings.Contains(m.View(), "Keyboard & mouse") {
		t.Errorf("overlay title missing from view:\n%s", m.View())
	}

	m = press(m, "x")
	if m.(model).mode != modeBrowse {
		t.Errorf("mode = %v, want browse after any key", m.(model).mode)
	}
	if got := titles(m); len(got) != 1 {
		t.Errorf("titles = %v, want unchanged", got)
	}
}

func TestQuestionMarkIsTextInForm(t *testing.T) {
	m := press(newTestModel(t), "n", "?")
	mm := m.(model)
	if mm.mode != modeForm || mm.inputs[0].Value() != "?" {
		t.Errorf("mode=%v name=%q, want form with name ?", mm.mode, mm.inputs[0].Value())
	}
}

func TestQuestionMarkIsTextWhileFiltering(t *testing.T) {
	m := press(add(newTestModel(t), "a", "X", "v"), "/", "?")
	mm := m.(model)
	if mm.mode != modeBrowse || mm.list.FilterState() != list.Filtering {
		t.Errorf("mode=%v filter=%v, want browse while filtering", mm.mode, mm.list.FilterState())
	}
}

// addMany adds n secrets named s1..sn (n ≤ 9); the last is selected.
func addMany(m tea.Model, n int) tea.Model {
	for i := 1; i <= n; i++ {
		m = add(m, "s"+string(rune('0'+i)), "X", "v")
	}
	return m
}

func TestWheelScrollsListWithoutMovingSelection(t *testing.T) {
	m := addMany(newTestModel(t), 9)
	view := m.(model).itemsInView()
	if view >= 9 {
		t.Fatalf("test needs more items than fit (%d)", view)
	}
	if got := selected(m); got != "s9" {
		t.Fatalf("selected %q, want s9", got)
	}

	// Scroll to the top: the first row now shows s1, selection is untouched.
	for range 9 {
		m = wheel(m, tea.MouseButtonWheelUp)
	}
	if got := selected(m); got != "s9" {
		t.Errorf("wheel changed selection to %q", got)
	}
	if row := strings.Split(m.View(), "\n")[listTopRow]; !strings.Contains(row, "s1") {
		t.Errorf("row %d = %q, want s1 after scrolling to top", listTopRow, row)
	}

	// Clicking the first visible row selects the item shown there.
	m = click(m, 5, listTopRow)
	if got := selected(m); got != "s1" {
		t.Errorf("click after scroll selected %q, want s1", got)
	}

	// Scrolling past the end stops at the last whole page.
	for range 20 {
		m = wheel(m, tea.MouseButtonWheelDown)
	}
	if row := strings.Split(m.View(), "\n")[listTopRow]; !strings.Contains(row, "s"+string(rune('0'+9-view+1))) {
		t.Errorf("row %d = %q, want first item of last page", listTopRow, row)
	}
}

func TestKeyboardScrollsViewToSelection(t *testing.T) {
	m := addMany(newTestModel(t), 9)
	for range 9 {
		m = wheel(m, tea.MouseButtonWheelUp)
	}
	m = press(m, "k") // moves selection s9 -> s8 and must bring it back into view

	if got := selected(m); got != "s8" {
		t.Fatalf("selected %q, want s8", got)
	}
	// s8 is index 7; the viewport scrolls just enough to show it on the last row.
	last := listTopRow + (m.(model).itemsInView()-1)*roomyRows
	if row := strings.Split(m.View(), "\n")[last]; !strings.Contains(row, "s8") {
		t.Errorf("row %d = %q, want selected s8 scrolled into view", last, row)
	}
}

func TestClickSelectsItemRow(t *testing.T) {
	m := addThree(newTestModel(t))
	leftW, _ := m.(model).paneWidths()

	m = click(m, 5, listTopRow)
	if got := selected(m); got != "alpha" {
		t.Errorf("click row 0 selected %q, want alpha", got)
	}
	m = click(m, 5, listTopRow+roomyRows)
	if got := selected(m); got != "bravo" {
		t.Errorf("click row 1 selected %q, want bravo", got)
	}

	for name, pos := range map[string][2]int{
		"spacing line": {5, listTopRow + roomyRows + 2},
		"past last":    {5, listTopRow + 3*roomyRows},
		"above list":   {5, listTopRow - 1},
		"detail pane":  {leftW + 10, listTopRow},
	} {
		m = click(m, pos[0], pos[1])
		if got := selected(m); got != "bravo" {
			t.Errorf("click on %s changed selection to %q", name, got)
		}
	}
}

// Guards the hard-coded geometry: fails if bubbles changes title/status heights.
func TestClickGeometryMatchesRenderedList(t *testing.T) {
	m := addThree(newTestModel(t))
	lines := strings.Split(m.View(), "\n")

	if !strings.Contains(lines[listTopRow], "alpha") {
		t.Errorf("row %d = %q, want alpha", listTopRow, lines[listTopRow])
	}
	if !strings.Contains(lines[listTopRow+roomyRows], "bravo") {
		t.Errorf("row %d = %q, want bravo", listTopRow+roomyRows, lines[listTopRow+roomyRows])
	}
}

func TestMouseIgnoredInModals(t *testing.T) {
	m := press(addThree(newTestModel(t)), "d")

	m = wheel(m, tea.MouseButtonWheelUp)
	m = click(m, 5, listTopRow)
	if m.(model).mode != modeConfirmDelete {
		t.Errorf("mode = %v, want confirm delete", m.(model).mode)
	}
	if got := selected(m); got != "charlie" {
		t.Errorf("selected %q, want charlie unchanged", got)
	}
}

func TestCategoryColorIsStableAndCaseInsensitive(t *testing.T) {
	if categoryColor("tokens") != categoryColor("  TOKENS ") {
		t.Error("same category with different case/spacing got different colours")
	}
	if categoryColor("") != subtle {
		t.Error("empty category should be muted")
	}
	seen := map[string]bool{}
	for _, c := range []string{"TOKENS", "INFRA", "DB", "CI", "MAIL", "CLOUD", "KEYS", "WEB"} {
		seen[categoryColor(c).Dark] = true
	}
	if len(seen) < 3 {
		t.Errorf("only %d distinct colours across 8 categories; hashing looks degenerate", len(seen))
	}
}

func TestSelectedRowFillsListWidth(t *testing.T) {
	m := addThree(newTestModel(t)).(model) // charlie (index 2) is selected

	render := func(index int) []string {
		var b strings.Builder
		rowDelegate{}.Render(&b, m.list, index, m.list.VisibleItems()[index])
		return strings.Split(b.String(), "\n")
	}

	for _, line := range render(2) {
		if got := lipgloss.Width(line); got != m.list.Width() {
			t.Errorf("selected line width = %d, want %d", got, m.list.Width())
		}
	}
	for _, line := range render(0) {
		if got := lipgloss.Width(line); got >= m.list.Width() {
			t.Errorf("unselected line padded to %d; only the selection is filled", got)
		}
	}
}
