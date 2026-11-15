package store

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/zalando/go-keyring"
)

// setup isolates the keyring (in-memory mock) and the config directory.
func setup(t *testing.T) {
	t.Helper()
	keyring.MockInit()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	t.Setenv("AppData", dir)
}

func mustSave(t *testing.T, title, category, value string) {
	t.Helper()
	if err := SaveSecret(SecretMeta{Title: title, Category: category, Created: "2026-01-02"}, value); err != nil {
		t.Fatalf("SaveSecret(%q): %v", title, err)
	}
}

func titles(t *testing.T) []string {
	t.Helper()
	items, err := LoadMetadata()
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, len(items))
	for i, m := range items {
		out[i] = m.Title
	}
	return out
}

func TestSaveRejectsDuplicateAndKeepsOriginalValue(t *testing.T) {
	setup(t)
	mustSave(t, "a", "X", "one")

	err := SaveSecret(SecretMeta{Title: "a", Category: "Y"}, "two")
	if !errors.Is(err, ErrExists) {
		t.Fatalf("err = %v, want ErrExists", err)
	}
	if v, _ := GetSecretValue("a"); v != "one" {
		t.Errorf("value = %q, want original %q", v, "one")
	}
	if got := titles(t); len(got) != 1 {
		t.Errorf("titles = %v, want one entry", got)
	}
}

func TestDelete(t *testing.T) {
	setup(t)
	mustSave(t, "a", "X", "one")
	mustSave(t, "b", "X", "two")

	if err := DeleteSecret("a"); err != nil {
		t.Fatal(err)
	}
	if got := titles(t); len(got) != 1 || got[0] != "b" {
		t.Errorf("titles = %v, want [b]", got)
	}
	if _, err := GetSecretValue("a"); !errors.Is(err, keyring.ErrNotFound) {
		t.Errorf("keyring value for a still present: err = %v", err)
	}
	if err := DeleteSecret("a"); !errors.Is(err, ErrNotFound) {
		t.Errorf("second delete err = %v, want ErrNotFound", err)
	}
}

func TestDeleteWhenKeyringValueAlreadyGone(t *testing.T) {
	setup(t)
	mustSave(t, "a", "X", "one")
	_ = keyring.Delete(ServiceName, "a")

	if err := DeleteSecret("a"); err != nil {
		t.Fatalf("err = %v, want metadata cleanup to succeed", err)
	}
	if got := titles(t); len(got) != 0 {
		t.Errorf("titles = %v, want empty", got)
	}
}

func TestUpdateCategoryOnlyKeepsValueAndCreated(t *testing.T) {
	setup(t)
	mustSave(t, "a", "X", "one")

	if err := UpdateSecret("a", SecretMeta{Title: "a", Category: "Y", Created: "1999-01-01"}, nil); err != nil {
		t.Fatal(err)
	}
	items, _ := LoadMetadata()
	if items[0].Category != "Y" || items[0].Created != "2026-01-02" {
		t.Errorf("meta = %+v, want category Y and original created date", items[0])
	}
	if v, _ := GetSecretValue("a"); v != "one" {
		t.Errorf("value = %q, want one", v)
	}
}

func TestUpdateNewValue(t *testing.T) {
	setup(t)
	mustSave(t, "a", "X", "one")

	nv := "two"
	if err := UpdateSecret("a", SecretMeta{Title: "a", Category: "X"}, &nv); err != nil {
		t.Fatal(err)
	}
	if v, _ := GetSecretValue("a"); v != "two" {
		t.Errorf("value = %q, want two", v)
	}
}

func TestUpdateRenameMovesValueAndRemovesOldEntry(t *testing.T) {
	setup(t)
	mustSave(t, "a", "X", "one")
	mustSave(t, "z", "X", "last")

	if err := UpdateSecret("a", SecretMeta{Title: "b", Category: "X"}, nil); err != nil {
		t.Fatal(err)
	}
	if got := titles(t); len(got) != 2 || got[0] != "b" || got[1] != "z" {
		t.Errorf("titles = %v, want [b z] (position preserved)", got)
	}
	if v, _ := GetSecretValue("b"); v != "one" {
		t.Errorf("value under new name = %q, want one", v)
	}
	if _, err := GetSecretValue("a"); !errors.Is(err, keyring.ErrNotFound) {
		t.Errorf("old keyring entry still present: err = %v", err)
	}
}

func TestUpdateErrors(t *testing.T) {
	setup(t)
	mustSave(t, "a", "X", "one")
	mustSave(t, "b", "X", "two")

	if err := UpdateSecret("a", SecretMeta{Title: "b"}, nil); !errors.Is(err, ErrExists) {
		t.Errorf("rename onto existing: err = %v, want ErrExists", err)
	}
	if v, _ := GetSecretValue("b"); v != "two" {
		t.Errorf("existing value clobbered: %q", v)
	}
	if err := UpdateSecret("nope", SecretMeta{Title: "nope"}, nil); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing: err = %v, want ErrNotFound", err)
	}
}

func TestLoadCollapsesDuplicatesFromOlderFiles(t *testing.T) {
	setup(t)
	file, _ := metadataPath()
	if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
		t.Fatal(err)
	}
	old := `[{"title":"a","category":"X","created":"1"},{"title":"b","category":"X","created":"2"},{"title":"a","category":"Y","created":"3"}]`
	if err := os.WriteFile(file, []byte(old), 0600); err != nil {
		t.Fatal(err)
	}

	items, err := LoadMetadata()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].Title != "a" || items[0].Category != "Y" || items[1].Title != "b" {
		t.Errorf("items = %+v, want [a(Y) b]", items)
	}
}

func TestCorruptMetadataIsNotOverwritten(t *testing.T) {
	setup(t)
	file, _ := metadataPath()
	if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("{not json"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := SaveSecret(SecretMeta{Title: "a"}, "v"); err == nil {
		t.Fatal("SaveSecret succeeded on corrupt metadata")
	}
	data, _ := os.ReadFile(file)
	if string(data) != "{not json" {
		t.Errorf("corrupt file was modified: %q", data)
	}
}

func TestMetadataFileMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no POSIX file modes")
	}
	setup(t)
	mustSave(t, "a", "X", "one")
	file, _ := metadataPath()
	st, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0600 {
		t.Errorf("mode = %v, want 0600", st.Mode().Perm())
	}
}
