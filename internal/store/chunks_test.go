package store

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

// pemLike is a multiline value with CRLF-free lines and a trailing newline,
// about size bytes long, like a private key.
func pemLike(size int) string {
	var b strings.Builder
	b.WriteString("-----BEGIN PRIVATE KEY-----\n")
	for i := 0; b.Len() < size; i++ {
		b.WriteString(strings.Repeat(string(rune('A'+i%26)), 64) + "\n")
	}
	b.WriteString("-----END PRIVATE KEY-----\n")
	return b.String()
}

func recOf(t *testing.T, title string) record {
	t.Helper()
	recs, err := loadRecords()
	if err != nil {
		t.Fatal(err)
	}
	i := recordIndex(recs, title)
	if i < 0 {
		t.Fatalf("no record %q", title)
	}
	return recs[i]
}

// entryExists reports whether the keyring holds chunk k of r.
func entryExists(r record, k int) bool {
	_, err := keyring.Get(ServiceName, chunkAccount(r.ID, k))
	return err == nil
}

func readOnlyVaultDir(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs POSIX permissions and a non-root user")
	}
	p, _ := vaultPath()
	dir := filepath.Dir(p)
	if err := os.Chmod(dir, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0700) })
}

func TestMultilineValueRoundTripsAcrossSeveralKeyringEntries(t *testing.T) {
	setup(t)
	for name, value := range map[string]string{
		"key":      pemLike(9000),
		"crlf":     "line one\r\nline two\r\n\r\n",
		"trailing": "no newline at the end",
		"spaces":   "  leading and trailing  \n\n",
	} {
		mustSave(t, name, "K", value)
		got, err := GetSecretValue(name)
		if err != nil || got != value {
			t.Errorf("%s: got %q, %v; want the exact value back", name, got, err)
		}
	}

	r := recOf(t, "key")
	if r.chunkCount() < 5 {
		t.Fatalf("a 9 KB value used %d keyring entries, want it split", r.chunkCount())
	}
	for k := range r.chunkCount() {
		part, err := keyring.Get(ServiceName, chunkAccount(r.ID, k))
		if err != nil || len(part) > chunkChars {
			t.Errorf("entry %d: %d chars, %v; every entry must fit the OS limits", k, len(part), err)
		}
	}
	if entryExists(r, r.chunkCount()) {
		t.Error("an entry exists past the recorded count")
	}
	if recOf(t, "trailing").Chunks != 0 {
		t.Error("a short value must stay a single entry without a chunks field")
	}
}

func TestValueSizeLimit(t *testing.T) {
	setup(t)
	atLimit := strings.Repeat("x", MaxValueBytes)
	mustSave(t, "max", "K", atLimit)
	if got, err := GetSecretValue("max"); err != nil || got != atLimit {
		t.Fatalf("a value of exactly %d bytes must round-trip: %v", MaxValueBytes, err)
	}

	tooBig := atLimit + "x"
	if err := SaveSecret(SecretMeta{Title: "big"}, tooBig); !errors.Is(err, ErrValueTooLarge) {
		t.Errorf("SaveSecret: err = %v, want ErrValueTooLarge", err)
	}
	if err := UpdateSecret("max", SecretMeta{Title: "max"}, &tooBig); !errors.Is(err, ErrValueTooLarge) {
		t.Errorf("UpdateSecret: err = %v, want ErrValueTooLarge", err)
	}
	if got, _ := GetSecretValue("max"); got != atLimit {
		t.Error("a rejected update changed the stored value")
	}
	err := RestoreSecrets([]Restore{{Meta: SecretMeta{Title: "ok"}, Value: &atLimit}, {Meta: SecretMeta{Title: "huge"}, Value: &tooBig}})
	if !errors.Is(err, ErrValueTooLarge) {
		t.Errorf("RestoreSecrets: err = %v, want ErrValueTooLarge", err)
	}
	if got := titles(t); len(got) != 1 || got[0] != "max" {
		t.Errorf("titles = %v; a rejected batch must write nothing", got)
	}
}

func TestUpdatingAValueLeavesNoOldEntriesBehind(t *testing.T) {
	setup(t)
	mustSave(t, "key", "K", pemLike(9000))
	old := recOf(t, "key")

	small := "now short"
	if err := UpdateSecret("key", SecretMeta{Title: "key", Category: "K"}, &small); err != nil {
		t.Fatal(err)
	}
	if got, _ := GetSecretValue("key"); got != small {
		t.Fatalf("value = %q", got)
	}
	for k := range old.chunkCount() {
		if entryExists(old, k) {
			t.Errorf("old entry %d is still in the keyring", k)
		}
	}
	if r := recOf(t, "key"); r.ID == old.ID || r.Chunks != 0 {
		t.Errorf("record = %+v; want a fresh ID and a single entry", r)
	}

	big := pemLike(6000)
	if err := UpdateSecret("key", SecretMeta{Title: "key", Category: "K"}, &big); err != nil {
		t.Fatal(err)
	}
	if got, _ := GetSecretValue("key"); got != big {
		t.Error("growing a value back to several entries lost data")
	}
}

