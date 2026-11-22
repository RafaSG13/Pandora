package store

import (
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/zalando/go-keyring"
)

// version is a previous value of a secret. Its keyring entries are the ones
// the value had when it was current, so nothing is copied or re-encrypted.
type version struct {
	ID       string `json:"id"`
	Chunks   int    `json:"chunks,omitempty"`
	Replaced string `json:"replaced,omitempty"` // YYYY-MM-DD the value stopped being current
}

func (v version) stored() storedValue { return storedValue{v.ID, max(1, v.Chunks)} }

// asRecord lets the version be read with the code that reads current values.
func (v version) asRecord() record { return record{ID: v.ID, Chunks: v.Chunks} }

func versionOf(s storedValue, replaced string) version {
	return version{ID: s.id, Chunks: chunksField(s.n), Replaced: replaced}
}

// ErrNoSuchVersion is returned when a secret has fewer previous values than asked for.
var ErrNoSuchVersion = errors.New("no such version")

var historyDepth atomic.Int32

// SetHistoryDepth sets how many previous values each secret keeps when its
// value is replaced; 0 keeps none. Callers set it once from the config file.
// Lowering it prunes a secret's history the next time that secret's value
// changes. The default is 0 (no history).
func SetHistoryDepth(n int) { historyDepth.Store(int32(max(0, n))) }

// currentValue reads r's value for comparison. readable says whether the value
// exists and decrypts, i.e. whether it can be kept as a previous version; a
// missing or corrupt value is not an error here, anything else is.
func currentValue(r record) (value string, readable bool, err error) {
	v, err := getValue(r)
	switch {
	case err == nil:
		return v, true, nil
	case errors.Is(err, keyring.ErrNotFound), errors.Is(err, errCorruptValue):
		return "", false, nil
	}
	return "", false, err
}

// replaceValue points r at a new value stored as id (n entries). The old value
// becomes the newest previous version when keepOld is set and history is
// enabled; versions beyond the depth are pruned. It returns the entries that
// are no longer referenced and must be deleted once the vault is written.
func (r *record) replaceValue(id string, n int, keepOld bool, today string) (unused []storedValue) {
	old := storedValue{r.ID, r.chunkCount()}
	r.ID, r.Chunks = id, chunksField(n)
	depth := int(historyDepth.Load())
	history := make([]version, 0, len(r.History)+1)
	if keepOld && depth > 0 {
		history = append(history, versionOf(old, today))
	} else {
		unused = append(unused, old)
	}
	history = append(history, r.History...)
	for len(history) > depth {
		unused = append(unused, history[len(history)-1].stored())
		history = history[:len(history)-1]
	}
	if len(history) == 0 {
		history = nil
	}
	r.History = history
	return unused
}

// dropStored deletes entries nothing refers to any more; best effort.
func dropStored(entries []storedValue) {
	for _, s := range entries {
		_ = dropValue(s.id, s.n)
	}
}

// VersionInfo describes one previous value. The value itself is never part of it.
type VersionInfo struct {
	Replaced string // YYYY-MM-DD the value stopped being current
}

// Versions lists the previous values of the secret titled title, newest
// first: element 0 is the one a restore of "back 1" brings back.
func Versions(title string) ([]VersionInfo, error) {
	recs, err := loadRecords()
	if err != nil {
		return nil, err
	}
	i := recordIndex(recs, title)
	if i < 0 {
		return nil, ErrNotFound
	}
	out := make([]VersionInfo, len(recs[i].History))
	for k, v := range recs[i].History {
		out[k] = VersionInfo{Replaced: v.Replaced}
	}
	return out, nil
}

// VersionCounts returns how many previous values each titled secret has;
// secrets without history are absent.
func VersionCounts() (map[string]int, error) {
	recs, err := loadRecords()
	if err != nil {
		return nil, err
	}
	counts := map[string]int{}
	for _, r := range recs {
		if len(r.History) > 0 {
			counts[r.Title] = len(r.History)
		}
	}
	return counts, nil
}

