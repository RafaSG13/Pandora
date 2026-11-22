package store

// DropEntriesForTests deletes the keyring entries of a secret's current value
// (back 0) or of its back-th previous value (back 1 is the newest), leaving the
// vault untouched, as a keyring reset or a system tool would. Tests only: never
// call it from production code.
func DropEntriesForTests(title string, back int) error {
	recs, err := loadRecords()
	if err != nil {
		return err
	}
	i := recordIndex(recs, title)
	if i < 0 {
		return ErrNotFound
	}
	if back == 0 {
		return dropValue(recs[i].ID, recs[i].chunkCount())
	}
	if back < 0 || back > len(recs[i].History) {
		return ErrNoSuchVersion
	}
	s := recs[i].History[back-1].stored()
	return dropValue(s.id, s.n)
}
