package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/zalando/go-keyring"

	"pandora/internal/store"
)

// newTestModel isolates keyring and config dir and returns a sized model.
func newTestModel(t *testing.T) tea.Model {
	t.Helper()
	keyring.MockInit()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	t.Setenv("AppData", dir)

	m, _ := New().Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return m
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

func add(m tea.Model, name, category, secret string) tea.Model {
	return press(m, "n", name, "enter", category, "enter", secret, "enter")
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
	m = press(m, "renamed", "enter", "Z", "enter", "enter")

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
	if mm.editing != "" || mm.inputs[0].Value() != "" || mm.inputs[1].Value() != "" || mm.inputs[2].Placeholder != secretPlaceholder {
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

	dir, _ := os.UserConfigDir()
	file := filepath.Join(dir, "pandora", "metadata.json")
	data := `[{"title":"c","category":"Z","created":"1"},{"title":"a","category":"X","created":"1"},{"title":"b","category":"X","created":"1"}]`
	if err := os.WriteFile(file, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}

	m = press(m, "r")
	if got := strings.Join(titles(m), ","); got != "c,a,b" {
		t.Errorf("titles = %s, want c,a,b", got)
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

func TestWheelMovesSelection(t *testing.T) {
	m := addThree(newTestModel(t))

	m = wheel(m, tea.MouseButtonWheelUp)
	if got := selected(m); got != "bravo" {
		t.Errorf("after wheel up selected %q, want bravo", got)
	}
	m = wheel(m, tea.MouseButtonWheelDown)
	if got := selected(m); got != "charlie" {
		t.Errorf("after wheel down selected %q, want charlie", got)
	}
}

func TestClickSelectsItemRow(t *testing.T) {
	m := addThree(newTestModel(t))
	leftW, _ := m.(model).paneWidths()

	m = click(m, 5, listTopRow)
	if got := selected(m); got != "alpha" {
		t.Errorf("click row 0 selected %q, want alpha", got)
	}
	m = click(m, 5, listTopRow+itemRows)
	if got := selected(m); got != "bravo" {
		t.Errorf("click row 1 selected %q, want bravo", got)
	}

	for name, pos := range map[string][2]int{
		"spacing line": {5, listTopRow + itemRows + 2},
		"past last":    {5, listTopRow + 3*itemRows},
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
	if !strings.Contains(lines[listTopRow+itemRows], "bravo") {
		t.Errorf("row %d = %q, want bravo", listTopRow+itemRows, lines[listTopRow+itemRows])
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
