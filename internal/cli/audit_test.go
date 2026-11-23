package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	auditTitle = "audit-title-xyz"
	auditValue = "audit-value-xyz"
)

// auditPath is where the audit log lives for the isolated config dir.
func auditPath(t *testing.T) string {
	t.Helper()
	vault, _ := vaultFiles(t)
	return filepath.Join(filepath.Dir(vault), "audit.log")
}

// logEntries runs `log --json` and returns "action:title" for each entry.
func logEntries(t *testing.T, args ...string) []string {
	t.Helper()
	code, stdout, stderr := harness{}.run(t, append([]string{"log", "--json"}, args...)...)
	if code != 0 {
		t.Fatalf("log: exit %d, stderr %q", code, stderr)
	}
	var entries []logEntry
	if err := json.Unmarshal([]byte(stdout), &entries); err != nil {
		t.Fatalf("log --json: %v\n%s", err, stdout)
	}
	var out []string
	for _, e := range entries {
		if _, err := time.Parse(time.RFC3339, e.Time); err != nil {
			t.Errorf("entry time %q is not RFC 3339", e.Time)
		}
		out = append(out, e.Action+":"+e.Title)
	}
	return out
}

func TestAuditLogRecordsEachActionOnceAndNeverTheValue(t *testing.T) {
	skipWithoutSh(t)
	isolate(t)
	writeConfig(t, `{"audit_log":true}`)
	env := map[string]string{passphraseEnv: "a long enough passphrase"}
	dir := t.TempDir()
	steps := [][]string{
		{"add", auditTitle},
		{"get", auditTitle},
		{"get", auditTitle, "--copy"},
		{"env", "-s", auditTitle},
		{"run", "-s", auditTitle, "--", "true"},
		{"export", "--values", "-o", filepath.Join(dir, "x.age")},
		{"export", "-o", filepath.Join(dir, "x.json")},
		{"import", filepath.Join(dir, "x.json"), "--overwrite"},
		{"rm", auditTitle, "-y"},
	}
	for _, args := range steps {
		h := harness{env: env, stdin: auditValue + "\n", clip: func(string) error { return nil }}
		if code, _, stderr := h.run(t, args...); code != 0 {
			t.Fatalf("%v: exit %d, stderr %q", args, code, stderr)
		}
	}
	got := logEntries(t)
	want := []string{
		"add:" + auditTitle, "get:" + auditTitle, "copy:" + auditTitle, "env:" + auditTitle,
		"run:" + auditTitle, "export:" + auditTitle, "export:", "import:" + auditTitle, "rm:" + auditTitle,
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("log:\n got %v\nwant %v", got, want)
	}

	raw, err := os.ReadFile(auditPath(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{auditTitle, auditValue, "xyz"} {
		if strings.Contains(string(raw), leak) {
			t.Errorf("the audit file contains %q", leak)
		}
	}
	_, stdout, stderr := harness{}.run(t, "log")
	if strings.Contains(stdout+stderr, auditValue) {
		t.Error("`log` printed a value")
	}
	if !strings.Contains(stdout, "TIME") || !strings.Contains(stdout, "ACTION") || !strings.Contains(stdout, auditTitle) {
		t.Errorf("table output:\n%s", stdout)
	}
}

func TestAuditLogRecordsHistoryRestore(t *testing.T) {
	withHistory(t)
	writeConfig(t, `{"audit_log":true,"history_depth":3}`)
	if code, _, stderr := (harness{}).run(t, "restore", "k"); code != 0 {
		t.Fatalf("restore: %d %q", code, stderr)
	}
	if got := logEntries(t); strings.Join(got, "|") != "restore:k" {
		t.Errorf("log = %v", got)
	}
}

func TestAuditLogIsOffByDefaultAndLogSaysSo(t *testing.T) {
	isolate(t)
	if code, _, stderr := (harness{stdin: "v\n"}).run(t, "add", "k"); code != 0 {
		t.Fatalf("add: %d %q", code, stderr)
	}
	if code, _, _ := (harness{}).run(t, "get", "k"); code != 0 {
		t.Fatal("get failed")
	}
	if _, err := os.Stat(auditPath(t)); err == nil {
		t.Error("an audit log was written with audit_log off")
	}
	code, stdout, stderr := harness{}.run(t, "log")
	if code != 0 || stdout != "" || !strings.Contains(stderr, "audit log is off") {
		t.Errorf("log: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if got := logEntries(t); len(got) != 0 {
		t.Errorf("entries = %v", got)
	}
}

func TestLogSinceFiltersAndRejectsBadValues(t *testing.T) {
	isolate(t)
	writeConfig(t, `{"audit_log":true}`)
	if code, _, _ := (harness{stdin: "v\n"}).run(t, "add", "k"); code != 0 {
		t.Fatal("add failed")
	}
	for _, since := range []string{"1h", "1d", "2000-01-01"} { // "0d" means "from now", which excludes it
		if got := logEntries(t, "--since", since); len(got) != 1 {
			t.Errorf("--since %s: %v, want the one entry", since, got)
		}
	}
	if got := logEntries(t, "--since", "2999-01-01"); len(got) != 0 {
		t.Errorf("--since in the future: %v", got)
	}
	for _, bad := range []string{"", "abc", "-1d", "7w", "2026-13-40", "d"} {
		code, stdout, stderr := harness{}.run(t, "log", "--since", bad)
		if code != 2 || stdout != "" || stderr == "" {
			t.Errorf("--since %q: exit %d, stdout %q, stderr %q; want exit 2", bad, code, stdout, stderr)
		}
	}
}

func TestParseSince(t *testing.T) {
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.Local)
	for in, want := range map[string]time.Time{
		"7d":         now.Add(-7 * 24 * time.Hour),
		"12h":        now.Add(-12 * time.Hour),
		"0h":         now,
		"2026-11-01": time.Date(2026, 11, 1, 0, 0, 0, 0, time.Local),
	} {
		if got, err := parseSince(in, now); err != nil || !got.Equal(want) {
			t.Errorf("%q = %v, %v; want %v", in, got, err, want)
		}
	}
}

// A log that cannot be written must not change what a command does.
func TestAuditWriteFailureWarnsOnceAndNeverFailsTheCommand(t *testing.T) {
	isolate(t)
	writeConfig(t, `{"audit_log":true}`)
	if err := os.Mkdir(auditPath(t), 0o700); err != nil { // a directory where the file should be
		t.Fatal(err)
	}
	if code, _, stderr := (harness{stdin: auditValue + "\n"}).run(t, "add", auditTitle); code != 0 {
		t.Fatalf("add: exit %d, stderr %q", code, stderr)
	}
	code, stdout, stderr := harness{}.run(t, "get", auditTitle)
	if code != 0 || stdout != auditValue+"\n" || strings.Count(stderr, "could not write the audit log") != 1 {
		t.Errorf("get: exit %d, stdout %q, stderr %q; want success and one warning", code, stdout, stderr)
	}
}

func TestLogReportsUnreadableLines(t *testing.T) {
	isolate(t)
	writeConfig(t, `{"audit_log":true}`)
	if code, _, _ := (harness{stdin: "v\n"}).run(t, "add", "k"); code != 0 {
		t.Fatal("add failed")
	}
	f, err := os.OpenFile(auditPath(t), os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("garbage\n")
	f.Close()
	code, stdout, stderr := harness{}.run(t, "log")
	if code != 0 || !strings.Contains(stdout, "add") || !strings.Contains(stderr, "1 audit log lines could not be read") {
		t.Errorf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}
