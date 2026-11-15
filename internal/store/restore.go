package store

import "github.com/zalando/go-keyring"

// Restore is one secret to bring back with RestoreSecrets. A nil Value leaves
// the keyring untouched for that title.
type Restore struct {
	Meta  SecretMeta
	Value *string
}

// RestoreSecrets records every entry's metadata exactly as given (replacing
// any entry with the same title, keeping its keyring ID and dates as given)
// and stores the values that are present. The vault is written once, so
// vault.json.bak holds the state from before the whole batch. If a keyring
// write or the vault write fails, values stored for titles that did not exist
// before are removed again (values replaced for existing titles cannot be
// restored).
func RestoreSecrets(entries []Restore) error {
	for _, e := range entries {
		if err := validateMeta(e.Meta); err != nil {
			return err
		}
	}
	existing, err := loadRecords()
	if err != nil {
		return err
	}
	recs := append([]record(nil), existing...)

	var created []string // IDs of titles that did not exist before
	rollback := func() {
		for _, id := range created {
			_ = keyring.Delete(ServiceName, id)
		}
	}

	for _, e := range entries {
		i := recordIndex(recs, e.Meta.Title)
		isNew := i < 0
		var id string
		if isNew {
			if id, err = newID(); err != nil {
				rollback()
				return err
			}
		} else {
			id = recs[i].ID
		}
		if e.Value != nil {
			sealed, err := sealValue(id, *e.Value)
			if err != nil {
				rollback()
				return err
			}
			if err := keyring.Set(ServiceName, id, sealed); err != nil {
				rollback()
				return err
			}
			if isNew {
				created = append(created, id)
			}
		}
		if isNew {
			recs = append(recs, record{ID: id, SecretMeta: e.Meta})
		} else {
			recs[i].SecretMeta = e.Meta
		}
	}
	if err := saveRecords(recs); err != nil {
		rollback()
		return err
	}
	return nil
}
