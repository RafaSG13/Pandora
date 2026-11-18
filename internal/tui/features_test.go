package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/zalando/go-keyring"

	"github.com/RafaSG13/Pandora/internal/gen"
	"github.com/RafaSG13/Pandora/internal/store"
)

// withClock makes the model read time from *now so tests can advance it.
func withClock(m tea.Model, now *time.Time) tea.Model {
	mm := m.(model)
	mm.now = func() time.Time { return *now }
	mm.lastActive = *now
	return mm
}

func tickAt(m tea.Model, t time.Time) tea.Model {
	m, _ = m.Update(tickMsg(t))
	return m
}

func ctrl(m tea.Model, t tea.KeyType) tea.Model {
	m, _ = m.Update(tea.KeyMsg{Type: t})
	return m
}

func visibleTitles(m tea.Model) []string {
	var out []string
	for _, it := range m.(model).list.VisibleItems() {
		out = append(out, it.(item).meta.Title)
	}
	return out
}

func sameStrings(a, b []string) bool { return strings.Join(a, "|") == strings.Join(b, "|") }

func TestRevealIsPerItemAndAutoHides(t *testing.T) {
	m := add(newTestModel(t), "a", "X", "alpha-secret")
	m = add(m, "b", "X", "hunter2") // b selected
	now := time.Now()
	m = withClock(m, &now)

	m = press(m, "v")
	if !m.(model).revealed("b") || !strings.Contains(m.View(), "hunter2") {
		t.Fatal("v did not reveal the selected secret's value")
	}

	m = press(m, "k", "j") // away and back: the reveal must not survive
	if m.(model).revealed("b") || strings.Contains(m.View(), "hunter2") {
		t.Error("reveal followed the selection instead of resetting")
	}

	m = press(m, "v")
	now = now.Add(revealTTL + time.Second)
	m = tickAt(m, now)
	if m.(model).revealTitle != "" || strings.Contains(m.View(), "hunter2") {
		t.Error("value still revealed after the auto-hide timeout")
	}
}

func TestCopyClearNowAndNoEarlyClearAfterSecondCopy(t *testing.T) {
	m := add(newTestModel(t), "a", "X", "val")
	t0 := time.Now()
	now := t0
	m = withClock(m, &now)

	m = press(m, "c")
	if clipboardLog[len(clipboardLog)-1] != "val" || m.(model).clipUntil.IsZero() {
		t.Fatalf("copy did not write the value / arm the timer: log=%q", clipboardLog)
	}

	m = press(m, "x")
	if clipboardLog[len(clipboardLog)-1] != "" || !m.(model).clipUntil.IsZero() {
		t.Errorf("x did not clear the clipboard now: log=%q", clipboardLog)
	}

	// Copy, then copy again 10s later: the first copy's deadline must not clear the second.
	m = press(m, "c")
	now = t0.Add(10 * time.Second)
	m = press(m, "c")
	now = t0.Add(clipboardTTL + time.Second) // past the first deadline only
	m = tickAt(m, now)
	if last := clipboardLog[len(clipboardLog)-1]; last != "val" || m.(model).clipUntil.IsZero() {
		t.Errorf("second copy cleared early by the first one's deadline: last=%q", last)
	}
	now = t0.Add(10*time.Second + clipboardTTL + time.Second)
	m = tickAt(m, now)
	if last := clipboardLog[len(clipboardLog)-1]; last != "" || !m.(model).clipUntil.IsZero() {
		t.Errorf("clipboard not cleared after the TTL: last=%q", last)
	}
}

func TestCopyReportsClipboardFailure(t *testing.T) {
	m := add(newTestModel(t), "a", "X", "val")
	writeSecretClipboard = func(string) error { return errors.New("no xclip") }

	m = press(m, "c")
	mm := m.(model)
	if !mm.statusErr || !strings.Contains(mm.statusMsg, "no xclip") || !mm.clipUntil.IsZero() {
		t.Errorf("status=%q err=%v clipUntil=%v, want a failure message and no countdown", mm.statusMsg, mm.statusErr, mm.clipUntil)
	}
}

