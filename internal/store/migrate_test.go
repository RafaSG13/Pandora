package store

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

// setupLegacy prepares a profile in the old plain-text format: metadata.json
// plus values in the keyring under the titles. No vault exists yet.
func setupLegacy(t *testing.T, metadata string, values map[string]string) (metaFile string) {
	t.Helper()
	keyring.MockInit()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	t.Setenv("AppData", dir)
	FastKDFForTests()
	t.Cleanup(Lock)
	Lock()

	metaFile, _ = legacyPath()
	if err := os.MkdirAll(filepath.Dir(metaFile), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(metaFile, []byte(metadata), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(metaFile+".bak", []byte(metadata), 0600); err != nil {
		t.Fatal(err)
	}
	for title, v := range values {
		if err := keyring.Set(ServiceName, title, v); err != nil {
			t.Fatal(err)
		}
	}
	return metaFile
}

const legacyJSON = `[
 {"title":"Pypi Key","category":"Dev","created":"2025-01-02"},
 {"title":"orphan","category":"X","created":"2025-02-03","notes":"no value"},
 {"title":"bad\u001b[2J","category":"X","created":"2025-02-03"},
 {"title":"Pypi Key","category":"Dev2","created":"2025-03-04"}
]`

func TestMigrateLegacy(t *testing.T) {
	metaFile := setupLegacy(t, legacyJSON, map[string]string{"Pypi Key": "pypi-value", "bad\x1b[2J": "bad-value"})
	if !LegacyExists() || Exists() {
		t.Fatal("setup: want legacy only")
	}

	if err := MigrateLegacy(testPassword); err != nil {
		t.Fatal(err)
	}
	if !IsUnlocked() || !Exists() {
		t.Error("want an unlocked vault after migration")
	}
	if got := titles(t); !reflect.DeepEqual(got, []string{"Pypi Key", "orphan", "bad\ufffd[2J"}) {
		t.Fatalf("titles = %q", got)
	}
	items, _ := LoadMetadata()
	if items[0].Category != "Dev2" {
		t.Errorf("duplicate not collapsed to the last entry: %+v", items[0])
	}
	if v, err := GetSecretValue("Pypi Key"); err != nil || v != "pypi-value" {
		t.Errorf("Pypi Key = %q, %v", v, err)
	}
	if v, err := GetSecretValue("bad\ufffd[2J"); err != nil || v != "bad-value" {
		t.Errorf("sanitised title value = %q, %v", v, err)
	}
	for _, old := range []string{"Pypi Key", "bad\x1b[2J"} {
		if _, err := keyring.Get(ServiceName, old); !errors.Is(err, keyring.ErrNotFound) {
			t.Errorf("old keyring account %q survived: err = %v", old, err)
		}
	}
	for _, f := range []string{metaFile, metaFile + ".bak"} {
		if _, err := os.Stat(f); !os.IsNotExist(err) {
			t.Errorf("%s still exists: %v", filepath.Base(f), err)
		}
	}
	if missing, err := MissingValues(); err != nil || !reflect.DeepEqual(missing, []string{"orphan"}) {
		t.Errorf("MissingValues = %v, %v; want [orphan]", missing, err)
	}
	if LegacyExists() {
		t.Error("LegacyExists after migration")
	}
}

func TestMigrateLegacyRejectsShortPasswordAndTouchesNothing(t *testing.T) {
	metaFile := setupLegacy(t, legacyJSON, map[string]string{"Pypi Key": "v"})
	if err := MigrateLegacy("short"); !errors.Is(err, ErrPasswordTooShort) {
		t.Fatalf("err = %v", err)
	}
	if Exists() || !LegacyExists() {
		t.Error("state changed by a rejected migration")
	}
	if v, _ := keyring.Get(ServiceName, "Pypi Key"); v != "v" {
		t.Errorf("legacy value changed: %q", v)
	}
	_ = metaFile
}

func TestMigrateLegacyKeyringFailureLeavesLegacyIntact(t *testing.T) {
	metaFile := setupLegacy(t, legacyJSON, map[string]string{"Pypi Key": "v"})
	keyring.MockInitWithError(errors.New("keychain locked"))

	if err := MigrateLegacy(testPassword); err == nil || !strings.Contains(err.Error(), "keychain locked") {
		t.Fatalf("err = %v, want the keyring error", err)
	}
	keyring.MockInit()
	if Exists() {
		t.Error("vault written despite failure")
	}
	if _, err := os.Stat(metaFile); err != nil {
		t.Errorf("metadata.json lost: %v", err)
	}
}

func TestMigrateLegacyWriteFailureLeavesLegacyIntact(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs POSIX permissions and a non-root user")
	}
	metaFile := setupLegacy(t, legacyJSON, map[string]string{"Pypi Key": "pypi-value"})
	dir := filepath.Dir(metaFile)
	if err := os.Chmod(dir, 0500); err != nil { // vault.json cannot be created
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0700)

	if err := MigrateLegacy(testPassword); err == nil {
		t.Fatal("MigrateLegacy succeeded although the vault could not be written")
	}
	if Exists() || IsUnlocked() {
		t.Error("vault or session left behind")
	}
	if _, err := os.Stat(metaFile); err != nil {
		t.Errorf("metadata.json lost: %v", err)
	}
	if v, err := keyring.Get(ServiceName, "Pypi Key"); err != nil || v != "pypi-value" {
		t.Errorf("legacy keyring value = %q, %v", v, err)
	}
}

func TestMigrateLegacyRequiresLegacyAndNoVault(t *testing.T) {
	setup(t) // vault exists, no legacy
	if err := MigrateLegacy(testPassword); err == nil {
		t.Error("MigrateLegacy succeeded without legacy metadata")
	}
}
