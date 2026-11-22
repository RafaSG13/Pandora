package cli

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/RafaSG13/Pandora/internal/store"
)

var expiringNow = time.Date(2026, 11, 22, 15, 30, 0, 0, time.Local)

// seedExpiries stores secrets around expiringNow; their values are all "VALUE".
func seedExpiries(t *testing.T) {
	t.Helper()
	for title, expires := range map[string]string{
		"gone":     "2026-11-10", // 12 days ago
		"yday":     "2026-11-21",
		"today":    "2026-11-22",
		"tomorrow": "2026-11-23",
		"d14":      "2026-12-06",
		"d15":      "2026-12-07",
		"never":    "",
	} {
		if err := store.SaveSecret(store.SecretMeta{Title: title, Category: "c", Created: "2026-01-01", Expires: expires}, "VALUE"); err != nil {
			t.Fatal(err)
		}
	}
}

func TestExpiringListsWindowBoundariesAndExitsThree(t *testing.T) {
	isolate(t)
	seedExpiries(t)
	tests := []struct {
		args []string
		want []string // titles, soonest first
	}{
		{nil, []string{"gone", "yday", "today", "tomorrow", "d14"}}, // default window is 14 days
		{[]string{"--within", "14d"}, []string{"gone", "yday", "today", "tomorrow", "d14"}},
		{[]string{"--within", "15"}, []string{"gone", "yday", "today", "tomorrow", "d14", "d15"}},
		{[]string{"--within", "1d"}, []string{"gone", "yday", "today", "tomorrow"}},
		{[]string{"--within", "0d"}, []string{"gone", "yday", "today"}},
	}
	for _, tc := range tests {
		code, stdout, stderr := harness{now: expiringNow}.run(t, append([]string{"expiring", "--json"}, tc.args...)...)
		var got []expiringEntry
		if err := json.Unmarshal([]byte(stdout), &got); err != nil {
			t.Fatalf("%v: stdout is not JSON: %v\n%s", tc.args, err, stdout)
		}
		var titles []string
		for _, e := range got {
			titles = append(titles, e.Title)
		}
		if code != 3 || strings.Join(titles, ",") != strings.Join(tc.want, ",") {
			t.Errorf("%v: exit %d, stderr %q, titles %v; want exit 3 and %v", tc.args, code, stderr, titles, tc.want)
		}
	}
}

func TestExpiringJSONShapeAndStatus(t *testing.T) {
	isolate(t)
	seedExpiries(t)
	_, stdout, _ := harness{now: expiringNow}.run(t, "expiring", "--json", "--within", "1d")
	var got []map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	want := []map[string]any{
		{"title": "gone", "category": "c", "expires": "2026-11-10", "days": float64(-12), "status": "expired"},
		{"title": "yday", "category": "c", "expires": "2026-11-21", "days": float64(-1), "status": "expired"},
		{"title": "today", "category": "c", "expires": "2026-11-22", "days": float64(0), "status": "expiring"},
		{"title": "tomorrow", "category": "c", "expires": "2026-11-23", "days": float64(1), "status": "expiring"},
	}
	if len(got) != len(want) {
		t.Fatalf("%d entries, want %d: %s", len(got), len(want), stdout)
	}
	for i := range want {
		for k, v := range want[i] {
			if got[i][k] != v {
				t.Errorf("entry %d %s = %v, want %v", i, k, got[i][k], v)
			}
		}
		if len(got[i]) != len(want[i]) {
			t.Errorf("entry %d has keys %v, want exactly %d", i, got[i], len(want[i]))
		}
	}
}

func TestExpiringNothingDueExitsZero(t *testing.T) {
	isolate(t)
	if err := store.SaveSecret(store.SecretMeta{Title: "later", Created: "2026-01-01", Expires: "2031-01-01"}, "VALUE"); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSecret(store.SecretMeta{Title: "none", Created: "2026-01-01"}, "VALUE"); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := harness{now: expiringNow}.run(t, "expiring")
	if code != 0 || stdout != "" {
		t.Errorf("table: exit %d, stdout %q, stderr %q; want 0 and no stdout", code, stdout, stderr)
	}
	code, stdout, _ = harness{now: expiringNow}.run(t, "expiring", "--json")
	if code != 0 || strings.TrimSpace(stdout) != "[]" {
		t.Errorf("json: exit %d, stdout %q; want 0 and []", code, stdout)
	}
}

func TestExpiringTableNeverPrintsValues(t *testing.T) {
	isolate(t)
	seedExpiries(t)
	code, stdout, stderr := harness{now: expiringNow}.run(t, "expiring")
	if code != 3 || strings.Contains(stdout+stderr, "VALUE") {
		t.Fatalf("exit %d; output must not contain values:\n%s%s", code, stdout, stderr)
	}
	for _, want := range []string{"TITLE", "expired 12d ago", "expires today", "expires in 14d"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("table lacks %q:\n%s", want, stdout)
		}
	}
}

func TestExpiringDefaultWindowComesFromConfig(t *testing.T) {
	isolate(t)
	seedExpiries(t)
	writeConfig(t, `{"expiry_warn_days":1}`)
	_, stdout, _ := harness{now: expiringNow}.run(t, "expiring", "--json")
	var got []expiringEntry
	if err := json.Unmarshal([]byte(stdout), &got); err != nil || len(got) != 4 {
		t.Fatalf("expiry_warn_days=1 listed %d entries (err %v), want 4:\n%s", len(got), err, stdout)
	}
	_, stdout, _ = harness{now: expiringNow}.run(t, "expiring", "--json", "--within", "14d")
	if err := json.Unmarshal([]byte(stdout), &got); err != nil || len(got) != 5 {
		t.Fatalf("--within must override the config: %d entries (err %v)", len(got), err)
	}
}

func TestExpiringRejectsBadFlagsAsUsageErrors(t *testing.T) {
	isolate(t)
	for _, args := range [][]string{
		{"--within", "abc"}, {"--within", "-1"}, {"--within", "3651d"}, {"--within", ""}, {"--within", "1w"}, {"extra"},
	} {
		code, stdout, stderr := harness{now: expiringNow}.run(t, append([]string{"expiring"}, args...)...)
		if code != 2 || stdout != "" || stderr == "" {
			t.Errorf("%v: exit %d, stdout %q, stderr %q; want exit 2, no stdout, a message", args, code, stdout, stderr)
		}
	}
}

func TestExpiringNeedsTheMasterPassword(t *testing.T) {
	isolate(t)
	code, stdout, _ := harness{env: map[string]string{masterEnv: "wrong password here"}, now: expiringNow}.run(t, "expiring")
	if code != 1 || stdout != "" {
		t.Errorf("wrong master password: exit %d, stdout %q; want exit 1", code, stdout)
	}
}
