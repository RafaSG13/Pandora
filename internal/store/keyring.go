// Package store persists secrets: names and metadata in an encrypted vault
// file in the user's config directory, values (encrypted under the same key)
// in the OS keyring under opaque random account IDs. Everything requires the
// master password: see Unlock.
package store

import (
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/zalando/go-keyring"
)

// ServiceName is the keyring service under which secret values are stored.
const ServiceName = "pandora-cli"

var (
	// ErrExists is returned when a secret with the requested name already exists.
	ErrExists = errors.New("a secret with that name already exists")
	// ErrNotFound is returned when the named secret is not in the metadata.
	ErrNotFound = errors.New("secret not found")
)

// SecretMeta is the non-sensitive description of a secret. It is persisted as JSON (encrypted); never put values here.
// Dates are YYYY-MM-DD. Updated, Notes, Expires and Tags are optional; writers
// store Tags in the form NormalizeTags returns.
type SecretMeta struct {
	Title    string   `json:"title"`
	Category string   `json:"category"`
	Username string   `json:"username,omitempty"`
	Created  string   `json:"created"`
	Updated  string   `json:"updated,omitempty"`
	Expires  string   `json:"expires,omitempty"`
	Notes    string   `json:"notes,omitempty"`
	Tags     []string `json:"tags,omitempty"`
}

var errCorruptValue = errors.New("stored value is corrupt or was not written by Pandora")

// sealValue encrypts value under the data key, binding it to the record id so
// ciphertexts cannot be swapped between entries.
func sealValue(id, value string) (string, error) {
	key, err := currentKey()
	if err != nil {
		return "", err
	}
	defer zero(key)
	return sealValueWith(key, id, value)
}

func sealValueWith(key []byte, id, value string) (string, error) {
	blob, err := seal(key, []byte(value), []byte(id))
	if err != nil {
		return "", err
	}
	return valuePrefix + base64.StdEncoding.EncodeToString(blob), nil
}

// openValue reverses sealValue.
func openValue(id, stored string) (string, error) {
	key, err := currentKey()
	if err != nil {
		return "", err
	}
	defer zero(key)
	return openValueWith(key, id, stored)
}

// openValueWith reverses sealValueWith under an explicit data key.
func openValueWith(key []byte, id, stored string) (string, error) {
	rest, ok := strings.CutPrefix(stored, valuePrefix)
	if !ok {
		return "", errCorruptValue
	}
	blob, err := base64.StdEncoding.DecodeString(rest)
	if err != nil {
		return "", errCorruptValue
	}
	plain, err := open(key, blob, []byte(id))
	if err != nil {
		return "", errCorruptValue
	}
	return string(plain), nil
}

// SaveSecret stores value (encrypted, split across keyring entries when it is
// long) under a new random ID and records meta in the vault. It returns
// ErrExists if a secret with the same title is already recorded and
// ErrValueTooLarge above MaxValueBytes.
func SaveSecret(meta SecretMeta, value string) error {
	if err := prepareMeta(&meta); err != nil {
		return err
	}
	if err := checkValueSize(value); err != nil {
		return err
	}
	release, err := lockVault()
	if err != nil {
		return err
	}
	defer release()
	recs, err := loadRecords()
	if err != nil {
		return err
	}
	if recordIndex(recs, meta.Title) >= 0 {
		return ErrExists
	}
	id, err := newID()
	if err != nil {
		return err
	}
	n, err := putValue(id, value)
	if err != nil {
		return err
	}
	if err := saveRecords(append(recs, record{ID: id, Chunks: chunksField(n), SecretMeta: meta})); err != nil {
		_ = dropValue(id, n)
		return err
	}
	return nil
}

// GetSecretValue returns the secret value stored under title. An unknown
// title returns keyring.ErrNotFound, like a value missing from the keyring.
func GetSecretValue(title string) (string, error) {
	recs, err := loadRecords()
	if err != nil {
		return "", err
	}
	i := recordIndex(recs, title)
	if i < 0 {
		return "", keyring.ErrNotFound
	}
	return getValue(recs[i])
}

