package store

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// Writers that load, modify and save the vault must not lose each other's
// changes: every concurrent add survives with a readable value.
func TestConcurrentWritersDoNotLoseSecrets(t *testing.T) {
	setup(t)
	const n = 24

	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- SaveSecret(SecretMeta{Title: fmt.Sprintf("s%02d", i), Created: "2026-01-02"}, fmt.Sprintf("v%02d", i))
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("SaveSecret: %v", err)
		}
	}

	if got := len(titles(t)); got != n {
		t.Fatalf("%d secrets survived, want %d", got, n)
	}
	for i := range n {
		v, err := GetSecretValue(fmt.Sprintf("s%02d", i))
		if err != nil || v != fmt.Sprintf("v%02d", i) {
			t.Errorf("s%02d = %q, %v", i, v, err)
		}
	}
}

// A writer that cannot get the lock in time fails with ErrBusy and changes
// nothing; once the holder lets go, writes work again.
func TestWriterGivesUpWhileAnotherHoldsTheLock(t *testing.T) {
	setup(t)
	mustSave(t, "kept", "", "v")

	old := lockTimeout
	lockTimeout = 150 * time.Millisecond
	t.Cleanup(func() { lockTimeout = old })

	release, err := lockVault()
	if err != nil {
		t.Fatal(err)
	}
	err = SaveSecret(SecretMeta{Title: "blocked"}, "x")
	if !errors.Is(err, ErrBusy) {
		t.Fatalf("err = %v, want ErrBusy", err)
	}
	if err := DeleteSecret("kept"); !errors.Is(err, ErrBusy) {
		t.Fatalf("delete err = %v, want ErrBusy", err)
	}
	release()

	if got := titles(t); len(got) != 1 || got[0] != "kept" {
		t.Fatalf("titles = %v, want [kept]", got)
	}
	if err := SaveSecret(SecretMeta{Title: "after"}, "x"); err != nil {
		t.Fatalf("after release: %v", err)
	}
}

// Failing validations and rejected writes must not leave the lock held.
func TestLockIsReleasedAfterFailedWrites(t *testing.T) {
	setup(t)
	mustSave(t, "a", "", "v")

	old := lockTimeout
	lockTimeout = 150 * time.Millisecond
	t.Cleanup(func() { lockTimeout = old })

	if err := SaveSecret(SecretMeta{Title: "a"}, "dup"); !errors.Is(err, ErrExists) {
		t.Fatalf("err = %v, want ErrExists", err)
	}
	if err := DeleteSecret("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if err := UpdateSecret("nope", SecretMeta{Title: "x"}, nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if err := SaveSecret(SecretMeta{Title: "b"}, "ok"); err != nil {
		t.Fatalf("a later write was blocked: %v", err)
	}
}

// Reads never take the lock, so a long write cannot freeze listing.
func TestReadsDoNotWaitForTheLock(t *testing.T) {
	setup(t)
	mustSave(t, "a", "", "v")

	release, err := lockVault()
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	if got := titles(t); len(got) != 1 {
		t.Fatalf("titles = %v", got)
	}
	if v, err := GetSecretValue("a"); err != nil || v != "v" {
		t.Fatalf("value = %q, %v", v, err)
	}
}

func TestStampChangesOnlyWhenTheVaultIsWritten(t *testing.T) {
	setup(t)
	mustSave(t, "a", "", "v")

	before, err := VaultStamp()
	if err != nil {
		t.Fatal(err)
	}
	_ = titles(t)
	if _, err := GetSecretValue("a"); err != nil {
		t.Fatal(err)
	}
	if again, _ := VaultStamp(); again != before {
		t.Fatal("reading changed the stamp")
	}

	mustSave(t, "b", "", "w")
	after, err := VaultStamp()
	if err != nil {
		t.Fatal(err)
	}
	if after == before {
		t.Fatal("writing did not change the stamp")
	}
}
