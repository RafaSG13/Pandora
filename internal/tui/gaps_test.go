package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/RafaSG13/Pandora/internal/store"
)

// seed stores a secret directly and reloads the list.
func seed(t *testing.T, m tea.Model, meta store.SecretMeta, value string) tea.Model {
	t.Helper()
	meta.Created = "2026-01-02"
	if err := store.SaveSecret(meta, value); err != nil {
		t.Fatal(err)
	}
	return press(m, "r")
}

func numberedLines(n int) string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("L%02d", i+1)
	}
	return strings.Join(lines, "\n")
}

func TestLockKeyLocksAtOnce(t *testing.T) {
	m := add(newTestModel(t), "a", "X", "v")
	m = press(m, "L")
	mm := m.(model)
	if mm.mode != modeUnlock || len(mm.list.Items()) != 0 || store.IsUnlocked() {
		t.Errorf("after L: mode %v, %d items, unlocked %v; want the unlock screen with nothing loaded", mm.mode, len(mm.list.Items()), store.IsUnlocked())
	}
}

func TestEscOnAnEditedFormAsksBeforeDiscarding(t *testing.T) {
	m := press(newTestModel(t), "n", "x", "esc")
	if mm := m.(model); mm.mode != modeForm || mm.statusMsg == "" {
		t.Fatalf("first esc: mode %v status %q; want the form kept with a warning", mm.mode, mm.statusMsg)
	}
	if m = press(m, "esc"); m.(model).mode != modeBrowse {
		t.Error("the second esc did not discard the form")
	}

	if m = press(newTestModel(t), "n", "esc"); m.(model).mode != modeBrowse {
		t.Error("esc on an untouched form should close it at once")
	}

	m = press(newTestModel(t), "n", "x", "esc", "y", "esc")
	if m.(model).mode != modeForm {
		t.Error("typing after the warning must re-arm the guard")
	}
}

func TestEscOnAnUntouchedEditFormClosesAtOnce(t *testing.T) {
	m := press(add(newTestModel(t), "a", "X", "v"), "e", "esc")
	if m.(model).mode != modeBrowse {
		t.Error("esc on an edit form with no changes should close it at once")
	}
	m = press(m, "e", "z", "esc")
	if m.(model).mode != modeForm {
		t.Error("esc after editing the name should warn first")
	}
}

func TestCtrlCOnAnEditedFormNeedsConfirmation(t *testing.T) {
	m := press(newTestModel(t), "n", "x")
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd != nil {
		if _, quit := cmd().(tea.QuitMsg); quit {
			t.Fatal("the first ctrl+c quit with unsaved changes")
		}
	}
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("the second ctrl+c did nothing")
	}
	if _, quit := cmd().(tea.QuitMsg); !quit {
		t.Error("the second ctrl+c did not quit")
	}
}

func TestLockClearsTheFormSnapshot(t *testing.T) {
	m := press(add(newTestModel(t), "visible-name", "X", "v"), "e")
	mm := m.(model)
	mm.lock()
	for i, v := range mm.formInitial {
		if v != "" {
			t.Errorf("formInitial[%d] = %q survived the lock", i, v)
		}
	}
}

func TestCopyUsernameUsesThePlainClipboardAndDropsTheSecretTimer(t *testing.T) {
	m := press(newTestModel(t), "n", "site", "enter", "X", "enter", "alice", "enter", "hunter2", "tab", "enter", "enter", "enter")
	m = press(m, "c")
	if m.(model).clipUntil.IsZero() {
		t.Fatal("copying the secret should arm the clipboard timer")
	}
	m = press(m, "u")
	if got := clipboardLog[len(clipboardLog)-1]; got != "alice" {
		t.Errorf("clipboard = %q, want the username", got)
	}
	if !m.(model).clipUntil.IsZero() {
		t.Error("the secret is no longer on the clipboard: its clear deadline should be dropped")
	}
}

func TestCopyUsernameWithoutOneWritesNothing(t *testing.T) {
	m := add(newTestModel(t), "a", "X", "v")
	before := len(clipboardLog)
	m = press(m, "u")
	if len(clipboardLog) != before {
		t.Errorf("clipboard written %v for a secret without a username", clipboardLog[before:])
	}
	if m.(model).statusMsg == "" {
		t.Error("no toast explaining there is no username")
	}
}

func TestFilterFindsAUsername(t *testing.T) {
	m := seed(t, newTestModel(t), store.SecretMeta{Title: "alpha", Username: "zed.operator"}, "v")
	m = seed(t, m, store.SecretMeta{Title: "bravo", Username: "someone"}, "v")
	for _, term := range []string{"user:zed", "zed.op"} {
		if got := visibleTitles(filterBy(m, term)); !sameStrings(got, []string{"alpha"}) {
			t.Errorf("filter %q shows %v, want only alpha", term, got)
		}
	}
}

func TestViewerShowsTheWholeValueAndScrolls(t *testing.T) {
	m := seed(t, newTestModel(t), store.SecretMeta{Title: "long"}, numberedLines(40))
	m = press(m, "V")
	if m.(model).mode != modeView || !strings.Contains(m.View(), "L01") {
		t.Fatalf("V did not open the viewer on the first line:\n%s", m.View())
	}
	if strings.Contains(m.View(), "L40") {
		t.Error("the end of a 40-line value should need scrolling at 100×30")
	}
	m = press(m, "G")
	if !strings.Contains(m.View(), "L40") || strings.Contains(m.View(), "L01") {
		t.Errorf("G did not scroll to the end:\n%s", m.View())
	}
	m = press(m, "esc")
	if m.(model).mode != modeBrowse {
		t.Error("esc did not close the viewer")
	}
}

