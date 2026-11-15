package store

import (
	"errors"
	"os"
	"path/filepath"
	"time"
)

// ErrBusy is returned when another Pandora process holds the vault write lock
// for longer than lockTimeout.
var ErrBusy = errors.New("vault is busy: another Pandora process is writing to it")

// lockTimeout bounds the wait for the write lock; lockPoll is the retry step.
// Writers hold the lock for a few keyring calls, so a wait this long means a
// stuck process. Tests shorten the timeout.
var (
	lockTimeout = 10 * time.Second
	lockPoll    = 20 * time.Millisecond
)

// lockVault takes the exclusive advisory lock that serialises every
// load-modify-write cycle on the vault (and the keyring writes that go with
// it), across goroutines and across processes. The lock is an OS file lock
// (flock / LockFileEx) on vault.lock next to the vault, so a crashed process
// releases it automatically; the file itself is never removed. Readers do not
// lock: the vault file is replaced atomically, so they always see a complete
// version.
//
// The lock is not re-entrant: callers must not call another locking function
// while holding it.
func lockVault() (release func(), err error) {
	p, err := vaultPath()
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, "vault.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(lockTimeout)
	for {
		ok, err := tryLock(f)
		if err != nil {
			f.Close()
			return nil, err
		}
		if ok {
			return func() { unlockFile(f); f.Close() }, nil
		}
		if !time.Now().Before(deadline) {
			f.Close()
			return nil, ErrBusy
		}
		time.Sleep(lockPoll)
	}
}

// Stamp identifies a version of the vault file. Two stamps are equal when the
// file's modification time and size are the same.
type Stamp struct{ mod, size int64 }

// VaultStamp returns the stamp of vault.json. A front end can poll it cheaply
// to notice that another process changed the vault. Coarse file-system
// timestamps could hide a same-size change made within one tick; reloading
// explicitly always works.
func VaultStamp() (Stamp, error) {
	p, err := vaultPath()
	if err != nil {
		return Stamp{}, err
	}
	fi, err := os.Stat(p)
	if err != nil {
		return Stamp{}, err
	}
	return Stamp{fi.ModTime().UnixNano(), fi.Size()}, nil
}
