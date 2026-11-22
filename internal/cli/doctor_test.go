package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RafaSG13/Pandora/internal/store"
)

// vaultFiles returns the paths of vault.json and vault.json.bak.
func vaultFiles(t *testing.T) (vault, backup string) {
	t.Helper()
	dir, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	vault = filepath.Join(dir, "pandora", "vault.json")
	return vault, vault + ".bak"
}

// damagedVault stores "old" and then "new" (so vault.json.bak holds only
// "old") and cuts vault.json in half.
func damagedVault(t *testing.T) (vault, backup string) {
	t.Helper()
	isolate(t)
	mustSave(t, "old", "c", "v-old")
	mustSave(t, "new", "c", "v-new")
	vault, backup = vaultFiles(t)
	raw, err := os.ReadFile(vault)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(vault, raw[:len(raw)/2], 0o600); err != nil {
		t.Fatal(err)
	}
	return vault, backup
}

func fileText(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestDoctorReportsHealthyVaultFiles(t *testing.T) {
	isolate(t)
	mustSave(t, "a", "c", "v")
	mustSave(t, "b", "c", "v")
	code, stdout, stderr := harness{}.run(t, "doctor")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	for _, want := range []string{"vault.json: OK (2 secrets", "vault.json.bak: OK (1 secrets", "OK: every entry has a value"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout lacks %q:\n%s", want, stdout)
		}
	}
}

