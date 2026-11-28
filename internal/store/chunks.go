package store

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/zalando/go-keyring"
)

const (
	// MaxValueBytes is the largest secret value Pandora stores.
	MaxValueBytes = 32 * 1024

	// chunkChars is the size of one keyring entry. The OS keyrings cap what
	// they accept per entry (Windows Credential Manager 2560 bytes; macOS about
	// 4 KB per `security` command once go-keyring base64-encodes the value), so
	// a sealed value is split across entries well below both.
	chunkChars = 2000
)

// ErrValueTooLarge is returned when a value exceeds MaxValueBytes.
var ErrValueTooLarge = fmt.Errorf("value is larger than %d KB", MaxValueBytes/1024)

func checkValueSize(value string) error {
	if len(value) > MaxValueBytes {
		return ErrValueTooLarge
	}
	return nil
}

// chunkAccount is the keyring account of chunk k of the value of record id:
// the id itself for the first chunk, "<id>.<k>" for the others.
func chunkAccount(id string, k int) string {
	if k == 0 {
		return id
	}
	return id + "." + strconv.Itoa(k)
}

// chunkCount is how many keyring entries hold the record's value. Records
// written before multi-chunk values have no Chunks field and use one.
func (r record) chunkCount() int { return max(1, r.Chunks) }

// storedValue identifies the keyring entries of one value.
type storedValue struct {
	id string
	n  int
}

// putValue encrypts value under the data key and stores it in keyring entries
// for record id. It returns the number of entries; on failure nothing it
// wrote is left behind.
func putValue(id, value string) (int, error) {
	sealed, err := sealValue(id, value)
	if err != nil {
		return 0, err
	}
	return putSealed(id, sealed)
}

// putSealed splits an already sealed (ASCII) value into entries.
func putSealed(id, sealed string) (int, error) {
	n := max(1, (len(sealed)+chunkChars-1)/chunkChars)
	for k := range n {
		part := sealed[k*chunkChars : min(len(sealed), (k+1)*chunkChars)]
		if err := keyring.Set(ServiceName, chunkAccount(id, k), part); err != nil {
			_ = dropValue(id, k)
			return 0, err
		}
	}
	return n, nil
}

// chunksField is the Chunks value to record for n entries (omitted for one).
func chunksField(n int) int {
	if n > 1 {
		return n
	}
	return 0
}

// getValue reads and decrypts the value of r. A missing entry returns
// keyring.ErrNotFound.
func getValue(r record) (string, error) {
	sealed, err := readSealed(r.ID, r.chunkCount())
	if err != nil {
		return "", err
	}
	return openValue(r.ID, sealed)
}

// readSealed concatenates the n keyring entries of record id without
// decrypting them. A missing entry returns keyring.ErrNotFound.
func readSealed(id string, n int) (string, error) {
	var sealed strings.Builder
	for k := range n {
		part, err := keyring.Get(ServiceName, chunkAccount(id, k))
		if err != nil {
			return "", err
		}
		sealed.WriteString(part)
	}
	return sealed.String(), nil
}

// hasValue reports whether every keyring entry of r exists. Errors other than
// "not found" are returned.
func hasValue(r record) (bool, error) {
	for k := range r.chunkCount() {
		if _, err := keyring.Get(ServiceName, chunkAccount(r.ID, k)); err != nil {
			if errors.Is(err, keyring.ErrNotFound) {
				return false, nil
			}
			return false, err
		}
	}
	return true, nil
}

// dropValue deletes the first n keyring entries of record id. Entries that are
// already gone are fine; it keeps going after an error and returns the first
// one that was not "not found".
func dropValue(id string, n int) error {
	var first error
	for k := range n {
		if err := keyring.Delete(ServiceName, chunkAccount(id, k)); err != nil && !errors.Is(err, keyring.ErrNotFound) && first == nil {
			first = err
		}
	}
	return first
}
