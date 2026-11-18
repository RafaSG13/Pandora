package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/RafaSG13/Pandora/internal/store"
)

// writeConfig writes config.json into the isolated config dir that
// newModelFor set up, and keeps the developer's environment out of New().
func writeConfig(t *testing.T, content string) {
	t.Helper()
	t.Setenv("PANDORA_LOCK_MINUTES", "")
	t.Setenv("PANDORA_ASCII", "")
	dir, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "pandora"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pandora", "config.json"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// restoreGlyphs puts the glyph set (and the borders drawn with it) back after a test.
func restoreGlyphs(t *testing.T) {
	t.Helper()
	prev := g.ascii
	t.Cleanup(func() { useGlyphs(prev) })
}

// withConfig returns a fresh model started with the given config.json, on a
// vault that already holds the secrets added by seed (which runs without it).
func withConfig(t *testing.T, content string, w, h int, seed func(tea.Model) tea.Model) tea.Model {
	t.Helper()
	m := newModelFor(t, true, w, h)
	if seed != nil {
		seed(m)
	}
	writeConfig(t, content)
	m, _ = New().Update(tea.WindowSizeMsg{Width: w, Height: h})
	mm := m.(model)
	mm.lockAfter = 0
	return mm
}

func TestConfigSetsInitialSortAndCompactRows(t *testing.T) {
	seed := func(m tea.Model) tea.Model {
		m = add(m, "zeta", "X", "1")
		return add(m, "alpha", "X", "2") // newest first by default: alpha, zeta
	}
	m := withConfig(t, `{"compact":true,"sort":"name"}`, 100, 30, seed)
	mm := m.(model)
	if !mm.compact || mm.rowsPer() != compactRows || mm.delegate().Height() != compactRows {
		t.Errorf("compact = %v, rows per item = %d", mm.compact, mm.rowsPer())
	}
	if mm.sort != sortName {
		t.Errorf("sort = %v, want name", mm.sort)
	}
	if got := titles(m); len(got) != 2 || got[0] != "alpha" || got[1] != "zeta" {
		t.Errorf("order = %v, want alpha, zeta", got)
	}

	roomy := withConfig(t, `{"sort":"created"}`, 100, 30, seed)
	if got := titles(roomy); got[0] != "zeta" || roomy.(model).compact {
		t.Errorf("created sort / roomy rows not kept: %v compact=%v", got, roomy.(model).compact)
	}
}

func TestConfigASCIIDrawsPlainGlyphsAndAsciiFalseOverridesLocale(t *testing.T) {
	restoreGlyphs(t)
	seed := func(m tea.Model) tea.Model { return addThree(m) }
	m := withConfig(t, `{"ascii":true}`, 100, 30, seed)
	if !g.ascii {
		t.Fatal(`"ascii": true did not switch the glyph set`)
	}
	out := m.View()
	for _, r := range out {
		if r > 127 {
			t.Fatalf("non-ASCII rune %q in view:\n%s", r, out)
		}
	}
	if !strings.Contains(out, "+--") {
		t.Errorf("borders are not ASCII:\n%s", out)
	}

	useGlyphs(true)
	withConfig(t, `{"ascii":false}`, 100, 30, nil)
	if g.ascii {
		t.Error(`"ascii": false did not override the glyph set`)
	}
}

func TestConfigClipboardSecondsControlsAutoClear(t *testing.T) {
	m := withConfig(t, `{"clipboard_seconds":2}`, 100, 30, func(m tea.Model) tea.Model { return add(m, "a", "X", "val") })
	now := time.Now()
	m = withClock(m, &now)
	m = press(m, "c")
	if got := clipboardLog[len(clipboardLog)-1]; got != "val" {
		t.Fatalf("copy did not reach the clipboard: %q", got)
	}
	now = now.Add(3 * time.Second) // past 2 s, well before the 15 s default
	m = tickAt(m, now)
	if got := clipboardLog[len(clipboardLog)-1]; got != "" || !m.(model).clipUntil.IsZero() {
		t.Errorf("clipboard not cleared after the configured 2 s: last=%q", got)
	}
}

func TestConfigRevealSecondsControlsAutoHide(t *testing.T) {
	m := withConfig(t, `{"reveal_seconds":2}`, 100, 30, func(m tea.Model) tea.Model { return add(m, "a", "X", "hunter2") })
	now := time.Now()
	m = withClock(m, &now)
	m = press(m, "v")
	if m.(model).revealTitle == "" {
		t.Fatal("value not revealed")
	}
	now = now.Add(3 * time.Second)
	m = tickAt(m, now)
	if m.(model).revealTitle != "" || strings.Contains(m.View(), "hunter2") {
		t.Error("value still revealed after the configured 2 s")
	}
}

func TestConfigExpiryWarnDaysDecidesWhatIsExpiring(t *testing.T) {
	addExpiring := func(m tea.Model, name string, days int) tea.Model {
		expires := time.Now().AddDate(0, 0, days).Format(store.DateLayout)
		return press(m, "n", name, "enter", "X", "enter", "v", "tab", expires, "enter", "", "enter", "", "enter")
	}
	m := withConfig(t, `{"expiry_warn_days":3}`, 100, 30, nil)
	m = addExpiring(m, "later", 5)
	if strings.Contains(m.View(), "expiring") {
		t.Errorf("a secret 5 days out counted with a 3-day window:\n%s", m.View())
	}
	m = addExpiring(m, "sooner", 2)
	if !strings.Contains(m.View(), "1 expiring") {
		t.Errorf("a secret 2 days out not counted:\n%s", m.View())
	}
}

func TestConfigGenerateDefaults(t *testing.T) {
	m := withConfig(t, `{"gen_length":64,"gen_charset":"hex"}`, 100, 30, nil)
	m = ctrl(press(m, "n"), tea.KeyCtrlG)
	v := m.(model).secret.Value()
	if len(v) != 64 || strings.Trim(v, "0123456789abcdef") != "" {
		t.Errorf("generated %q, want 64 hex characters", v)
	}

	// An in-between length lands on the nearest preset (ties go to the shorter one).
	m = withConfig(t, `{"gen_length":40}`, 100, 30, nil)
	m = ctrl(press(m, "n"), tea.KeyCtrlG)
	if got := len(m.(model).secret.Value()); got != 32 {
		t.Errorf("gen_length 40 generated %d characters, want the nearest preset 32", got)
	}
}

func TestConfigWarningWaitsForUnlockAndKeepsLayout(t *testing.T) {
	const bad = `{"lock_minutes":"x","gen_length":3}`
	for _, size := range [][2]int{{60, 18}, {100, 30}} {
		w, h := size[0], size[1]
		addThree(newModelFor(t, true, w, h)) // the empty welcome screen is taller than 60x18 by itself
		store.Lock()
		writeConfig(t, bad)

		m, _ := New().Update(tea.WindowSizeMsg{Width: w, Height: h})
		if m.(model).mode != modeUnlock {
			t.Fatalf("mode = %v, want the unlock screen", m.(model).mode)
		}
		if strings.Contains(m.View(), "config.json") {
			t.Errorf("%dx%d: warning shown on the unlock screen, where it would expire unseen", w, h)
		}

		m = submitPassword(t, m, testPassword)
		out := m.View()
		want := "config.json: gen_length" // the toast is truncated to the detail pane at 60 columns
		if w < 80 {
			want = "config.json"
		}
		if !strings.Contains(out, want) {
			t.Errorf("%dx%d: no warning toast after unlocking:\n%s", w, h, out)
		}
		if got := strings.Count(out, "\n") + 1; got != h {
			t.Errorf("%dx%d: view is %d lines tall with the toast showing", w, h, got)
		}
		store.Lock()
	}
}

func TestConfigWarningShownImmediatelyWhenAlreadyUnlocked(t *testing.T) {
	newModelFor(t, true, 100, 30) // leaves the store unlocked
	writeConfig(t, `{"nope":1}`)
	m, _ := New().Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if out := m.View(); !strings.Contains(out, `unknown key "nope"`) {
		t.Errorf("no warning toast:\n%s", out)
	}
}

func TestMissingConfigShowsNoToast(t *testing.T) {
	m := newTestModel(t)
	if m.(model).statusMsg != "" {
		t.Errorf("toast without a config file: %q", m.(model).statusMsg)
	}
}