func TestSortCyclesNameCategoryCreated(t *testing.T) {
	m := add(newTestModel(t), "c", "Z", "1")
	m = add(m, "a", "Y", "1")
	m = add(m, "b", "X", "1")

	if got := visibleTitles(m); !sameStrings(got, []string{"c", "a", "b"}) {
		t.Fatalf("default order = %v, want insertion order", got)
	}
	m = press(m, "s")
	if got := visibleTitles(m); !sameStrings(got, []string{"a", "b", "c"}) {
		t.Errorf("sorted by name = %v", got)
	}
	m = press(m, "s")
	if got := visibleTitles(m); !sameStrings(got, []string{"b", "a", "c"}) {
		t.Errorf("sorted by category = %v, want b(X) a(Y) c(Z)", got)
	}
	m = press(m, "s")
	if got := visibleTitles(m); !sameStrings(got, []string{"c", "a", "b"}) {
		t.Errorf("sort did not cycle back to date added: %v", got)
	}
}

func TestCategoryFilterByNumberKeys(t *testing.T) {
	m := add(newTestModel(t), "a", "INFRA", "1")
	m = add(m, "b", "TOKENS", "1")
	m = add(m, "c", "tokens", "1") // same category, different case

	m = press(m, "2") // categories sort to INFRA, TOKENS
	if got := visibleTitles(m); !sameStrings(got, []string{"b", "c"}) {
		t.Errorf("category 2 shows %v, want [b c]", got)
	}
	m = press(m, "2") // same key toggles off
	if got := visibleTitles(m); len(got) != 3 {
		t.Errorf("repeating the key did not clear the filter: %v", got)
	}
	m = press(m, "1")
	if got := visibleTitles(m); !sameStrings(got, []string{"a"}) {
		t.Errorf("category 1 shows %v, want [a]", got)
	}
	m = press(m, "0")
	if got := visibleTitles(m); len(got) != 3 {
		t.Errorf("0 did not clear the filter: %v", got)
	}
	m = press(m, "9")
	if got := visibleTitles(m); len(got) != 3 || !strings.Contains(m.(model).statusMsg, "No category 9") {
		t.Errorf("out-of-range category changed the list or gave no message: %v %q", got, m.(model).statusMsg)
	}
}

func TestCategoryFilterResetsWhenCategoryDisappears(t *testing.T) {
	m := add(newTestModel(t), "a", "INFRA", "1")
	m = add(m, "b", "TOKENS", "1")
	m = press(m, "2", "d", "y") // filter to TOKENS, delete its only secret
	if got := visibleTitles(m); !sameStrings(got, []string{"a"}) || m.(model).category != "" {
		t.Errorf("after deleting the last TOKENS secret: shows %v, category=%q; want all secrets", got, m.(model).category)
	}
}

func TestFormSavesExpiresAndNotesAndEditSetsUpdated(t *testing.T) {
	m := press(newTestModel(t), "n", "a", "enter", "X", "enter", "v", "tab", "2030-01-31", "enter", "see https://x.example", "enter", "Prod, db,PROD", "enter")

	metas, _ := store.LoadMetadata()
	if len(metas) != 1 || metas[0].Expires != "2030-01-31" || metas[0].Notes != "see https://x.example" || metas[0].Updated != "" || strings.Join(metas[0].Tags, ",") != "db,prod" {
		t.Fatalf("saved metadata = %+v", metas)
	}

	m = press(m, "e")
	mm := m.(model)
	if mm.inputs[fieldExpires].Value() != "2030-01-31" || mm.inputs[fieldNotes].Value() != "see https://x.example" || mm.inputs[fieldTags].Value() != "db, prod" {
		t.Fatalf("edit form not prefilled: expires=%q notes=%q", mm.inputs[fieldExpires].Value(), mm.inputs[fieldNotes].Value())
	}
	m = ctrl(m, tea.KeyCtrlS)
	if m.(model).mode != modeBrowse {
		t.Fatalf("ctrl+s did not save: mode=%v status=%q", m.(model).mode, m.(model).statusMsg)
	}
	metas, _ = store.LoadMetadata()
	if metas[0].Updated != time.Now().Format(store.DateLayout) || metas[0].Expires != "2030-01-31" || strings.Join(metas[0].Tags, ",") != "db,prod" {
		t.Errorf("after edit: %+v, want Updated=today and Expires kept", metas[0])
	}
}

