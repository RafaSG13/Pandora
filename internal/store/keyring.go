// Package store persists secrets: values in the OS keyring, non-sensitive
// metadata in a JSON file in the user's config directory.
package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
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

// SecretMeta is the non-sensitive description of a secret. It is persisted as JSON; never put values here.
// Dates are YYYY-MM-DD. Updated, Notes and Expires are optional.
type SecretMeta struct {
	Title    string `json:"title"`
	Category string `json:"category"`
	Created  string `json:"created"`
	Updated  string `json:"updated,omitempty"`
	Expires  string `json:"expires,omitempty"`
	Notes    string `json:"notes,omitempty"`
}

// SaveSecret stores value in the OS keyring under meta.Title and records meta in the metadata file.
// It returns ErrExists if a secret with the same title is already recorded.
func SaveSecret(meta SecretMeta, value string) error {
	items, err := LoadMetadata()
	if err != nil {
		return err
	}
	if indexOf(items, meta.Title) >= 0 {
		return ErrExists
	}
	if err := keyring.Set(ServiceName, meta.Title, value); err != nil {
		return err
	}
	if err := writeMetadata(append(items, meta)); err != nil {
		_ = keyring.Delete(ServiceName, meta.Title)
		return err
	}
	return nil
}

// GetSecretValue returns the secret value stored under title.
func GetSecretValue(title string) (string, error) {
	return keyring.Get(ServiceName, title)
}

// UpdateSecret changes the secret recorded as oldTitle. meta.Title,
// meta.Category, meta.Expires and meta.Notes replace the stored fields;
// meta.Created is ignored and the original date is kept, and Updated is set to
// today. A non-nil newValue replaces the stored value; nil keeps it. Renaming
// to a title that is already in use returns ErrExists.
func UpdateSecret(oldTitle string, meta SecretMeta, newValue *string) error {
	items, err := LoadMetadata()
	if err != nil {
		return err
	}
	idx := indexOf(items, oldTitle)
	if idx < 0 {
		return ErrNotFound
	}
	renamed := meta.Title != oldTitle
	if renamed && indexOf(items, meta.Title) >= 0 {
		return ErrExists
	}

	value := ""
	switch {
	case newValue != nil:
		value = *newValue
	case renamed:
		if value, err = keyring.Get(ServiceName, oldTitle); err != nil {
			return err
		}
	}
	if newValue != nil || renamed {
		if err := keyring.Set(ServiceName, meta.Title, value); err != nil {
			return err
		}
	}

	meta.Created = items[idx].Created
	meta.Updated = time.Now().Format(DateLayout)
	items[idx] = meta
	if err := writeMetadata(items); err != nil {
		if renamed {
			_ = keyring.Delete(ServiceName, meta.Title)
		}
		return err
	}
	if renamed {
		if err := keyring.Delete(ServiceName, oldTitle); err != nil && !errors.Is(err, keyring.ErrNotFound) {
			return err
		}
	}
	return nil
}

// DeleteSecret removes the secret's value from the keyring and its metadata entry.
// A value already missing from the keyring is not an error, so half-deleted
// secrets can still be cleaned up.
func DeleteSecret(title string) error {
	items, err := LoadMetadata()
	if err != nil {
		return err
	}
	idx := indexOf(items, title)
	if idx < 0 {
		return ErrNotFound
	}
	if err := keyring.Delete(ServiceName, title); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return err
	}
	return writeMetadata(append(items[:idx], items[idx+1:]...))
}

// LoadMetadata reads all secret metadata. A missing file yields an empty slice.
// Entries sharing a title (possible in files written by older versions) are
// collapsed to the most recent one.
func LoadMetadata() ([]SecretMeta, error) {
	file, err := metadataPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(file)
	if os.IsNotExist(err) {
		return []SecretMeta{}, nil
	} else if err != nil {
		return nil, err
	}

	var raw []SecretMeta
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	items := make([]SecretMeta, 0, len(raw))
	for _, m := range raw {
		if i := indexOf(items, m.Title); i >= 0 {
			items[i] = m
		} else {
			items = append(items, m)
		}
	}
	return items, nil
}

func metadataPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "pandora", "metadata.json"), nil
}

// writeMetadata replaces the metadata file atomically (temp file + rename),
// first keeping the previous contents as metadata.json.bak.
func writeMetadata(items []SecretMeta) error {
	file, err := metadataPath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(file)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	out, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "metadata-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(out); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if old, err := os.ReadFile(file); err == nil {
		if err := os.WriteFile(file+".bak", old, 0600); err != nil {
			return err
		}
	}
	return os.Rename(tmp.Name(), file) // CreateTemp files are 0600
}

func indexOf(items []SecretMeta, title string) int {
	for i, m := range items {
		if m.Title == title {
			return i
		}
	}
	return -1
}
