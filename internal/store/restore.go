package store

import (
	"slices"
	"time"
)

// Restore is one secret to bring back with RestoreSecrets. A nil Value leaves
// the keyring untouched for that title.
type Restore struct {
	Meta  SecretMeta
	Value *string
}

// RestoreSecrets records every entry's metadata exactly as given (replacing
// any entry with the same title and keeping its dates) and stores the values
// that are present. The vault is written once, so vault.json.bak holds the
// state from before the whole batch. A value is written under a fresh ID and
// the entries it replaces are removed only after the vault points at the new
// ones; if a keyring write or the vault write fails, everything stored by the
// batch is removed again and nothing that existed is touched.
func RestoreSecrets(entries []Restore) error {
	entries = slices.Clone(entries) // prepareMeta normalises Tags in place
	for i := range entries {
		if err := prepareMeta(&entries[i].Meta); err != nil {
			return err
		}
		if v := entries[i].Value; v != nil {
			if err := checkValueSize(*v); err != nil {
				return err
			}
		}
	}
	release, err := lockVault()
	if err != nil {
		return err
	}
	defer release()
	existing, err := loadRecords()
	if err != nil {
		return err
	}
	recs := append([]record(nil), existing...)
	today := time.Now().Format(DateLayout)

	var created, unused []storedValue
	rollback := func() {
		for _, s := range created {
			_ = dropValue(s.id, s.n)
		}
	}

	for _, e := range entries {
		i := recordIndex(recs, e.Meta.Title)
		value, keepOld := e.Value, false
		if value != nil && i >= 0 {
			cur, readable, err := currentValue(recs[i])
			if err != nil {
				rollback()
				return err
			}
			if readable && cur == *value {
				value = nil // the value did not change: keep it, and its history
			} else {
				keepOld = readable
			}
		}
		id, n := "", 0
		if value != nil {
			if id, err = newID(); err != nil {
				rollback()
				return err
			}
			if n, err = putValue(id, *value); err != nil {
				rollback()
				return err
			}
			created = append(created, storedValue{id, n})
		}
		switch {
		case i < 0 && value == nil:
			if id, err = newID(); err != nil {
				rollback()
				return err
			}
			recs = append(recs, record{ID: id, SecretMeta: e.Meta})
		case i < 0:
			recs = append(recs, record{ID: id, Chunks: chunksField(n), SecretMeta: e.Meta})
		case value == nil:
			recs[i].SecretMeta = e.Meta
		default:
			rec := recs[i]
			unused = append(unused, rec.replaceValue(id, n, keepOld, today)...)
			rec.SecretMeta = e.Meta
			recs[i] = rec
		}
	}
	if err := saveRecords(recs); err != nil {
		rollback()
		return err
	}
	dropStored(unused) // best effort: the vault no longer refers to them
	return nil
}
