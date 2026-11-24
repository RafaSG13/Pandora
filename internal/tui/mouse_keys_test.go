package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestRebindingCopyAndQuitChangesKeysAndHints(t *testing.T) {
	m := withConfig(t, `{"keys":{"copy":["y"],"quit":["Q"]}}`, 100, 30, func(m tea.Model) tea.Model {
		return add(m, "a", "X", "s3cret")
	})

	m = press(m, "c")
	if len(clipboardLog) != 0 {
		t.Fatalf("old key still copies: %q", clipboardLog)
	}
	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}); cmd != nil {
		t.Error("old quit key still quits")
	}
	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Q")}); cmd == nil {
		t.Error("rebound quit key does nothing")
	}

	m = press(m, "y")
	if len(clipboardLog) != 1 || clipboardLog[0] != "s3cret" {
		t.Fatalf("clipboard = %q, want the secret after y", clipboardLog)
	}

	if view := ansi.Strip(m.View()); !strings.Contains(view, "y copy") || !strings.Contains(view, "Q quit") {
		t.Errorf("footer does not show the effective keys:\n%s", view)
	}
	m = press(m, "?")
	help := ansi.Strip(m.View())
	if !strings.Contains(help, "y ") || strings.Contains(help, "enter/c") {
		t.Errorf("help overlay does not show the effective keys:\n%s", help)
	}
}

// listRowClick clicks the first line of the n-th visible row.
func listRowClick(m tea.Model, n int) tea.Model {
	return click(m, 5, listTopRow+n*m.(model).rowsPer())
}

func TestDoubleClickCopiesOnlyForTwoQuickClicksOnTheSameRow(t *testing.T) {
	now := time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC)
	m := withClock(addThree(newTestModel(t)), &now)

	m = listRowClick(m, 0)
	now = now.Add(200 * time.Millisecond)
	m = listRowClick(m, 0)
	if len(clipboardLog) != 1 {
		t.Fatalf("double-click wrote %d times, want 1", len(clipboardLog))
	}

	now = now.Add(100 * time.Millisecond)
	m = listRowClick(m, 0) // a third click starts a new pair
	if len(clipboardLog) != 1 {
		t.Errorf("third click copied again: %d writes", len(clipboardLog))
	}

	clipboardLog = nil
	now = now.Add(time.Second)
	m = listRowClick(m, 1)
	now = now.Add(600 * time.Millisecond)
	m = listRowClick(m, 1)
	if len(clipboardLog) != 0 {
		t.Errorf("slow clicks copied: %q", clipboardLog)
	}

	now = now.Add(time.Second)
	m = listRowClick(m, 0)
	now = now.Add(200 * time.Millisecond)
	m = listRowClick(m, 1)
	if len(clipboardLog) != 0 {
		t.Errorf("clicks on different rows copied: %q", clipboardLog)
	}
}

// viewPos finds the first cell of needle in the rendered, ANSI-stripped view.
func viewPos(t *testing.T, m tea.Model, needle string) (x, y int) {
	t.Helper()
	for y, line := range strings.Split(ansi.Strip(m.View()), "\n") {
		if i := strings.Index(line, needle); i >= 0 {
			return ansi.StringWidth(line[:i]), y
		}
	}
	t.Fatalf("%q not in view:\n%s", needle, ansi.Strip(m.View()))
	return 0, 0
}

func TestClickingTheValueBoxRevealsAndHides(t *testing.T) {
	m := add(newTestModel(t), "a", "X", "s3cret")
	title := selected(m)

	x, y := viewPos(t, m, maskedValue)
	_, createdY := viewPos(t, m, "Created")

	m = click(m, x, createdY)
	if m.(model).revealed(title) {
		t.Fatal("clicking a metadata row revealed the value")
	}
	m = click(m, x, y)
	if !m.(model).revealed(title) {
		t.Fatal("clicking the value box did not reveal")
	}
	if view := ansi.Strip(m.View()); !strings.Contains(view, "s3cret") {
		t.Errorf("revealed value not drawn:\n%s", view)
	}
	m = click(m, x, y)
	if m.(model).revealed(title) {
		t.Error("second click did not hide")
	}
}

func TestApplyBackgroundForcesDarkOrLight(t *testing.T) {
	t.Cleanup(func() { lipgloss.SetHasDarkBackground(true) })
	origNoColor := noColor
	t.Cleanup(func() { noColor = origNoColor })
	noColor = false

	applyBackground("light")
	if lipgloss.HasDarkBackground() {
		t.Error(`"light" left a dark background`)
	}
	applyBackground("dark")
	if !lipgloss.HasDarkBackground() {
		t.Error(`"dark" left a light background`)
	}
}
