package store

import (
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
)

// MissingValues returns the titles of vault entries whose value is not in
// the keyring (for example after the keyring was reset or an entry was deleted
// with a system tool). The opposite problem, keyring entries without a vault
// record, cannot be detected portably because go-keyring cannot enumerate entries.
// Any keyring error other than "not found" (a locked keychain, no Secret
// Service) is returned instead of being reported as a missing value.
func MissingValues() ([]string, error) {
	recs, err := loadRecords()
	if err != nil {
		return nil, err
	}
	var missing []string
	for _, r := range recs {
		if _, err := keyring.Get(ServiceName, r.ID); err != nil {
			if errors.Is(err, keyring.ErrNotFound) {
				missing = append(missing, r.Title)
				continue
			}
			return nil, fmt.Errorf("reading %q from the keyring: %w", r.Title, err)
		}
	}
	return missing, nil
}
