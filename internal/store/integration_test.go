//go:build integration

package store

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

// TestRealKeyring exercises the store against the operating system's real
// keyring (Secret Service on Linux). It is excluded from normal builds and runs
// in CI (see .github/workflows/ci.yml) inside a throwaway D-Bus session.
//
// It writes to the real keyring under ServiceName, so it only runs when
// PANDORA_INTEGRATION=1 is set explicitly; never set that on a machine whose
// keychain you care about unless you accept a few temporary entries.
func TestRealKeyring(t *testing.T) {
	if os.Getenv("PANDORA_INTEGRATION") != "1" {
		t.Skip("set PANDORA_INTEGRATION=1 to run against the real keyring")
	}
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	t.Setenv("AppData", dir)

	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatal(err)
	}
	title := "pandora-itest-" + hex.EncodeToString(b[:])
	renamed := title + "-renamed"
	t.Cleanup(func() {
		_ = DeleteSecret(title)
		_ = DeleteSecret(renamed)
	})

	if err := SaveSecret(SecretMeta{Title: title, Category: "TEST", Created: "2026-01-02"}, "v1\nline2"); err != nil {
		t.Fatalf("SaveSecret: %v", err)
	}
	if got, err := GetSecretValue(title); err != nil || got != "v1\nline2" {
		t.Fatalf("GetSecretValue = %q, %v", got, err)
	}

	next := "v2"
	if err := UpdateSecret(title, SecretMeta{Title: renamed, Category: "TEST"}, &next); err != nil {
		t.Fatalf("UpdateSecret rename: %v", err)
	}
	if got, err := GetSecretValue(renamed); err != nil || got != next {
		t.Fatalf("after rename GetSecretValue = %q, %v", got, err)
	}
	if _, err := GetSecretValue(title); !IsValueMissing(err) {
		t.Fatalf("old entry still readable after rename: %v", err)
	}

	if err := DeleteSecret(renamed); err != nil {
		t.Fatalf("DeleteSecret: %v", err)
	}
	if _, err := GetSecretValue(renamed); !IsValueMissing(err) {
		t.Fatalf("entry still readable after delete: %v", err)
	}
}
