package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/zalando/go-keyring"

	"github.com/RafaSG13/Pandora/internal/store"
)

const testMaster = "correct horse battery"

// isolate points the keyring at the in-memory mock and the config directory
// at a temp dir, so tests never touch the real keychain or vault, and creates
// an unlocked vault with testMaster as its password.
func isolate(t *testing.T) {
	t.Helper()
	keyring.MockInit()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	t.Setenv("AppData", dir)
	store.FastKDFForTests()
	t.Cleanup(store.Lock)
	// The commands apply history_depth and audit_log from the config on
	// unlock; those are package settings, so every test starts from "off".
	resetSettings := func() { store.SetHistoryDepth(0); store.SetAuditEnabled(false) }
	resetSettings()
	t.Cleanup(resetSettings)
	store.Lock()
	if err := store.Create(testMaster); err != nil {
		t.Fatal(err)
	}
}

// harness is one CLI invocation with controllable terminal and environment.
// Every run starts like a new process: the vault is locked, and the master
// password comes from PANDORA_MASTER_PASSWORD unless noMaster is set or env
// already defines it.
type harness struct {
	env    map[string]string
	tty    bool
	outTTY bool               // stdout is a terminal
	clip   func(string) error // replaces the clipboard write of get --copy
	x11    bool               // the session is X11
	// clearLater replaces the scheduling of the clipboard-clearing helper (default: no-op).
	clearLater func(string, time.Duration) error
	// clearIfHolds replaces the clipboard clear of the helper command (default: fails the test).
	clearIfHolds func([sha256.Size]byte) (bool, error)
	hidden       []string // answers returned by successive hidden prompts
	stdin        string
	noMaster     bool
	now          time.Time // replaces the clock when set
}

func (h harness) run(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	store.Lock()
	env := map[string]string{}
	for k, v := range h.env {
		env[k] = v
	}
	if _, ok := env[masterEnv]; !ok && !h.noMaster {
		env[masterEnv] = testMaster
	}
	var out, errb bytes.Buffer
	a := newApp(strings.NewReader(h.stdin), &out, &errb)
	a.getenv = func(k string) string { return env[k] }
	a.environ = func() []string {
		e := []string{"PATH=" + os.Getenv("PATH")}
		for k, v := range env {
			e = append(e, k+"="+v)
		}
		return e
	}
	if !h.now.IsZero() {
		a.now = func() time.Time { return h.now }
	}
	a.isTTY = func() bool { return h.tty }
	a.isOutTTY = func() bool { return h.outTTY }
	a.copySecret = func(string) error { t.Fatal("test touched the real clipboard"); return nil }
	if h.clip != nil {
		a.copySecret = h.clip
	}
	a.clearLater = func(string, time.Duration) error { return nil }
	if h.clearLater != nil {
		a.clearLater = h.clearLater
	}
	a.clearIfHolds = func([sha256.Size]byte) (bool, error) {
		t.Fatal("test touched the real clipboard")
		return false, nil
	}
	if h.clearIfHolds != nil {
		a.clearIfHolds = h.clearIfHolds
	}
	a.onX11 = func() bool { return h.x11 }
	a.sleep = func(time.Duration) {}
	answers := append([]string(nil), h.hidden...)
	a.readHidden = func(string) (string, error) {
		if len(answers) == 0 {
			t.Fatal("unexpected hidden prompt")
		}
		v := answers[0]
		answers = answers[1:]
		return v, nil
	}
	a.scryptWorkFactor = 10
	code = a.run(args)
	if !h.noMaster && !store.IsUnlocked() {
		_ = store.Unlock(testMaster) // lets the test inspect the store afterwards
	}
	return code, out.String(), errb.String()
}

func mustSave(t *testing.T, title, category, value string) {
	t.Helper()
	meta := store.SecretMeta{Title: title, Category: category, Created: "2026-01-02", Expires: "2031-03-04", Notes: "note " + title}
	if err := store.SaveSecret(meta, value); err != nil {
		t.Fatal(err)
	}
}

func skipWithoutSh(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("needs sh")
	}
}