// versionRecord finds the record and checks that back (1 = newest previous
// value) exists.
func versionRecord(recs []record, title string, back int) (int, error) {
	i := recordIndex(recs, title)
	if i < 0 {
		return 0, ErrNotFound
	}
	if back < 1 || back > len(recs[i].History) {
		return 0, fmt.Errorf("%w: %q has %d previous values, asked for %d", ErrNoSuchVersion, title, len(recs[i].History), back)
	}
	return i, nil
}

// GetVersion returns the value that was current before the back-th most
// recent replacement (back 1 is the previous value). A value missing from the
// keyring returns keyring.ErrNotFound.
func GetVersion(title string, back int) (string, error) {
	recs, err := loadRecords()
	if err != nil {
		return "", err
	}
	i, err := versionRecord(recs, title, back)
	if err != nil {
		return "", err
	}
	return getValue(recs[i].History[back-1].asRecord())
}

// RestoreVersion makes a previous value current again. The value it replaces
// becomes the newest previous version, so the restore can itself be undone,
// and no keyring entry is written or removed: only the vault changes. It fails
// without changing anything when that version's value is missing.
func RestoreVersion(title string, back int) error {
	release, err := lockVault()
	if err != nil {
		return err
	}
	defer release()
	recs, err := loadRecords()
	if err != nil {
		return err
	}
	i, err := versionRecord(recs, title, back)
	if err != nil {
		return err
	}
	chosen := recs[i].History[back-1]
	ok, err := hasValue(chosen.asRecord())
	if err != nil {
		return err
	}
	if !ok {
		return keyring.ErrNotFound
	}

	today := time.Now().Format(DateLayout)
	cur := storedValue{recs[i].ID, recs[i].chunkCount()}
	history := make([]version, 0, len(recs[i].History))
	history = append(history, versionOf(cur, today))
	history = append(history, recs[i].History[:back-1]...)
	history = append(history, recs[i].History[back:]...)
	recs[i].ID, recs[i].Chunks = chosen.ID, chosen.Chunks
	recs[i].History = history
	recs[i].Updated = today
	return saveRecords(recs)
}

// MissingVersion names a previous value whose keyring entries are gone.
type MissingVersion struct {
	Title string
	Back  int // 1 is the newest previous value
}

// MissingVersions lists previous values missing from the keyring, like
// MissingValues does for current ones.
func MissingVersions() ([]MissingVersion, error) {
	recs, err := loadRecords()
	if err != nil {
		return nil, err
	}
	var missing []MissingVersion
	for _, r := range recs {
		for k, v := range r.History {
			ok, err := hasValue(v.asRecord())
			if err != nil {
				return nil, fmt.Errorf("reading a previous value of %q from the keyring: %w", r.Title, err)
			}
			if !ok {
				missing = append(missing, MissingVersion{r.Title, k + 1})
			}
		}
	}
	return missing, nil
}

// DropMissingVersions removes from the vault every previous value whose
// keyring entries are gone (and any entries of it that remain) and returns how
// many it removed.
func DropMissingVersions() (int, error) {
	release, err := lockVault()
	if err != nil {
		return 0, err
	}
	defer release()
	recs, err := loadRecords()
	if err != nil {
		return 0, err
	}
	var leftovers []storedValue
	removed := 0
	for i := range recs {
		var keep []version
		for _, v := range recs[i].History {
			ok, err := hasValue(v.asRecord())
			if err != nil {
				return 0, err
			}
			if ok {
				keep = append(keep, v)
				continue
			}
			leftovers = append(leftovers, v.stored())
			removed++
		}
		recs[i].History = keep
	}
	if removed == 0 {
		return 0, nil
	}
	if err := saveRecords(recs); err != nil {
		return 0, err
	}
	dropStored(leftovers)
	return removed, nil
}
