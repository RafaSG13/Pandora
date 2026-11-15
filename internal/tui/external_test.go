package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/RafaSG13/Pandora/internal/store"
)

func selectedTitle(m tea.Model) string {
	if it, ok := m.(model).list.SelectedItem().(item); ok {
		return it.meta.Title
	}
	return ""
}

// A change made by another process (the CLI, a second instance) shows up on
// the next tick without pressing r, and the selection stays on the same
// secret even when the newcomer sorts before it.
func TestExternalChangeIsPickedUpOnTick(t *testing.T) {
	m := add(add(newTestModel(t), "a", "X", "1"), "b", "X", "2")
	m = ctrl(m, tea.KeyDown)
	if got := selectedTitle(m); got != "b" {
		t.Fatalf("selected = %q, want b", got)
	}

	if err := store.SaveSecret(store.SecretMeta{Title: "from-cli", Created: "2000-01-01"}, "v"); err != nil {
		t.Fatal(err)
	}
	m = tickAt(m, time.Now())

	if got := visibleTitles(m); len(got) != 3 {
		t.Fatalf("titles = %v, want the new secret listed", got)
	}
	if got := selectedTitle(m); got != "b" {
		t.Errorf("selection moved to %q after the reload", got)
	}
	if !strings.Contains(m.(model).statusMsg, "changed elsewhere") {
		t.Errorf("no hint about the reload: %q", m.(model).statusMsg)
	}
}

// Deleting the selected secret elsewhere leaves a valid selection.
func TestExternalDeleteOfSelectedKeepsAValidSelection(t *testing.T) {
	m := add(add(newTestModel(t), "a", "X", "1"), "b", "X", "2")
	sel := selectedTitle(m)
	if err := store.DeleteSecret(sel); err != nil {
		t.Fatal(err)
	}
	m = tickAt(m, time.Now())

	if got := visibleTitles(m); len(got) != 1 || got[0] == sel {
		t.Fatalf("titles = %v after deleting %q elsewhere", got, sel)
	}
	if selectedTitle(m) == "" {
		t.Error("nothing selected although a secret is listed")
	}
}

// The TUI's own writes already reload, so they must not be reported as
// changes made elsewhere.
func TestOwnWritesAreNotReportedAsExternal(t *testing.T) {
	m := add(newTestModel(t), "a", "X", "1")
	m = tickAt(m, time.Now())
	if strings.Contains(m.(model).statusMsg, "changed elsewhere") {
		t.Errorf("own save reported as external: %q", m.(model).statusMsg)
	}
}

// A form or a delete confirmation is never disturbed by an outside change;
// the list refreshes on the first tick after the dialog closes.
func TestExternalChangeWaitsForDialogsToClose(t *testing.T) {
	for name, open := range map[string]string{"form": "n", "delete": "d"} {
		t.Run(name, func(t *testing.T) {
			m := add(newTestModel(t), "a", "X", "1")
			m = press(m, open)
			mode := m.(model).mode
			if mode == modeBrowse {
				t.Fatalf("%q did not open a dialog", open)
			}

			if err := store.SaveSecret(store.SecretMeta{Title: "from-cli"}, "v"); err != nil {
				t.Fatal(err)
			}
			m = tickAt(m, time.Now())
			if m.(model).mode != mode {
				t.Fatalf("mode changed to %v while the dialog was open", m.(model).mode)
			}
			if got := titles(m); len(got) != 1 {
				t.Fatalf("list reloaded under an open dialog: %v", got)
			}

			m = press(m, "esc")
			m = tickAt(m, time.Now())
			if got := titles(m); len(got) != 2 {
				t.Fatalf("titles = %v after closing, want the outside change", got)
			}
		})
	}
}

// Deleting the last row must not leave the list with nothing selected, or
// enter, e and d would silently do nothing.
func TestDeletingTheLastRowSelectsTheNewLastOne(t *testing.T) {
	m := add(add(newTestModel(t), "a", "X", "1"), "b", "X", "2")
	if got := selectedTitle(m); got != "b" {
		t.Fatalf("selected = %q, want the last row b", got)
	}
	m = press(m, "d", "y")
	if got := selectedTitle(m); got != "a" {
		t.Errorf("selected = %q after deleting the last row, want a", got)
	}
}
