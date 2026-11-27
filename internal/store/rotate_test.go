package store

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/zalando/go-keyring"
)

const rotatedPassword = "another long password"

// rotationFixture creates secrets that exercise every kind of keyring entry: a
// short value, one spanning several chunks, and one with a history version.
func rotationFixture(t *testing.T) (values map[string]string) {
	t.Helper()
	setupHistory(t, 2)
	long := strings.Repeat("0123456789abcdef", 400) // 6400 chars sealed: several chunks
	values = map[string]string{"short": "s3cret", "long": long, "hist": "new-value"}
	mustSave(t, "short", "", values["short"])
	mustSave(t, "long", "", values["long"])
	mustSave(t, "hist", "", "old-value")
	next := values["hist"]
	if err := UpdateSecret("hist", SecretMeta{Title: "hist", Created: "2026-01-02"}, &next); err != nil {
		t.Fatal(err)
	}
	return values
}

func TestRotateKeyKeepsValuesAndLocksOutTheOldKey(t *testing.T) {
	values := rotationFixture(t)
	recs, _ := loadRecords()
	if recs[recordIndex(recs, "long")].chunkCount() < 2 {
		t.Fatal("fixture: the long value must span several chunks")
	}
	var oldIDs []storedValue
	for _, r := range recs {
		oldIDs = append(oldIDs, storedValue{r.ID, r.chunkCount()})
		for _, v := range r.History {
			oldIDs = append(oldIDs, v.stored())
		}
	}
	shareBefore, err := ShareKey()
	if err != nil {
		t.Fatal(err)
	}
	oldVault := readVaultBytes(t)

	if err := RotateKey(testPassword, rotatedPassword); err != nil {
		t.Fatal(err)
	}

	for title, want := range values {
		if got, err := GetSecretValue(title); err != nil || got != want {
			t.Errorf("GetSecretValue(%q) = %d bytes, %v; want the original", title, len(got), err)
		}
	}
	if got, err := GetVersion("hist", 1); err != nil || got != "old-value" {
		t.Errorf("GetVersion = %q, %v; want the previous value", got, err)
	}
	for _, s := range oldIDs {
		for k := range s.n {
			if _, err := keyring.Get(ServiceName, chunkAccount(s.id, k)); !errors.Is(err, keyring.ErrNotFound) {
				t.Errorf("old entry %s still in the keyring: %v", chunkAccount(s.id, k), err)
			}
		}
	}
	if got, err := ShareKey(); err != nil || got != shareBefore {
		t.Errorf("share key changed by rotation: %q, %v", got, err)
	}
	if cur := readVaultBytes(t); !bytes.Equal(cur, mustRead(t, backupPath(t))) {
		t.Error("vault.json.bak does not equal the new vault.json")
	}

	Lock()
	if err := Unlock(testPassword); !errors.Is(err, ErrWrongPassword) {
		t.Errorf("Unlock(old) = %v, want ErrWrongPassword", err)
	}
	if err := Unlock(rotatedPassword); err != nil {
		t.Errorf("Unlock(new): %v", err)
	}

	// The core property: the old password plus a pre-rotation copy of
	// vault.json yields a key that no longer opens any current value.
	vf, err := parseVault(oldVault)
	if err != nil {
		t.Fatal(err)
	}
	oldKey, err := vf.unwrap(testPassword)
	if err != nil {
		t.Fatal(err)
	}
	recs, _ = loadRecords()
	for _, r := range recs {
		sealed, err := readSealed(r.ID, r.chunkCount())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := openValueWith(oldKey, r.ID, sealed); err == nil {
			t.Errorf("%q still opens with the pre-rotation data key", r.Title)
		}
	}
}

func backupPath(t *testing.T) string {
	t.Helper()
	p, err := vaultPath()
	if err != nil {
		t.Fatal(err)
	}
	return p + ".bak"
}

