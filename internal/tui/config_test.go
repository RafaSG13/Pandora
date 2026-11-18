package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/RafaSG13/Pandora/internal/config"
	"github.com/RafaSG13/Pandora/internal/gen"
	"github.com/RafaSG13/Pandora/internal/store"
)

// writeConfig puts content in config.json inside the (temp) config directory.
func writeConfig(t *testing.T, content string) {
	t.Helper()
	dir, err := store.ConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, config.FileName), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// configuredModel starts a model, with an unlocked vault, whose config.json
// holds content. The idle lock is left as configured.
func configuredModel(t *testing.T, content string) tea.Model {
	t.Helper()
	newModelFor(t, true, 100, 30) // isolates HOME, keyring and clipboard; creates the vault
	writeConfig(t, content)
	origGlyphs, origPane, origModal := g, paneStyle, modalStyle
	t.Cleanup(func() { g, paneStyle, modalStyle = origGlyphs, origPane, origModal })
	m, _ := New().Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return m
}

func TestRevealSecondsFromConfig(t *testing.T) {
	m := add(configuredModel(t, `{"reveal_seconds": 3}`), "a", "X", "hunter2")
	now := time.Now()
	m = withClock(m, &now)
	m = press(m, "v")
	now = now.Add(2 * time.Second)
	if m = tickAt(m, now); m.(model).revealTitle == "" {
		t.Fatal("hidden after 2 s, before the configured 3 s")
	}
	now = now.Add(2 * time.Second)
	if m = tickAt(m, now); m.(model).revealTitle != "" {
		t.Error("still revealed 4 s after revealing, with reveal_seconds=3 (the default is 10)")
	}
}

func TestClipboardSecondsFromConfig(t *testing.T) {
	m := add(configuredModel(t, `{"clipboard_seconds": 40}`), "a", "X", "val")
	t0 := time.Now()
	now := t0
	m = withClock(m, &now)
	m = press(m, "c")
	if got := m.(model).clipUntil.Sub(t0); got != 40*time.Second {
		t.Errorf("clipboard clears after %v, want 40s", got)
	}
	now = t0.Add(30 * time.Second) // past the default 15 s, before 40 s
	m = tickAt(m, now)
	if last := clipboardLog[len(clipboardLog)-1]; last != "val" {
		t.Errorf("clipboard cleared early (last write %q)", last)
	}
	now = t0.Add(41 * time.Second)
	m = tickAt(m, now)
	if last := clipboardLog[len(clipboardLog)-1]; last != "" {
		t.Errorf("clipboard not cleared after the configured time (last write %q)", last)
	}
}

func TestExpiryWarnDaysFromConfig(t *testing.T) {
	in20 := time.Now().AddDate(0, 0, 20).Format(store.DateLayout)
	for _, c := range []struct {
		name, json string
		want       int
	}{
		{"default window is 14 days", ``, 0},
		{"30-day window warns", `{"expiry_warn_days": 30}`, 1},
	} {
		m := configuredModel(t, c.json)
		m = press(m, "n", "a", "enter", "X", "enter", "v", "tab", in20, "enter", "", "enter", "", "enter")
		if got := m.(model).expiring; got != c.want {
			t.Errorf("%s: %d expiring, want %d", c.name, got, c.want)
		}
	}
}

func TestSortCompactAndGeneratorDefaultsFromConfig(t *testing.T) {
	m := configuredModel(t, `{"sort": "category", "compact": true, "gen_length": 48, "gen_charset": "hex"}`).(model)
	if m.sort != sortCategory || !m.compact {
		t.Errorf("sort=%v compact=%v, want category and compact", m.sort, m.compact)
	}
	var tm tea.Model = press(m, "n", "k", "enter", "X", "enter")
	tm = ctrl(tm, tea.KeyCtrlG)
	val := tm.(model).secret.Value()
	if len(val) != 48 || strings.Trim(val, "0123456789abcdef") != "" {
		t.Errorf("generated %q, want 48 hex characters", val)
	}
	if gen.Lengths[tm.(model).genLen] != 48 {
		t.Errorf("form default length index %d, want the 48 preset", tm.(model).genLen)
	}
}

func TestASCIIFromConfigAppliesToBordersToo(t *testing.T) {
	m := add(configuredModel(t, `{"ascii": true}`), "a", "X", "val")
	if !g.ascii {
		t.Fatal("ascii=true in config.json did not switch to the ASCII glyphs")
	}
	for name, view := range map[string]string{
		"browse": m.View(),
		"form":   press(m, "n").View(),
		"help":   press(m, "?").View(),
	} {
		for _, r := range view {
			if r > 127 {
				t.Errorf("%s view has the non-ASCII character %q", name, r)
				break
			}
		}
	}
}

func TestLockMinutesFileAndEnvironment(t *testing.T) {
	m := configuredModel(t, `{"lock_minutes": 2}`).(model)
	if m.lockAfter != 2*time.Minute {
		t.Errorf("lock after %v, want 2m from the file", m.lockAfter)
	}
	t.Setenv("PANDORA_LOCK_MINUTES", "0")
	if m := configuredModel(t, `{"lock_minutes": 2}`).(model); m.lockAfter != 0 {
		t.Errorf("lock after %v, want the environment (0 = off) to beat the file", m.lockAfter)
	}
}

func TestConfigProblemsAreShownOnceAndPandoraStillStarts(t *testing.T) {
	m := configuredModel(t, `{"clipboard_seconds": 0, "bogus": 1}`).(model)
	if !strings.Contains(m.statusMsg, "config.json") || !m.statusErr || !strings.Contains(m.statusMsg, "+1 more") {
		t.Fatalf("status %q (err=%v), want a config.json notice with the count of the rest", m.statusMsg, m.statusErr)
	}
	if m.clipTTL != config.DefaultClipboardSeconds*time.Second {
		t.Errorf("bad value was not replaced by the default: %v", m.clipTTL)
	}
	now := time.Now()
	var tm tea.Model = withClock(m, &now)
	now = now.Add(configToastTTL + time.Second)
	if tm = tickAt(tm, now); tm.(model).statusMsg != "" {
		t.Errorf("config notice still visible after %v: %q", configToastTTL, tm.(model).statusMsg)
	}
}

func TestConfigProblemsWaitForTheUnlockScreenToPass(t *testing.T) {
	newModelFor(t, true, 100, 30)
	writeConfig(t, `{"reveal_seconds": 999}`)
	store.Lock()
	m, _ := New().Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if m.(model).mode != modeUnlock || m.(model).statusMsg != "" {
		t.Fatalf("mode=%v status=%q, want the unlock screen with no notice yet", m.(model).mode, m.(model).statusMsg)
	}
	m = submitPassword(t, m, testPassword)
	if mm := m.(model); mm.mode != modeBrowse || !strings.Contains(mm.statusMsg, "reveal_seconds") {
		t.Errorf("after unlock: mode=%v status=%q, want the config notice", mm.mode, mm.statusMsg)
	}
}
