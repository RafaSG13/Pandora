package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RafaSG13/Pandora/internal/config"
	"github.com/RafaSG13/Pandora/internal/store"
)

func writeConfigFile(t *testing.T, content string) {
	t.Helper()
	dir, err := store.ConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, config.FileName), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestConfigPathPrintsTheFileInsideTheConfigDir(t *testing.T) {
	isolate(t)
	dir, _ := store.ConfigDir()
	code, stdout, stderr := harness{noMaster: true}.run(t, "config", "path")
	if code != 0 || strings.TrimSpace(stdout) != filepath.Join(dir, "config.json") {
		t.Errorf("config path = %d %q (%s), want %q", code, stdout, stderr, filepath.Join(dir, "config.json"))
	}
}

func TestConfigGetShowsEffectiveSettingsWithoutUnlocking(t *testing.T) {
	isolate(t)
	writeConfigFile(t, `{"clipboard_seconds": 30, "sort": "name"}`)
	h := harness{noMaster: true, env: map[string]string{"PANDORA_LOCK_MINUTES": "0"}}

	code, stdout, stderr := h.run(t, "config", "get")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	for _, want := range []string{"lock_minutes = 0\n", "clipboard_seconds = 30\n", "sort = name\n", "reveal_seconds = 10\n", "compact = false\n"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("config get lacks %q in:\n%s", want, stdout)
		}
	}

	if code, stdout, _ := h.run(t, "config", "get", "clipboard_seconds"); code != 0 || stdout != "30\n" {
		t.Errorf("config get clipboard_seconds = %d %q, want 30", code, stdout)
	}
}

func TestConfigGetWarnsAboutProblemsButSucceeds(t *testing.T) {
	isolate(t)
	writeConfigFile(t, `{"reveal_seconds": 0, "nonsense": true}`)
	code, stdout, stderr := harness{noMaster: true}.run(t, "config", "get", "reveal_seconds")
	if code != 0 || stdout != "10\n" {
		t.Errorf("config get = %d %q, want the default 10 and exit 0", code, stdout)
	}
	if strings.Count(stderr, "warning:") != 2 || !strings.Contains(stderr, "reveal_seconds") || !strings.Contains(stderr, "nonsense") {
		t.Errorf("stderr %q, want one warning per problem", stderr)
	}
}

func TestConfigUsageErrorsExitTwo(t *testing.T) {
	isolate(t)
	for _, args := range [][]string{
		{"config"},
		{"config", "set", "x"},
		{"config", "path", "extra"},
		{"config", "get", "nope"},
		{"config", "get", "a", "b"},
	} {
		if code, _, stderr := (harness{noMaster: true}).run(t, args...); code != 2 || stderr == "" {
			t.Errorf("%v: exit %d, stderr %q; want a usage error (2)", args, code, stderr)
		}
	}
}
