package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

func TestClearClipboardHelperPassesTheHashToTheClear(t *testing.T) {
	isolate(t)
	want := sha256.Sum256([]byte("v"))
	var got [sha256.Size]byte
	calls := 0
	h := harness{
		stdin:    hex.EncodeToString(want[:]) + "\n",
		noMaster: true,
		clearIfHolds: func(w [sha256.Size]byte) (bool, error) {
			got = w
			calls++
			return true, nil
		},
	}
	if code, _, stderr := h.run(t, "__clear-clipboard", "15"); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if calls != 1 || got != want {
		t.Errorf("clearIfHolds called %d times with %x, want once with %x", calls, got, want)
	}
}

func TestClearClipboardHelperRejectsBadInput(t *testing.T) {
	isolate(t)
	good := hex.EncodeToString(make([]byte, sha256.Size)) + "\n"
	for name, c := range map[string]struct{ stdin, secs string }{
		"zero seconds":     {good, "0"},
		"not a number":     {good, "x"},
		"too many seconds": {good, "99999"},
		"not hex":          {"zz\n", "5"},
		"wrong length":     {"abcd\n", "5"},
		"empty stdin":      {"", "5"},
	} {
		h := harness{stdin: c.stdin, noMaster: true}
		if code, _, _ := h.run(t, "__clear-clipboard", c.secs); code != 2 {
			t.Errorf("%s: exit %d, want 2", name, code)
		}
	}
}

func TestGetCopySchedulesTheClipboardClear(t *testing.T) {
	isolate(t)
	mustSave(t, "tok", "", "s3cret")
	type call struct {
		value string
		after time.Duration
	}
	var calls []call
	h := harness{
		clip:       func(string) error { return nil },
		clearLater: func(v string, d time.Duration) error { calls = append(calls, call{v, d}); return nil },
	}
	code, _, stderr := h.run(t, "get", "--copy", "tok")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if len(calls) != 1 || calls[0] != (call{"s3cret", 15 * time.Second}) {
		t.Errorf("scheduled %v, want once with the value and 15 s", calls)
	}
	if !strings.Contains(stderr, "cleared in 15 s") {
		t.Errorf("stderr %q does not announce the clearing", stderr)
	}
	if strings.Contains(stderr, "s3cret") {
		t.Error("stderr leaks the value")
	}
}

func TestGetCopyWarnsOnX11Only(t *testing.T) {
	isolate(t)
	mustSave(t, "tok", "", "s3cret")
	copied := func(string) error { return nil }
	_, _, onX := harness{clip: copied, x11: true}.run(t, "get", "--copy", "tok")
	_, _, off := harness{clip: copied}.run(t, "get", "--copy", "tok")
	if !strings.Contains(onX, "X11") || strings.Contains(off, "X11") {
		t.Errorf("X11 warning: on X11 %q, elsewhere %q", onX, off)
	}
}
