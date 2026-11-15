package tui

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"github.com/RafaSG13/Pandora/internal/store"
	"github.com/RafaSG13/Pandora/internal/version"
)

// copied returns a model with secret "a" (value "val") copied to the stub
// clipboard, a controllable clock, and the clock's start time.
func copied(t *testing.T) (tea.Model, *time.Time) {
	t.Helper()
	m := add(newTestModel(t), "a", "X", "val")
	now := time.Now()
	m = withClock(m, &now)
	m = press(m, "c")
	if m.(model).clipUntil.IsZero() || clipboardLog[len(clipboardLog)-1] != "val" {
		t.Fatalf("copy did not arm the clear: log=%q", clipboardLog)
	}
	return m, &now
}

func TestDeadlineLeavesClipboardChangedSinceCopy(t *testing.T) {
	m, now := copied(t)
	clipboardLog = append(clipboardLog, "something else the user copied")
	writes := len(clipboardLog)

	*now = now.Add(clipboardTTL + time.Second)
	m = tickAt(m, *now)

	mm := m.(model)
	if len(clipboardLog) != writes {
		t.Errorf("deadline overwrote foreign clipboard content: log=%q", clipboardLog)
	}
	if !mm.clipUntil.IsZero() || strings.Contains(mm.View(), "clears in") {
		t.Error("countdown still pending after the deadline")
	}
	if mm.statusMsg != "" {
		t.Errorf("automatic clear should be silent when content changed, got toast %q", mm.statusMsg)
	}
}

func TestDeadlineClearsWhenClipboardUnreadable(t *testing.T) {
	m, now := copied(t)
	readClipboard = func() (string, error) { return "", errors.New("no paste tool") }

	*now = now.Add(clipboardTTL + time.Second)
	m = tickAt(m, *now)

	if last := clipboardLog[len(clipboardLog)-1]; last != "" || !m.(model).clipUntil.IsZero() {
		t.Errorf("unreadable clipboard must still be cleared: last=%q", last)
	}
}

func TestDeadlineClearsClipboardWithTrailingNewline(t *testing.T) {
	m, now := copied(t)
	readClipboard = func() (string, error) { return "val\n", nil }

	*now = now.Add(clipboardTTL + time.Second)
	m = tickAt(m, *now)

	if last := clipboardLog[len(clipboardLog)-1]; last != "" {
		t.Errorf("clipboard holding the secret plus a newline was left in place: last=%q", last)
	}
	if !m.(model).clipUntil.IsZero() {
		t.Error("countdown still pending")
	}
}

func TestClearKeyReportsChangedClipboardAndClearsNothing(t *testing.T) {
	m, _ := copied(t)
	clipboardLog = append(clipboardLog, "other")
	writes := len(clipboardLog)

	m = press(m, "x")

	mm := m.(model)
	if len(clipboardLog) != writes {
		t.Errorf("x overwrote foreign clipboard content: log=%q", clipboardLog)
	}
	if !mm.clipUntil.IsZero() {
		t.Error("countdown still pending after x")
	}
	if mm.statusErr || !strings.Contains(mm.statusMsg, "changed") {
		t.Errorf("toast = %q (err=%v), want a notice that the clipboard changed", mm.statusMsg, mm.statusErr)
	}
}

func TestClearKeyClearsWhenClipboardStillHoldsSecret(t *testing.T) {
	m, _ := copied(t)
	m = press(m, "x")
	if last := clipboardLog[len(clipboardLog)-1]; last != "" || !m.(model).clipUntil.IsZero() {
		t.Errorf("x left the secret on the clipboard: last=%q", last)
	}
	if !strings.Contains(m.(model).statusMsg, "cleared") {
		t.Errorf("toast = %q, want a cleared notice", m.(model).statusMsg)
	}
}

