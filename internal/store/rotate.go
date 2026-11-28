package store

import (
	"crypto/rand"
	"errors"
	"fmt"
	"strings"

	"github.com/zalando/go-keyring"
)

// RotationCleanupError reports that the key was rotated and the vault is
// complete, but some old data could not be removed. Show it as a warning.
type RotationCleanupError struct {
	// Problems describes what is left behind; it never contains a value.
	Problems []string
}

func (e *RotationCleanupError) Error() string {
	return "data key rotated, but some old data could not be removed: " + strings.Join(e.Problems, "; ")
}

// RotateKey changes the master password to newPw and replaces the data key
// with a fresh random one. Unlike ChangePassword, anyone who kept an old copy
// of vault.json and knows the old password can no longer read the current
// keyring values, because they are re-encrypted under the new key.
//
// Under the vault write lock it: unwraps the data key with oldPw; decrypts the
// records and the share state; re-encrypts every current and previous value
// into fresh keyring entries (values whose entry was already missing keep
// their reference, `doctor` reports them; a value that does not decrypt aborts
// the rotation); and writes the new vault.json (new KDF salt, new wrapped
// key). Until that write nothing referenced by the old vault changes, so any
// earlier failure removes the new entries and leaves the vault untouched.
// After the write it replaces vault.json.bak, switches an unlocked session to
// the new key, re-encrypts the audit log and deletes the old keyring entries;
// failures there are returned as *RotationCleanupError. The share identity and
// peers are unchanged, so peers need nothing.
func RotateKey(oldPw, newPw string) error {
	release, err := lockVault()
	if err != nil {
		return err
	}
	defer release()
	vf, err := readVault()
	if err != nil {
		return err
	}
	oldKey, err := vf.unwrap(oldPw)
	if err != nil {
		return err
	}
	defer zero(oldKey)
	if err := checkPassword(newPw); err != nil {
		return err
	}
	recs, err := vf.records(oldKey)
	if err != nil {
		return err
	}
	sh, err := vf.sharing(oldKey)
	if err != nil {
		return err
	}
	newKey := make([]byte, keyLen)
	if _, err := rand.Read(newKey); err != nil {
		return err
	}
	defer zero(newKey)

	var written, old []storedValue
	// move re-encrypts the value in n entries of id under newKey. It returns
	// the new id and entry count, or the old ones when the value is missing.
	move := func(title, id string, n int) (string, int, error) {
		sealed, err := readSealed(id, n)
		if errors.Is(err, keyring.ErrNotFound) {
			return id, n, nil
		} else if err != nil {
			return "", 0, err
		}
		plain, err := openValueWith(oldKey, id, sealed)
		if err != nil {
			return "", 0, fmt.Errorf("secret %q: %w", title, errCorruptValue)
		}
		nid, err := newID()
		if err != nil {
			return "", 0, err
		}
		resealed, err := sealValueWith(newKey, nid, plain)
		if err != nil {
			return "", 0, err
		}
		k, err := putSealed(nid, resealed)
		if err != nil {
			return "", 0, err
		}
		written = append(written, storedValue{nid, k})
		old = append(old, storedValue{id, n})
		return nid, k, nil
	}
	fail := func(err error) error {
		dropStored(written)
		return err
	}
	for i := range recs {
		r := &recs[i]
		nid, k, err := move(r.Title, r.ID, r.chunkCount())
		if err != nil {
			return fail(err)
		}
		r.ID, r.Chunks = nid, chunksField(k)
		for j := range r.History {
			v := &r.History[j]
			nid, k, err := move(r.Title, v.ID, max(1, v.Chunks))
			if err != nil {
				return fail(err)
			}
			v.ID, v.Chunks = nid, chunksField(k)
		}
	}

	p, err := newKDF()
	if err != nil {
		return fail(err)
	}
	kek := deriveKEK(newPw, p)
	defer zero(kek)
	wrapped, err := seal(kek, newKey, []byte(aadKey))
	if err != nil {
		return fail(err)
	}
	nvf := vaultFile{Version: vaultVersion, KDF: p, WrappedKey: wrapped}
	if err := nvf.setRecords(newKey, recs); err != nil {
		return fail(err)
	}
	if sh != nil {
		if err := nvf.setSharing(newKey, sh); err != nil {
			return fail(err)
		}
	}
	if err := writeVault(nvf); err != nil {
		return fail(err)
	}

	// The vault is committed: collect problems instead of returning early.
	var problems []string
	if err := replaceBackupWithCurrent(); err != nil {
		problems = append(problems, "vault.json.bak still holds the old key: "+err.Error())
	}
	if IsUnlocked() {
		setSession(newKey, wrapped)
	}
	if err := resealAudit(oldKey, newKey); err != nil {
		problems = append(problems, "audit log: "+err.Error())
	}
	failed := 0
	for _, s := range old {
		if err := dropValue(s.id, s.n); err != nil {
			failed++
		}
	}
	if failed > 0 {
		problems = append(problems, fmt.Sprintf("%d old keyring entries could not be removed", failed))
	}
	if len(problems) > 0 {
		return &RotationCleanupError{Problems: problems}
	}
	return nil
}
