package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

func readVaultBytes(t *testing.T) []byte {
	t.Helper()
	p, _ := vaultPath()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestNothingSensitiveIsStoredInPlainText(t *testing.T) {
	setup(t)
	meta := SecretMeta{Title: "Pypi Key", Category: "Cat-Sensitive", Created: "2026-01-02", Notes: "note-sensitive", Expires: "2031-05-06"}
	if err := SaveSecret(meta, "value-sensitive"); err != nil {
		t.Fatal(err)
	}
	mustSave(t, "second", "Other", "v2") // creates vault.json.bak from the first write

	p, _ := vaultPath()
	bak, err := os.ReadFile(p + ".bak")
	if err != nil {
		t.Fatal(err)
	}
	secrets := []string{"Pypi Key", "Cat-Sensitive", "note-sensitive", "2031-05-06", "value-sensitive"}
	for name, data := range map[string][]byte{"vault.json": readVaultBytes(t), "vault.json.bak": bak} {
		for _, s := range secrets {
			if bytes.Contains(data, []byte(s)) {
				t.Errorf("%s contains %q", name, s)
			}
		}
	}

	if _, err := keyring.Get(ServiceName, "Pypi Key"); !errors.Is(err, keyring.ErrNotFound) {
		t.Errorf("keyring still has an account named after the title: err = %v", err)
	}
	id := idOf(t, "Pypi Key")
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(id) {
		t.Errorf("account %q is not a 32-hex random ID", id)
	}
	stored, err := keyring.Get(ServiceName, id)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stored, "value-sensitive") || !strings.HasPrefix(stored, "pv2:") {
		t.Errorf("keyring value is not encrypted: %q", stored)
	}
}

func TestWrongPasswordLeavesVaultUntouched(t *testing.T) {
	setup(t)
	mustSave(t, "a", "X", "one")
	before := readVaultBytes(t)
	Lock()

	if err := Unlock("not the password"); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("err = %v, want ErrWrongPassword", err)
	}
	if IsUnlocked() {
		t.Error("vault unlocked by a wrong password")
	}
	if !bytes.Equal(before, readVaultBytes(t)) {
		t.Error("vault changed after a failed unlock")
	}
	if err := Unlock(testPassword); err != nil {
		t.Fatal(err)
	}
	if v, _ := GetSecretValue("a"); v != "one" {
		t.Errorf("value = %q after unlock", v)
	}
}

func TestLockedStoreRefusesEverything(t *testing.T) {
	setup(t)
	mustSave(t, "a", "X", "one")
	Lock()

	nv := "x"
	calls := map[string]error{
		"LoadMetadata":   func() error { _, err := LoadMetadata(); return err }(),
		"SaveSecret":     SaveSecret(SecretMeta{Title: "b"}, "v"),
		"GetSecretValue": func() error { _, err := GetSecretValue("a"); return err }(),
		"UpdateSecret":   UpdateSecret("a", SecretMeta{Title: "a"}, &nv),
		"DeleteSecret":   DeleteSecret("a"),
		"MissingValues":  func() error { _, err := MissingValues(); return err }(),
		"RestoreSecrets": RestoreSecrets([]Restore{{Meta: SecretMeta{Title: "c"}}}),
	}
	for name, err := range calls {
		if !errors.Is(err, ErrLocked) {
			t.Errorf("%s while locked: err = %v, want ErrLocked", name, err)
		}
	}
}

func TestChangePasswordKeepsValuesAndRejectsOldPassword(t *testing.T) {
	setup(t)
	mustSave(t, "a", "X", "one")
	mustSave(t, "b", "X", "two")

	if err := ChangePassword("wrong wrong wrong", "another long password"); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("wrong old password: err = %v", err)
	}
	if err := ChangePassword(testPassword, "short"); !errors.Is(err, ErrPasswordTooShort) {
		t.Fatalf("short new password: err = %v", err)
	}
	if err := ChangePassword(testPassword, "another long password"); err != nil {
		t.Fatal(err)
	}

	Lock()
	if err := Unlock(testPassword); !errors.Is(err, ErrWrongPassword) {
		t.Errorf("old password still works: err = %v", err)
	}
	if err := Unlock("another long password"); err != nil {
		t.Fatal(err)
	}
	for title, want := range map[string]string{"a": "one", "b": "two"} {
		if v, err := GetSecretValue(title); err != nil || v != want {
			t.Errorf("%s = %q, %v; want %q", title, v, err, want)
		}
	}
	p, _ := vaultPath()
	bak, _ := os.ReadFile(p + ".bak")
	if !bytes.Equal(bak, readVaultBytes(t)) {
		t.Error("backup still holds the old key wrapping after a password change")
	}
}

func TestSwappedKeyringCiphertextsAreRejected(t *testing.T) {
	setup(t)
	mustSave(t, "a", "X", "one")
	mustSave(t, "b", "X", "two")
	ida, idb := idOf(t, "a"), idOf(t, "b")
	sa, _ := keyring.Get(ServiceName, ida)
	sb, _ := keyring.Get(ServiceName, idb)
	if err := keyring.Set(ServiceName, ida, sb); err != nil {
		t.Fatal(err)
	}
	if err := keyring.Set(ServiceName, idb, sa); err != nil {
		t.Fatal(err)
	}

	for _, title := range []string{"a", "b"} {
		if v, err := GetSecretValue(title); err == nil {
			t.Errorf("GetSecretValue(%q) = %q after swap, want error", title, v)
		}
	}
}

