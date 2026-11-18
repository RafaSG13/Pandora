package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func write(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(Path(dir), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestNoFileAndNoEnvGivesTheDefaults(t *testing.T) {
	for _, dir := range []string{"", t.TempDir()} {
		c, warns := Load(dir, env(nil))
		if c != Defaults() || len(warns) != 0 {
			t.Errorf("dir %q: %+v, warnings %q; want the defaults and no warnings", dir, c, warns)
		}
	}
}

func TestPrecedenceEnvThenFileThenDefault(t *testing.T) {
	file := `{"lock_minutes": 30, "ascii": true, "compact": true}`
	for _, c := range []struct {
		name      string
		file      string
		env       map[string]string
		wantLock  int
		wantASCII bool
	}{
		{"default", ``, nil, DefaultLockMinutes, false},
		{"file beats default", file, nil, 30, true},
		{"env beats file", file, map[string]string{"PANDORA_LOCK_MINUTES": "2"}, 2, true},
		{"env 0 disables even when the file sets a time", file, map[string]string{"PANDORA_LOCK_MINUTES": "0"}, 0, true},
		{"env ascii", ``, map[string]string{"PANDORA_ASCII": "1"}, DefaultLockMinutes, true},
		{"env ascii other value is not a switch", ``, map[string]string{"PANDORA_ASCII": "0"}, DefaultLockMinutes, false},
	} {
		dir := ""
		if c.file != "" {
			dir = write(t, c.file)
		}
		got, warns := Load(dir, env(c.env))
		if got.LockMinutes != c.wantLock || got.ASCII != c.wantASCII || len(warns) != 0 {
			t.Errorf("%s: lock=%d ascii=%v warnings=%q, want lock=%d ascii=%v", c.name, got.LockMinutes, got.ASCII, warns, c.wantLock, c.wantASCII)
		}
	}
}

func TestInvalidEnvLockFallsBackToFileThenDefaultWithAWarning(t *testing.T) {
	for _, v := range []string{"abc", "-1", "1.5", "999999999"} {
		c, warns := Load(write(t, `{"lock_minutes": 30}`), env(map[string]string{"PANDORA_LOCK_MINUTES": v}))
		if c.LockMinutes != 30 || len(warns) != 1 || !strings.Contains(warns[0], "PANDORA_LOCK_MINUTES") {
			t.Errorf("env %q: lock=%d warnings=%q, want the file's 30 and one warning", v, c.LockMinutes, warns)
		}
	}
}

func TestEveryKeyIsReadFromTheFile(t *testing.T) {
	c, warns := Load(write(t, `{
		"lock_minutes": 0, "ascii": true, "clipboard_seconds": 30, "reveal_seconds": 20,
		"expiry_warn_days": 7, "gen_length": 48, "gen_charset": "hex", "sort": "name", "compact": true}`), env(nil))
	want := Config{LockMinutes: 0, ASCII: true, ClipboardSeconds: 30, RevealSeconds: 20, ExpiryWarnDays: 7,
		GenLength: 48, GenCharset: "hex", Sort: "name", Compact: true}
	if c != want || len(warns) != 0 {
		t.Errorf("got %+v, warnings %q; want %+v", c, warns, want)
	}
}

func TestBadValuesFallBackPerKeyAndEachWarns(t *testing.T) {
	d := Defaults()
	for _, c := range []struct{ name, json string }{
		{"clipboard too small", `{"clipboard_seconds": 0}`},
		{"clipboard too large", `{"clipboard_seconds": 601}`},
		{"reveal too large", `{"reveal_seconds": 301}`},
		{"expiry days too large", `{"expiry_warn_days": 366}`},
		{"negative lock", `{"lock_minutes": -1}`},
		{"null", `{"sort": null}`},
		{"null lock must not disable the idle lock", `{"lock_minutes": null}`},
		{"fractional", `{"clipboard_seconds": 1.5}`},
		{"string for number", `{"reveal_seconds": "10"}`},
		{"number for bool", `{"compact": 1}`},
		{"gen length not a preset", `{"gen_length": 20}`},
		{"unknown charset", `{"gen_charset": "emoji"}`},
		{"unknown sort", `{"sort": "size"}`},
	} {
		got, warns := Load(write(t, c.json), env(nil))
		if got != d || len(warns) != 1 {
			t.Errorf("%s: %+v, warnings %q; want the defaults and exactly one warning", c.name, got, warns)
		}
	}

	// One bad key does not discard the good ones next to it.
	got, warns := Load(write(t, `{"clipboard_seconds": 0, "reveal_seconds": 20}`), env(nil))
	if got.RevealSeconds != 20 || got.ClipboardSeconds != DefaultClipboardSeconds || len(warns) != 1 {
		t.Errorf("got %+v, warnings %q; want reveal 20, default clipboard, one warning", got, warns)
	}
}

func TestBoundsAreInclusive(t *testing.T) {
	got, warns := Load(write(t, `{"clipboard_seconds": 1, "reveal_seconds": 300, "expiry_warn_days": 365, "lock_minutes": 525600}`), env(nil))
	if got.ClipboardSeconds != 1 || got.RevealSeconds != 300 || got.ExpiryWarnDays != 365 || got.LockMinutes != 525600 || len(warns) != 0 {
		t.Errorf("got %+v, warnings %q; want the edge values accepted", got, warns)
	}
}

func TestUnknownKeysAreIgnoredWithAWarningThatCannotInjectControlCharacters(t *testing.T) {
	got, warns := Load(write(t, `{"zzz": 1, "evil\u001b[2J": 2, "compact": true}`), env(nil))
	if !got.Compact || len(warns) != 2 {
		t.Fatalf("got %+v, warnings %q; want compact kept and two warnings", got, warns)
	}
	for _, w := range warns {
		if strings.ContainsRune(w, 0x1b) || !strings.Contains(w, "unknown key") {
			t.Errorf("warning %q: want an escaped unknown-key notice", w)
		}
	}
}

func TestUnusableFilesGiveTheDefaultsAndOneWarning(t *testing.T) {
	for name, content := range map[string]string{
		"empty":     ``,
		"not json":  `lock_minutes = 5`,
		"an array":  `[1, 2]`,
		"a string":  `"hi"`,
		"too large": `{"compact": true}` + strings.Repeat(" ", maxFileBytes),
	} {
		got, warns := Load(write(t, content), env(nil))
		if got != Defaults() || len(warns) != 1 {
			t.Errorf("%s: %+v, warnings %q; want the defaults and one warning", name, got, warns)
		}
	}

	// A directory named config.json is not a settings file.
	dir := t.TempDir()
	if err := os.Mkdir(Path(dir), 0o700); err != nil {
		t.Fatal(err)
	}
	if got, warns := Load(dir, env(nil)); got != Defaults() || len(warns) != 1 {
		t.Errorf("directory: %+v, warnings %q", got, warns)
	}
}

func TestUnreadableFileWarnsWithoutRevealingThePath(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs POSIX permissions and a non-root user")
	}
	dir := write(t, `{"compact": true}`)
	if err := os.Chmod(Path(dir), 0); err != nil {
		t.Fatal(err)
	}
	got, warns := Load(dir, env(nil))
	if got != Defaults() || len(warns) != 1 || strings.Contains(warns[0], filepath.Base(dir)) {
		t.Errorf("got %+v, warnings %q; want the defaults and a warning without the path", got, warns)
	}
}

func TestValueFormatsEveryKeyAndRejectsUnknown(t *testing.T) {
	c := Defaults()
	for _, k := range Keys() {
		if v, ok := c.Value(k); !ok || v == "" {
			t.Errorf("Value(%q) = %q, %v", k, v, ok)
		}
	}
	if v, _ := c.Value("clipboard_seconds"); v != "15" {
		t.Errorf("clipboard_seconds = %q", v)
	}
	if v, _ := c.Value("ascii"); v != "false" {
		t.Errorf("ascii = %q", v)
	}
	if _, ok := c.Value("nope"); ok {
		t.Error("unknown key reported as known")
	}
}
