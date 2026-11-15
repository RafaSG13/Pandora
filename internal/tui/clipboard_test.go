package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// fakeClipboard replaces the system clipboard; the returned func reads its content.
func fakeClipboard(t *testing.T) func() string {
	t.Helper()
	var content string
	orig := writeClipboard
	writeClipboard = func(s string) error { content = s; return nil }
	t.Cleanup(func() { writeClipboard = orig })
	return func() string { return content }
}

func tick(m tea.Model, gen int) tea.Model {
	m, _ = m.Update(clipboardTickMsg{gen})
	return m
}

func TestCopyCountsDownThenClears(t *testing.T) {
	clip := fakeClipboard(t)
	m := add(newTestModel(t), "a", "X", "s3cret")

	m = press(m, "c")
	if clip() != "s3cret" || m.(model).clipLeft != clipboardSeconds {
		t.Fatalf("clipboard=%q left=%d after copy", clip(), m.(model).clipLeft)
	}
	if v := m.View(); !strings.Contains(v, "clears in 15s") {
		t.Error("countdown not shown in the view")
	}

	gen := m.(model).copyGen
	for i := 0; i < clipboardSeconds-1; i++ {
		m = tick(m, gen)
	}
	if clip() != "s3cret" || m.(model).clipLeft != 1 {
		t.Fatalf("cleared too early: clipboard=%q left=%d", clip(), m.(model).clipLeft)
	}
	m = tick(m, gen)
	if clip() != "" || m.(model).clipLeft != 0 || !strings.Contains(m.(model).statusMsg, "cleared") {
		t.Errorf("not cleared at zero: clipboard=%q left=%d status=%q", clip(), m.(model).clipLeft, m.(model).statusMsg)
	}
}

func TestSecondCopyIgnoresTicksFromFirst(t *testing.T) {
	clip := fakeClipboard(t)
	m := add(newTestModel(t), "a", "X", "one")
	m = add(m, "b", "X", "two")

	m = press(m, "c")
	first := m.(model).copyGen
	for i := 0; i < 10; i++ {
		m = tick(m, first)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = press(m, "c") // restarts the countdown for "one"
	if m.(model).clipLeft != clipboardSeconds {
		t.Fatalf("left = %d, want restart at %d", m.(model).clipLeft, clipboardSeconds)
	}

	for i := 0; i < clipboardSeconds; i++ {
		m = tick(m, first) // stale ticks from the first copy
	}
	if clip() != "one" || m.(model).clipLeft != clipboardSeconds {
		t.Errorf("stale ticks affected the new copy: clipboard=%q left=%d", clip(), m.(model).clipLeft)
	}
}

func TestClearNowCancelsCountdown(t *testing.T) {
	clip := fakeClipboard(t)
	m := add(newTestModel(t), "a", "X", "s")
	m = press(m, "c")
	gen := m.(model).copyGen

	m = press(m, "x")
	if clip() != "" || m.(model).clipLeft != 0 {
		t.Fatalf("clipboard=%q left=%d after x", clip(), m.(model).clipLeft)
	}

	// Something else is copied by the user; the cancelled countdown must not wipe it.
	_ = writeClipboard("user data")
	m = tick(m, gen)
	if clip() != "user data" {
		t.Errorf("cancelled countdown cleared the clipboard: %q", clip())
	}
}

func TestQuitClearsPendingClipboard(t *testing.T) {
	clip := fakeClipboard(t)
	m := add(newTestModel(t), "a", "X", "s")
	m = press(m, "c")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})

	if clip() != "" {
		t.Errorf("clipboard = %q after quit, want cleared", clip())
	}
	if cmd == nil {
		t.Error("q did not quit")
	}
}

func TestCopyFailureIsReportedAndStartsNoCountdown(t *testing.T) {
	orig := writeClipboard
	writeClipboard = func(string) error { return errors.New("no clipboard tool") }
	t.Cleanup(func() { writeClipboard = orig })

	m := add(newTestModel(t), "a", "X", "s")
	m = press(m, "c")

	mm := m.(model)
	if !mm.statusErr || !strings.Contains(mm.statusMsg, "no clipboard tool") || mm.clipLeft != 0 {
		t.Errorf("status=%q err=%v left=%d, want error and no countdown", mm.statusMsg, mm.statusErr, mm.clipLeft)
	}
}

func TestRevealIsPerItemAndResetsWhenSelectionMoves(t *testing.T) {
	m := add(newTestModel(t), "a", "X", "one")
	m = add(m, "b", "X", "two") // b selected

	m = press(m, "v")
	if m.(model).revealed != "b" || !strings.Contains(m.View(), "two") {
		t.Fatalf("b not revealed: %q", m.(model).revealed)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp}) // a selected
	if strings.Contains(m.View(), "one") {
		t.Error("a's value shown without asking")
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown}) // back on b
	if m.(model).revealed != "" || strings.Contains(m.View(), "two") {
		t.Error("b stayed revealed after leaving and returning")
	}
}

func TestRevealToggleAndAutoHide(t *testing.T) {
	m := add(newTestModel(t), "a", "X", "one")

	m = press(m, "v")
	gen := m.(model).revealGen
	m = press(m, "v")
	if m.(model).revealed != "" {
		t.Fatal("second v should hide")
	}

	m = press(m, "v") // reveal again: older timer (gen) is now stale
	m, _ = m.Update(hideRevealMsg{gen})
	if m.(model).revealed != "a" {
		t.Error("stale auto-hide timer hid a newer reveal")
	}
	m, _ = m.Update(hideRevealMsg{m.(model).revealGen})
	if m.(model).revealed != "" {
		t.Error("current auto-hide timer did not hide the value")
	}
}

func TestRenamingRevealedSecretHidesIt(t *testing.T) {
	m := add(newTestModel(t), "a", "X", "one")
	m = press(m, "v", "e")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = press(m, "z", "enter", "enter", "enter")

	if m.(model).revealed != "" || strings.Contains(m.View(), "one") {
		t.Errorf("revealed = %q after rename", m.(model).revealed)
	}
}
