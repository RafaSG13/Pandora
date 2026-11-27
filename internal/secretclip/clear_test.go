package secretclip

import (
	"crypto/sha256"
	"errors"
	"testing"
)

func TestClearIfHoldsUsing(t *testing.T) {
	want := sha256.Sum256([]byte("hunter2"))
	cases := []struct {
		name        string
		read        func() (string, error)
		wantCleared bool
	}{
		{"holds the value", func() (string, error) { return "hunter2", nil }, true},
		{"holds the value plus a newline", func() (string, error) { return "hunter2\n", nil }, true},
		{"holds something else", func() (string, error) { return "other", nil }, false},
		{"unreadable", func() (string, error) { return "", errors.New("no paste tool") }, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var writes []string
			cleared, err := ClearIfHoldsUsing(c.read, func(s string) error { writes = append(writes, s); return nil }, want)
			if err != nil || cleared != c.wantCleared {
				t.Fatalf("cleared=%v err=%v, want cleared=%v", cleared, err, c.wantCleared)
			}
			if c.wantCleared && (len(writes) != 1 || writes[0] != "") {
				t.Errorf("writes = %q, want one empty write", writes)
			}
			if !c.wantCleared && len(writes) != 0 {
				t.Errorf("clipboard was written although it changed: %q", writes)
			}
		})
	}
}

func TestClearIfHoldsUsingReportsWriteError(t *testing.T) {
	boom := errors.New("boom")
	_, err := ClearIfHoldsUsing(func() (string, error) { return "v", nil }, func(string) error { return boom }, sha256.Sum256([]byte("v")))
	if !errors.Is(err, boom) {
		t.Errorf("err = %v, want the write error", err)
	}
}