func TestInvalidExpiresKeepsFormOpen(t *testing.T) {
	m := press(newTestModel(t), "n", "a", "enter", "X", "enter", "v", "tab", "tomorrow", "enter", "", "enter", "", "enter")
	mm := m.(model)
	if mm.mode != modeForm || !mm.statusErr || !strings.Contains(mm.statusMsg, "Expires") {
		t.Errorf("mode=%v status=%q, want open form with an Expires error", mm.mode, mm.statusMsg)
	}
	if len(titles(m)) != 0 {
		t.Error("secret saved despite invalid expiry")
	}
}

func TestUpdateSecretStampsUpdatedAndKeepsBackup(t *testing.T) {
	m := add(newTestModel(t), "a", "X", "v")
	_ = m
	if err := store.UpdateSecret("a", store.SecretMeta{Title: "a", Category: "Y", Notes: "n"}, nil); err != nil {
		t.Fatal(err)
	}
	metas, _ := store.LoadMetadata()
	if metas[0].Updated != time.Now().Format(store.DateLayout) || metas[0].Created == "" || metas[0].Notes != "n" {
		t.Errorf("metadata after update = %+v", metas[0])
	}
	dir, _ := os.UserConfigDir()
	bak, err := os.ReadFile(filepath.Join(dir, "pandora", "vault.json.bak"))
	cur, _ := os.ReadFile(filepath.Join(dir, "pandora", "vault.json"))
	if err != nil || len(bak) == 0 || string(bak) == string(cur) {
		t.Errorf("backup missing or not the previous contents: err=%v", err)
	}
}

func TestFormGenerateUsesSelectedLengthAndShowsValue(t *testing.T) {
	m := press(newTestModel(t), "n")
	m = ctrl(m, tea.KeyCtrlG)
	in := m.(model).secret
	if len(in.Value()) != gen.Lengths[1] {
		t.Fatalf("default generated length = %d, want %d", len(in.Value()), gen.Lengths[1])
	}
	if !strings.Contains(m.View(), in.Value()) {
		t.Error("generated secret is not visible in the form")
	}

	m = ctrl(m, tea.KeyCtrlL) // next length
	m = ctrl(m, tea.KeyCtrlG)
	if got := len(m.(model).secret.Value()); got != gen.Lengths[2] {
		t.Errorf("after ctrl+l generated length = %d, want %d", got, gen.Lengths[2])
	}
}

func TestExpiryInfo(t *testing.T) {
	now := time.Date(2026, 6, 15, 13, 0, 0, 0, time.Local)
	cases := []struct {
		expires string
		label   string
		level   expiryLevel
	}{
		{"", "", expiryNone},
		{"garbage", "", expiryNone},
		{"2026-06-10", "expired 5d ago", expiryExpired},
		{"2026-06-15", "expires today", expirySoon},
		{"2026-06-20", "expires in 5d", expirySoon},
		{"2026-06-29", "expires in 14d", expirySoon},
		{"2026-06-30", "expires in 15d", expiryFar},
	}
	for _, c := range cases {
		label, level := expiryInfo(store.SecretMeta{Expires: c.expires}, now)
		if label != c.label || level != c.level {
			t.Errorf("expires %q: got (%q, %v), want (%q, %v)", c.expires, label, level, c.label, c.level)
		}
	}
}

func TestExpiryShownInListHeaderAndDetail(t *testing.T) {
	soon := time.Now().AddDate(0, 0, 3).Format(store.DateLayout)
	m := press(newTestModel(t), "n", "a", "enter", "X", "enter", "v", "tab", soon, "enter", "", "enter", "", "enter")

	out := m.View()
	if !strings.Contains(out, "1 expiring") {
		t.Errorf("header has no expiry warning:\n%s", out)
	}
	if strings.Count(out, "expires in 3d") < 2 { // list row and detail card
		t.Errorf("expiry label missing from list row or detail:\n%s", out)
	}
}

// submitPassword types pw and presses enter, then runs the resulting commands
// (the key derivation runs off the UI thread in the real program) and feeds
// the unlock result back.
func submitPassword(t *testing.T, m tea.Model, pw string) tea.Model {
	t.Helper()
	m = press(m, pw)
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		return m
	}
	if !m.(model).unlocking {
		t.Fatal("enter did not start unlocking")
	}
	var cmds []tea.Cmd
	if batch, ok := cmd().(tea.BatchMsg); ok {
		cmds = batch
	} else {
		t.Fatal("expected a batch of commands")
	}
	for _, c := range cmds {
		if res, ok := c().(unlockedMsg); ok {
			m, _ = m.Update(res)
		}
	}
	return m
}

