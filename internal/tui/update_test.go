package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
