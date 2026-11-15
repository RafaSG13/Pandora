package store

import (
	"errors"
	"reflect"
	"testing"

	"github.com/zalando/go-keyring"
)

var errMock = errors.New("keychain locked")

func TestMissingValuesReportsOnlyEntriesWithoutValue(t *testing.T) {
	setup(t)
	mustSave(t, "ok", "X", "v1")
	mustSave(t, "gone", "X", "v2")
	mustSave(t, "also-gone", "X", "v3")
	for _, title := range []string{"gone", "also-gone"} {
		if err := keyring.Delete(ServiceName, idOf(t, title)); err != nil {
			t.Fatal(err)
		}
	}

	got, err := MissingValues()
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"gone", "also-gone"}; !reflect.DeepEqual(got, want) {
		t.Errorf("MissingValues = %v, want %v", got, want)
	}

	// DeleteSecret clears an orphan even though its value is already gone.
	if err := DeleteSecret("gone"); err != nil {
		t.Fatal(err)
	}
	got, _ = MissingValues()
	if want := []string{"also-gone"}; !reflect.DeepEqual(got, want) {
		t.Errorf("after cleanup MissingValues = %v, want %v", got, want)
	}
}

func TestMissingValuesEmptyWhenConsistent(t *testing.T) {
	setup(t)
	if got, err := MissingValues(); err != nil || len(got) != 0 {
		t.Fatalf("empty store: got %v, %v", got, err)
	}
	mustSave(t, "a", "X", "v")
	if got, err := MissingValues(); err != nil || len(got) != 0 {
		t.Fatalf("consistent store: got %v, %v", got, err)
	}
}

func TestMissingValuesPropagatesKeyringErrors(t *testing.T) {
	setup(t)
	mustSave(t, "a", "X", "v")
	keyring.MockInitWithError(errMock)
	defer keyring.MockInit()

	got, err := MissingValues()
	if err == nil {
		t.Fatalf("got %v, nil; want the keyring error, not a false 'missing'", got)
	}
}

func TestRestoreSecretsKeepsDatesAndReplacesInOneWrite(t *testing.T) {
	setup(t)
	mustSave(t, "keep", "X", "old-keep")
	mustSave(t, "replace", "X", "old-replace")

	newVal, metaOnly := "new-replace", SecretMeta{Title: "meta-only", Category: "C", Created: "2020-05-06"}
	err := RestoreSecrets([]Restore{
		{Meta: SecretMeta{Title: "replace", Category: "Y", Created: "2019-01-01", Expires: "2030-01-01", Notes: "n"}, Value: &newVal},
		{Meta: metaOnly},
	})
	if err != nil {
		t.Fatal(err)
	}

	items, _ := LoadMetadata()
	if got := titles(t); !reflect.DeepEqual(got, []string{"keep", "replace", "meta-only"}) {
		t.Fatalf("titles = %v", got)
	}
	if r := items[1]; r.Category != "Y" || r.Created != "2019-01-01" || r.Expires != "2030-01-01" || r.Notes != "n" || r.Updated != "" {
		t.Errorf("replaced meta = %+v, want imported fields verbatim", r)
	}
	if v, _ := GetSecretValue("replace"); v != "new-replace" {
		t.Errorf("replace value = %q", v)
	}
	if v, _ := GetSecretValue("keep"); v != "old-keep" {
		t.Errorf("untouched value = %q", v)
	}
	if _, err := GetSecretValue("meta-only"); err == nil {
		t.Error("metadata-only entry must not get a keyring value")
	}
}