// UpdateSecret changes the secret recorded as oldTitle. meta.Title,
// meta.Category, meta.Username, meta.Expires, meta.Notes and meta.Tags replace the stored fields;
// meta.Created is ignored and the original date is kept, and Updated is set to
// today. A non-nil newValue replaces the stored value; nil keeps it. Renaming
// to a title that is already in use returns ErrExists. A rename only touches
// the vault: the keyring entries keep their ID. A new value is written under
// a fresh ID and the old entries are removed only after the vault points at
// the new ones, so a failure never leaves a half-replaced value. The replaced
// value is kept as a previous version when SetHistoryDepth enables history
// (and is dropped past that depth); setting the value it already has changes
// nothing but the metadata.
func UpdateSecret(oldTitle string, meta SecretMeta, newValue *string) error {
	if err := prepareMeta(&meta); err != nil {
		return err
	}
	if newValue != nil {
		if err := checkValueSize(*newValue); err != nil {
			return err
		}
	}
	release, err := lockVault()
	if err != nil {
		return err
	}
	defer release()
	recs, err := loadRecords()
	if err != nil {
		return err
	}
	idx := recordIndex(recs, oldTitle)
	if idx < 0 {
		return ErrNotFound
	}
	if meta.Title != oldTitle && recordIndex(recs, meta.Title) >= 0 {
		return ErrExists
	}

	var fresh *storedValue
	var unused []storedValue
	if newValue != nil {
		cur, readable, err := currentValue(recs[idx])
		if err != nil {
			return err
		}
		if readable && cur == *newValue {
			newValue = nil // the value did not change: no new version
		} else {
			id, err := newID()
			if err != nil {
				return err
			}
			n, err := putValue(id, *newValue)
			if err != nil {
				return err
			}
			fresh = &storedValue{id, n}
			unused = recs[idx].replaceValue(id, n, readable, time.Now().Format(DateLayout))
		}
	}

	meta.Created = recs[idx].Created
	meta.Updated = time.Now().Format(DateLayout)
	recs[idx].SecretMeta = meta
	if err := saveRecords(recs); err != nil {
		if fresh != nil {
			_ = dropValue(fresh.id, fresh.n)
		}
		return err
	}
	dropStored(unused) // best effort: the vault no longer refers to them
	return nil
}

// DeleteSecret removes the secret's value from the keyring and its vault entry.
// A value already missing from the keyring is not an error, so half-deleted
// secrets can still be cleaned up.
func DeleteSecret(title string) error {
	release, err := lockVault()
	if err != nil {
		return err
	}
	defer release()
	recs, err := loadRecords()
	if err != nil {
		return err
	}
	idx := recordIndex(recs, title)
	if idx < 0 {
		return ErrNotFound
	}
	unused := []storedValue{{recs[idx].ID, recs[idx].chunkCount()}}
	for _, v := range recs[idx].History {
		unused = append(unused, v.stored())
	}
	for _, s := range unused {
		if err := dropValue(s.id, s.n); err != nil {
			return err
		}
	}
	return saveRecords(append(recs[:idx], recs[idx+1:]...))
}

// LoadMetadata reads all secret metadata. An empty vault yields an empty
// slice; a locked one ErrLocked. Entries sharing a title are collapsed to the
// most recent one.
func LoadMetadata() ([]SecretMeta, error) {
	recs, err := loadRecords()
	if err != nil {
		return nil, err
	}
	items := make([]SecretMeta, 0, len(recs))
	for _, r := range recs {
		if i := indexOf(items, r.Title); i >= 0 {
			items[i] = r.SecretMeta
		} else {
			items = append(items, r.SecretMeta)
		}
	}
	return items, nil
}

func indexOf(items []SecretMeta, title string) int {
	for i, m := range items {
		if m.Title == title {
			return i
		}
	}
	return -1
}
