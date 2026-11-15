package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/zalando/go-keyring"

	"github.com/RafaSG13/Pandora/internal/store"
)

func TestStartsLockedWhenVaultExistsAndLoadsNothing(t *testing.T) {
	m := newTestModel(t)
	if err := store.SaveSecret(store.SecretMeta{Title: "stored-title", Category: "X", Created: "2026-01-01"}, "v"); err != nil {
		t.Fatal(err)
	}
	store.Lock()

	fresh, _ := New().Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if fm := fresh.(model); fm.mode != modeUnlock || len(fm.list.Items()) != 0 {
		t.Fatalf("mode %v with %d items; want the unlock screen with an empty list", fm.mode, len(fm.list.Items()))
	}
	if out := fresh.View(); strings.Contains(out, "stored-title") || !strings.Contains(out, "Locked") {
		t.Errorf("unlock screen wrong:\n%s", out)
	}

	fresh = submitPassword(t, fresh, testPassword)
	if got := titles(fresh); len(got) != 1 || got[0] != "stored-title" {
		t.Errorf("titles after unlock = %v", got)
	}
	_ = m
}

func TestSetupCreatesVaultAfterValidation(t *testing.T) {
	m := newModelFor(t, false, 100, 30)
	if mm := m.(model); mm.mode != modeSetup || !strings.Contains(m.View(), "Create a master password") {
		t.Fatalf("mode %v; want the setup screen:\n%s", mm.mode, m.View())
	}

	// Too short.
	m = press(m, "short", "enter") // enter on the first field moves to the confirmation
	m = submitPassword(t, m, "short")
	if mm := m.(model); mm.mode != modeSetup || store.Exists() || !strings.Contains(mm.View(), "at least 10") {
		t.Fatalf("short password accepted or not reported: mode %v exists %v", mm.mode, store.Exists())
	}

	// Mismatch.
	m = typeBoth(m, "long enough password", "another long password")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if mm := m.(model); mm.mode != modeSetup || store.Exists() || !strings.Contains(mm.View(), "do not match") {
		t.Fatalf("mismatch accepted or not reported: mode %v exists %v", mm.mode, store.Exists())
	}
	if mm := m.(model); mm.pwField != 0 || mm.pwInput.Value() != "" || mm.pwConfirm.Value() != "" {
		t.Fatalf("after a mismatch the form must start over: field %d, password %q, confirm %q", mm.pwField, mm.pwInput.Value(), mm.pwConfirm.Value())
	}

	// Valid, typed straight into the fresh form (no manual reset).
	m = press(m, "long enough password", "enter", "long enough password")
	m = enterAndRun(t, m)
	if m.(model).mode != modeBrowse || !store.Exists() || !store.IsUnlocked() {
		t.Fatalf("setup did not finish: mode %v exists %v unlocked %v", m.(model).mode, store.Exists(), store.IsUnlocked())
	}
	store.Lock()
	if err := store.Unlock("long enough password"); err != nil {
		t.Errorf("created vault does not open with the chosen password: %v", err)
	}
}

func TestPastedPasswordWithTrailingNewlineStillMatches(t *testing.T) {
	m := newModelFor(t, false, 100, 30)
	paste := func(m tea.Model, s string) tea.Model {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s), Paste: true})
		return m
	}
	m = paste(m, "long enough password\n") // e.g. copied from a file or a manager
	m = press(m, "enter")
	m = paste(m, "long enough password")
	m = enterAndRun(t, m)
	if m.(model).mode != modeBrowse || !store.IsUnlocked() {
		t.Fatalf("mode %v unlocked %v; a trailing newline in a paste must not cause a mismatch", m.(model).mode, store.IsUnlocked())
	}
	store.Lock()
	if err := store.Unlock("long enough password"); err != nil {
		t.Errorf("password stored with the newline: %v", err)
	}
}

// typeBoth resets both setup fields and types pw and confirm into them.
func typeBoth(m tea.Model, pw, confirm string) tea.Model {
	mm := m.(model)
	mm.pwInput.SetValue("")
	mm.pwConfirm.SetValue("")
	mm.unlockErr = ""
	mm.focusPassword(0)
	m = mm
	m = press(m, pw, "enter") // enter moves to the confirmation field
	return press(m, confirm)
}

