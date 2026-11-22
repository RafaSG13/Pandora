package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/RafaSG13/Pandora/internal/store"
)

// withHistory stores "k" with the values v1, v2, v3 (v3 current) and history on.
func withHistory(t *testing.T) {
	t.Helper()
	isolate(t)
	store.SetHistoryDepth(3)
	t.Cleanup(func() { store.SetHistoryDepth(0) })
	mustSave(t, "k", "c", "v1")
	for _, v := range []string{"v2", "v3"} {
		if err := store.UpdateSecret("k", store.SecretMeta{Title: "k", Category: "c"}, &v); err != nil {
			t.Fatal(err)
		}
	}
}

func TestHistoryListsVersionsNewestFirstWithoutValues(t *testing.T) {
	withHistory(t)
	code, stdout, stderr := harness{}.run(t, "history", "k")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 3 || !strings.HasPrefix(lines[0], "VERSION") || !strings.HasPrefix(lines[1], "-1") || !strings.HasPrefix(lines[2], "-2") {
		t.Errorf("table:\n%s", stdout)
	}
	for _, v := range []string{"v1", "v2", "v3"} {
		if strings.Contains(stdout, v) {
			t.Errorf("history printed a value (%s):\n%s", v, stdout)
		}
	}

	code, stdout, _ = harness{}.run(t, "history", "k", "--json")
	var got []versionEntry
	if err := json.Unmarshal([]byte(stdout), &got); err != nil || code != 0 || len(got) != 2 || got[0].Version != -1 || got[1].Version != -2 || got[0].Replaced == "" {
		t.Errorf("json: exit %d, %v, %+v", code, err, got)
	}
	var raw []map[string]any
	_ = json.Unmarshal([]byte(stdout), &raw)
	if len(raw[0]) != 2 {
		t.Errorf("json keys = %v, want exactly version and replaced", raw[0])
	}
}

func TestHistoryOfASecretWithNoPreviousValues(t *testing.T) {
	isolate(t)
	mustSave(t, "plain", "c", "v")
	code, stdout, stderr := harness{}.run(t, "history", "plain")
	if code != 0 || stdout != "" || !strings.Contains(stderr, "no previous values") {
		t.Errorf("table: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	code, stdout, _ = harness{}.run(t, "history", "plain", "--json")
	if code != 0 || strings.TrimSpace(stdout) != "[]" {
		t.Errorf("json: exit %d, stdout %q, want []", code, stdout)
	}
	if code, _, stderr := (harness{}).run(t, "history", "nope"); code != 1 || !strings.Contains(stderr, `"nope" not found`) {
		t.Errorf("unknown title: exit %d, stderr %q", code, stderr)
	}
}

func TestGetVersionPrintsAPreviousValue(t *testing.T) {
	withHistory(t)
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"get", "k"}, "v3\n"},
		{[]string{"get", "k", "--version", "0"}, "v3\n"},
		{[]string{"get", "k", "--version", "-1"}, "v2\n"},
		{[]string{"get", "--version", "-2", "k", "-q"}, "v1"},
	} {
		code, stdout, stderr := harness{}.run(t, tc.args...)
		if code != 0 || stdout != tc.want {
			t.Errorf("%v: exit %d, stdout %q (%s), want %q", tc.args, code, stdout, stderr, tc.want)
		}
	}

	var copied string
	code, stdout, _ := harness{clip: func(v string) error { copied = v; return nil }}.run(t, "get", "k", "--version", "-1", "--copy")
	if code != 0 || copied != "v2" || stdout != "" {
		t.Errorf("--copy: exit %d, copied %q, stdout %q", code, copied, stdout)
	}
}

