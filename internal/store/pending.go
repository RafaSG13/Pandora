package store

import (
	"encoding/json"
	"fmt"
)

// aadPending binds the pending-cleanup list to its role in vault.json.
const aadPending = "pandora-v2-pending"

// pendingEntry is the stored form of one storedValue.
type pendingEntry struct {
	ID string `json:"id"`
	N  int    `json:"n"`
}

// pending decrypts the list of old keyring entries a key rotation still has
// to remove. An absent list is empty.
func (vf vaultFile) pending(key []byte) ([]storedValue, error) {
	if len(vf.Pending) == 0 {
		return nil, nil
	}
	plain, err := open(key, vf.Pending, []byte(aadPending))
	if err != nil {
		return nil, fmt.Errorf("%w: the pending-cleanup list was modified or is damaged", ErrVaultCorrupt)
	}
	var entries []pendingEntry
	if err := json.Unmarshal(plain, &entries); err != nil {
		return nil, fmt.Errorf("%w: the pending-cleanup list was modified or is damaged", ErrVaultCorrupt)
	}
	out := make([]storedValue, len(entries))
	for i, e := range entries {
		out[i] = storedValue{e.ID, e.N}
	}
	return out, nil
}

// setPending seals s as the pending-cleanup list; an empty s removes it.
func (vf *vaultFile) setPending(key []byte, s []storedValue) error {
	if len(s) == 0 {
		vf.Pending = nil
		return nil
	}
	entries := make([]pendingEntry, len(s))
	for i, v := range s {
		entries[i] = pendingEntry{v.id, v.n}
	}
	plain, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	vf.Pending, err = seal(key, plain, []byte(aadPending))
	return err
}

// PendingCleanup reports how many old keyring values left by a key rotation still wait to be removed.
func PendingCleanup() (int, error) {
	key, err := currentKey()
	if err != nil {
		return 0, err
	}
	defer zero(key)
	vf, err := readVault()
	if err != nil {
		return 0, err
	}
	if _, err := openRecords(vf, key); err != nil {
		return 0, err
	}
	p, err := vf.pending(key)
	return len(p), err
}

// DropPendingCleanup removes them and returns how many could not be removed.
// Nothing is written when the list was already empty.
func DropPendingCleanup() (left int, err error) {
	key, err := currentKey()
	if err != nil {
		return 0, err
	}
	defer zero(key)
	release, err := lockVault()
	if err != nil {
		return 0, err
	}
	defer release()
	vf, err := readVault()
	if err != nil {
		return 0, err
	}
	if _, err := openRecords(vf, key); err != nil {
		return 0, err
	}
	p, err := vf.pending(key)
	if err != nil || len(p) == 0 {
		return 0, err
	}
	var failed []storedValue
	for _, s := range p {
		if dropValue(s.id, s.n) != nil {
			failed = append(failed, s)
		}
	}
	if err := vf.setPending(key, failed); err != nil {
		return len(failed), err
	}
	if err := writeVault(vf); err != nil {
		return len(failed), err
	}
	return len(failed), nil
}