func TestLockLeavesChangedClipboardButStopsCountdown(t *testing.T) {
	m, now := copied(t)
	clipboardLog = append(clipboardLog, "other")
	writes := len(clipboardLog)
	mm := m.(model)
	mm.lockAfter = time.Minute
	m = mm

	*now = now.Add(2 * time.Minute)
	m = tickAt(m, *now)

	if m.(model).mode != modeUnlock {
		t.Fatalf("mode = %v, want the unlock screen", m.(model).mode)
	}
	if len(clipboardLog) != writes {
		t.Errorf("lock overwrote foreign clipboard content: log=%q", clipboardLog)
	}
	if !m.(model).clipUntil.IsZero() {
		t.Error("countdown still pending after lock")
	}
}

func TestClipboardHashIsForgottenOnceHandled(t *testing.T) {
	m, _ := copied(t)
	if m.(model).clipHash == [32]byte{} {
		t.Fatal("no hash remembered after copy")
	}
	m = press(m, "x")
	if m.(model).clipHash != [32]byte{} {
		t.Error("hash kept after the clipboard was handled")
	}
}

// --- NO_COLOR ---

// sgrSeq matches SGR sequences; hasColorSGR flags those that set a foreground
// or background colour (bold, reverse and other attributes are allowed).
var sgrSeq = regexp.MustCompile("\x1b\\[([0-9;:]*)m")

func hasColorSGR(s string) bool {
	for _, m := range sgrSeq.FindAllStringSubmatch(s, -1) {
		for _, p := range strings.FieldsFunc(m[1], func(r rune) bool { return r == ';' || r == ':' }) {
			n, _ := strconv.Atoi(p)
			if n == 38 || n == 48 || (n >= 30 && n <= 37) || (n >= 40 && n <= 47) || (n >= 90 && n <= 97) || (n >= 100 && n <= 107) {
				return true
			}
		}
	}
	return false
}

func TestDetectNoColor(t *testing.T) {
	cases := []struct {
		env  map[string]string
		want bool
	}{
		{nil, false},
		{map[string]string{"NO_COLOR": "1"}, true},
		{map[string]string{"NO_COLOR": ""}, false},
		{map[string]string{"TERM": "dumb"}, true},
		{map[string]string{"TERM": "xterm-256color"}, false},
	}
	for _, c := range cases {
		if got := detectNoColor(func(k string) string { return c.env[k] }); got != c.want {
			t.Errorf("env %v: got %v, want %v", c.env, got, c.want)
		}
	}
}

// screens renders the browse view, the form and the help overlay of a model
// holding two secrets.
func screens(m tea.Model) map[string]string {
	m = add(m, "alpha", "TOKENS", "1")
	m = add(m, "bravo", "INFRA", "2") // bravo selected
	out := map[string]string{"browse": m.View()}
	out["form"] = press(m, "n").View()
	out["help"] = press(m, "?").View()
	return out
}

func TestNoColorEmitsNoColourSequences(t *testing.T) {
	// Sanity: with colour on, the same screens do carry colour codes.
	coloured := newTestModel(t)
	lipgloss.SetColorProfile(termenv.TrueColor)
	for name, out := range screens(coloured) {
		if !hasColorSGR(out) {
			t.Fatalf("%s: expected colour sequences in the coloured baseline", name)
		}
	}

	for _, env := range []map[string]string{{"NO_COLOR": "1"}, {"TERM": "dumb"}} {
		newTestModel(t) // isolates keyring and clipboard; restores colour state afterwards
		for k, v := range env {
			t.Setenv(k, v)
		}
		lipgloss.SetColorProfile(termenv.TrueColor) // as if the terminal advertised truecolor
		m, _ := New().Update(tea.WindowSizeMsg{Width: 100, Height: 30})
		if !noColor {
			t.Fatalf("env %v did not switch to colourless output", env)
		}
		for name, out := range screens(m) {
			if hasColorSGR(out) {
				t.Errorf("env %v, %s view contains colour escape codes:\n%q", env, name, out)
			}
		}
	}
}

