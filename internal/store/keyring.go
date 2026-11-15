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
// Dates are YYYY-MM-DD. Updated, Notes and Expires are optional.
type SecretMeta struct {
	Title    string `json:"title"`
	Category string `json:"category"`
	Created  string `json:"created"`
	Updated  string `json:"updated,omitempty"`
	Expires  string `json:"expires,omitempty"`
	Notes    string `json:"notes,omitempty"`
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

// SaveSecret stores value (encrypted) in the OS keyring under a new random ID and records meta in the vault.
// It returns ErrExists if a secret with the same title is already recorded.
func SaveSecret(meta SecretMeta, value string) error {
	if err := validateMeta(meta); err != nil {
		return err
	}
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
	sealed, err := sealValue(id, value)
	if err != nil {
		return err
	}
	if err := keyring.Set(ServiceName, id, sealed); err != nil {
		return err
	}
	if err := saveRecords(append(recs, record{ID: id, SecretMeta: meta})); err != nil {
		_ = keyring.Delete(ServiceName, id)
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
	stored, err := keyring.Get(ServiceName, recs[i].ID)
	if err != nil {
		return "", err
	}
	return openValue(recs[i].ID, stored)
}

// UpdateSecret changes the secret recorded as oldTitle. meta.Title,
// meta.Category, meta.Expires and meta.Notes replace the stored fields;
// meta.Created is ignored and the original date is kept, and Updated is set to
// today. A non-nil newValue replaces the stored value; nil keeps it. Renaming
// to a title that is already in use returns ErrExists. A rename only touches
// the vault: the keyring entry keeps its ID.
func UpdateSecret(oldTitle string, meta SecretMeta, newValue *string) error {
	if err := validateMeta(meta); err != nil {
		return err
	}
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

	id := recs[idx].ID
	if newValue != nil {
		sealed, err := sealValue(id, *newValue)
		if err != nil {
			return err
		}
		if err := keyring.Set(ServiceName, id, sealed); err != nil {
			return err
		}
	}

	meta.Created = recs[idx].Created
	meta.Updated = time.Now().Format(DateLayout)
	recs[idx].SecretMeta = meta
	return saveRecords(recs)
}

// DeleteSecret removes the secret's value from the keyring and its vault entry.
// A value already missing from the keyring is not an error, so half-deleted
// secrets can still be cleaned up.
func DeleteSecret(title string) error {
	recs, err := loadRecords()
	if err != nil {
		return err
	}
	idx := recordIndex(recs, title)
	if idx < 0 {
		return ErrNotFound
	}
	if err := keyring.Delete(ServiceName, recs[idx].ID); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return err
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
