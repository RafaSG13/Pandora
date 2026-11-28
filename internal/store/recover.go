package store

import (
	"errors"
	"fmt"
	"os"
	"time"
)

// FileHealth is what InspectVault found in one vault file.
type FileHealth int

const (
	// FileOK means the file parses, the password opens it and its records decrypt.
	FileOK FileHealth = iota
	// FileMissing means the file does not exist.
	FileMissing
	// FileCorrupt means the file is unreadable as a vault, has invalid
	// parameters, or its data does not decrypt or parse.
	FileCorrupt
	// FileNewer means it was written by a newer Pandora than this build.
	FileNewer
	// FileWrongPassword means the file is a vault the password does not open
	// (a wrong password and a tampered key are indistinguishable).
	FileWrongPassword
)

// FileReport describes one vault file.
type FileReport struct {
	Health   FileHealth
	Entries  int       // number of secrets, FileOK only
	Modified time.Time // file modification time, zero when missing
	Err      error     // why the file is not FileOK; never contains secret data
}

// VaultReport describes vault.json and its backup vault.json.bak.
type VaultReport struct {
	Vault, Backup FileReport
}

// Errors returned by RestoreVaultBackup besides ErrWrongPassword.
var (
	// ErrRestoreNotNeeded means vault.json is healthy, so nothing was touched.
	ErrRestoreNotNeeded = errors.New("vault.json is not damaged; nothing to restore")
	// ErrNoUsableBackup means vault.json.bak is missing or damaged.
	ErrNoUsableBackup = errors.New("there is no usable vault.json.bak")
	// ErrRestoreRefused means vault.json is not damaged in a way a backup can
	// fix: it was written by a newer version, or the password does not open it.
	ErrRestoreRefused = errors.New("vault.json is not damaged in a way the backup can repair")
)

// BackupExists reports whether vault.json.bak is present.
func BackupExists() bool {
	p, err := vaultPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(p + ".bak")
	return err == nil
}

// InspectVault checks vault.json and vault.json.bak against password without
// needing an unlocked session and without changing anything. Argon2 runs once
// per file that parses. The error is only for I/O problems other than a
// missing file.
func InspectVault(password string) (VaultReport, error) {
	p, err := vaultPath()
	if err != nil {
		return VaultReport{}, err
	}
	var rep VaultReport
	if rep.Vault, err = inspectFile(p, password); err != nil {
		return VaultReport{}, err
	}
	if rep.Backup, err = inspectFile(p+".bak", password); err != nil {
		return VaultReport{}, err
	}
	return rep, nil
}

func inspectFile(path, password string) (FileReport, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return FileReport{Health: FileMissing, Err: err}, nil
	} else if err != nil {
		return FileReport{}, err
	}
	rep := inspectBytes(raw, password)
	if info, err := os.Stat(path); err == nil {
		rep.Modified = info.ModTime()
	}
	return rep, nil
}

// inspectBytes classifies the content of a vault file.
func inspectBytes(raw []byte, password string) FileReport {
	vf, err := parseVault(raw)
	switch {
	case errors.Is(err, errNewerVault):
		return FileReport{Health: FileNewer, Err: err}
	case err != nil:
		return FileReport{Health: FileCorrupt, Err: err}
	}
	key, err := vf.unwrap(password)
	if err != nil {
		return FileReport{Health: FileWrongPassword, Err: err}
	}
	defer zero(key)
	recs, err := vf.records(key)
	if err != nil {
		return FileReport{Health: FileCorrupt, Err: err}
	}
	if _, err := vf.sharing(key); err != nil {
		return FileReport{Health: FileCorrupt, Err: err}
	}
	return FileReport{Health: FileOK, Entries: len(recs)}
}

// RestoreVaultBackup replaces a damaged or missing vault.json with
// vault.json.bak. It only acts when the password opens the backup, the backup
// is fully readable, and vault.json is missing or corrupt (not merely locked
// by a different password, and not written by a newer version): a wrong
// password never triggers a restore and a healthy vault is never replaced.
// The damaged file is kept as vault.json.corrupt, and vault.json.bak is left
// as it was. Changes made after the backup was taken are lost; values written
// by those changes stay in the keyring unreferenced.
func RestoreVaultBackup(password string) error {
	release, err := lockVault()
	if err != nil {
		return err
	}
	defer release()
	p, err := vaultPath()
	if err != nil {
		return err
	}
	bak, err := os.ReadFile(p + ".bak")
	if os.IsNotExist(err) {
		return fmt.Errorf("%w: it does not exist", ErrNoUsableBackup)
	} else if err != nil {
		return err
	}
	cur, err := os.ReadFile(p)
	hasCurrent := err == nil
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	switch b := inspectBytes(bak, password); b.Health {
	case FileOK:
	case FileWrongPassword:
		return ErrWrongPassword
	default:
		return fmt.Errorf("%w: %v", ErrNoUsableBackup, b.Err)
	}
	if hasCurrent {
		switch v := inspectBytes(cur, password); v.Health {
		case FileOK:
			return ErrRestoreNotNeeded
		case FileCorrupt:
		default:
			return fmt.Errorf("%w: %v", ErrRestoreRefused, v.Err)
		}
		if err := replaceFile(p+".corrupt", cur, false); err != nil {
			return fmt.Errorf("keeping the damaged file: %w", err)
		}
	}

	return replaceFile(p, bak, false)
}
