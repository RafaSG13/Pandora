package store

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func setupAudit(t *testing.T) string {
	t.Helper()
	setup(t)
	SetAuditEnabled(true)
	t.Cleanup(func() { SetAuditEnabled(false) })
	p, err := auditPath()
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func actions(entries []AuditEntry) []string {
	var out []string
	for _, e := range entries {
		out = append(out, e.Action+":"+e.Title)
	}
	return out
}

func TestAuditIsOffByDefaultAndWritesNothing(t *testing.T) {
	setup(t)
	if err := RecordAudit("get", "k"); err != nil {
		t.Fatal(err)
	}
	if AuditExists() {
		t.Error("a log was written although audit is off")
	}
	if entries, bad, err := ReadAudit(time.Time{}); err != nil || len(entries) != 0 || bad != 0 {
		t.Errorf("ReadAudit on a missing log = %v, %d, %v", entries, bad, err)
	}
}

func TestAuditRecordsInOrderAndNeverWritesTitlesInTheClear(t *testing.T) {
	path := setupAudit(t)
	for _, e := range [][2]string{{"add", "very-secret-title"}, {"reveal", "very-secret-title"}, {"export", ""}} {
		if err := RecordAudit(e[0], e[1]); err != nil {
			t.Fatal(err)
		}
	}
	entries, bad, err := ReadAudit(time.Time{})
	if err != nil || bad != 0 {
		t.Fatalf("ReadAudit: %v, %d unreadable", err, bad)
	}
	want := []string{"add:very-secret-title", "reveal:very-secret-title", "export:"}
	if got := actions(entries); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("entries = %v, want %v", got, want)
	}
	if d := time.Since(entries[0].Time); d < 0 || d > time.Minute {
		t.Errorf("entry time %v is not about now", entries[0].Time)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{"very-secret-title", "reveal", "export", "title", "action"} { // long enough that base64 will not contain them by chance
		if strings.Contains(string(raw), leak) {
			t.Errorf("the log file contains %q in the clear:\n%s", leak, raw)
		}
	}
	if runtime.GOOS != "windows" {
		if info, _ := os.Stat(path); info.Mode().Perm() != 0o600 {
			t.Errorf("mode = %v, want 0600", info.Mode().Perm())
		}
	}
}

func TestReadAuditSinceFiltersByTime(t *testing.T) {
	setupAudit(t)
	if err := RecordAudit("get", "old"); err != nil {
		t.Fatal(err)
	}
	cut := time.Now().Add(50 * time.Millisecond)
	time.Sleep(100 * time.Millisecond)
	if err := RecordAudit("get", "new"); err != nil {
		t.Fatal(err)
	}
	got, _, _ := ReadAudit(cut)
	if len(got) != 1 || got[0].Title != "new" {
		t.Errorf("since the cut: %v", actions(got))
	}
	if got, _, _ := ReadAudit(time.Now().Add(time.Hour)); len(got) != 0 {
		t.Errorf("since the future: %v", actions(got))
	}
}

func TestAuditRotatesAtTheSizeCapAndKeepsOneOlderFile(t *testing.T) {
	path := setupAudit(t)
	old := auditMaxBytes
	auditMaxBytes = 600
	t.Cleanup(func() { auditMaxBytes = old })

	var written []string
	for i := range 30 {
		title := strings.Repeat("t", 1+i%3) + string(rune('a'+i%26))
		if err := RecordAudit("get", title); err != nil {
			t.Fatal(err)
		}
		written = append(written, "get:"+title)
	}
	for _, p := range []string{path, path + ".1"} {
		info, err := os.Stat(p)
		if err != nil || info.Size() > auditMaxBytes {
			t.Fatalf("%s: %v (size %v, cap %d)", filepath.Base(p), err, info, auditMaxBytes)
		}
	}
	if _, err := os.Stat(path + ".2"); err == nil {
		t.Error("more than one rotated file is kept")
	}
	got, bad, err := ReadAudit(time.Time{})
	if err != nil || bad != 0 {
		t.Fatalf("ReadAudit: %v, %d", err, bad)
	}
	// What remains is the most recent stretch, in order, ending with the last write.
	tail := written[len(written)-len(got):]
	if len(got) == 0 || len(got) >= len(written) || strings.Join(actions(got), "|") != strings.Join(tail, "|") {
		t.Errorf("after rotation: %d entries, got %v, want the tail %v", len(got), actions(got), tail)
	}
}

func TestAuditNeedsAnUnlockedVault(t *testing.T) {
	setupAudit(t)
	if err := RecordAudit("get", "k"); err != nil {
		t.Fatal(err)
	}
	Lock()
	if err := RecordAudit("get", "k"); !errors.Is(err, ErrLocked) {
		t.Errorf("RecordAudit locked = %v, want ErrLocked", err)
	}
	if _, _, err := ReadAudit(time.Time{}); !errors.Is(err, ErrLocked) {
		t.Errorf("ReadAudit locked = %v, want ErrLocked", err)
	}
}

func TestReadAuditCountsDamagedAndForeignLinesAndKeepsTheRest(t *testing.T) {
	path := setupAudit(t)
	if err := RecordAudit("get", "first"); err != nil {
		t.Fatal(err)
	}

	foreign := make([]byte, keyLen)
	foreign[0] = 1
	sealed, err := seal(foreign, []byte(`{"time":"2026-01-01T00:00:00Z","action":"get","title":"x"}`), []byte(aadAudit))
	if err != nil {
		t.Fatal(err)
	}
	wrongAAD, err := currentKey()
	if err != nil {
		t.Fatal(err)
	}
	other, _ := seal(wrongAAD, []byte(`{"time":"2026-01-01T00:00:00Z","action":"get","title":"y"}`), []byte("not-the-audit-aad"))
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("not base64 at all!\n")
	f.WriteString(base64.StdEncoding.EncodeToString(sealed) + "\n")
	f.WriteString(base64.StdEncoding.EncodeToString(other) + "\n")
	f.Close()
	if err := RecordAudit("get", "last"); err != nil {
		t.Fatal(err)
	}

	got, bad, err := ReadAudit(time.Time{})
	if err != nil || bad != 3 {
		t.Fatalf("ReadAudit: %v, %d unreadable, want 3", err, bad)
	}
	if want := "get:first|get:last"; strings.Join(actions(got), "|") != want {
		t.Errorf("entries = %v, want %s", actions(got), want)
	}
}

// The data key does not change with the password, so the log stays readable.
func TestAuditSurvivesAPasswordChange(t *testing.T) {
	setupAudit(t)
	if err := RecordAudit("get", "k"); err != nil {
		t.Fatal(err)
	}
	if err := ChangePassword(testPassword, "another long password"); err != nil {
		t.Fatal(err)
	}
	Lock()
	if err := Unlock("another long password"); err != nil {
		t.Fatal(err)
	}
	if got, _, err := ReadAudit(time.Time{}); err != nil || len(got) != 1 {
		t.Errorf("after the password change: %v, %v", actions(got), err)
	}
}
