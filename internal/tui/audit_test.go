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

func auditLogPath(t *testing.T) string {
	t.Helper()
	dir, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, "pandora", "audit.log")
}

func auditActions(t *testing.T) string {
	t.Helper()
	entries, bad, err := store.ReadAudit(time.Time{})
	if err != nil || bad != 0 {
		t.Fatalf("ReadAudit: %v, %d unreadable", err, bad)
	}
	var out []string
	for _, e := range entries {
		out = append(out, e.Action+":"+e.Title)
	}
	return strings.Join(out, "|")
}

func auditModel(t *testing.T) tea.Model {
	t.Helper()
	t.Cleanup(func() { store.SetAuditEnabled(false); store.SetHistoryDepth(0) })
	seed := func(m tea.Model) tea.Model { return add(m, "k", "X", "v1") }
	return withConfig(t, `{"audit_log":true}`, 100, 30, seed)
}

func TestTUIActionsAreAuditedOnceEach(t *testing.T) {
	m := auditModel(t)
	m = press(m, "v") // reveal
	m = press(m, "v") // hide: not an access
	m = press(m, "c") // copy
	m = editValue(m, "v2")
	m = press(m, "H", "r", "esc") // reveal a previous value
	m = press(m, "H", "enter")    // restore it
	m = press(m, "d", "y")        // delete

	want := "reveal:k|copy:k|edit:k|reveal:k|restore:k|rm:k"
	if got := auditActions(t); got != want {
		t.Errorf("audit log = %s, want %s", got, want)
	}
	raw, err := os.ReadFile(auditLogPath(t))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "reveal") || strings.Contains(string(raw), `"k"`) {
		t.Error("the log file holds readable text")
	}
	_ = m
}

func TestTUIAddIsAudited(t *testing.T) {
	m := auditModel(t)
	m = add(m, "second", "Y", "s3cret")
	if got := auditActions(t); got != "add:second" {
		t.Errorf("audit log = %s", got)
	}
	_ = m
}

func TestTUIWritesNoAuditLogUnlessEnabled(t *testing.T) {
	t.Cleanup(func() { store.SetHistoryDepth(0) })
	m := add(newTestModel(t), "k", "X", "v1")
	m = press(m, "v", "c")
	if _, err := os.Stat(auditLogPath(t)); err == nil {
		t.Error("an audit log was written with audit_log off")
	}
	_ = m
}

func TestTUIAuditWriteFailureShowsAToastAndDoesNotBlockTheAction(t *testing.T) {
	m := auditModel(t)
	if err := os.Mkdir(auditLogPath(t), 0o700); err != nil { // a directory where the file should be
		t.Fatal(err)
	}
	m = press(m, "v")
	mm := m.(model)
	if mm.revealTitle != "k" || mm.revealValue != "v1" {
		t.Errorf("the reveal was blocked by the audit failure: %q %q", mm.revealTitle, mm.revealValue)
	}
	if !strings.Contains(mm.statusMsg, "audit log") || !mm.statusErr {
		t.Errorf("status = %q (err %v), want a toast about the audit log", mm.statusMsg, mm.statusErr)
	}
}