func TestDoctorWithAnUnusableBackupStillPassesAndSaysSo(t *testing.T) {
	isolate(t)
	mustSave(t, "a", "c", "v")
	mustSave(t, "b", "c", "v")
	_, backup := vaultFiles(t)
	if err := os.WriteFile(backup, []byte("garbage"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ := harness{}.run(t, "doctor")
	if code != 0 || !strings.Contains(stdout, "vault.json.bak: DAMAGED") || !strings.Contains(stdout, "replaces it") {
		t.Errorf("exit %d, stdout:\n%s", code, stdout)
	}
}

func TestDoctorFindsDamagedVaultAndOnlyFixRestoresIt(t *testing.T) {
	vault, backup := damagedVault(t)
	damaged := fileText(t, vault)

	code, stdout, stderr := harness{}.run(t, "doctor")
	if code != 1 || !strings.Contains(stdout, "vault.json: DAMAGED") || !strings.Contains(stdout, "vault.json.bak: OK (1 secrets") || !strings.Contains(stderr, "--fix") {
		t.Fatalf("exit %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if fileText(t, vault) != damaged {
		t.Fatal("doctor without --fix changed vault.json")
	}

	// Declining the question changes nothing either.
	code, _, stderr = harness{tty: true, stdin: "n\n"}.run(t, "doctor", "--fix")
	if code != 1 || !strings.Contains(stderr, "cancelled") || fileText(t, vault) != damaged {
		t.Fatalf("declined: exit %d, stderr %q", code, stderr)
	}

	bak := fileText(t, backup)
	code, stdout, stderr = harness{}.run(t, "doctor", "--fix", "-y")
	if !strings.Contains(stdout, "Restored vault.json from vault.json.bak (1 secrets)") {
		t.Fatalf("exit %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if fileText(t, vault) != bak || fileText(t, backup) != bak || fileText(t, vault+".corrupt") != damaged {
		t.Error("files after the restore are not: vault = bak, bak unchanged, damaged kept as .corrupt")
	}
	// "new" was written after the backup: its value is still in the keyring
	// but nothing points to it, so the vault is healthy and lists only "old".
	if code != 0 || !strings.Contains(stdout, "Checked 1 metadata entries") {
		t.Errorf("exit %d, stdout:\n%s", code, stdout)
	}
	if got, _ := store.LoadMetadata(); len(got) != 1 || got[0].Title != "old" {
		t.Errorf("metadata after restore = %+v", got)
	}
	if code, stdout, _ := (harness{}).run(t, "get", "old"); code != 0 || stdout != "v-old\n" {
		t.Errorf("get old = %d %q", code, stdout)
	}
}

func TestDoctorFixAsksOnATerminalAndRestoresOnYes(t *testing.T) {
	vault, backup := damagedVault(t)
	code, stdout, stderr := harness{tty: true, stdin: "y\n"}.run(t, "doctor", "--fix")
	if code != 0 || !strings.Contains(stdout, "Restored") || fileText(t, vault) != fileText(t, backup) {
		t.Errorf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if !strings.Contains(stderr, "Replace vault.json with vault.json.bak (1 secrets") {
		t.Errorf("the question did not say what is replaced: %q", stderr)
	}
}

func TestDoctorFixNeverRestoresWithAWrongPassword(t *testing.T) {
	vault, backup := damagedVault(t)
	damaged, bak := fileText(t, vault), fileText(t, backup)
	env := map[string]string{masterEnv: "not the right password"}
	code, stdout, stderr := harness{env: env}.run(t, "doctor", "--fix", "-y")
	if code != 1 || !strings.Contains(stderr, "wrong master password") || strings.Contains(stdout, "Restored") {
		t.Errorf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if fileText(t, vault) != damaged || fileText(t, backup) != bak {
		t.Error("files changed after a wrong password")
	}
	if _, err := os.Stat(vault + ".corrupt"); err == nil {
		t.Error("wrote .corrupt after a wrong password")
	}
}

func TestDoctorRetriesWrongPasswordsOnATerminal(t *testing.T) {
	vault, backup := damagedVault(t)
	h := harness{tty: true, noMaster: true, hidden: []string{"wrong password one", "wrong password two", testMaster}, stdin: "y\n"}
	code, stdout, stderr := h.run(t, "doctor", "--fix")
	if code != 0 || strings.Count(stderr, "Wrong password, try again.") != 2 || fileText(t, vault) != fileText(t, backup) {
		t.Errorf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}

func TestDoctorCannotRepairWithoutAUsableBackup(t *testing.T) {
	vault, backup := damagedVault(t)
	if err := os.WriteFile(backup, []byte("also garbage"), 0o600); err != nil {
		t.Fatal(err)
	}
	damaged := fileText(t, vault)
	code, stdout, stderr := harness{}.run(t, "doctor", "--fix", "-y")
	if code != 1 || !strings.Contains(stderr, "no usable vault.json.bak") || strings.Contains(stdout, "Restored") {
		t.Errorf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if fileText(t, vault) != damaged || fileText(t, backup) != "also garbage" {
		t.Error("a file was overwritten although nothing could be restored")
	}

	if err := os.Remove(backup); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := (harness{}).run(t, "doctor", "--fix", "-y"); code != 1 || !strings.Contains(stderr, "no usable") {
		t.Errorf("no backup: exit %d, stderr %q", code, stderr)
	}
}

func TestDoctorRestoresAMissingVaultFile(t *testing.T) {
	vault, backup := damagedVault(t)
	if err := os.Remove(vault); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ := harness{}.run(t, "doctor", "--fix", "-y")
	if code != 0 || !strings.Contains(stdout, "vault.json: missing") || fileText(t, vault) != fileText(t, backup) {
		t.Errorf("exit %d, stdout:\n%s", code, stdout)
	}
}

func TestDoctorLeavesAVaultFromANewerVersionAlone(t *testing.T) {
	isolate(t)
	mustSave(t, "a", "c", "v")
	mustSave(t, "b", "c", "v")
	vault, _ := vaultFiles(t)
	newer := strings.Replace(fileText(t, vault), `"version": 2`, `"version": 3`, 1)
	if newer == fileText(t, vault) {
		t.Fatal("test could not bump the version")
	}
	if err := os.WriteFile(vault, []byte(newer), 0o600); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := harness{}.run(t, "doctor", "--fix", "-y")
	if code != 1 || !strings.Contains(stderr, "newer Pandora") || fileText(t, vault) != newer {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
}

// After a restore the backup may point at values the last change replaced.
// They must be listed, and not deleted in the same run even with -y.
func TestDoctorAfterRestoreDoesNotDeleteEntriesWithoutValue(t *testing.T) {
	isolate(t)
	mustSave(t, "doomed", "c", "first")
	// The update stores the value under a new keyring entry and removes the
	// old one, so the backup (which still has the old entry) points at nothing.
	second := "second"
	if err := store.UpdateSecret("doomed", store.SecretMeta{Title: "doomed", Category: "c", Created: "2026-01-02"}, &second); err != nil {
		t.Fatal(err)
	}
	vault, _ := vaultFiles(t)
	if err := os.WriteFile(vault, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := harness{}.run(t, "doctor", "--fix", "-y")
	if code != 1 || !strings.Contains(stdout, "Restored") || !strings.Contains(stdout, "1 entries have no value") || !strings.Contains(stderr, "review") {
		t.Fatalf("exit %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if got, _ := store.LoadMetadata(); len(got) != 1 {
		t.Fatalf("an entry was removed in the same run: %+v", got)
	}
	// A second, explicit run removes it.
	if code, stdout, _ := (harness{}).run(t, "doctor", "--fix", "-y"); code != 0 || !strings.Contains(stdout, "Removed 1") {
		t.Errorf("second run: exit %d, stdout %q", code, stdout)
	}
}

func TestOtherCommandsPointAtDoctorWhenTheVaultIsDamaged(t *testing.T) {
	damagedVault(t)
	for _, args := range [][]string{{"list"}, {"get", "old"}, {"expiring"}} {
		code, stdout, stderr := harness{}.run(t, args...)
		if code != 1 || stdout != "" || !strings.Contains(stderr, "pandora doctor") {
			t.Errorf("%v: exit %d, stdout %q, stderr %q; want exit 1 and a hint", args, code, stdout, stderr)
		}
	}
}
