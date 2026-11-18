package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RafaSG13/Pandora/internal/config"
)

// writeConfig writes config.json into the config dir isolate set up.
func writeConfig(t *testing.T, content string) {
	t.Helper()
	path, err := config.Path()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestConfigPathWorksWithoutAFile(t *testing.T) {
	isolate(t)
	code, stdout, stderr := harness{}.run(t, "config", "path")
	want := filepath.Join("pandora", "config.json")
	if code != 0 || stderr != "" || !strings.HasSuffix(strings.TrimSpace(stdout), want) {
		t.Fatalf("exit %d, stdout %q, stderr %q; want a path ending in %s", code, stdout, stderr, want)
	}
	if _, err := os.Stat(strings.TrimSpace(stdout)); err == nil {
		t.Error("config path created the file")
	}
}

func TestConfigGetListsEveryKeyInOrder(t *testing.T) {
	isolate(t)
	writeConfig(t, `{"gen_length":40}`)
	code, stdout, stderr := harness{noMaster: true}.run(t, "config", "get")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	if len(lines) != len(config.Keys) {
		t.Fatalf("%d lines, want %d:\n%s", len(lines), len(config.Keys), stdout)
	}
	for i, k := range config.Keys {
		if !strings.HasPrefix(lines[i], k+" = ") {
			t.Errorf("line %d = %q, want key %s", i, lines[i], k)
		}
	}
	if !strings.Contains(stdout, "gen_length = 40\n") || !strings.Contains(stdout, "lock_minutes = 5\n") || !strings.Contains(stdout, "ascii = auto\n") {
		t.Errorf("effective values wrong:\n%s", stdout)
	}
}

func TestConfigGetOneKeyShowsEffectiveValue(t *testing.T) {
	isolate(t)
	writeConfig(t, `{"gen_length":40,"lock_minutes":9}`)
	if code, stdout, _ := (harness{}).run(t, "config", "get", "gen_length"); code != 0 || stdout != "40\n" {
		t.Errorf("gen_length: exit %d, stdout %q", code, stdout)
	}
	h := harness{env: map[string]string{"PANDORA_LOCK_MINUTES": "2"}}
	if code, stdout, _ := h.run(t, "config", "get", "lock_minutes"); code != 0 || stdout != "2\n" {
		t.Errorf("env override: exit %d, stdout %q, want 2", code, stdout)
	}
}

func TestConfigUsageErrors(t *testing.T) {
	isolate(t)
	for _, args := range [][]string{
		{"config"},
		{"config", "get", "nope"},
		{"config", "set", "gen_length"},
		{"config", "path", "extra"},
		{"config", "get", "a", "b"},
	} {
		code, stdout, stderr := harness{}.run(t, args...)
		if code != 2 || stdout != "" || stderr == "" {
			t.Errorf("%v: exit %d, stdout %q, stderr %q; want exit 2 with a message", args, code, stdout, stderr)
		}
	}
	if _, _, stderr := (harness{}).run(t, "config", "get", "nope"); !strings.Contains(stderr, "gen_length") {
		t.Errorf("unknown key message does not list the valid keys: %q", stderr)
	}
}

func TestGenUsesConfigDefaultsAndFlagsStillWin(t *testing.T) {
	isolate(t)
	writeConfig(t, `{"gen_length":40,"gen_charset":"hex"}`)

	_, stdout, stderr := harness{}.run(t, "gen")
	line := strings.TrimSpace(stdout)
	if stderr != "" || len(line) != 40 || strings.Trim(line, "0123456789abcdef") != "" {
		t.Errorf("gen = %q (stderr %q), want 40 hex characters", line, stderr)
	}
	_, stdout, _ = harness{}.run(t, "gen", "--length", "16")
	if got := strings.TrimSpace(stdout); len(got) != 16 || strings.Trim(got, "0123456789abcdef") != "" {
		t.Errorf("gen --length 16 = %q, want 16 hex characters", got)
	}
	_, stdout, _ = harness{}.run(t, "gen", "--charset", "symbols", "--length", "128")
	if got := strings.TrimSpace(stdout); strings.Trim(got, "0123456789abcdef") == "" {
		t.Errorf("--charset symbols did not override the configured hex: %q", got)
	}
}

func TestInvalidConfigWarnsOnStderrAndNeverFailsTheCommand(t *testing.T) {
	isolate(t)
	writeConfig(t, `{"gen_length":3,"bogus":true}`)
	code, stdout, stderr := harness{}.run(t, "gen")
	if code != 0 || len(strings.TrimSpace(stdout)) != config.DefaultGenLength {
		t.Errorf("exit %d, stdout %q; want the default length", code, stdout)
	}
	if !strings.Contains(stderr, "pandora: warning: config.json: gen_length") || !strings.Contains(stderr, `unknown key "bogus"`) {
		t.Errorf("stderr = %q", stderr)
	}
	if strings.Contains(stdout, "warning") {
		t.Error("warning leaked to stdout")
	}

	writeConfig(t, "{broken")
	if code, _, stderr := (harness{}).run(t, "config", "get", "gen_length"); code != 0 || !strings.Contains(stderr, "not a valid JSON object") {
		t.Errorf("malformed file: exit %d, stderr %q", code, stderr)
	}
}