func TestAddThenGetRoundTrip(t *testing.T) {
	isolate(t)
	code, _, stderr := harness{stdin: "s3cret value\n"}.run(t, "add", "github", "-c", "Tokens", "--notes", "ci", "--expires", "2031-05-06", "--tag", "Prod,db", "-t", "ci")
	if code != 0 {
		t.Fatalf("add exit %d: %s", code, stderr)
	}
	code, stdout, stderr := harness{}.run(t, "get", "github")
	if code != 0 || stdout != "s3cret value\n" {
		t.Fatalf("get = %d %q (%s), want the value plus newline", code, stdout, stderr)
	}

	items, _ := store.LoadMetadata()
	want := store.SecretMeta{Title: "github", Category: "Tokens", Created: items[0].Created, Expires: "2031-05-06", Notes: "ci", Tags: []string{"ci", "db", "prod"}}
	if len(items) != 1 || !reflect.DeepEqual(items[0], want) || items[0].Created == "" {
		t.Errorf("metadata = %+v, want %+v", items, want)
	}
}

func TestAddStripsOnlyOneTrailingNewline(t *testing.T) {
	isolate(t)
	if code, _, e := (harness{stdin: "line\r\n\n"}).run(t, "add", "a"); code != 0 {
		t.Fatal(e)
	}
	if v, _ := store.GetSecretValue("a"); v != "line\r\n" {
		t.Errorf("value = %q", v)
	}
	if code, _, e := (harness{stdin: "x\r\n"}).run(t, "add", "b"); code != 0 {
		t.Fatal(e)
	}
	if v, _ := store.GetSecretValue("b"); v != "x" {
		t.Errorf("CRLF value = %q, want %q", v, "x")
	}
}

func TestAddFlagsMayFollowTitle(t *testing.T) {
	isolate(t)
	if code, _, e := (harness{stdin: "v"}).run(t, "add", "t", "-c", "K"); code != 0 {
		t.Fatal(e)
	}
	if items, _ := store.LoadMetadata(); len(items) != 1 || items[0].Category != "K" {
		t.Errorf("items = %+v", items)
	}
}

func TestAddRejectsDuplicateEmptyValueAndBadDate(t *testing.T) {
	isolate(t)
	mustSave(t, "dup", "X", "original")

	code, _, stderr := harness{stdin: "other\n"}.run(t, "add", "dup")
	if code != 1 || !strings.Contains(stderr, "already exists") {
		t.Errorf("duplicate: code %d, stderr %q", code, stderr)
	}
	if v, _ := store.GetSecretValue("dup"); v != "original" {
		t.Errorf("duplicate overwrote the value: %q", v)
	}

	for _, stdin := range []string{"", "\n"} {
		if code, _, stderr := (harness{stdin: stdin}).run(t, "add", "empty"); code != 1 || !strings.Contains(stderr, "empty value") {
			t.Errorf("empty stdin %q: code %d, stderr %q", stdin, code, stderr)
		}
	}

	code, _, stderr = harness{stdin: "v"}.run(t, "add", "bad", "--expires", "2031-13-40")
	if code != 2 || !strings.Contains(stderr, "YYYY-MM-DD") {
		t.Errorf("bad date: code %d, stderr %q", code, stderr)
	}
	if got := titlesOf(t); !reflect.DeepEqual(got, []string{"dup"}) {
		t.Errorf("rejected adds left entries behind: %v", got)
	}
}

func TestAddRejectsBadTagsWithUsageError(t *testing.T) {
	isolate(t)
	for _, tag := range []string{"two words", strings.Repeat("x", store.MaxTagLen+1), "a\x1b[2J"} {
		code, _, stderr := harness{stdin: "v"}.run(t, "add", "bad", "--tag", tag)
		if code != 2 || !strings.Contains(stderr, "--tag") {
			t.Errorf("--tag %q: code %d, stderr %q; want usage error 2", tag, code, stderr)
		}
	}
	if got := titlesOf(t); len(got) != 0 {
		t.Errorf("rejected adds left entries behind: %v", got)
	}
}