func TestNoColorSelectedRowStaysDistinguishable(t *testing.T) {
	newTestModel(t)
	t.Setenv("NO_COLOR", "1")
	m, _ := New().Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = add(m, "alpha", "X", "1")
	m = add(m, "bravo", "X", "2") // bravo selected

	var marked []string
	for _, line := range strings.Split(ansi.Strip(m.(model).view()), "\n") {
		if strings.Contains(line, g.marker+" "+g.dot) || strings.Contains(line, g.marker+" "+g.orphan) {
			marked = append(marked, line)
		}
	}
	if len(marked) != 1 || !strings.Contains(marked[0], "bravo") {
		t.Errorf("rows carrying the selection marker = %q, want exactly the selected bravo row", marked)
	}
}

func TestNoColorExpiredAndSoonDifferByGlyphInCompactRows(t *testing.T) {
	newTestModel(t)
	t.Setenv("NO_COLOR", "1")
	m, _ := New().Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	soon := time.Now().AddDate(0, 0, 3).Format(store.DateLayout)
	for _, s := range []struct{ name, expires string }{{"soonie", soon}, {"stale", "2000-01-01"}} {
		m = press(m, "n", s.name, "enter", "X", "enter", "v", "enter", s.expires, "enter", "", "enter")
	}
	m = press(m, "t") // compact rows

	rows := map[string]string{}
	for _, line := range strings.Split(ansi.Strip(m.(model).view()), "\n") {
		for _, name := range []string{"soonie", "stale"} {
			if strings.Contains(line, g.dot+" "+name) || strings.Contains(line, g.orphan+" "+name) {
				rows[name] = line
			}
		}
	}
	if len(rows) != 2 {
		t.Fatalf("compact rows not found in view:\n%s", ansi.Strip(m.(model).view()))
	}
	if !strings.Contains(rows["soonie"], g.clock) || strings.Contains(rows["soonie"], g.clock+"!") {
		t.Errorf("soon row = %q, want a plain clock glyph", rows["soonie"])
	}
	if !strings.Contains(rows["stale"], g.clock+"!") {
		t.Errorf("expired row = %q, want clock glyph plus !", rows["stale"])
	}
}

// --- help overlay ---

func sizedModel(t *testing.T, w, h int) tea.Model {
	t.Helper()
	m, _ := newTestModel(t).Update(tea.WindowSizeMsg{Width: w, Height: h})
	return m
}

func TestHelpOverlayScrollsToEveryLineAtSmallSizes(t *testing.T) {
	for _, size := range [][2]int{{60, 18}, {70, 18}, {80, 20}, {100, 18}, {100, 24}, {120, 30}} {
		w, h := size[0], size[1]
		m := press(add(sizedModel(t, w, h), "a", "X", "v"), "?")
		m, _ = m.Update(tea.WindowSizeMsg{Width: w, Height: h})

		seen := map[string]bool{}
		capture := func() {
			out := m.(model).view()
			if got := lipgloss.Height(out); got != h {
				t.Fatalf("%dx%d scroll=%d: view is %d lines, want %d", w, h, m.(model).helpScroll, got, h)
			}
			if got := lipgloss.Width(out); got > w {
				t.Fatalf("%dx%d: view is %d columns wide", w, h, got)
			}
			for _, l := range strings.Split(ansi.Strip(out), "\n") {
				seen[strings.TrimSpace(l)] = true
			}
		}
		capture()
		for range 60 {
			if !m.(model).helpScrollable() {
				break // everything is already on screen
			}
			m = press(m, "j")
			if m.(model).mode != modeHelp {
				t.Fatalf("%dx%d: j closed the help overlay", w, h)
			}
			capture()
		}
		for _, l := range m.(model).helpLines() {
			plain := strings.TrimSpace(ansi.Strip(l))
			if plain == "" {
				continue
			}
			found := false
			for s := range seen {
				if strings.Contains(s, plain) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%dx%d: help line %q never shown while scrolling", w, h, plain)
			}
		}
		if !containsLine(seen, "pandora "+version.Get().Version) {
			t.Errorf("%dx%d: version line not reachable", w, h)
		}
	}
}

