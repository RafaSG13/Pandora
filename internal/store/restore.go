package store

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
	for _, e := range entries {
		if err := validateMeta(e.Meta); err != nil {
			return err
		}
		if e.Value != nil {
			if err := checkValueSize(*e.Value); err != nil {
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

	var created, replaced []storedValue
	rollback := func() {
		for _, s := range created {
			_ = dropValue(s.id, s.n)
		}
	}

	for _, e := range entries {
		i := recordIndex(recs, e.Meta.Title)
		id, n := "", 0
		if e.Value != nil {
			if id, err = newID(); err != nil {
				rollback()
				return err
			}
			if n, err = putValue(id, *e.Value); err != nil {
				rollback()
				return err
			}
			created = append(created, storedValue{id, n})
		}
		switch {
		case i < 0 && e.Value == nil:
			if id, err = newID(); err != nil {
				rollback()
				return err
			}
			recs = append(recs, record{ID: id, SecretMeta: e.Meta})
		case i < 0:
			recs = append(recs, record{ID: id, Chunks: chunksField(n), SecretMeta: e.Meta})
		case e.Value == nil:
			recs[i].SecretMeta = e.Meta
		default:
			replaced = append(replaced, storedValue{recs[i].ID, recs[i].chunkCount()})
			recs[i] = record{ID: id, Chunks: chunksField(n), SecretMeta: e.Meta}
		}
	}
	if err := saveRecords(recs); err != nil {
		rollback()
		return err
	}
	for _, s := range replaced {
		_ = dropValue(s.id, s.n) // best effort: the vault no longer refers to them
	}
	return nil
}
