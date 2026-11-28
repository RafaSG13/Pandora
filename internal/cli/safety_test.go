package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetNeutralisesEscapeSequencesOnlyOnATerminal(t *testing.T) {
	isolate(t)
	mustSave(t, "k", "", "a\x1b]0;PWNED\x07b")

	code, out, stderr := harness{outTTY: true}.run(t, "get", "k")
	if code != 0 || out != "a^[]0;PWNED^Gb\n" {
		t.Errorf("terminal: exit %d, output %q, stderr %q", code, out, stderr)
	}
	code, out, stderr = harness{outTTY: false}.run(t, "get", "k")
	if code != 0 || out != "a\x1b]0;PWNED\x07b\n" {
		t.Errorf("pipe: exit %d, output %q, stderr %q; want the raw bytes", code, out, stderr)
	}
}

func TestEnvOutputIsSafeToSourceInSh(t *testing.T) {
	skipWithoutSh(t)
	isolate(t)
	value := "`touch pwned`$(touch pwned2)"
	mustSave(t, "k", "", value)
	code, out, stderr := harness{}.run(t, "env", "-s", "k=K")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "f"), []byte(out), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", "-c", `set -a; . ./f; printf %s "$K"`)
	cmd.Dir = dir
	got, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != value {
		t.Errorf("sourced value = %q, want %q (file %q)", got, value, out)
	}
	for _, f := range []string{"pwned", "pwned2"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
			t.Errorf("sourcing the output ran a command: %s was created", f)
		}
	}
	batch, err := parseDotenv([]byte(out), importOptions{})
	if err != nil || len(batch.secrets) != 1 || *batch.secrets[0].Value != value {
		t.Errorf("parseDotenv round trip failed: %v", err)
	}
}

func TestSecretEnvFilterFoldsCaseOnlyWhereAsked(t *testing.T) {
	if !isSecretEnv("pandora_master_password=x", true) {
		t.Error("a differently cased password variable must be dropped on Windows")
	}
	if isSecretEnv("pandora_master_password=x", false) {
		t.Error("names are case-sensitive off Windows")
	}
	if !isSecretEnv(masterEnv+"=x", false) || !isSecretEnv(passphraseEnv+"=", false) {
		t.Error("exact names must always be dropped")
	}
	if isSecretEnv("PANDORA_MASTER_PASSWORD_FILE=x", true) || isSecretEnv("PATH=/bin", true) {
		t.Error("unrelated variables must stay")
	}
}

func TestTakeSecretEnvRemovesThePasswordsFromTheProcess(t *testing.T) {
	t.Setenv(masterEnv, "x")
	os.Unsetenv(passphraseEnv)
	got := takeSecretEnv()
	if len(got) != 1 || got[masterEnv] != "x" {
		t.Errorf("takeSecretEnv = %v, want only the master password", got)
	}
	if _, ok := os.LookupEnv(masterEnv); ok {
		t.Error("the password is still in the process environment")
	}
	if !strings.Contains(strings.Join(os.Environ(), "\n"), "PATH=") {
		t.Error("other variables must be untouched")
	}
}
