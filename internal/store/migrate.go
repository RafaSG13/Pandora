package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/zalando/go-keyring"
)

// CleanupError reports legacy leftovers that could not be removed after a
// successful migration. The vault is complete; callers should show it as a
// warning. Removal is not secure erasure: on SSD/APFS the old plaintext may
// remain recoverable.
type CleanupError struct {
	Leftovers []string
}

func (e *CleanupError) Error() string {
	return "could not remove old plain-text data: " + strings.Join(e.Leftovers, "; ")
}

func legacyPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "pandora", "metadata.json"), nil
}

// LegacyExists reports whether a plain-text metadata.json from an older
// version is present.
func LegacyExists() bool {
	p, err := legacyPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}

// LegacyCount returns how many secrets MigrateLegacy would carry over, or 0
// when the old metadata is missing or unreadable.
func LegacyCount() int {
	items, err := loadLegacy()
	if err != nil {
		return 0
	}
	return len(items)
}

// legacyItem is one old entry: its sanitised metadata and the keyring account
// (the raw, unsanitised title) holding its value.
type legacyItem struct {
	meta    SecretMeta
	account string
}

// loadLegacy reads the old plain-text metadata, collapsing duplicate titles
// (last wins) and replacing control characters with U+FFFD.
func loadLegacy() ([]legacyItem, error) {
	file, err := legacyPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var raw []SecretMeta
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("reading metadata.json: %w", err)
	}
	clean := func(s string) string {
		return strings.Map(func(r rune) rune {
			if unicode.IsControl(r) {
				return unicode.ReplacementChar
			}
			return r
		}, s)
	}
	items := make([]legacyItem, 0, len(raw))
	for _, m := range raw {
		it := legacyItem{account: m.Title}
		m.Title, m.Category, m.Notes = clean(m.Title), clean(m.Category), clean(m.Notes)
		it.meta = m
		replaced := false
		for i := range items {
			if items[i].meta.Title == m.Title {
				items[i], replaced = it, true
				break
			}
		}
		if !replaced {
			items = append(items, it)
		}
	}
	return items, nil
}

// MigrateLegacy converts the plain-text metadata.json (and the values stored
// in the keyring under the secrets' titles) into an encrypted vault protected
// by password, and leaves the vault unlocked. Entries whose value is missing
// from the keyring are kept so doctor can list them. On failure the legacy
// files and keyring entries are left untouched.
func MigrateLegacy(password string) error {
	release, err := lockVault()
	if err != nil {
		return err
	}
	defer release()
	if !LegacyExists() {
		return errors.New("no legacy metadata to migrate")
	}
	if Exists() {
		return ErrVaultExists
	}
	items, err := loadLegacy()
	if err != nil {
		return err
	}
	vf, key, err := newVault(password)
	if err != nil {
		return err
	}
	defer zero(key)

	var created []storedValue
	undo := func() {
		for _, s := range created {
			_ = dropValue(s.id, s.n)
		}
	}
	recs := make([]record, 0, len(items))
	for _, it := range items {
		id, err := newID()
		if err != nil {
			undo()
			return err
		}
		rec := record{ID: id, SecretMeta: it.meta}
		value, err := keyring.Get(ServiceName, it.account)
		if errors.Is(err, keyring.ErrNotFound) {
			recs = append(recs, rec)
			continue
		} else if err != nil {
			undo()
			return fmt.Errorf("reading %q from the keyring: %w", it.meta.Title, err)
		}
		sealed, err := sealValueWith(key, id, value)
		if err != nil {
			undo()
			return err
		}
		n, err := putSealed(id, sealed)
		if err != nil {
			undo()
			return err
		}
		created = append(created, storedValue{id, n})
		rec.Chunks = chunksField(n)
		recs = append(recs, rec)
	}

	if err := vf.setRecords(key, recs); err != nil {
		undo()
		return err
	}
	if err := writeVault(vf); err != nil {
		undo()
		return err
	}
	setSession(key)

	var left []string
	for _, it := range items {
		if err := keyring.Delete(ServiceName, it.account); err != nil && !errors.Is(err, keyring.ErrNotFound) {
			left = append(left, "old keyring entry for a secret: "+err.Error())
		}
	}
	legacy, err := legacyPath()
	if err != nil {
		return err
	}
	for _, f := range []string{legacy, legacy + ".bak"} {
		if err := os.Remove(f); err != nil && !os.IsNotExist(err) {
			left = append(left, err.Error())
		}
	}
	if len(left) > 0 {
		return &CleanupError{Leftovers: left}
	}
	return nil
}
