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

// damageVaultData flips one byte of vault.json's encrypted records, leaving
// the file valid JSON so only the AEAD tag catches it.
func damageVaultData(t *testing.T) {
	t.Helper()
	p, _ := vaultPath()
	vf, err := parseVault(mustRead(t, p))
	if err != nil {
		t.Fatal(err)
	}
	vf.Data[30] ^= 1
	out, err := marshalVault(vf)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, out, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestPasswordChangeRefusesADamagedVaultAndKeepsTheBackup(t *testing.T) {
	setup(t)
	mustSave(t, "a", "", "1")
	mustSave(t, "b", "", "2") // leaves a healthy vault.json.bak
	bak := mustRead(t, backupPath(t))
	damageVaultData(t)

	if err := ChangePassword(testPassword, rotatedPassword); !errors.Is(err, ErrVaultCorrupt) {
		t.Fatalf("ChangePassword = %v, want ErrVaultCorrupt", err)
	}
	other, err := newSharing()
	if err != nil {
		t.Fatal(err)
	}
	otherKey, err := other.shareKey()
	if err != nil {
		t.Fatal(err)
	}
	if err := AddPeer("p", otherKey); !errors.Is(err, ErrVaultCorrupt) {
		t.Fatalf("AddPeer = %v, want ErrVaultCorrupt", err)
	}
	if !bytes.Equal(mustRead(t, backupPath(t)), bak) {
		t.Error("vault.json.bak changed although the vault was damaged")
	}
	rep, err := InspectVault(testPassword)
	if err != nil || rep.Backup.Health != FileOK {
		t.Errorf("backup after the refused writes: %+v, %v; want FileOK", rep.Backup, err)
	}
}

func TestRotateKeyReplacesTheShareIdentityAndKeepsPeers(t *testing.T) {
	setup(t)
	mustSave(t, "k", "", "v")
	before, err := ShareKey()
	if err != nil {
		t.Fatal(err)
	}
	other, _ := newSharing()
	otherKey, _ := other.shareKey()
	if err := AddPeer("p", otherKey); err != nil {
		t.Fatal(err)
	}
	peersBefore, _ := Peers()

	got, err := RotateKey(testPassword, rotatedPassword)
	if err != nil {
		t.Fatal(err)
	}
	after, err := ShareKey()
	if err != nil || after == before {
		t.Fatalf("ShareKey after rotation = %q, %v; want a new identity", after, err)
	}
	if got != after {
		t.Errorf("RotateKey returned %q, want the new share key %q", got, after)
	}
	peersAfter, err := Peers()
	if err != nil || len(peersAfter) != 1 || peersAfter[0] != peersBefore[0] {
		t.Errorf("Peers after rotation = %v, %v; want %v", peersAfter, err, peersBefore)
	}
}

func TestRotateKeyKeepsLeftoversUntilDoctorRemovesThem(t *testing.T) {
	setup(t)
	mustSave(t, "stuck", "", "x")
	mustSave(t, "fine", "", "y")
	stuck := idOf(t, "stuck")
	dropOld = func(id string, n int) error {
		if id == stuck {
			return errors.New("keychain is busy")
		}
		return dropValue(id, n)
	}
	restore := func() { dropOld = dropValue }
	t.Cleanup(restore)

	_, err := RotateKey(testPassword, rotatedPassword)
	var cleanup *PasswordCleanupError
	if !errors.As(err, &cleanup) {
		t.Fatalf("RotateKey = %v, want *PasswordCleanupError", err)
	}
	if n, err := PendingCleanup(); err != nil || n != 1 {
		t.Fatalf("PendingCleanup = %d, %v; want 1", n, err)
	}
	if _, err := keyring.Get(ServiceName, stuck); err != nil {
		t.Fatalf("the old entry should still be there: %v", err)
	}

	restore()
	if left, err := DropPendingCleanup(); err != nil || left != 0 {
		t.Fatalf("DropPendingCleanup = %d, %v; want 0, nil", left, err)
	}
	if _, err := keyring.Get(ServiceName, stuck); !errors.Is(err, keyring.ErrNotFound) {
		t.Errorf("old entry after cleanup: %v, want ErrNotFound", err)
	}
	if n, err := PendingCleanup(); err != nil || n != 0 {
		t.Errorf("PendingCleanup after cleanup = %d, %v; want 0", n, err)
	}
}

func TestRotateKeyRemovesLeftoverChunksOfAMissingValue(t *testing.T) {
	setup(t)
	mustSave(t, "long", "", strings.Repeat("0123456789abcdef", 400))
	recs, _ := loadRecords()
	r := recs[recordIndex(recs, "long")]
	if r.chunkCount() < 2 {
		t.Fatal("fixture: the value must span several chunks")
	}
	if err := keyring.Delete(ServiceName, chunkAccount(r.ID, 0)); err != nil {
		t.Fatal(err)
	}
	if _, err := RotateKey(testPassword, rotatedPassword); err != nil {
		t.Fatal(err)
	}
	if _, err := keyring.Get(ServiceName, chunkAccount(r.ID, 1)); !errors.Is(err, keyring.ErrNotFound) {
		t.Errorf("leftover chunk after rotation: %v, want ErrNotFound", err)
	}
	if _, err := GetSecretValue("long"); !IsValueMissing(err) {
		t.Errorf("GetSecretValue = %v, want the value still reported as missing", err)
	}
}

func TestAuditFromAStaleSessionIsRefused(t *testing.T) {
	setupAudit(t)
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
	if _, err := RotateKey(testPassword, rotatedPassword); err != nil {
		t.Fatal(err)
	}
	Lock()
	setSession(key, wrapped)

	if err := RecordAudit("get", "k"); !errors.Is(err, ErrKeyChanged) {
		t.Fatalf("RecordAudit = %v, want ErrKeyChanged", err)
	}
	Lock()
	if err := Unlock(rotatedPassword); err != nil {
		t.Fatal(err)
	}
	if _, bad, err := ReadAudit(time.Time{}); err != nil || bad != 0 {
		t.Errorf("ReadAudit = %d unreadable, %v; want none", bad, err)
	}
}

func TestValidateTextRejectsInvisibleFormatting(t *testing.T) {
	for _, s := range []string{"a\u202eb", "a\u200bb", "a\ufeffb", "a\u2066b"} {
		if err := ValidateText(s); !errors.Is(err, ErrInvalidText) {
			t.Errorf("ValidateText(%q) = %v, want ErrInvalidText", s, err)
		}
	}
	if err := ValidateText("👨\u200d👩"); err != nil {
		t.Errorf("a zero-width joiner in an emoji sequence was rejected: %v", err)
	}
}

func TestPrepareMetaRejectsABadDate(t *testing.T) {
	if err := prepareMeta(&SecretMeta{Title: "t", Expires: "tomorrow"}); !errors.Is(err, ErrBadDate) {
		t.Errorf("prepareMeta = %v, want ErrBadDate", err)
	}
}