func TestFailedUpdateKeepsTheOldValue(t *testing.T) {
	setup(t)
	original := pemLike(5000)
	mustSave(t, "key", "K", original)
	readOnlyVaultDir(t) // the vault cannot be rewritten

	next := pemLike(7000)
	if err := UpdateSecret("key", SecretMeta{Title: "key", Category: "K"}, &next); err == nil {
		t.Fatal("UpdateSecret succeeded although the vault could not be written")
	}
	if got, err := GetSecretValue("key"); err != nil || got != original {
		t.Errorf("old value after a failed update: %v (equal: %v)", err, got == original)
	}
}

func TestDeleteRemovesEveryEntry(t *testing.T) {
	setup(t)
	mustSave(t, "key", "K", pemLike(9000))
	r := recOf(t, "key")
	if err := DeleteSecret("key"); err != nil {
		t.Fatal(err)
	}
	for k := range r.chunkCount() {
		if entryExists(r, k) {
			t.Errorf("entry %d survived the delete", k)
		}
	}
}

func TestMissingChunkIsAMissingValue(t *testing.T) {
	setup(t)
	mustSave(t, "key", "K", pemLike(9000))
	mustSave(t, "fine", "K", "v")
	r := recOf(t, "key")
	if err := keyring.Delete(ServiceName, chunkAccount(r.ID, 2)); err != nil {
		t.Fatal(err)
	}

	if _, err := GetSecretValue("key"); !IsValueMissing(err) {
		t.Errorf("err = %v, want a missing-value error", err)
	}
	missing, err := MissingValues()
	if err != nil || len(missing) != 1 || missing[0] != "key" {
		t.Errorf("MissingValues = %v, %v; want [key]", missing, err)
	}
	if err := DeleteSecret("key"); err != nil {
		t.Errorf("a half-missing secret must still be deletable: %v", err)
	}
}

func TestTamperedChunksAreRejected(t *testing.T) {
	setup(t)
	mustSave(t, "a", "K", pemLike(9000))
	mustSave(t, "b", "K", pemLike(9000))
	ra, rb := recOf(t, "a"), recOf(t, "b")
	get := func(r record, k int) string {
		v, err := keyring.Get(ServiceName, chunkAccount(r.ID, k))
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	put := func(r record, k int, v string) {
		if err := keyring.Set(ServiceName, chunkAccount(r.ID, k), v); err != nil {
			t.Fatal(err)
		}
	}

	// Reordered entries of one secret.
	c0, c1 := get(ra, 0), get(ra, 1)
	put(ra, 0, c1)
	put(ra, 1, c0)
	if v, err := GetSecretValue("a"); err == nil {
		t.Errorf("reordered entries accepted: %d bytes", len(v))
	}
	put(ra, 0, c0)
	put(ra, 1, c1)

	// A truncated last entry.
	last := ra.chunkCount() - 1
	full := get(ra, last)
	put(ra, last, full[:len(full)-4])
	if _, err := GetSecretValue("a"); err == nil {
		t.Error("truncated entry accepted")
	}
	put(ra, last, full)

	// An entry taken from another secret.
	put(ra, 1, get(rb, 1))
	if _, err := GetSecretValue("a"); err == nil {
		t.Error("entry from another secret accepted")
	}
	put(ra, 1, c1)
	if got, err := GetSecretValue("a"); err != nil || got != pemLike(9000) {
		t.Errorf("restoring the entries did not restore the value: %v", err)
	}
}

func TestRestoreReplacesLargeValuesCleanlyAndRollsBack(t *testing.T) {
	setup(t)
	mustSave(t, "key", "K", pemLike(9000))
	old := recOf(t, "key")

	next := pemLike(3000)
	if err := RestoreSecrets([]Restore{{Meta: SecretMeta{Title: "key", Category: "K"}, Value: &next}}); err != nil {
		t.Fatal(err)
	}
	if got, _ := GetSecretValue("key"); got != next {
		t.Error("restored value differs")
	}
	for k := range old.chunkCount() {
		if entryExists(old, k) {
			t.Errorf("replaced entry %d is still in the keyring", k)
		}
	}

	readOnlyVaultDir(t)
	other := "other"
	err := RestoreSecrets([]Restore{
		{Meta: SecretMeta{Title: "key", Category: "K"}, Value: &other},
		{Meta: SecretMeta{Title: "new"}, Value: &other},
	})
	if err == nil {
		t.Fatal("RestoreSecrets succeeded although the vault could not be written")
	}
	if got, _ := GetSecretValue("key"); got != next {
		t.Error("a failed batch changed an existing value")
	}
}

func TestMigrationCarriesLargeMultilineValues(t *testing.T) {
	big := pemLike(7000)
	metaFile := setupLegacy(t, `[{"title":"ssh","category":"K","created":"2025-01-01"}]`, map[string]string{"ssh": big})
	if err := MigrateLegacy(testPassword); err != nil {
		t.Fatal(err)
	}
	if got, err := GetSecretValue("ssh"); err != nil || got != big {
		t.Errorf("migrated value: %v (equal: %v)", err, got == big)
	}
	if recOf(t, "ssh").chunkCount() < 2 {
		t.Error("a 7 KB value was not split")
	}
	if _, err := keyring.Get(ServiceName, "ssh"); err == nil {
		t.Error("old keyring account survived")
	}
	_ = metaFile
}