func TestPlainKeyringValueIsRejected(t *testing.T) {
	setup(t)
	mustSave(t, "a", "X", "one")
	if err := keyring.Set(ServiceName, idOf(t, "a"), "plain"); err != nil {
		t.Fatal(err)
	}
	if _, err := GetSecretValue("a"); err == nil || !strings.Contains(err.Error(), "corrupt") {
		t.Errorf("err = %v, want corrupt-value error", err)
	}
}

func TestTamperedVaultDataIsRejectedAndNotOverwritten(t *testing.T) {
	setup(t)
	mustSave(t, "a", "X", "one")

	var vf vaultFile
	if err := json.Unmarshal(readVaultBytes(t), &vf); err != nil {
		t.Fatal(err)
	}
	vf.Data[len(vf.Data)/2] ^= 0xff
	tampered, _ := json.Marshal(vf)
	p, _ := vaultPath()
	if err := os.WriteFile(p, tampered, 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadMetadata(); err == nil {
		t.Fatal("LoadMetadata accepted tampered data")
	}
	if err := SaveSecret(SecretMeta{Title: "b"}, "v"); err == nil {
		t.Fatal("SaveSecret succeeded on tampered vault")
	}
	if got := readVaultBytes(t); !bytes.Equal(got, tampered) {
		t.Error("tampered vault was overwritten")
	}
}

func TestHostileKDFParametersAreRejected(t *testing.T) {
	setup(t)
	Lock()
	for name, mutate := range map[string]func(*vaultFile){
		"memory":  func(v *vaultFile) { v.KDF.MemoryKiB = 4194304 },
		"time":    func(v *vaultFile) { v.KDF.Time = 1000 },
		"threads": func(v *vaultFile) { v.KDF.Threads = 0 },
		"alg":     func(v *vaultFile) { v.KDF.Alg = "scrypt" },
		"version": func(v *vaultFile) { v.Version = 1 },
	} {
		var vf vaultFile
		if err := json.Unmarshal(readVaultBytes(t), &vf); err != nil {
			t.Fatal(err)
		}
		mutate(&vf)
		out, _ := json.Marshal(vf)
		p, _ := vaultPath()
		if err := os.WriteFile(p, out, 0600); err != nil {
			t.Fatal(err)
		}
		err := Unlock(testPassword)
		if err == nil || errors.Is(err, ErrWrongPassword) {
			t.Errorf("%s: err = %v, want a parameter error without running the KDF", name, err)
		}
		if IsUnlocked() {
			t.Errorf("%s: unlocked with hostile parameters", name)
		}
	}
}

func TestControlCharactersAreRejected(t *testing.T) {
	setup(t)
	if err := ValidateText("a\x1b[2Jb"); !errors.Is(err, ErrInvalidText) {
		t.Errorf("ValidateText(ESC) = %v", err)
	}
	if err := ValidateText("naïve 日本 ✓"); err != nil {
		t.Errorf("ValidateText(printable unicode) = %v", err)
	}
	for _, m := range []SecretMeta{
		{Title: "a\x1b[2J"}, {Title: "ok", Category: "x\ny"}, {Title: "ok", Notes: "tab\there"},
	} {
		if err := SaveSecret(m, "v"); !errors.Is(err, ErrInvalidText) {
			t.Errorf("SaveSecret(%+v) = %v, want ErrInvalidText", m, err)
		}
	}
	mustSave(t, "fine", "X", "v")
	if err := UpdateSecret("fine", SecretMeta{Title: "fine", Notes: "bad\x07"}, nil); !errors.Is(err, ErrInvalidText) {
		t.Errorf("UpdateSecret = %v, want ErrInvalidText", err)
	}
	if err := RestoreSecrets([]Restore{{Meta: SecretMeta{Title: "r\x1b"}}}); !errors.Is(err, ErrInvalidText) {
		t.Errorf("RestoreSecrets = %v, want ErrInvalidText", err)
	}
	if got := titles(t); len(got) != 1 {
		t.Errorf("titles = %v, want only [fine]", got)
	}
}

func TestCreateRequiresLongPasswordAndNoExistingVault(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir+"/.config")
	t.Setenv("AppData", dir)
	FastKDFForTests()
	t.Cleanup(Lock)
	Lock()

	if err := Create("too short"); !errors.Is(err, ErrPasswordTooShort) {
		t.Fatalf("9 chars: err = %v", err)
	}
	if Exists() {
		t.Fatal("vault written for a rejected password")
	}
	if err := Create("日本語日本語日本語日"); err != nil { // 10 runes, more than 10 bytes
		t.Fatalf("10 runes: %v", err)
	}
	if err := Create(testPassword); !errors.Is(err, ErrVaultExists) {
		t.Errorf("second Create: err = %v, want ErrVaultExists", err)
	}
}