func TestViewerClosesWhenTheRevealExpires(t *testing.T) {
	now := time.Now()
	m := withClock(seed(t, newTestModel(t), store.SecretMeta{Title: "long"}, numberedLines(5)), &now)
	m = press(m, "V")
	if m.(model).mode != modeView {
		t.Fatal("the viewer did not open")
	}
	now = now.Add(m.(model).cfg.RevealTTL + time.Second)
	m = tickAt(m, now)
	if mm := m.(model); mm.mode != modeBrowse || mm.revealValue != "" {
		t.Errorf("after the reveal time: mode %v, value kept %v; want the viewer closed and the value dropped", mm.mode, mm.revealValue != "")
	}
}

func TestViewerKeysRenewTheRevealDeadline(t *testing.T) {
	now := time.Now()
	m := withClock(seed(t, newTestModel(t), store.SecretMeta{Title: "long"}, numberedLines(40)), &now)
	m = press(m, "V")
	ttl := m.(model).cfg.RevealTTL
	now = now.Add(ttl - time.Second)
	m = press(m, "j")
	now = now.Add(ttl - time.Second)
	if m = tickAt(m, now); m.(model).mode != modeView {
		t.Error("a key press should keep the viewer open for another reveal time")
	}
}

func TestLockWhileViewingDropsTheValue(t *testing.T) {
	m := seed(t, newTestModel(t), store.SecretMeta{Title: "long"}, numberedLines(40))
	m = press(press(m, "V"), "j", "j")
	mm := m.(model)
	mm.lock()
	if mm.revealValue != "" || mm.viewTop != 0 || mm.mode != modeUnlock {
		t.Errorf("after lock: value kept %v, viewTop %d, mode %v", mm.revealValue != "", mm.viewTop, mm.mode)
	}
}

func seedCategories(t *testing.T, n int) tea.Model {
	t.Helper()
	m := newTestModel(t)
	for i := 1; i <= n; i++ {
		m = seed(t, m, store.SecretMeta{Title: fmt.Sprintf("s%02d", i), Category: fmt.Sprintf("C%02d", i)}, "v")
	}
	return m
}

func TestPickerReachesCategoriesBeyondTheNinth(t *testing.T) {
	m := press(seedCategories(t, 11), "f")
	if m.(model).mode != modePicker {
		t.Fatal("f did not open the picker")
	}
	m = press(m, "G", "enter")
	if mm := m.(model); mm.mode != modeBrowse || !sameStrings(visibleTitles(m), []string{"s11"}) {
		t.Errorf("after picking C11: mode %v, list %v; want only s11", mm.mode, visibleTitles(m))
	}
	m = press(m, "f", "enter") // the cursor starts on the active category: picking it again clears it
	if got := visibleTitles(m); len(got) != 11 {
		t.Errorf("picking the active category again left %v, want all 11", got)
	}
}

func TestPickerNeedsACategoryOrTag(t *testing.T) {
	m := press(add(newTestModel(t), "a", "", "v"), "f")
	if m.(model).mode != modeBrowse {
		t.Error("the picker opened with no categories or tags to pick from")
	}
}

func TestPickerCombinesCategoryAndTag(t *testing.T) {
	m := newTestModel(t)
	m = seed(t, m, store.SecretMeta{Title: "a", Category: "X", Tags: []string{"prod"}}, "v")
	m = seed(t, m, store.SecretMeta{Title: "b", Category: "Y", Tags: []string{"prod"}}, "v")
	m = seed(t, m, store.SecretMeta{Title: "c", Category: "X"}, "v")
	m = press(m, "1")      // category X
	m = press(m, "f", "G") // the last entry is the tag
	m = press(m, "enter")
	if got := visibleTitles(m); !sameStrings(got, []string{"a"}) {
		t.Errorf("category X + tag prod shows %v, want only a", got)
	}
}

func TestQualifiedFilter(t *testing.T) {
	items := []store.SecretMeta{
		{Title: "db-main", Category: "Infra", Tags: []string{"prod", "db"}, Username: "root"},
		{Title: "db-test", Category: "Infra", Tags: []string{"staging", "db"}},
		{Title: "web", Category: "Apps", Tags: []string{"prod"}, Username: "deploy", Notes: "foo:bar"},
	}
	targets := make([]string, len(items))
	for i, it := range items {
		targets[i] = item{meta: it}.FilterValue()
	}
	indexes := func(term string) []int {
		var out []int
		for _, r := range qualifiedFilter(term, targets) {
			out = append(out, r.Index)
		}
		return out
	}
	tests := []struct {
		term string
		want []int
	}{
		{"tag:prod", []int{0, 2}},
		{"TAG:PROD main", []int{0}},
		{"cat:infra", []int{0, 1}},
		{"category:app", []int{2}},
		{"user:ro", []int{0}},
		{"tag:prod cat:apps", []int{2}},
		{"tag:", []int{0, 1, 2}},
		{"cat:", []int{0, 1, 2}},
		{"tag:nothing", nil},
		{"foo:bar", []int{2}}, // not a qualifier: free text, found in the notes
	}
	for _, tt := range tests {
		got := indexes(tt.term)
		if fmt.Sprint(got) != fmt.Sprint(tt.want) {
			t.Errorf("filter %q keeps %v, want %v", tt.term, got, tt.want)
		}
	}
}
