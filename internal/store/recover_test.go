package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
)

// twoGenerations stores "old" and then "new", so vault.json holds both and
// vault.json.bak holds only "old". It returns the two file paths.
func twoGenerations(t *testing.T) (vault, backup string) {
	t.Helper()
	setup(t)
	mustSave(t, "old", "X", "v-old")
	mustSave(t, "new", "X", "v-new")
	p, err := vaultPath()
	if err != nil {
		t.Fatal(err)
	}
	return p, p + ".bak"
}

func readBytes(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func writeBytes(t *testing.T, path string, b []byte) {
	t.Helper()
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestInspectVaultClassifiesEachFile(t *testing.T) {
	vault, backup := twoGenerations(t)

	rep, err := InspectVault(testPassword)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Vault.Health != FileOK || rep.Vault.Entries != 2 || rep.Backup.Health != FileOK || rep.Backup.Entries != 1 || rep.Vault.Modified.IsZero() {
		t.Errorf("healthy pair: %+v", rep)
	}

	// A vault whose data was changed but whose key still unwraps.
	good := readBytes(t, vault)
	var vf vaultFile
	if err := json.Unmarshal(good, &vf); err != nil {
		t.Fatal(err)
	}
	vf.Data[len(vf.Data)-1] ^= 0xff
	tampered, _ := json.Marshal(vf)

	vf.Data[len(vf.Data)-1] ^= 0xff
	vf.KDF.MemoryKiB = 1 << 30
	hugeKDF, _ := json.Marshal(vf)

	vf.KDF.MemoryKiB = defaultKDF.MemoryKiB
	vf.Version = vaultVersion + 1
	future, _ := json.Marshal(vf)

	vf.Version = 0
	versionZero, _ := json.Marshal(vf)

	tests := []struct {
		name string
		raw  []byte // nil removes the file
		want FileHealth
	}{
		{"missing", nil, FileMissing},
		{"truncated", good[:len(good)/2], FileCorrupt},
		{"not json", []byte("garbage"), FileCorrupt},
		{"empty file", []byte{}, FileCorrupt},
		{"empty object (version 0)", []byte("{}"), FileCorrupt},
		{"version zero", versionZero, FileCorrupt},
		{"changed data", tampered, FileCorrupt},
		{"out-of-range KDF", hugeKDF, FileCorrupt},
		{"newer version", future, FileNewer},
	}
	for _, tc := range tests {
		if tc.raw == nil {
			os.Remove(vault)
		} else {
			writeBytes(t, vault, tc.raw)
		}
		rep, err := InspectVault(testPassword)
		if err != nil || rep.Vault.Health != tc.want {
			t.Errorf("%s: health %v (err %v), want %v", tc.name, rep.Vault.Health, err, tc.want)
		}
		if tc.want != FileOK && rep.Vault.Err == nil {
			t.Errorf("%s: no reason reported", tc.name)
		}
		if rep.Backup.Health != FileOK {
			t.Errorf("%s: backup health %v, want OK", tc.name, rep.Backup.Health)
		}
	}

	writeBytes(t, vault, good)
	rep, err = InspectVault("another long password")
	if err != nil || rep.Vault.Health != FileWrongPassword || rep.Backup.Health != FileWrongPassword {
		t.Errorf("wrong password: %+v (%v)", rep, err)
	}
	os.Remove(backup)
	if rep, _ := InspectVault(testPassword); rep.Backup.Health != FileMissing {
		t.Errorf("no backup: %+v", rep.Backup)
	}
}

func TestRestoreVaultBackupBringsBackTheBackupState(t *testing.T) {
	vault, backup := twoGenerations(t)
	bakBefore := readBytes(t, backup)
	damaged := []byte(`{"version":2,"kdf":`) // a cut-off write
	writeBytes(t, vault, damaged)

	if err := RestoreVaultBackup(testPassword); err != nil {
		t.Fatal(err)
	}
	if got := readBytes(t, vault+".corrupt"); !bytes.Equal(got, damaged) {
		t.Errorf("damaged file was not kept: %q", got)
	}
	if !bytes.Equal(readBytes(t, backup), bakBefore) {
		t.Error("the backup changed")
	}
	if !bytes.Equal(readBytes(t, vault), bakBefore) {
		t.Error("vault.json is not the backup")
	}

	Lock()
	if err := Unlock(testPassword); err != nil {
		t.Fatal(err)
	}
	if got := titles(t); !reflect.DeepEqual(got, []string{"old"}) {
		t.Errorf("titles after restore = %v, want only the backup's [old]", got)
	}
	if v, err := GetSecretValue("old"); err != nil || v != "v-old" {
		t.Errorf("old = %q (%v): values must still open after a restore", v, err)
	}
	mustSave(t, "after", "X", "v") // the restored vault accepts writes again
}

func TestRestoreVaultBackupWorksWhenVaultIsMissing(t *testing.T) {
	vault, _ := twoGenerations(t)
	if err := os.Remove(vault); err != nil {
		t.Fatal(err)
	}
	if err := RestoreVaultBackup(testPassword); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(vault + ".corrupt"); err == nil {
		t.Error("a .corrupt file was written although there was nothing damaged")
	}
	Lock()
	if err := Unlock(testPassword); err != nil || len(titles(t)) != 1 {
		t.Errorf("unlock %v, titles %v", err, titles(t))
	}
}

func TestRestoreVaultBackupAlsoRepairsDamagedData(t *testing.T) {
	vault, _ := twoGenerations(t)
	var vf vaultFile
	if err := json.Unmarshal(readBytes(t, vault), &vf); err != nil {
		t.Fatal(err)
	}
	vf.Data[0] ^= 0xff
	bad, _ := json.Marshal(vf)
	writeBytes(t, vault, bad)
	if err := RestoreVaultBackup(testPassword); err != nil {
		t.Fatal(err)
	}
}

func TestRestoreVaultBackupRefusesWhenItShould(t *testing.T) {
	vault, backup := twoGenerations(t)
	good := readBytes(t, vault)
	bak := readBytes(t, backup)
	damaged := []byte("garbage")

	other, _, err := newVault("a different master password")
	if err != nil {
		t.Fatal(err)
	}
	otherRaw, _ := json.Marshal(other)

	var vf vaultFile
	_ = json.Unmarshal(good, &vf)
	vf.Version = vaultVersion + 1
	future, _ := json.Marshal(vf)

	tests := []struct {
		name          string
		vault, backup []byte // nil removes the file
		password      string
		want          error
	}{
		{"wrong password, damaged vault", damaged, bak, "not the password", ErrWrongPassword},
		{"healthy vault", good, bak, testPassword, ErrRestoreNotNeeded},
		{"damaged vault, damaged backup", damaged, damaged, testPassword, ErrNoUsableBackup},
		{"damaged vault, no backup", damaged, nil, testPassword, ErrNoUsableBackup},
		{"a different vault in place (other password)", otherRaw, bak, testPassword, ErrRestoreRefused},
		{"vault from a newer version", future, bak, testPassword, ErrRestoreRefused},
	}
	for _, tc := range tests {
		for path, content := range map[string][]byte{vault: tc.vault, backup: tc.backup} {
			if content == nil {
				os.Remove(path)
			} else {
				writeBytes(t, path, content)
			}
		}
		err := RestoreVaultBackup(tc.password)
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: error %v, want %v", tc.name, err, tc.want)
		}
		if !bytes.Equal(readBytes(t, vault), tc.vault) {
			t.Errorf("%s: vault.json was changed", tc.name)
		}
		if tc.backup != nil && !bytes.Equal(readBytes(t, backup), tc.backup) {
			t.Errorf("%s: vault.json.bak was changed", tc.name)
		}
		if _, err := os.Stat(vault + ".corrupt"); err == nil {
			t.Errorf("%s: wrote a .corrupt file without restoring", tc.name)
		}
	}
}

func TestCorruptVaultErrorsAreRecognisable(t *testing.T) {
	vault, _ := twoGenerations(t)
	writeBytes(t, vault, []byte("{nope"))
	if _, err := LoadMetadata(); !errors.Is(err, ErrVaultCorrupt) {
		t.Errorf("LoadMetadata error %v, want ErrVaultCorrupt", err)
	}
	if err := Unlock(testPassword); !errors.Is(err, ErrVaultCorrupt) {
		t.Errorf("Unlock error %v, want ErrVaultCorrupt", err)
	}
}