func TestIdleLockHidesEverythingAndNeedsThePassword(t *testing.T) {
	m := add(newTestModel(t), "visible-title", "X", "val")
	mm := m.(model)
	mm.lockAfter = 5 * time.Minute
	now := time.Now()
	m = withClock(mm, &now)

	m = press(m, "c", "v")
	now = now.Add(4 * time.Minute)
	m = press(m, "j") // activity resets the idle clock
	now = now.Add(4 * time.Minute)
	m = tickAt(m, now)
	if m.(model).locked() {
		t.Fatal("locked although the user was active 4 minutes ago")
	}

	now = now.Add(2 * time.Minute)
	m = tickAt(m, now)
	if m.(model).mode != modeUnlock || store.IsUnlocked() {
		t.Fatalf("mode = %v, unlocked = %v; want the unlock screen and a locked store", m.(model).mode, store.IsUnlocked())
	}
	if n := len(m.(model).list.Items()); n != 0 {
		t.Errorf("%d list items kept in memory while locked", n)
	}
	if clipboardLog[len(clipboardLog)-1] != "" || !m.(model).clipUntil.IsZero() || m.(model).revealTitle != "" {
		t.Errorf("lock left secrets exposed: log=%q clipUntil=%v reveal=%q", clipboardLog, m.(model).clipUntil, m.(model).revealTitle)
	}
	if out := m.View(); !strings.Contains(out, "Locked") || strings.Contains(out, "visible-title") {
		t.Errorf("lock screen wrong:\n%s", out)
	}

	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}) // typed into the field, not "quit"
	if cmd != nil {
		if _, quit := cmd().(tea.QuitMsg); quit {
			t.Fatal("q quit the app from the unlock screen")
		}
	}
	m = ctrl(ctrl(m, tea.KeyBackspace), tea.KeyEnter) // empty password: nothing happens
	if m.(model).mode != modeUnlock || store.IsUnlocked() || m.(model).unlocking {
		t.Fatal("a key press unlocked the vault")
	}

	m = submitPassword(t, m, "wrong wrong wrong")
	if mm := m.(model); mm.mode != modeUnlock || store.IsUnlocked() || !strings.Contains(mm.View(), "Wrong password") {
		t.Fatalf("wrong password: mode %v, unlocked %v\n%s", mm.mode, store.IsUnlocked(), mm.View())
	}
	if got := m.(model).pwInput.Value(); got != "" {
		t.Errorf("password field not cleared after a failure: %q", got)
	}

	m = submitPassword(t, m, testPassword)
	if m.(model).mode != modeBrowse || !store.IsUnlocked() {
		t.Fatalf("correct password: mode %v, unlocked %v", m.(model).mode, store.IsUnlocked())
	}
	if got := titles(m); len(got) != 1 || got[0] != "visible-title" {
		t.Errorf("titles after unlock = %v", got)
	}
}

func TestIdleLockDiscardsAnOpenForm(t *testing.T) {
	m := newTestModel(t)
	mm := m.(model)
	mm.lockAfter = time.Minute
	now := time.Now()
	m = withClock(mm, &now)

	m = press(m, "n", "half-typed-name", "enter", "cat", "enter", "typed-secret")
	now = now.Add(2 * time.Minute)
	m = tickAt(m, now)

	for i, in := range m.(model).inputs {
		if in.Value() != "" {
			t.Errorf("form field %d still holds %q after the lock", i, in.Value())
		}
	}
	m = submitPassword(t, m, testPassword)
	if m.(model).mode != modeBrowse {
		t.Errorf("mode after unlock = %v, want browse (never back into the form)", m.(model).mode)
	}
}