func TestAddPromptsHiddenOnTerminal(t *testing.T) {
	isolate(t)
	code, _, stderr := harness{tty: true, hidden: []string{"typed secret"}}.run(t, "add", "prompted")
	if code != 0 {
		t.Fatal(stderr)
	}
	if v, _ := store.GetSecretValue("prompted"); v != "typed secret" {
		t.Errorf("value = %q", v)
	}
}

func TestGetMissingExitsOneWithMessageOnStderr(t *testing.T) {
	isolate(t)
	code, stdout, stderr := harness{}.run(t, "get", "nope")
	if code != 1 || stdout != "" || !strings.Contains(stderr, `"nope" not found`) {
		t.Errorf("code %d stdout %q stderr %q", code, stdout, stderr)
	}
}

func TestListNeverShowsValuesAndFiltersByCategory(t *testing.T) {
	isolate(t)
	mustSave(t, "one", "Tokens", "VALUE-ONE")
	mustSave(t, "two", "Infra", "VALUE-TWO")

	code, stdout, _ := harness{}.run(t, "list")
	if code != 0 || strings.Contains(stdout, "VALUE") || !strings.Contains(stdout, "one") || !strings.Contains(stdout, "two") {
		t.Fatalf("table: code %d %q", code, stdout)
	}

	code, stdout, _ = harness{}.run(t, "list", "--category", "tokens", "--json")
	if code != 0 || strings.Contains(stdout, "VALUE") {
		t.Fatalf("json: code %d %q", code, stdout)
	}
	var got []listEntry
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	want := []listEntry{{Title: "one", Category: "Tokens", Created: "2026-01-02", Expires: "2031-03-04", Tags: []string{}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}

	if _, stdout, _ = (harness{}).run(t, "list", "--category", "none", "--json"); strings.TrimSpace(stdout) != "[]" {
		t.Errorf("empty result = %q, want []", stdout)
	}
}

func TestListFiltersByTagsAndShowsThemInJSON(t *testing.T) {
	isolate(t)
	for title, tags := range map[string][]string{
		"both":  {"prod", "db"},
		"prod":  {"prod"},
		"plain": nil,
	} {
		if err := store.SaveSecret(store.SecretMeta{Title: title, Created: "2026-01-02", Tags: tags}, "v"); err != nil {
			t.Fatal(err)
		}
	}
	titlesFor := func(args ...string) []string {
		t.Helper()
		code, stdout, stderr := harness{}.run(t, append([]string{"list", "--json"}, args...)...)
		if code != 0 {
			t.Fatalf("list %v: %d %s", args, code, stderr)
		}
		var got []listEntry
		if err := json.Unmarshal([]byte(stdout), &got); err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, e := range got {
			out = append(out, e.Title)
		}
		slices.Sort(out)
		return out
	}
	for _, c := range []struct {
		args []string
		want []string
	}{
		{[]string{"--tag", "prod"}, []string{"both", "prod"}},
		{[]string{"-t", "PROD"}, []string{"both", "prod"}}, // case-insensitive
		{[]string{"--tag", "prod", "--tag", "db"}, []string{"both"}},
		{[]string{"--tag", "prod,db"}, []string{"both"}}, // all of them, not any
		{[]string{"--tag", "nope"}, nil},
		{nil, []string{"both", "plain", "prod"}},
	} {
		if got := titlesFor(c.args...); !reflect.DeepEqual(got, c.want) {
			t.Errorf("list %v = %v, want %v", c.args, got, c.want)
		}
	}

	code, stdout, _ := harness{}.run(t, "list", "--tag", "db")
	if code != 0 || !strings.Contains(stdout, "TAGS") || !strings.Contains(stdout, "db,prod") {
		t.Errorf("table = %q, want a TAGS column with db,prod", stdout)
	}
	if code, _, stderr := (harness{}).run(t, "list", "--tag", "two words"); code != 2 || !strings.Contains(stderr, "--tag") {
		t.Errorf("bad filter tag: code %d, stderr %q", code, stderr)
	}
}

func TestRmRefusesWithoutYesOnNonTTY(t *testing.T) {
	isolate(t)
	mustSave(t, "keep", "X", "v")

	code, _, stderr := harness{}.run(t, "rm", "keep")
	if code != 1 || !strings.Contains(stderr, "-y") {
		t.Errorf("code %d stderr %q, want refusal mentioning -y", code, stderr)
	}
	if got := titlesOf(t); !reflect.DeepEqual(got, []string{"keep"}) {
		t.Fatalf("secret was deleted without confirmation: %v", got)
	}
	if _, err := store.GetSecretValue("keep"); err != nil {
		t.Errorf("value lost: %v", err)
	}

	if code, _, e := (harness{}).run(t, "rm", "keep", "-y"); code != 0 {
		t.Fatalf("rm -y: %s", e)
	}
	if got := titlesOf(t); len(got) != 0 {
		t.Errorf("still present: %v", got)
	}
	if code, _, _ := (harness{}).run(t, "rm", "keep", "-y"); code != 1 {
		t.Errorf("rm of a missing secret exit %d, want 1", code)
	}
}

func TestRmConfirmsOnTerminal(t *testing.T) {
	isolate(t)
	mustSave(t, "x", "X", "v")

	if code, _, _ := (harness{tty: true, stdin: "n\n"}).run(t, "rm", "x"); code != 1 {
		t.Errorf("declined: exit %d, want 1", code)
	}
	if got := titlesOf(t); len(got) != 1 {
		t.Fatalf("declined confirmation still deleted: %v", got)
	}
	if code, _, e := (harness{tty: true, stdin: "y\n"}).run(t, "rm", "x"); code != 0 {
		t.Fatal(e)
	}
	if got := titlesOf(t); len(got) != 0 {
		t.Errorf("confirmed rm left %v", got)
	}
}

func TestRunInjectsEnvForChildOnlyAndPropagatesExitCode(t *testing.T) {
	skipWithoutSh(t)
	isolate(t)
	mustSave(t, "api-key", "X", "KEY-VALUE")
	mustSave(t, "db pass.v2", "X", "DB-VALUE")
	t.Setenv("MY_KEY", "") // make sure the parent never holds the secret
	os.Unsetenv("MY_KEY")

	h := harness{env: map[string]string{"PANDORA_PASSPHRASE": "hunter2", "KEEP": "me"}}
	code, stdout, stderr := h.run(t, "run", "-s", "api-key=MY_KEY", "-s", "db pass.v2", "--",
		"sh", "-c", `printf '%s|%s|%s|[%s][%s]' "$MY_KEY" "$DB_PASS_V2" "$KEEP" "$PANDORA_PASSPHRASE" "$PANDORA_MASTER_PASSWORD"; exit 7`)
	if code != 7 {
		t.Errorf("exit code = %d, want 7 (stderr %q)", code, stderr)
	}
	if want := "KEY-VALUE|DB-VALUE|me|[][]"; stdout != want {
		t.Errorf("child saw %q, want %q", stdout, want)
	}
	if os.Getenv("MY_KEY") != "" || os.Getenv("DB_PASS_V2") != "" {
		t.Error("secret leaked into the parent environment")
	}
}

func TestRunPassesStdinAndReturnsZero(t *testing.T) {
	skipWithoutSh(t)
	isolate(t)
	mustSave(t, "tok", "X", "T")
	code, stdout, stderr := harness{stdin: "from-stdin"}.run(t, "run", "-s", "tok", "--", "sh", "-c", `cat; printf ' %s' "$TOK"`)
	if code != 0 || stdout != "from-stdin T" {
		t.Errorf("code %d stdout %q stderr %q", code, stdout, stderr)
	}
}

func TestRunMissingSecretFailsBeforeRunningAnything(t *testing.T) {
	skipWithoutSh(t)
	isolate(t)
	mustSave(t, "present", "X", "v")
	marker := filepath.Join(t.TempDir(), "ran")

	code, _, stderr := harness{}.run(t, "run", "-s", "present", "-s", "absent", "--", "sh", "-c", "touch "+marker)
	if code != 1 || !strings.Contains(stderr, `"absent" not found`) {
		t.Errorf("code %d stderr %q", code, stderr)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Error("command ran although a secret was missing")
	}
}

func TestRunUsageErrors(t *testing.T) {
	isolate(t)
	mustSave(t, "a", "X", "v")
	mustSave(t, "a-b", "X", "v")
	mustSave(t, "a_b", "X", "v")
	for name, args := range map[string][]string{
		"no secrets":      {"run", "--", "true"},
		"no command":      {"run", "-s", "a"},
		"empty command":   {"run", "-s", "a", "--"},
		"env name clash":  {"run", "-s", "a-b", "-s", "a_b", "--", "true"},
		"empty env name":  {"run", "-s", "a=", "--", "true"},
		"stray argument":  {"run", "-s", "a", "extra", "--", "true"},
		"underivable env": {"run", "-s", "é", "--", "true"},
	} {
		if code, _, _ := (harness{}).run(t, args...); code != 2 {
			t.Errorf("%s: exit %d, want 2", name, code)
		}
	}
}

func TestEnvName(t *testing.T) {
	for in, want := range map[string]string{
		"github-token": "GITHUB_TOKEN",
		"my api.key2":  "MY_API_KEY2",
		"already_OK":   "ALREADY_OK",
	} {
		if got := EnvName(in); got != want {
			t.Errorf("EnvName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestUsageAndHelpExitCodes(t *testing.T) {
	isolate(t)
	if code, stdout, _ := (harness{}).run(t, "help"); code != 0 || !strings.Contains(stdout, "Commands:") {
		t.Errorf("help: %d %q", code, stdout)
	}
	if code, stdout, _ := (harness{}).run(t, "add", "-h"); code != 0 || !strings.Contains(stdout, "Usage: pandora add") {
		t.Errorf("add -h: %d %q", code, stdout)
	}
	if code, _, stderr := (harness{}).run(t, "bogus"); code != 2 || !strings.Contains(stderr, "unknown command") {
		t.Errorf("unknown: %d %q", code, stderr)
	}
	if code, _, _ := (harness{}).run(t, "get"); code != 2 {
		t.Errorf("get without title: exit %d, want 2", code)
	}
	if code, _, _ := (harness{}).run(t, "list", "--nope"); code != 2 {
		t.Errorf("bad flag: exit %d, want 2", code)
	}
}

func TestDoctorDetectsAndFixesOrphan(t *testing.T) {
	isolate(t)
	mustSave(t, "healthy", "X", "v")
	if err := store.RestoreSecrets([]store.Restore{{Meta: store.SecretMeta{Title: "orphan", Category: "X", Created: "2026-01-02"}}}); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := harness{}.run(t, "doctor")
	if code != 1 || !strings.Contains(stdout, "orphan") || strings.Contains(stdout, "healthy\n") {
		t.Errorf("report: code %d stdout %q stderr %q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "cannot be detected") {
		t.Errorf("limitation not stated: %q", stdout)
	}

	if code, _, _ := (harness{}).run(t, "doctor", "--fix"); code != 1 {
		t.Errorf("--fix without -y on non-TTY: exit %d, want refusal", code)
	}
	if code, _, _ := (harness{tty: true, stdin: "n\n"}).run(t, "doctor", "--fix"); code != 1 {
		t.Errorf("--fix declined: exit %d", code)
	}
	if got := titlesOf(t); len(got) != 2 {
		t.Fatalf("entries removed without confirmation: %v", got)
	}

	if code, _, e := (harness{}).run(t, "doctor", "--fix", "-y"); code != 0 {
		t.Fatalf("--fix -y: %d %s", code, e)
	}
	if got := titlesOf(t); !reflect.DeepEqual(got, []string{"healthy"}) {
		t.Errorf("after fix: %v", got)
	}
	if v, _ := store.GetSecretValue("healthy"); v != "v" {
		t.Error("fix touched a healthy secret")
	}
	if code, stdout, _ := (harness{}).run(t, "doctor"); code != 0 || !strings.Contains(stdout, "OK") {
		t.Errorf("second run: %d %q", code, stdout)
	}
}

func titlesOf(t *testing.T) []string {
	t.Helper()
	items, err := store.LoadMetadata()
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, len(items))
	for i, m := range items {
		out[i] = m.Title
	}
	return out
}