// enterAndRun presses enter and feeds the unlock result back to the model.
func enterAndRun(t *testing.T, m tea.Model) tea.Model {
	t.Helper()
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter produced no command")
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok {
		t.Fatal("expected a batch of commands")
	}
	for _, c := range batch {
		if res, ok := c().(unlockedMsg); ok {
			m, _ = m.Update(res)
		}
	}
	return m
}

func TestSetupMigratesLegacyStore(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	t.Setenv("AppData", dir)
	store.FastKDFForTests()
	store.Lock()
	t.Cleanup(store.Lock)

	cfg, _ := os.UserConfigDir()
	legacy := filepath.Join(cfg, "pandora", "metadata.json")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte(`[{"title":"one","category":"X","created":"2025-01-01"},{"title":"two","category":"X","created":"2025-01-02"}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := keyring.Set(store.ServiceName, "one", "old-value"); err != nil {
		t.Fatal(err)
	}

	m, _ := New().Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if out := m.View(); !strings.Contains(out, "Migrate 2 secrets to an encrypted vault") {
		t.Fatalf("migration heading missing:\n%s", out)
	}
	m = typeBoth(m, "long enough password", "long enough password")
	m = enterAndRun(t, m)

	if m.(model).mode != modeBrowse || len(titles(m)) != 2 {
		t.Fatalf("mode %v titles %v; want browse with both secrets", m.(model).mode, titles(m))
	}
	if v, err := store.GetSecretValue("one"); err != nil || v != "old-value" {
		t.Errorf("migrated value = %q, %v", v, err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Errorf("metadata.json still present: %v", err)
	}
}

func TestAuthScreensFitSmallTerminalsAndKeepWindowHeight(t *testing.T) {
	for _, size := range [][2]int{{60, 18}, {80, 24}, {100, 30}} {
		w, h := size[0], size[1]
		setup := newModelFor(t, false, w, h)
		migrating := setup.(model)
		migrating.migrating, migrating.legacyN = true, 12
		withErr := setup.(model)
		withErr.unlockErr = "✗ Passwords do not match"
		withErr.pwField = 1

		locked := newModelFor(t, true, w, h).(model)
		locked.lock()

		for name, m := range map[string]model{"setup": setup.(model), "migrating": migrating, "setup error": withErr, "unlock": locked} {
			out := m.view()
			if got := lipgloss.Height(out); got != h {
				t.Errorf("%s %dx%d: view is %d lines, want %d\n%s", name, w, h, got, h, out)
			}
			if got := lipgloss.Width(out); got > w {
				t.Errorf("%s %dx%d: view is %d columns wide", name, w, h, got)
			}
		}
	}
}

func TestRevealTogglesWhatIsTypedAndResetsOnLock(t *testing.T) {
	m := newModelFor(t, false, 60, 18)
	m = press(m, "visible-pass-123")
	if out := m.View(); strings.Contains(out, "visible-pass-123") {
		t.Fatalf("password shown before asking for it:\n%s", out)
	}
	m = ctrl(m, tea.KeyCtrlR)
	out := m.View()
	if !strings.Contains(out, "visible-pass-123") || !strings.Contains(out, "ctrl+r hide") {
		t.Fatalf("ctrl+r did not reveal the typed text:\n%s", out)
	}
	if !strings.Contains(out, "type it again") || !strings.Contains(out, "[") {
		t.Errorf("the repeat box lacks its hint or brackets:\n%s", out)
	}
	m = ctrl(m, tea.KeyCtrlR)
	if strings.Contains(m.View(), "visible-pass-123") {
		t.Error("ctrl+r did not hide the text again")
	}

	// A revealed box is masked again after an idle lock.
	locked := newModelFor(t, true, 60, 18).(model)
	locked.setReveal(true)
	locked.lock()
	if locked.pwShow || locked.pwInput.EchoMode != 1 { // 1 = textinput.EchoPassword
		t.Errorf("lock left the password box revealed: show=%v echo=%v", locked.pwShow, locked.pwInput.EchoMode)
	}
}
