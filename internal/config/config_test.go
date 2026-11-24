package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

// isolate points the user config directory at a temp dir.
func isolate(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	t.Setenv("AppData", dir)
}

func writeFile(t *testing.T, content string) {
	t.Helper()
	p, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func env(kv map[string]string) func(string) string {
	return func(k string) string { return kv[k] }
}

func TestMissingFileGivesDefaultsWithoutWarnings(t *testing.T) {
	isolate(t)
	s, warns := Load(env(nil))
	if len(warns) != 0 {
		t.Fatalf("warnings = %v", warns)
	}
	want := defaults()
	if !reflect.DeepEqual(s, want) {
		t.Fatalf("settings = %+v, want %+v", s, want)
	}
	if s.ASCII != nil {
		t.Error("ascii must stay unset so the locale decides")
	}
}

func TestPathEndsInPandoraConfigJSON(t *testing.T) {
	isolate(t)
	p, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(p) != "config.json" || filepath.Base(filepath.Dir(p)) != "pandora" {
		t.Errorf("Path() = %q", p)
	}
	if _, err := os.Stat(filepath.Dir(p)); err == nil {
		t.Error("Path must not create the directory")
	}
}

func TestPrecedenceEnvOverFileOverDefault(t *testing.T) {
	cases := []struct {
		name     string
		file     string
		env      map[string]string
		lock     time.Duration
		ascii    string
		wantWarn int
	}{
		{"defaults", "", nil, 5 * time.Minute, "auto", 0},
		{"file", `{"lock_minutes":9,"ascii":true}`, nil, 9 * time.Minute, "true", 0},
		{"env beats file", `{"lock_minutes":9,"ascii":false}`, map[string]string{"PANDORA_LOCK_MINUTES": "2", "PANDORA_ASCII": "1"}, 2 * time.Minute, "true", 0},
		{"env zero disables", `{"lock_minutes":9}`, map[string]string{"PANDORA_LOCK_MINUTES": "0"}, 0, "auto", 0},
		{"invalid env falls to file", `{"lock_minutes":9}`, map[string]string{"PANDORA_LOCK_MINUTES": "abc"}, 9 * time.Minute, "auto", 0},
		{"negative env falls to file", `{"lock_minutes":9}`, map[string]string{"PANDORA_LOCK_MINUTES": "-3"}, 9 * time.Minute, "auto", 0},
		{"huge env falls to file", `{"lock_minutes":9}`, map[string]string{"PANDORA_LOCK_MINUTES": "10081"}, 9 * time.Minute, "auto", 0},
		{"empty env falls to default", "", map[string]string{"PANDORA_LOCK_MINUTES": " "}, 5 * time.Minute, "auto", 0},
		{"ascii env other than 1 ignored", `{"ascii":false}`, map[string]string{"PANDORA_ASCII": "0"}, 5 * time.Minute, "false", 0},
		{"ascii env 0 without file", "", map[string]string{"PANDORA_ASCII": "0"}, 5 * time.Minute, "auto", 0},
		{"env wins over invalid file value", `{"lock_minutes":"x"}`, map[string]string{"PANDORA_LOCK_MINUTES": "2"}, 2 * time.Minute, "auto", 1},
		{"env padded with spaces", "", map[string]string{"PANDORA_LOCK_MINUTES": " 10 "}, 10 * time.Minute, "auto", 0},
	}
	for _, c := range cases {
		isolate(t)
		if c.file != "" {
			writeFile(t, c.file)
		}
		s, warns := Load(env(c.env))
		got, _ := s.Get("ascii")
		if s.LockAfter != c.lock || got != c.ascii || len(warns) != c.wantWarn {
			t.Errorf("%s: lock=%v ascii=%s warns=%v, want lock=%v ascii=%s warns=%d", c.name, s.LockAfter, got, warns, c.lock, c.ascii, c.wantWarn)
		}
	}
}

func TestEachKeyAcceptsBoundsAndRejectsOutside(t *testing.T) {
	cases := []struct {
		key         string
		lo, hi, def int
	}{
		{"lock_minutes", 0, 10080, 5},
		{"clipboard_seconds", 1, 3600, 15},
		{"reveal_seconds", 1, 3600, 10},
		{"expiry_warn_days", 0, 3650, 14},
		{"gen_length", 8, 128, 24},
		{"history_depth", 0, 20, 3},
	}
	load := func(t *testing.T, key string, v int) (string, []string) {
		t.Helper()
		isolate(t)
		writeFile(t, fmt.Sprintf(`{%q:%d}`, key, v))
		s, warns := Load(env(nil))
		got, _ := s.Get(key)
		return got, warns
	}
	for _, c := range cases {
		for _, v := range []int{c.lo, c.hi} {
			if got, warns := load(t, c.key, v); got != strconv.Itoa(v) || len(warns) != 0 {
				t.Errorf("%s=%d: got %s, warns %v", c.key, v, got, warns)
			}
		}
		for _, v := range []int{c.lo - 1, c.hi + 1} {
			got, warns := load(t, c.key, v)
			if got != strconv.Itoa(c.def) || len(warns) != 1 || !strings.HasPrefix(warns[0], "config.json: "+c.key+": must be between") {
				t.Errorf("%s=%d: got %s, warns %v; want default %d and one range warning", c.key, v, got, warns, c.def)
			}
		}
	}
}

func TestOutOfRangeWarningText(t *testing.T) {
	isolate(t)
	writeFile(t, `{"gen_length":3}`)
	s, warns := Load(env(nil))
	if s.GenLength != DefaultGenLength {
		t.Errorf("gen_length = %d, want default", s.GenLength)
	}
	if want := []string{"config.json: gen_length: must be between 8 and 128"}; !reflect.DeepEqual(warns, want) {
		t.Errorf("warnings = %q, want %q", warns, want)
	}
}

func TestStringAndBoolKeys(t *testing.T) {
	isolate(t)
	writeFile(t, `{"gen_charset":"hex","sort":"category","compact":true,"ascii":false}`)
	s, warns := Load(env(nil))
	if len(warns) != 0 {
		t.Fatalf("warnings = %v", warns)
	}
	if s.GenCharset != "hex" || s.Sort != "category" || !s.Compact || s.ASCII == nil || *s.ASCII {
		t.Errorf("settings = %+v", s)
	}

	for _, bad := range []string{`{"gen_charset":"emoji"}`, `{"sort":"size"}`, `{"gen_charset":1}`, `{"sort":true}`} {
		isolate(t)
		writeFile(t, bad)
		s, warns := Load(env(nil))
		if len(warns) != 1 || s.GenCharset != DefaultGenCharset || s.Sort != DefaultSort {
			t.Errorf("%s: settings %+v, warnings %v", bad, s, warns)
		}
	}
}

func TestWrongTypesWarnAndKeepDefaults(t *testing.T) {
	for _, in := range []string{
		`{"lock_minutes":"x"}`, `{"lock_minutes":1.5}`, `{"lock_minutes":null}`, `{"lock_minutes":true}`,
		`{"ascii":"yes"}`, `{"ascii":1}`, `{"compact":"true"}`, `{"gen_length":"24"}`,
	} {
		isolate(t)
		writeFile(t, in)
		s, warns := Load(env(nil))
		if len(warns) != 1 || !reflect.DeepEqual(s, defaults()) {
			t.Errorf("%s: settings %+v, warnings %v", in, s, warns)
		}
	}
}

func TestUnknownKeyWarnsAndIsIgnored(t *testing.T) {
	isolate(t)
	writeFile(t, `{"foo":1,"reveal_seconds":30}`)
	s, warns := Load(env(nil))
	if want := []string{`config.json: unknown key "foo" ignored`}; !reflect.DeepEqual(warns, want) {
		t.Errorf("warnings = %q, want %q", warns, want)
	}
	if s.RevealTTL != 30*time.Second {
		t.Errorf("a valid sibling key was dropped: %+v", s)
	}
}

func TestMalformedFileGivesOneWarningAndStillAppliesEnv(t *testing.T) {
	for _, in := range []string{"{not json", "", "[]", "null", `"text"`, "42"} {
		isolate(t)
		writeFile(t, in)
		s, warns := Load(env(map[string]string{"PANDORA_LOCK_MINUTES": "7"}))
		if want := []string{"config.json: not a valid JSON object"}; !reflect.DeepEqual(warns, want) {
			t.Errorf("%q: warnings = %q", in, warns)
		}
		if s.LockAfter != 7*time.Minute || s.GenLength != DefaultGenLength {
			t.Errorf("%q: settings = %+v", in, s)
		}
	}
}

func TestOversizeFileIsIgnored(t *testing.T) {
	isolate(t)
	writeFile(t, `{"gen_length":40,"x":"`+strings.Repeat("a", 70<<10)+`"}`)
	s, warns := Load(env(nil))
	if want := []string{"config.json: file larger than 64 KB, ignored"}; !reflect.DeepEqual(warns, want) {
		t.Errorf("warnings = %q", warns)
	}
	if s.GenLength != DefaultGenLength {
		t.Error("oversize file was applied")
	}
}

func TestUnreadableFileWarns(t *testing.T) {
	isolate(t)
	p, _ := Path()
	if err := os.MkdirAll(p, 0o700); err != nil { // a directory where the file should be
		t.Fatal(err)
	}
	s, warns := Load(env(nil))
	if len(warns) != 1 || !strings.HasPrefix(warns[0], "config.json: ") || !reflect.DeepEqual(s, defaults()) {
		t.Errorf("settings %+v, warnings %q", s, warns)
	}
}

func TestWarningsAreSortedByKey(t *testing.T) {
	isolate(t)
	writeFile(t, `{"zzz":1,"gen_length":3,"aaa":2,"lock_minutes":"x"}`)
	for range 20 {
		_, warns := Load(env(nil))
		want := []string{
			`config.json: unknown key "aaa" ignored`,
			`config.json: gen_length: must be between 8 and 128`,
			`config.json: lock_minutes: must be an integer`,
			`config.json: unknown key "zzz" ignored`,
		}
		if !reflect.DeepEqual(warns, want) {
			t.Fatalf("warnings = %q, want %q", warns, want)
		}
	}
}

func TestGetCoversEveryKeyAndRejectsUnknown(t *testing.T) {
	s := defaults()
	for _, k := range Keys {
		if _, ok := s.Get(k); !ok {
			t.Errorf("Get(%q) not found", k)
		}
	}
	if _, ok := s.Get("nope"); ok {
		t.Error("Get accepted an unknown key")
	}
	if v, _ := s.Get("lock_minutes"); v != "5" {
		t.Errorf("lock_minutes = %q", v)
	}
}

func TestKeysOverrideOnlyWhatTheFileNames(t *testing.T) {
	isolate(t)
	writeFile(t, `{"keys":{"copy":["y"]}}`)
	s, warns := Load(env(nil))
	if len(warns) != 0 {
		t.Fatalf("warnings = %q", warns)
	}
	want := DefaultKeyBindings()
	want["copy"] = []string{"y"}
	if !reflect.DeepEqual(s.KeyBindings, want) {
		t.Errorf("bindings = %v, want %v", s.KeyBindings, want)
	}
	if got, _ := s.Get("keys"); !strings.HasPrefix(got, "copy=y copy_username=u reveal=v ") {
		t.Errorf("Get(keys) = %q", got)
	}
}

func TestKeysAreRejectedAsAWhole(t *testing.T) {
	for name, body := range map[string]string{
		"unknown action":     `{"keys":{"zap":["z"]}}`,
		"list-nav key":       `{"keys":{"reveal":["j"]}}`,
		"digit":              `{"keys":{"reveal":["1"]}}`,
		"collides":           `{"keys":{"reveal":["n"]}}`,
		"empty list":         `{"keys":{"copy":[]}}`,
		"not a single key":   `{"keys":{"copy":["esc"]}}`,
		"ctrl+c is reserved": `{"keys":{"copy":["ctrl+c"]}}`,
		"valid next to bad":  `{"keys":{"copy":["y"],"reveal":["/"]}}`,
		"wrong type":         `{"keys":["y"]}`,
	} {
		t.Run(name, func(t *testing.T) {
			isolate(t)
			writeFile(t, body)
			s, warns := Load(env(nil))
			if len(warns) != 1 || !strings.Contains(warns[0], "keys: ") {
				t.Fatalf("warnings = %q, want one for keys", warns)
			}
			if !reflect.DeepEqual(s.KeyBindings, DefaultKeyBindings()) {
				t.Errorf("bindings changed despite the error: %v", s.KeyBindings)
			}
		})
	}
}

func TestKeyCollisionIsJudgedAfterMerging(t *testing.T) {
	isolate(t)
	writeFile(t, `{"keys":{"reveal":["n"],"new":["m"]}}`)
	s, warns := Load(env(nil))
	if len(warns) != 0 {
		t.Fatalf("warnings = %q", warns)
	}
	if s.KeyBindings["reveal"][0] != "n" || s.KeyBindings["new"][0] != "m" {
		t.Errorf("bindings = %v", s.KeyBindings)
	}
}

func TestBackgroundAcceptsAutoLightDarkOnly(t *testing.T) {
	isolate(t)
	writeFile(t, `{"background":"light"}`)
	if s, warns := Load(env(nil)); len(warns) != 0 || s.Background != "light" {
		t.Errorf("background = %q, warnings %q", s.Background, warns)
	}
	writeFile(t, `{"background":"blue"}`)
	s, warns := Load(env(nil))
	if len(warns) != 1 || !strings.Contains(warns[0], "background: must be one of auto, light, dark") || s.Background != DefaultBackground {
		t.Errorf("background = %q, warnings %q", s.Background, warns)
	}
}
