package tui

import (
	"regexp"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/RafaSG13/Pandora/internal/store"
)

func pasteText(m tea.Model, s string) tea.Model {
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s), Paste: true})
	return m
}

// pemBlock is a multi-line value of about size bytes with a trailing newline.
func pemBlock(size int) string {
	var b strings.Builder
	b.WriteString("-----BEGIN PRIVATE KEY-----\n")
	for i := 0; b.Len() < size; i++ {
		b.WriteString(strings.Repeat(string(rune('A'+i%26)), 64) + "\n")
	}
	b.WriteString("-----END PRIVATE KEY-----\n")
	return b.String()
}

// openSecretField opens the add form with name filled in and the secret focused.
func openSecretField(m tea.Model, name string) tea.Model {
	return press(m, "n", name, "enter", "Keys", "enter")
}

func TestTypedMultilineSecretIsSavedWithItsNewlines(t *testing.T) {
	m := openSecretField(newTestModel(t), "ssh")
	m = press(m, "line one", "enter", "line two", "enter", "line three")
	m = ctrl(m, tea.KeyCtrlS)

	if m.(model).mode != modeBrowse {
		t.Fatalf("ctrl+s did not save: %q", m.(model).statusMsg)
	}
	if v, _ := store.GetSecretValue("ssh"); v != "line one\nline two\nline three" {
		t.Errorf("stored %q", v)
	}

	m = press(m, "v")
	out := m.View()
	for _, want := range []string{"line one", "line two", "line three"} {
		if !strings.Contains(out, want) {
			t.Errorf("revealed value lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(m.(model).View(), "line one") && m.(model).revealValue == "" {
		t.Error("revealed text shown without being kept for the reveal window")
	}
}

func TestPastedKeyIsNotTruncatedByTheEditor(t *testing.T) {
	long := strings.Repeat("a", 900)                // wider than the editor's old 500-column cap
	key := pemBlock(12000) + "token:" + long + "\n" // far more than its old 99-row cap
	m := openSecretField(newTestModel(t), "big")
	m = pasteText(m, key)
	m = ctrl(m, tea.KeyCtrlS)

	got, err := store.GetSecretValue("big")
	if err != nil || got != key {
		t.Fatalf("pasted %d bytes, stored %d (err %v)", len(key), len(got), err)
	}
}

func TestSecretFieldKeepsTextMaskedUntilRevealed(t *testing.T) {
	m := newModelFor(t, true, 60, 18)
	m = openSecretField(m, "k")
	m = press(m, "alpha-secret", "enter", "beta-secret", "enter", "gamma-secret")

	out := m.View()
	if strings.Contains(out, "secret") && strings.Contains(out, "alpha") {
		t.Fatalf("secret text on screen while masked:\n%s", out)
	}
	if !strings.Contains(out, "(3 lines)") {
		t.Errorf("masked box does not say how many lines it holds:\n%s", out)
	}

	m = ctrl(m, tea.KeyCtrlR)
	if out := m.View(); !strings.Contains(out, "gamma-secret") {
		t.Errorf("ctrl+r did not show the text:\n%s", out)
	}
	m = ctrl(m, tea.KeyCtrlR)
	if strings.Contains(m.View(), "gamma-secret") {
		t.Error("ctrl+r did not hide the text again")
	}
}

func TestUpDownMoveInsideTheSecretBeforeLeavingIt(t *testing.T) {
	m := openSecretField(newTestModel(t), "k")
	m = press(m, "one", "enter", "two", "enter", "three")
	up := func(m tea.Model) tea.Model { return ctrl(m, tea.KeyUp) }

	m = up(m)
	if m.(model).focusIndex != fieldSecret || m.(model).secret.Line() != 1 {
		t.Fatalf("first up: focus %d line %d; want to stay in the secret on line 1", m.(model).focusIndex, m.(model).secret.Line())
	}
	m = up(up(m))
	if m.(model).focusIndex != fieldCategory {
		t.Errorf("up from the first line: focus %d, want the field above the secret", m.(model).focusIndex)
	}

	m = press(m, "tab") // back to the secret; its cursor is on the first line
	for range 2 {
		m = ctrl(m, tea.KeyDown)
	}
	if m.(model).focusIndex != fieldSecret {
		t.Fatal("down inside the secret left it too early")
	}
	m = ctrl(m, tea.KeyDown) // past the last line
	if m.(model).focusIndex != fieldExpires {
		t.Errorf("down from the last line: focus %d, want Expires", m.(model).focusIndex)
	}
}

func TestEditKeepsMultilineValueWhenSecretIsLeftEmpty(t *testing.T) {
	m := openSecretField(newTestModel(t), "ssh")
	m = pasteText(m, "a\nb\nc\n")
	m = ctrl(m, tea.KeyCtrlS)

	m = press(m, "e", "tab", "tab", "tab", "tab") // walk past the empty secret
	m = ctrl(m, tea.KeyCtrlS)
	if v, _ := store.GetSecretValue("ssh"); v != "a\nb\nc\n" {
		t.Errorf("value after a metadata-only edit: %q", v)
	}

	m = press(m, "e", "tab", "tab")
	m = pasteText(m, "x\ny")
	m = ctrl(m, tea.KeyCtrlS)
	if v, _ := store.GetSecretValue("ssh"); v != "x\ny" {
		t.Errorf("value after replacing it: %q", v)
	}
}

func TestOversizedValueKeepsTheFormOpenWithAClearError(t *testing.T) {
	m := openSecretField(newTestModel(t), "huge")
	m = pasteText(m, strings.Repeat("é", 20000)) // 20000 characters, 40000 bytes
	m = ctrl(m, tea.KeyCtrlS)

	mm := m.(model)
	if mm.mode != modeForm || !mm.statusErr || !strings.Contains(mm.statusMsg, "larger than 32 KB") {
		t.Errorf("mode %v, status %q; want the form open with the size error", mm.mode, mm.statusMsg)
	}
	if got := titles(m); len(got) != 0 {
		t.Errorf("titles = %v after a rejected save", got)
	}
}

func TestRevealedValueIsDroppedWhenHiddenExpiredOrLocked(t *testing.T) {
	m := openSecretField(newTestModel(t), "k")
	m = pasteText(m, "s3cr3t-value")
	m = ctrl(m, tea.KeyCtrlS)
	now := time.Now()
	mm := m.(model)
	mm.lockAfter = 10 * time.Minute
	m = withClock(mm, &now)

	m = press(m, "v")
	if m.(model).revealValue != "s3cr3t-value" {
		t.Fatal("reveal did not read the value")
	}
	m = press(m, "v")
	if m.(model).revealValue != "" {
		t.Error("hiding kept the plaintext in the model")
	}

	m = press(m, "v")
	now = now.Add(revealTTL + time.Second)
	m = tickAt(m, now)
	if m.(model).revealValue != "" || strings.Contains(m.View(), "s3cr3t-value") {
		t.Error("auto-hide kept the plaintext")
	}

	m = press(m, "v")
	now = now.Add(11 * time.Minute)
	m = tickAt(m, now)
	if m.(model).mode != modeUnlock || m.(model).revealValue != "" {
		t.Errorf("lock: mode %v, plaintext kept: %v", m.(model).mode, m.(model).revealValue != "")
	}
}

func TestMultilineLayoutKeepsTheWindowHeight(t *testing.T) {
	notes := "see the wiki page for how to rotate this key and who to tell afterwards"
	for _, size := range [][2]int{{60, 18}, {80, 24}, {100, 30}, {120, 40}} {
		w, h := size[0], size[1]
		m := newModelFor(t, true, w, h)
		if err := store.SaveSecret(store.SecretMeta{Title: "ssh", Category: "Keys", Created: "2026-01-01", Notes: notes}, pemBlock(5000)); err != nil {
			t.Fatal(err)
		}
		m = press(m, "r")

		check := func(name string, m tea.Model) {
			out := m.(model).view()
			if got := lipgloss.Height(out); got != h {
				t.Errorf("%dx%d %s: view is %d lines, want %d\n%s", w, h, name, got, h, out)
			}
			if got := lipgloss.Width(out); got > w {
				t.Errorf("%dx%d %s: view is %d columns wide", w, h, name, got)
			}
		}

		check("list", m)
		m = press(m, "v") // reveal 5 KB
		check("revealed", m)
		if out := m.View(); !regexp.MustCompile(`\+\d+ (more )?lines`).MatchString(out) {
			t.Errorf("%dx%d: a long revealed value does not say it is cut:\n%s", w, h, out)
		}
		m = press(m, "v", "e", "tab", "tab")
		m = pasteText(m, pemBlock(3000))
		check("form masked", m)
		m = ctrl(m, tea.KeyCtrlR)
		check("form shown", m)
	}
}
