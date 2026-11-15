package store

import "github.com/zalando/go-keyring"

// Restore is one secret to bring back with RestoreSecrets. A nil Value leaves
// the keyring untouched for that title.
type Restore struct {
	Meta  SecretMeta
	Value *string
}

// RestoreSecrets records every entry's metadata exactly as given (replacing
// any entry with the same title and keeping its dates) and stores the values
// that are present. The metadata file is written once, so metadata.json.bak
// holds the state from before the whole batch. If a keyring write or the
// metadata write fails, values stored for titles that did not exist before
// are removed again (values replaced for existing titles cannot be restored).
func RestoreSecrets(entries []Restore) error {
	existing, err := LoadMetadata()
	if err != nil {
		return err
	}
	items := append([]SecretMeta(nil), existing...)

	var stored []string
	rollback := func() {
		for _, t := range stored {
			if indexOf(existing, t) < 0 {
				_ = keyring.Delete(ServiceName, t)
			}
		}
	}

	for _, e := range entries {
		if e.Value != nil {
			if err := keyring.Set(ServiceName, e.Meta.Title, *e.Value); err != nil {
				rollback()
				return err
			}
			stored = append(stored, e.Meta.Title)
		}
		if i := indexOf(items, e.Meta.Title); i >= 0 {
			items[i] = e.Meta
		} else {
			items = append(items, e.Meta)
		}
	}
	if err := writeMetadata(items); err != nil {
		rollback()
		return err
	}
	return nil
}
