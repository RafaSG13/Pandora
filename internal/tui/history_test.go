package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/RafaSG13/Pandora/internal/store"
)

// editValue changes the selected secret's value through the form.
func editValue(m tea.Model, value string) tea.Model {
	return press(m, "e", "enter", "enter", value, "tab", "enter", "enter", "enter")
}

// withVersions adds secret "k" holding v1 and edits it through the form to
// v2 ... v<n>, so it has n-1 previous values.
func withVersions(t *testing.T, n int) tea.Model {
	t.Helper()
	t.Cleanup(func() { store.SetHistoryDepth(0) })
	m := add(newTestModel(t), "k", "X", "v1")
	for i := 2; i <= n; i++ {
		m = editValue(m, fmt.Sprintf("v%d", i))
	}
	return m
}

func TestEditingThroughTheFormKeepsThePreviousValueAndHListsIt(t *testing.T) {
	m := withVersions(t, 2)
	if v, _ := store.GetSecretValue("k"); v != "v2" {
		t.Fatalf("value = %q", v)
	}
	if out := m.View(); !strings.Contains(out, "1 previous") {
		t.Errorf("the detail card does not mention the previous value:\n%s", out)
	}

	m = press(m, "H")
	mm := m.(model)
	if mm.mode != modeHistory {
		t.Fatalf("mode = %v, want the history screen", mm.mode)
	}
	out := m.View()
	for _, want := range []string{`History of "k"`, "-1", "replaced"} {
		if !strings.Contains(out, want) {
			t.Errorf("history screen lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "v1") {
		t.Errorf("the previous value is visible before pressing r:\n%s", out)
	}

	m = press(m, "r")
	if out := m.View(); !strings.Contains(out, "v1") || strings.Contains(out, "v2") {
		t.Errorf("r must show the previous value (v1) and not the current one:\n%s", out)
	}
	m = press(m, "r")
	if strings.Contains(m.View(), "v1") {
		t.Error("r did not hide the value again")
	}

	m = press(m, "r", "esc")
	mm = m.(model)
	if mm.mode != modeBrowse || mm.hist.title != "" || mm.hist.value != "" || mm.hist.shown {
		t.Errorf("closing left state behind: %+v (mode %v)", mm.hist, mm.mode)
	}
	if strings.Contains(m.View(), "v1") {
		t.Error("the previous value is still on screen after closing")
	}
}

func TestRestoreFromTheHistoryScreenMakesThatValueCurrent(t *testing.T) {
	m := withVersions(t, 3) // current v3, previous v2 then v1
	m = press(m, "H")
	m = ctrl(m, tea.KeyDown) // choose v1
	m = press(m, "r")
	if out := m.View(); !strings.Contains(out, "v1") {
		t.Fatalf("the chosen version is not shown:\n%s", out)
	}
	m = ctrl(m, tea.KeyUp)
	if m.(model).hist.shown || strings.Contains(m.View(), "v1") {
		t.Error("moving the choice left the revealed value on screen")
	}
	m = ctrl(m, tea.KeyDown)

	m = ctrl(m, tea.KeyEnter)
	mm := m.(model)
	if mm.mode != modeBrowse || mm.hist.title != "" {
		t.Fatalf("restore did not return to the list: mode %v", mm.mode)
	}
	if v, _ := store.GetSecretValue("k"); v != "v1" {
		t.Errorf("value after restore = %q, want v1", v)
	}
	if got, _ := store.GetVersion("k", 1); got != "v3" {
		t.Errorf("the replaced value must be the newest previous one, got %q", got)
	}
	if !strings.Contains(m.View(), "Restored") {
		t.Errorf("no confirmation toast:\n%s", m.View())
	}
	if mm.versions["k"] != 2 {
		t.Errorf("versions = %v, want 2 for k", mm.versions)
	}
}

func TestHistoryScreenWithNothingToShowSaysSo(t *testing.T) {
	m := add(newTestModel(t), "plain", "X", "v")
	m = press(m, "H")
	if mm := m.(model); mm.mode != modeBrowse || !strings.Contains(mm.statusMsg, "No previous values") {
		t.Errorf("mode %v, status %q", mm.mode, mm.statusMsg)
	}
}

func TestHistoryDepthZeroInTheConfigKeepsNothing(t *testing.T) {
	t.Cleanup(func() { store.SetHistoryDepth(0) })
	seed := func(m tea.Model) tea.Model { return add(m, "k", "X", "v1") }
	m := withConfig(t, `{"history_depth":0}`, 100, 30, seed)
	m = editValue(m, "v2")
	if v, _ := store.GetSecretValue("k"); v != "v2" {
		t.Fatalf("value = %q", v)
	}
	m = press(m, "H")
	if m.(model).mode != modeBrowse || !strings.Contains(m.(model).statusMsg, "No previous values") {
		t.Errorf("history_depth 0 still produced a history (mode %v)", m.(model).mode)
	}
}

func TestRevealedPreviousValueHidesAfterTheRevealTime(t *testing.T) {
	m := withVersions(t, 2)
	now := time.Now()
	m = withClock(m, &now)
	m = press(m, "H", "r")
	if !m.(model).hist.shown {
		t.Fatal("r did not reveal")
	}
	now = now.Add(m.(model).cfg.RevealTTL + time.Second)
	m = tickAt(m, now)
	if mm := m.(model); mm.hist.shown || mm.hist.value != "" || strings.Contains(mm.View(), "v1") {
		t.Error("the previous value stayed revealed after the deadline")
	}
}

func TestIdleLockForgetsThePreviousValues(t *testing.T) {
	m := withVersions(t, 2)
	mm := m.(model)
	mm.lockAfter = 5 * time.Minute
	now := time.Now()
	m = withClock(mm, &now)
	m = press(m, "H", "r")
	now = now.Add(6 * time.Minute)
	m = tickAt(m, now)

	mm = m.(model)
	if mm.mode != modeUnlock || mm.hist.title != "" || mm.hist.value != "" || len(mm.hist.infos) != 0 || mm.versions != nil {
		t.Errorf("lock left history state behind: mode %v, %+v, versions %v", mm.mode, mm.hist, mm.versions)
	}
	if out := m.View(); strings.Contains(out, "v1") || strings.Contains(out, `"k"`) {
		t.Errorf("lock screen shows history:\n%s", out)
	}
}

func TestDeleteConfirmationMentionsPreviousValues(t *testing.T) {
	m := withVersions(t, 3)
	m = press(m, "d")
	if out := m.View(); !strings.Contains(out, "2 previous values") {
		t.Errorf("the confirmation does not say previous values are deleted too:\n%s", out)
	}
	m = press(m, "y")
	if _, err := store.GetVersion("k", 1); err == nil {
		t.Error("previous values survived the delete")
	}
}

// The history screen must keep the view exactly as tall as the window and no
// wider, at the smallest size and with more versions than it can list.
func TestHistoryScreenFitsTheTerminal(t *testing.T) {
	long := strings.Repeat("a very long previous value ", 8) + "\nsecond line"
	for _, size := range [][2]int{{60, 18}, {80, 24}, {100, 30}} {
		t.Cleanup(func() { store.SetHistoryDepth(0) })
		m := newModelFor(t, true, size[0], size[1])
		m = add(m, "k", "X", long)
		store.SetHistoryDepth(20)
		for i := range 12 {
			m = editValue(m, fmt.Sprintf("value %d %s", i, long))
		}
		m = press(m, "H")
		for step := range 14 {
			m = press(m, "r")
			out := m.View()
			if h := lipgloss.Height(out); h != size[1] {
				t.Fatalf("%dx%d step %d: view is %d lines tall\n%s", size[0], size[1], step, h, out)
			}
			if w := lipgloss.Width(out); w > size[0] {
				t.Fatalf("%dx%d step %d: view is %d columns wide", size[0], size[1], step, w)
			}
			m = ctrl(m, tea.KeyDown)
		}
		if got := m.(model).hist.cursor; got != 11 {
			t.Errorf("%dx%d: cursor = %d, want it clamped to the last of 12 versions", size[0], size[1], got)
		}
	}
}
