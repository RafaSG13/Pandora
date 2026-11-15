package store

import (
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
)

// MissingValues returns the titles of metadata entries whose value is not in
// the keyring (for example after the keyring was reset or an entry was deleted
// with a system tool). The opposite problem, keyring entries without metadata,
// cannot be detected portably because go-keyring cannot enumerate entries.
// Any keyring error other than "not found" (a locked keychain, no Secret
// Service) is returned instead of being reported as a missing value.
func MissingValues() ([]string, error) {
	items, err := LoadMetadata()
	if err != nil {
		return nil, err
	}
	var missing []string
	for _, m := range items {
		if _, err := keyring.Get(ServiceName, m.Title); err != nil {
			if errors.Is(err, keyring.ErrNotFound) {
				missing = append(missing, m.Title)
				continue
			}
			return nil, fmt.Errorf("reading %q from the keyring: %w", m.Title, err)
		}
	}
	return missing, nil
}