func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestRotateKeyKeepsTheAuditLogReadable(t *testing.T) {
	setupAudit(t)
	mustSave(t, "k", "", "v")
	if err := RecordAudit("get", "k"); err != nil {
		t.Fatal(err)
	}
	if err := RotateKey(testPassword, rotatedPassword); err != nil {
		t.Fatal(err)
	}
	entries, bad, err := ReadAudit(time.Time{})
	if err != nil || bad != 0 || len(entries) != 1 || entries[0].Title != "k" {
		t.Errorf("ReadAudit after rotation = %v, %d unreadable, %v", entries, bad, err)
	}
}

func TestRotateKeyAbortsOnACorruptValueAndChangesNothing(t *testing.T) {
	setup(t)
	mustSave(t, "good", "", "fine")
	mustSave(t, "bad", "", "doomed")
	if err := keyring.Set(ServiceName, idOf(t, "bad"), "pv2:AAAA"); err != nil {
		t.Fatal(err)
	}
	before := readVaultBytes(t)

	if err := RotateKey(testPassword, rotatedPassword); !errors.Is(err, errCorruptValue) {
		t.Fatalf("RotateKey = %v, want errCorruptValue", err)
	}
	if !bytes.Equal(readVaultBytes(t), before) {
		t.Error("vault.json changed although the rotation aborted")
	}
	Lock()
	if err := Unlock(testPassword); err != nil {
		t.Fatalf("old password no longer works: %v", err)
	}
	if v, err := GetSecretValue("good"); err != nil || v != "fine" {
		t.Errorf("good = %q, %v", v, err)
	}
}

func TestRotateKeyKeepsReferencesToMissingValues(t *testing.T) {
	setup(t)
	mustSave(t, "gone", "", "x")
	mustSave(t, "here", "", "y")
	if err := keyring.Delete(ServiceName, idOf(t, "gone")); err != nil {
		t.Fatal(err)
	}
	if err := RotateKey(testPassword, rotatedPassword); err != nil {
		t.Fatal(err)
	}
	if _, err := GetSecretValue("gone"); !IsValueMissing(err) {
		t.Errorf("missing value after rotation: %v, want it still reported as missing", err)
	}
	if v, err := GetSecretValue("here"); err != nil || v != "y" {
		t.Errorf("here = %q, %v", v, err)
	}
}

func TestRotateKeyRejectsAWrongOldPassword(t *testing.T) {
	setup(t)
	if err := RotateKey("not the password", rotatedPassword); !errors.Is(err, ErrWrongPassword) {
		t.Errorf("RotateKey = %v, want ErrWrongPassword", err)
	}
	if err := RotateKey(testPassword, "short"); !errors.Is(err, ErrPasswordTooShort) {
		t.Errorf("RotateKey with a short password = %v, want ErrPasswordTooShort", err)
	}
}

func TestSessionOfARotatedVaultLocksItself(t *testing.T) {
	setup(t)
	mustSave(t, "k", "", "v")
	key, err := currentKey()
	if err != nil {
		t.Fatal(err)
	}
	session.mu.Lock()
	wrapped := append([]byte(nil), session.wrapped...)
	session.mu.Unlock()
	Lock()
	if err := Unlock(testPassword); err != nil {
		t.Fatal(err)
	}
	if err := RotateKey(testPassword, rotatedPassword); err != nil {
		t.Fatal(err)
	}
	Lock()
	setSession(key, wrapped) // another process still holding the pre-rotation key

	if _, err := LoadMetadata(); !errors.Is(err, ErrKeyChanged) {
		t.Fatalf("LoadMetadata = %v, want ErrKeyChanged", err)
	}
	if IsUnlocked() {
		t.Error("the stale session stayed unlocked")
	}
}

func TestPasswordChangeElsewhereIsNotAKeyChange(t *testing.T) {
	setup(t)
	mustSave(t, "k", "", "v")
	if err := ChangePassword(testPassword, rotatedPassword); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadMetadata(); err != nil || !IsUnlocked() {
		t.Errorf("LoadMetadata after a plain password change = %v, unlocked=%v", err, IsUnlocked())
	}
}