func TestLockTimeoutFromEnv(t *testing.T) {
	cases := map[string]time.Duration{"": defaultLockTTL, "x": defaultLockTTL, "-3": defaultLockTTL, "0": 0, "10": 10 * time.Minute}
	for in, want := range cases {
		if got := lockTimeout(in); got != want {
			t.Errorf("lockTimeout(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestToastExpiresAfterThreeSeconds(t *testing.T) {
	m := newTestModel(t)
	now := time.Now()
	m = withClock(m, &now)
	m = add(m, "a", "X", "v")
	if m.(model).statusMsg == "" || !strings.Contains(m.View(), "Saved") {
		t.Fatalf("no toast after saving:\n%s", m.View())
	}
	now = now.Add(toastTTL + time.Second)
	m = tickAt(m, now)
	if m.(model).statusMsg != "" || strings.Contains(m.View(), "Saved") {
		t.Error("toast still visible after its timeout")
	}
}

func TestCompactRowsGeometry(t *testing.T) {
	m := press(addThree(newTestModel(t)), "t")
	lines := strings.Split(m.View(), "\n")
	for i, name := range []string{"alpha", "bravo", "charlie"} {
		if !strings.Contains(lines[listTopRow+i*compactRows], name) {
			t.Errorf("compact row %d = %q, want %s", i, lines[listTopRow+i*compactRows], name)
		}
	}
	m = click(m, 5, listTopRow+1)
	if got := selected(m); got != "bravo" {
		t.Errorf("click on compact row 1 selected %q, want bravo", got)
	}
	m = press(m, "t") // back to roomy: the same click row now hits alpha's description line
	m = click(m, 5, listTopRow+roomyRows)
	if got := selected(m); got != "bravo" {
		t.Errorf("click after toggling back selected %q, want bravo", got)
	}
}

func TestTerminalTooSmallGuard(t *testing.T) {
	m := add(newTestModel(t), "a", "X", "v")
	m, _ = m.Update(tea.WindowSizeMsg{Width: 50, Height: 10})
	if out := m.View(); !strings.Contains(out, "Terminal too small") || strings.Contains(out, "PANDORA") {
		t.Errorf("small terminal view = %q", out)
	}
	m, _ = m.Update(tea.WindowSizeMsg{Width: minWidth, Height: minHeight})
	if out := m.View(); strings.Contains(out, "Terminal too small") {
		t.Errorf("minimum size rejected:\n%s", out)
	}
}

func TestViewFitsTerminalHeight(t *testing.T) {
	for _, size := range [][2]int{{60, 18}, {80, 24}, {100, 30}, {140, 50}} {
		m := addThree(newTestModel(t))
		m, _ = m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		if got := strings.Count(m.View(), "\n") + 1; got != size[1] {
			t.Errorf("%dx%d: view is %d lines tall", size[0], size[1], got)
		}
	}
}

func TestPositionIndicatorAndScrollbar(t *testing.T) {
	// The ASCII thumb ("#") is also the ASCII logo glyph, so pin the Unicode set.
	prev := g
	g = newGlyphs(false)
	t.Cleanup(func() { g = prev })
	m := addMany(newTestModel(t), 9)
	out := m.View()
	if !strings.Contains(out, "9/9") {
		t.Errorf("position indicator missing:\n%s", out)
	}
	if !strings.Contains(out, g.thumb) {
		t.Errorf("scrollbar missing although the list overflows:\n%s", out)
	}
	if few := addThree(newTestModel(t)).View(); strings.Contains(few, g.thumb) {
		t.Error("scrollbar drawn although everything fits")
	}
}

func TestPanesFitContent(t *testing.T) {
	m := add(newTestModel(t), "a", "X", "v")
	full := addMany(newTestModel(t), 9)
	if m.(model).paneOuter() >= full.(model).paneOuter() {
		t.Errorf("one secret pane (%d) is not shorter than nine (%d)", m.(model).paneOuter(), full.(model).paneOuter())
	}
	if m.(model).paneOuter() < minPaneOuter {
		t.Errorf("pane shrank below its minimum: %d", m.(model).paneOuter())
	}
}

func TestWelcomeScreenOnFirstRun(t *testing.T) {
	m := newTestModel(t)
	if out := m.View(); !strings.Contains(out, "Welcome to Pandora") || !strings.Contains(out, "n new") {
		t.Errorf("first-run view lacks the welcome:\n%s", out)
	}
	if out := add(m, "a", "X", "v").View(); strings.Contains(out, "Welcome to Pandora") {
		t.Error("welcome still shown with a secret present")
	}
}

func TestErrorStateWhenVaultIsCorrupt(t *testing.T) {
	newTestModel(t) // creates and unlocks the vault
	cfg, _ := os.UserConfigDir()
	if err := os.WriteFile(filepath.Join(cfg, "pandora", "vault.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	m, _ := New().Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	out := m.View()
	if !strings.Contains(out, "Could not load your secrets") || !strings.Contains(out, "r retry") {
		t.Errorf("error state missing:\n%s", out)
	}
	if strings.Contains(out, "Welcome to Pandora") {
		t.Error("corrupt vault shown as a fresh install")
	}
}

func TestOrphanDetectionMarksRowAndDetail(t *testing.T) {
	m := add(newTestModel(t), "a", "X", "v")
	m = add(m, "b", "X", "v")
	keyring.MockInit() // every value is gone ...
	bv := "v"
	if err := store.UpdateSecret("b", store.SecretMeta{Title: "b", Category: "X"}, &bv); err != nil { // ... then b gets its back
		t.Fatal(err)
	}

	msg := checkOrphans([]string{"a", "b"})()
	om, ok := msg.(orphanMsg)
	if !ok || !om.missing["a"] || om.missing["b"] {
		t.Fatalf("checkOrphans = %#v, want only a missing", msg)
	}

	m, _ = m.Update(msg)
	m = press(m, "k") // select a
	out := m.View()
	if !strings.Contains(out, g.orphan+" a") || !strings.Contains(out, "Value missing from the keyring") {
		t.Errorf("orphan not flagged in list and detail:\n%s", out)
	}
}

func TestEmptyListFooterOffersOnlyApplicableKeys(t *testing.T) {
	out := newTestModel(t).View()
	last := out[strings.LastIndex(out, "\n")+1:]
	if strings.Contains(last, "copy") || strings.Contains(last, "reveal") || !strings.Contains(last, "? help") {
		t.Errorf("footer = %q", last)
	}
}

func TestDetectASCII(t *testing.T) {
	env := func(kv map[string]string) func(string) string { return func(k string) string { return kv[k] } }
	cases := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{"unset locale", map[string]string{}, false},
		{"utf-8 locale", map[string]string{"LANG": "en_US.UTF-8"}, false},
		{"utf8 spelling", map[string]string{"LC_ALL": "C.utf8"}, false},
		{"C locale", map[string]string{"LANG": "C"}, true},
		{"latin1", map[string]string{"LC_CTYPE": "en_US.ISO-8859-1"}, true},
		{"LC_ALL beats LANG", map[string]string{"LC_ALL": "C", "LANG": "en_US.UTF-8"}, true},
		{"explicit flag", map[string]string{"PANDORA_ASCII": "1", "LANG": "en_US.UTF-8"}, true},
	}
	for _, c := range cases {
		if got := detectASCII(env(c.env)); got != c.want {
			t.Errorf("%s: detectASCII = %v, want %v", c.name, got, c.want)
		}
	}
	for _, s := range []string{newGlyphs(true).dot, newGlyphs(true).mask, newGlyphs(true).rule, newGlyphs(true).border.TopLeft, newGlyphs(true).border.Left} {
		for _, r := range s {
			if r > 127 {
				t.Errorf("ASCII glyph set contains %q", s)
			}
		}
	}
}

func TestHelpOverlayListsNewKeys(t *testing.T) {
	m := press(add(newTestModel(t), "a", "X", "v"), "?")
	out := m.View()
	for _, want := range []string{"cycle sort", "compact rows", "clear clipboard", "ctrl+g", "category filter"} {
		if !strings.Contains(out, want) {
			t.Errorf("help overlay lacks %q:\n%s", want, out)
		}
	}
}

func TestASCIIReplacerKeepsColumnWidth(t *testing.T) {
	in := "✓ Saved — ok… ↑/↓ → 100×30 · • ● ◆ ▌ █ ░ ┃ ┆"
	out := asciiReplacer.Replace(in)
	if lipgloss.Width(out) != lipgloss.Width(in) {
		t.Errorf("width changed: %q (%d) -> %q (%d)", in, lipgloss.Width(in), out, lipgloss.Width(out))
	}
	for _, r := range out {
		if r > 127 {
			t.Errorf("non-ASCII %q left in %q", r, out)
		}
	}
}
