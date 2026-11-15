package cli

import (
	"errors"
	"flag"
	"fmt"

	"github.com/RafaSG13/Pandora/internal/store"
)

// maxPasswordAttempts is how many times a password prompt is repeated.
const maxPasswordAttempts = 3

var errPasswordMismatch = errors.New("passwords do not match")

// warnEnv prints, once per process, that a secret was taken from the
// environment, where other processes of the same user may read it.
func (a *app) warnEnv(name string) {
	if a.warnedEnv[name] {
		return
	}
	if a.warnedEnv == nil {
		a.warnedEnv = map[string]bool{}
	}
	a.warnedEnv[name] = true
	what := "passphrase"
	if name == masterEnv {
		what = "master password"
	}
	fmt.Fprintf(a.err, "pandora: warning: %s read from %s; other processes of your user may be able to read it\n", what, name)
}

// unlock makes the vault usable: it asks for the master password (or reads it
// from PANDORA_MASTER_PASSWORD when there is no terminal) and, on the very
// first run, creates the vault, migrating a plain-text store from an older
// version. It is a no-op when the vault is already unlocked.
func (a *app) unlock() error {
	if store.IsUnlocked() {
		return nil
	}
	if store.Exists() {
		return a.unlockExisting()
	}
	return a.createVault()
}

func (a *app) unlockExisting() error {
	if pw := a.getenv(masterEnv); pw != "" {
		a.warnEnv(masterEnv)
		if err := store.Unlock(pw); errors.Is(err, store.ErrWrongPassword) {
			return fmt.Errorf("wrong master password in %s", masterEnv)
		} else if err != nil {
			return err
		}
		return nil
	}
	if !a.isTTY() {
		return fmt.Errorf("no master password: set %s or run in a terminal", masterEnv)
	}
	for attempt := 1; attempt <= maxPasswordAttempts; attempt++ {
		pw, err := a.readHidden("Master password: ")
		if err != nil {
			return err
		}
		err = store.Unlock(pw)
		if err == nil {
			return nil
		}
		if !errors.Is(err, store.ErrWrongPassword) {
			return err
		}
		if attempt < maxPasswordAttempts {
			fmt.Fprintln(a.err, "Wrong password, try again.")
		}
	}
	return errors.New("wrong master password")
}

// newPassword asks for a new password twice on the terminal.
func (a *app) newPassword(prompt string) (string, error) {
	pw, err := a.readHidden(prompt)
	if err != nil {
		return "", err
	}
	again, err := a.readHidden("Confirm: ")
	if err != nil {
		return "", err
	}
	if pw != again {
		return "", errPasswordMismatch
	}
	return pw, nil
}

func (a *app) createVault() error {
	if !a.isTTY() {
		return errors.New("no vault yet: run pandora in a terminal to create one")
	}
	migrating := store.LegacyExists()
	fmt.Fprintln(a.err, "Choose a password you will remember: it cannot be recovered. 'pandora export --values' makes a backup.")
	for attempt := 1; attempt <= maxPasswordAttempts; attempt++ {
		pw, err := a.newPassword("New master password: ")
		if err == nil {
			if migrating {
				err = store.MigrateLegacy(pw)
			} else {
				err = store.Create(pw)
			}
		}
		var cleanup *store.CleanupError
		switch {
		case err == nil, errors.As(err, &cleanup):
			if cleanup != nil {
				fmt.Fprintf(a.err, "pandora: warning: %v\n", cleanup)
			}
			if migrating {
				items, _ := store.LoadMetadata()
				fmt.Fprintf(a.err, "Migrated %d secrets to the encrypted vault.\n", len(items))
			}
			return nil
		case errors.Is(err, store.ErrPasswordTooShort), errors.Is(err, errPasswordMismatch):
			fmt.Fprintf(a.err, "%v\n", err)
		default:
			return err
		}
	}
	return errors.New("too many invalid attempts")
}

// cmdPasswd changes the master password. The data key stays the same, so the
// values in the keyring remain valid.
func (a *app) cmdPasswd(args []string) error {
	if _, err := a.parse("passwd", "passwd", flag.NewFlagSet("passwd", flag.ContinueOnError), args, 0, 0); err != nil {
		return err
	}
	if !a.isTTY() {
		return errors.New("passwd needs a terminal")
	}
	if !store.Exists() {
		return errors.New("no vault yet: run pandora once to create one")
	}
	old, err := a.readHidden("Current master password: ")
	if err != nil {
		return err
	}
	pw, err := a.newPassword("New master password: ")
	if err != nil {
		return err
	}
	if err := store.ChangePassword(old, pw); err != nil {
		return err
	}
	fmt.Fprintln(a.err, "Master password changed.")
	return nil
}