func containsLine(seen map[string]bool, want string) bool {
	for s := range seen {
		if strings.Contains(s, want) {
			return true
		}
	}
	return false
}

func TestHelpScrollKeys(t *testing.T) {
	m := press(add(sizedModel(t, 60, 18), "a", "X", "v"), "?")
	hm := func() model { return m.(model) }
	if !hm().helpScrollable() {
		t.Fatal("help should not fit in 60x18")
	}
	_, view := hm().helpLayout()
	total := len(hm().helpLines())
	maxScroll := clampHelpScroll(1<<30, total, view)

	key := func(msg tea.KeyMsg) { m, _ = m.Update(msg) }
	key(tea.KeyMsg{Type: tea.KeyDown})
	if hm().helpScroll != 1 {
		t.Errorf("down: scroll = %d, want 1", hm().helpScroll)
	}
	key(tea.KeyMsg{Type: tea.KeyUp})
	key(tea.KeyMsg{Type: tea.KeyUp})
	if hm().helpScroll != 0 {
		t.Errorf("up past the top: scroll = %d, want 0", hm().helpScroll)
	}
	key(tea.KeyMsg{Type: tea.KeyPgDown})
	if hm().helpScroll != view-1 {
		t.Errorf("pgdown: scroll = %d, want one page (%d)", hm().helpScroll, view-1)
	}
	key(tea.KeyMsg{Type: tea.KeyEnd})
	if hm().helpScroll != maxScroll {
		t.Errorf("end: scroll = %d, want %d", hm().helpScroll, maxScroll)
	}
	key(tea.KeyMsg{Type: tea.KeyPgDown})
	if hm().helpScroll != maxScroll {
		t.Errorf("pgdown past the end: scroll = %d, want %d", hm().helpScroll, maxScroll)
	}
	key(tea.KeyMsg{Type: tea.KeyHome})
	if hm().helpScroll != 0 {
		t.Errorf("home: scroll = %d, want 0", hm().helpScroll)
	}
	if hm().mode != modeHelp {
		t.Fatal("a scroll key closed the overlay")
	}

	if out := ansi.Strip(hm().view()); !strings.Contains(out, "1-") || !strings.Contains(out, "/"+strconv.Itoa(total)) {
		t.Errorf("no position hint for %d lines in:\n%s", total, out)
	}

	for _, closer := range []string{"esc", "?", "q", "x"} {
		if hm().mode != modeHelp {
			m = press(m, "?")
		}
		m = press(m, closer)
		if hm().mode != modeBrowse {
			t.Errorf("%q did not close the overlay", closer)
		}
	}
}

func TestHelpOverlayReopensAtTop(t *testing.T) {
	m := press(add(sizedModel(t, 60, 18), "a", "X", "v"), "?", "j", "j", "j", "esc", "?")
	if got := m.(model).helpScroll; got != 0 {
		t.Errorf("reopened help starts at scroll %d, want 0", got)
	}
}

func TestHelpScrollKeysCloseWhenEverythingFits(t *testing.T) {
	m := press(add(newTestModel(t), "a", "X", "v"), "?")
	if m.(model).helpScrollable() {
		t.Fatal("help should fit at 100x30")
	}
	m = press(m, "j")
	if m.(model).mode != modeBrowse {
		t.Error("with nothing to scroll, any key should still close the overlay")
	}
}

func TestHelpOverlayShowsVersion(t *testing.T) {
	m := press(add(newTestModel(t), "a", "X", "v"), "?")
	if want := "pandora " + version.Get().Version; !strings.Contains(m.View(), want) {
		t.Errorf("help overlay lacks %q:\n%s", want, m.View())
	}
}
