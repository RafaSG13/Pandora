package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/RafaSG13/Pandora/internal/store"
)

func TestCommandsNeedTheMasterPassword(t *testing.T) {
	isolate(t)
	mustSave(t, "github", "Tokens", "v")
	file := filepath.Join(t.TempDir(), "in.json")
	if err := os.WriteFile(file, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{
		{"list"}, {"get", "github"}, {"add", "x"}, {"rm", "github", "-y"},
		{"run", "-s", "github", "--", "true"}, {"doctor"}, {"export"}, {"import", file},
	} {
		code, stdout, stderr := harness{noMaster: true, stdin: "v"}.run(t, args...)
		if code != 1 || !strings.Contains(stderr, masterEnv) || stdout != "" {
			t.Errorf("%v: exit %d, stdout %q, stderr %q; want exit 1 naming %s", args, code, stdout, stderr, masterEnv)
		}
		if store.IsUnlocked() {
			t.Errorf("%v left the vault unlocked", args)
		}
	}
}

func TestEnvPasswordUnlocksAndWarnsOnce(t *testing.T) {
	isolate(t)
	mustSave(t, "github", "Tokens", "tok")

	code, stdout, stderr := harness{}.run(t, "get", "github")
	if code != 0 || stdout != "tok\n" {
		t.Fatalf("get = %d %q (%s)", code, stdout, stderr)
	}
	if n := strings.Count(stderr, "other processes of your user"); n != 1 || !strings.Contains(stderr, masterEnv) {
		t.Errorf("stderr %q: want one warning naming %s", stderr, masterEnv)
	}

	code, _, stderr = harness{env: map[string]string{masterEnv: "wrong wrong wrong"}}.run(t, "get", "github")
	if code != 1 || !strings.Contains(stderr, "wrong master password") {
		t.Errorf("wrong env password: exit %d, stderr %q", code, stderr)
	}
}

func TestTerminalPromptRetriesThenGivesUp(t *testing.T) {
	isolate(t)
	mustSave(t, "github", "Tokens", "tok")

	code, stdout, stderr := harness{noMaster: true, tty: true, hidden: []string{"nope nope nope", testMaster}}.run(t, "get", "github")
	if code != 0 || stdout != "tok\n" || !strings.Contains(stderr, "try again") {
		t.Errorf("second attempt: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}

	bad := []string{"bad bad bad 1", "bad bad bad 2", "bad bad bad 3"}
	code, stdout, stderr = harness{noMaster: true, tty: true, hidden: bad}.run(t, "get", "github")
	if code != 1 || stdout != "" || !strings.Contains(stderr, "wrong master password") {
		t.Errorf("three wrong passwords: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if store.IsUnlocked() {
		t.Error("vault unlocked after three wrong passwords")
	}
}

func TestFirstRunCreatesVaultOnTerminalOnly(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	t.Setenv("AppData", dir)
	store.FastKDFForTests()
	t.Cleanup(store.Lock)

	code, _, stderr := harness{noMaster: true}.run(t, "list")
	if code != 1 || !strings.Contains(stderr, "run pandora in a terminal") || store.Exists() {
		t.Fatalf("no TTY: exit %d, stderr %q, vault exists %v", code, stderr, store.Exists())
	}

	// Too short, mismatched, then valid.
	hidden := []string{"short", "short", "long enough pw 1", "different pw 22", "long enough pw 1", "long enough pw 1"}
	code, _, stderr = harness{noMaster: true, tty: true, hidden: hidden}.run(t, "list")
	if code != 0 {
		t.Fatalf("setup: exit %d, stderr %q", code, stderr)
	}
	for _, want := range []string{"cannot be recovered", "at least 10 characters", "do not match"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr lacks %q: %q", want, stderr)
		}
	}
	store.Lock()
	if err := store.Unlock("long enough pw 1"); err != nil {
		t.Errorf("created vault does not open with the chosen password: %v", err)
	}
}

func TestFirstRunMigratesLegacyStore(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	t.Setenv("AppData", dir)
	store.FastKDFForTests()
	t.Cleanup(store.Lock)

	cfg, _ := os.UserConfigDir()
	legacy := filepath.Join(cfg, "pandora", "metadata.json")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte(`[{"title":"github","category":"Tokens","created":"2025-01-01"}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := keyring.Set(store.ServiceName, "github", "old-value"); err != nil {
		t.Fatal(err)
	}

	pw := "migrated password"
	code, _, stderr := harness{noMaster: true, tty: true, hidden: []string{pw, pw}}.run(t, "list")
	if code != 0 || !strings.Contains(stderr, "Migrated 1 secrets") {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	code, stdout, _ := harness{env: map[string]string{masterEnv: pw}}.run(t, "get", "github")
	if code != 0 || stdout != "old-value\n" {
		t.Errorf("get after migration = %d %q", code, stdout)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Errorf("metadata.json still present: %v", err)
	}
}

func TestPasswdChangesThePassword(t *testing.T) {
	isolate(t)
	mustSave(t, "github", "Tokens", "tok")
	next := "a brand new password"

	if code, _, stderr := (harness{tty: true, noMaster: true, hidden: []string{"wrong wrong wrong", next, next}}).run(t, "passwd"); code != 1 || !strings.Contains(stderr, "wrong master password") {
		t.Errorf("wrong current password: exit %d, stderr %q", code, stderr)
	}
	if code, _, stderr := (harness{tty: true, noMaster: true, hidden: []string{testMaster, next, "typo"}}).run(t, "passwd"); code != 1 || !strings.Contains(stderr, "do not match") {
		t.Errorf("mismatch: exit %d, stderr %q", code, stderr)
	}
	if code, _, stderr := (harness{tty: true, noMaster: true, hidden: []string{testMaster, "short", "short"}}).run(t, "passwd"); code != 1 || !strings.Contains(stderr, "at least") {
		t.Errorf("short: exit %d, stderr %q", code, stderr)
	}
	if code, _, stderr := (harness{noMaster: true}).run(t, "passwd"); code != 1 || !strings.Contains(stderr, "terminal") {
		t.Errorf("no TTY: exit %d, stderr %q", code, stderr)
	}

	code, _, stderr := harness{tty: true, noMaster: true, hidden: []string{testMaster, next, next}}.run(t, "passwd")
	if code != 0 || !strings.Contains(stderr, "Master password changed.") {
		t.Fatalf("passwd: exit %d, stderr %q", code, stderr)
	}
	if code, _, _ := (harness{}).run(t, "get", "github"); code != 1 {
		t.Errorf("old password still accepted: exit %d", code)
	}
	code, stdout, _ := harness{env: map[string]string{masterEnv: next}}.run(t, "get", "github")
	if code != 0 || stdout != "tok\n" {
		t.Errorf("get with new password = %d %q", code, stdout)
	}
}

func TestCompleteListsNamesOnlyWithMasterPasswordInEnv(t *testing.T) {
	isolate(t)
	mustSave(t, "github", "Tokens", "v")

	code, stdout, stderr := harness{noMaster: true, tty: true}.run(t, "__complete", "titles")
	if code != 0 || stdout != "" || stderr != "" {
		t.Errorf("without env: exit %d, stdout %q, stderr %q; want silence", code, stdout, stderr)
	}
	code, stdout, _ = harness{}.run(t, "__complete", "titles")
	if code != 0 || stdout != "github\n" {
		t.Errorf("with env: exit %d, stdout %q", code, stdout)
	}
	code, stdout, stderr = harness{env: map[string]string{masterEnv: "wrong wrong wrong"}}.run(t, "__complete", "titles")
	if code != 0 || stdout != "" || stderr != "" {
		t.Errorf("wrong env: exit %d, stdout %q, stderr %q; want silence", code, stdout, stderr)
	}
}

func TestImportRejectsControlCharactersAndWritesNothing(t *testing.T) {
	isolate(t)
	doc := `{"format":"pandora-export","version":1,"secrets":[
	  {"title":"fine","category":"X","created":"2026-01-01"},
	  {"title":"evil\u001b[2J","category":"X","created":"2026-01-01"}]}`
	if err := store.Unlock(testMaster); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "evil.json")
	if err := os.WriteFile(file, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}

	code, _, stderr := harness{}.run(t, "import", file)
	if code != 1 || !strings.Contains(stderr, "control characters") {
		t.Errorf("exit %d, stderr %q; want exit 1 about control characters", code, stderr)
	}
	if got := titlesOf(t); !reflect.DeepEqual(got, []string{}) {
		t.Errorf("titles = %v, want nothing written", got)
	}
}

func TestAddRejectsControlCharactersInNotes(t *testing.T) {
	isolate(t)
	code, _, stderr := harness{stdin: "v"}.run(t, "add", "x", "--notes", "a\x1b[31mred")
	if code != 2 || !strings.Contains(stderr, "control characters") {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
}

func TestExportPassphraseMustBeLongEnough(t *testing.T) {
	isolate(t)
	seed(t)
	file := filepath.Join(t.TempDir(), "b.age")

	code, _, stderr := withPass("123456789").run(t, "export", "--values", "-o", file)
	if code != 1 || !strings.Contains(stderr, "at least 10 characters") {
		t.Errorf("env passphrase: exit %d, stderr %q", code, stderr)
	}
	code, _, stderr = harness{tty: true, hidden: []string{"123456789", "123456789"}}.run(t, "export", "--values", "-o", file)
	if code != 1 || !strings.Contains(stderr, "at least 10 characters") {
		t.Errorf("prompted passphrase: exit %d, stderr %q", code, stderr)
	}
	if _, err := os.Stat(file); err == nil {
		t.Error("export written with a weak passphrase")
	}

	code, _, stderr = withPass("1234567890").run(t, "export", "--values", "-o", file)
	if code != 0 || !strings.Contains(stderr, passphraseEnv) {
		t.Errorf("10 chars: exit %d, stderr %q; want success with an env warning", code, stderr)
	}
}

func TestExportWithoutValuesSaysTitlesArePlainText(t *testing.T) {
	isolate(t)
	seed(t)
	file := filepath.Join(t.TempDir(), "meta.json")
	_, _, stderr := harness{}.run(t, "export", "-o", file)
	if !strings.Contains(stderr, "titles are not encrypted") {
		t.Errorf("stderr %q", stderr)
	}
}
