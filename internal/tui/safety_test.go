package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/RafaSG13/Pandora/internal/store"
)

const evilValue = "x\x1b]52;c;QQ==\x07y"

func assertInert(t *testing.T, where, view string) {
	t.Helper()
	if strings.Contains(view, "\x1b]52") {
		t.Errorf("%s draws the escape sequence raw:\n%q", where, view)
	}
	if !strings.Contains(view, "^[]52") {
		t.Errorf("%s does not show the value in caret notation:\n%s", where, view)
	}
}

func TestRevealedValuesCannotInjectEscapeSequences(t *testing.T) {
	m := seed(t, newTestModel(t), store.SecretMeta{Title: "evil"}, evilValue)
	assertInert(t, "the detail card", press(m, "v").View())
	if got, _ := store.GetSecretValue("evil"); got != evilValue {
		t.Fatalf("the stored value must stay raw, got %q", got)
	}
	m = press(m, "v") // hide
	assertInert(t, "the viewer", press(m, "V").View())
}

func TestHistoryScreenCannotInjectEscapeSequences(t *testing.T) {
	store.SetHistoryDepth(2)
	t.Cleanup(func() { store.SetHistoryDepth(0) })
	m := seed(t, newTestModel(t), store.SecretMeta{Title: "evil"}, evilValue)
	next := "fine"
	if err := store.UpdateSecret("evil", store.SecretMeta{Title: "evil", Created: "2026-01-02"}, &next); err != nil {
		t.Fatal(err)
	}
	m = press(m, "r", "H", "r")
	if m.(model).mode != modeHistory {
		t.Fatalf("mode = %v, want the history screen", m.(model).mode)
	}
	assertInert(t, "the history screen", m.View())
}

func TestFormSecretIsMaskedAgainAfterTheRevealTime(t *testing.T) {
	now := time.Now()
	m := withClock(press(newTestModel(t), "n"), &now)
	m = ctrl(m, tea.KeyCtrlG)
	val := m.(model).secret.Value()
	if !strings.Contains(m.View(), val) {
		t.Fatal("the generated secret should be visible right after ctrl+g")
	}
	now = now.Add(m.(model).cfg.RevealTTL + time.Second)
	m = tickAt(m, now)
	if strings.Contains(m.View(), val) {
		t.Error("the generated secret stayed visible after the reveal time")
	}
}