func TestGetVersionErrors(t *testing.T) {
	withHistory(t)
	code, stdout, stderr := harness{}.run(t, "get", "k", "--version", "-3")
	if code != 1 || stdout != "" || !strings.Contains(stderr, "pandora history k") {
		t.Errorf("past the history: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	code, stdout, stderr = harness{}.run(t, "get", "k", "--version", "1")
	if code != 2 || stdout != "" || !strings.Contains(stderr, "-1") {
		t.Errorf("positive version: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if code, _, _ := (harness{}).run(t, "get", "k", "--version", "x"); code != 2 {
		t.Errorf("non-numeric version: exit %d, want 2", code)
	}
}

func TestRestoreBringsBackAPreviousValueAndCanBeUndone(t *testing.T) {
	withHistory(t)
	code, stdout, stderr := harness{}.run(t, "restore", "k")
	if code != 0 || stdout != "" || !strings.Contains(stderr, "Restored version -1") {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if _, out, _ := (harness{}).run(t, "get", "k"); out != "v2\n" {
		t.Errorf("after restore get = %q, want v2", out)
	}
	if _, out, _ := (harness{}).run(t, "get", "k", "--version", "-1"); out != "v3\n" {
		t.Errorf("the replaced value must be version -1, got %q", out)
	}

	if code, _, _ := (harness{}).run(t, "restore", "k", "--version", "-2"); code != 0 { // v1
		t.Fatal("restore -2 failed")
	}
	if _, out, _ := (harness{}).run(t, "get", "k"); out != "v1\n" {
		t.Errorf("after restore -2 get = %q, want v1", out)
	}
}

func TestRestoreRejectsBadVersionsAndChangesNothing(t *testing.T) {
	withHistory(t)
	for _, tc := range []struct {
		args []string
		code int
	}{
		{[]string{"restore", "k", "--version", "0"}, 2},
		{[]string{"restore", "k", "--version", "1"}, 2},
		{[]string{"restore", "k", "--version", "-9"}, 1},
		{[]string{"restore", "nope"}, 1},
	} {
		if code, _, stderr := (harness{}).run(t, tc.args...); code != tc.code || stderr == "" {
			t.Errorf("%v: exit %d, stderr %q; want exit %d", tc.args, code, stderr, tc.code)
		}
	}
	if _, out, _ := (harness{}).run(t, "get", "k"); out != "v3\n" {
		t.Errorf("a refused restore changed the value to %q", out)
	}
}

func TestRestoreFailsCleanlyWhenThatValueIsGone(t *testing.T) {
	withHistory(t)
	dropEntries(t, "k", 1)
	code, _, stderr := harness{}.run(t, "restore", "k")
	if code != 1 || !strings.Contains(stderr, "missing from the keyring") || !strings.Contains(stderr, "doctor") {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
	if _, out, _ := (harness{}).run(t, "get", "k"); out != "v3\n" {
		t.Errorf("current value changed to %q", out)
	}
}

// dropEntries removes keyring entries behind the vault's back: back 0 is the
// current value, 1 the newest previous one.
func dropEntries(t *testing.T, title string, back int) {
	t.Helper()
	if err := store.DropEntriesForTests(title, back); err != nil {
		t.Fatal(err)
	}
}

func TestImportOverwriteKeepsTheReplacedValueAsAVersion(t *testing.T) {
	isolate(t)
	t.Cleanup(func() { store.SetHistoryDepth(0) })
	mustSave(t, "tok", "c", "old")
	file := tempFile(t, "tok=new\n")

	code, stdout, stderr := harness{}.run(t, "import", file, "--format", "dotenv", "--overwrite")
	if code != 0 || !strings.Contains(stdout, "replaced 1") {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	// the dotenv title is "tok", so it replaced the secret of that name
	if _, out, _ := (harness{}).run(t, "get", "tok", "--version", "-1"); out != "old\n" {
		t.Errorf("previous value = %q, want old (default history_depth keeps versions)", out)
	}

	// With history switched off in the config nothing is kept.
	writeConfig(t, `{"history_depth":0}`)
	file2 := tempFile(t, "tok=newer\n")
	if code, _, stderr := (harness{}).run(t, "import", file2, "--format", "dotenv", "--overwrite"); code != 0 {
		t.Fatalf("second import: exit %d, stderr %q", code, stderr)
	}
	if _, out, _ := (harness{}).run(t, "get", "tok"); out != "newer\n" {
		t.Errorf("current = %q", out)
	}
	if infos, _ := store.Versions("tok"); len(infos) != 0 {
		t.Errorf("history_depth 0 still leaves %d versions (the existing one is pruned on change)", len(infos))
	}
}

func TestDoctorReportsAndRemovesMissingPreviousValues(t *testing.T) {
	withHistory(t)
	dropEntries(t, "k", 2) // v1
	code, stdout, stderr := harness{}.run(t, "doctor")
	if code != 1 || !strings.Contains(stdout, "1 previous values are missing") || !strings.Contains(stdout, "k (version -2)") || !strings.Contains(stderr, "--fix") {
		t.Fatalf("exit %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	code, stdout, _ = harness{}.run(t, "doctor", "--fix", "-y")
	if code != 0 || !strings.Contains(stdout, "Removed 0 metadata entries and 1 previous values") {
		t.Fatalf("fix: exit %d, stdout:\n%s", code, stdout)
	}
	if infos, _ := store.Versions("k"); len(infos) != 1 {
		t.Errorf("versions after cleanup = %d, want 1", len(infos))
	}
	if code, stdout, _ := (harness{}).run(t, "doctor"); code != 0 || !strings.Contains(stdout, "OK: every entry has a value") {
		t.Errorf("second doctor: exit %d, stdout:\n%s", code, stdout)
	}
}

func TestDoctorKeepsAnEntryWhoseOtherValuesCanStillBeRestored(t *testing.T) {
	withHistory(t)
	dropEntries(t, "k", 0)
	code, stdout, _ := harness{}.run(t, "doctor")
	if code != 1 || !strings.Contains(stdout, "k (has 2 previous values") {
		t.Fatalf("exit %d, stdout:\n%s", code, stdout)
	}
	code, _, stderr := harness{}.run(t, "doctor", "--fix", "-y")
	if code != 1 || !strings.Contains(stderr, "pandora restore") {
		t.Errorf("fix: exit %d, stderr %q", code, stderr)
	}
	if got, _ := store.LoadMetadata(); len(got) != 1 {
		t.Fatalf("doctor --fix removed an entry that could be restored: %+v", got)
	}
	if code, _, _ := (harness{}).run(t, "restore", "k"); code != 0 {
		t.Fatal("restore failed")
	}
	if _, out, _ := (harness{}).run(t, "get", "k"); out != "v2\n" {
		t.Errorf("after restore get = %q", out)
	}
}

func TestDoctorStillRemovesEntriesWithNothingLeftToRestore(t *testing.T) {
	withHistory(t)
	dropEntries(t, "k", 0)
	dropEntries(t, "k", 1)
	dropEntries(t, "k", 2)
	code, stdout, _ := harness{}.run(t, "doctor", "--fix", "-y")
	if code != 0 || !strings.Contains(stdout, "Removed 1 metadata entries and 2 previous values") {
		t.Fatalf("exit %d, stdout:\n%s", code, stdout)
	}
	if got, _ := store.LoadMetadata(); len(got) != 0 {
		t.Errorf("entries left: %+v", got)
	}
}
