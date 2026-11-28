package cli

import (
	"errors"
	"flag"
	"fmt"

	"github.com/RafaSG13/Pandora/internal/store"
)

// cmdDoctor checks vault.json and its backup, then the keyring values. With
// --fix it restores a damaged vault.json from vault.json.bak and removes
// entries whose value is gone.
func (a *app) cmdDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	var fix, yes bool
	fs.BoolVar(&fix, "fix", false, "repair: restore a damaged vault.json from vault.json.bak, and remove metadata entries whose value is missing")
	fs.BoolVar(&yes, "y", false, "with --fix, repair without asking")
	fs.BoolVar(&yes, "yes", false, "with --fix, repair without asking")
	if _, err := a.parse("doctor", "doctor [--fix] [-y]", fs, args, 0, 0); err != nil {
		return err
	}

	restored, err := a.checkVaultFiles(fix, yes)
	if err != nil {
		return err
	}
	items, err := store.LoadMetadata()
	if err != nil {
		return err
	}
	missing, err := store.MissingValues()
	if err != nil {
		return err
	}
	missingVersions, err := store.MissingVersions()
	if err != nil {
		return err
	}
	counts, err := store.VersionCounts() // usable previous values per title
	if err != nil {
		return err
	}
	for _, v := range missingVersions {
		counts[v.Title]--
	}
	// An entry whose current value is gone but that still has usable previous
	// values is not removed: the user can restore one of them.
	var removable, recoverable []string
	for _, t := range missing {
		if counts[t] > 0 {
			recoverable = append(recoverable, t)
		} else {
			removable = append(removable, t)
		}
	}

	pending, err := store.PendingCleanup()
	if err != nil {
		return err
	}
	fmt.Fprintf(a.out, "Checked %d metadata entries.\n", len(items))
	if pending > 0 && fix {
		// No record refers to these entries, so removing them needs no question.
		left, err := store.DropPendingCleanup()
		if err != nil {
			return err
		}
		fmt.Fprintf(a.out, "Removed %d old keyring entries left by a key rotation.\n", pending-left)
		pending = left
	}
	if pending > 0 {
		fmt.Fprintf(a.out, "%d old keyring entries left by a key rotation are still in the keyring.\n", pending)
	}
	if len(missing) == 0 && len(missingVersions) == 0 && pending == 0 {
		fmt.Fprintln(a.out, "OK: every entry has a value in the keyring.")
	}
	if len(missing) > 0 {
		fmt.Fprintf(a.out, "%d entries have no value in the keyring:\n", len(missing))
		for _, t := range missing {
			if counts[t] > 0 {
				fmt.Fprintf(a.out, "  %s (has %d previous values: 'pandora restore' can bring one back)\n", t, counts[t])
			} else {
				fmt.Fprintf(a.out, "  %s\n", t)
			}
		}
	}
	if len(missingVersions) > 0 {
		fmt.Fprintf(a.out, "%d previous values are missing from the keyring:\n", len(missingVersions))
		for _, v := range missingVersions {
			fmt.Fprintf(a.out, "  %s (version %d)\n", v.Title, -v.Back)
		}
	}
	fmt.Fprintln(a.out, "Note: keyring entries without metadata cannot be detected; the keyring API cannot list entries.")

	if len(missing) == 0 && len(missingVersions) == 0 {
		if pending == 0 {
			return nil
		}
		if fix {
			return &exitError{code: exitRuntime, msg: "some old keyring entries could not be removed; run 'pandora doctor --fix' again"}
		}
		return &exitError{code: exitRuntime, msg: "inconsistencies found; run 'pandora doctor --fix' to remove them"}
	}
	if restored {
		// The backup predates the last change, so values that change replaced
		// are gone: look before deleting anything.
		return &exitError{code: exitRuntime, msg: "the restored vault refers to values the keyring no longer has; review the list, then run 'pandora doctor --fix' again to remove those entries"}
	}
	if !fix {
		return &exitError{code: exitRuntime, msg: "inconsistencies found; run 'pandora doctor --fix' to remove the orphaned entries"}
	}
	question := fmt.Sprintf("Remove %d metadata entries and %d previous values that have no value?", len(removable), len(missingVersions))
	if len(removable)+len(missingVersions) == 0 {
		return &exitError{code: exitRuntime, msg: "nothing was removed: the entries without a current value still have previous values; restore one with 'pandora restore'"}
	}
	if err := a.requireConfirmation(yes, question); err != nil {
		return err
	}
	if len(missingVersions) > 0 {
		if _, err := store.DropMissingVersions(); err != nil {
			return err
		}
	}
	for _, t := range removable {
		if err := store.DeleteSecret(t); err != nil {
			return fmt.Errorf("removing %q: %w", t, err)
		}
		a.audit("rm", t)
	}
	fmt.Fprintf(a.out, "Removed %d metadata entries and %d previous values.\n", len(removable), len(missingVersions))
	if len(recoverable) > 0 {
		return &exitError{code: exitRuntime, msg: fmt.Sprintf("%d entries without a current value were kept because they have previous values; restore one with 'pandora restore'", len(recoverable))}
	}
	return nil
}

// checkVaultFiles verifies vault.json and vault.json.bak with the master
// password, prints what it found, restores the backup when fix allows it, and
// leaves the vault unlocked. restored reports whether the backup was used.
func (a *app) checkVaultFiles(fix, yes bool) (restored bool, err error) {
	if !store.Exists() && !store.BackupExists() {
		return false, a.unlock() // no vault at all: like every other command
	}
	pw, rep, err := a.inspectWithPassword()
	if err != nil {
		return false, err
	}
	fmt.Fprintf(a.out, "vault.json: %s\n", describeFile(rep.Vault))
	fmt.Fprintf(a.out, "vault.json.bak: %s\n", describeFile(rep.Backup))

	switch rep.Vault.Health {
	case store.FileOK:
		if rep.Backup.Health == store.FileCorrupt || rep.Backup.Health == store.FileWrongPassword {
			fmt.Fprintln(a.out, "The backup is not usable; the next change to the vault replaces it.")
		}
		return false, store.Unlock(pw)

	case store.FileNewer:
		return false, errors.New("vault.json was written by a newer Pandora; update Pandora (nothing was changed)")

	case store.FileWrongPassword:
		return false, errors.New("the master password does not open vault.json, so it is not restored from the backup (nothing was changed)")
	}

	// vault.json is missing or damaged.
	if rep.Backup.Health != store.FileOK {
		return false, &exitError{code: exitRuntime, msg: "vault.json cannot be repaired: there is no usable vault.json.bak. If you have a backup made with 'pandora export --values', import it into a new vault"}
	}
	if !fix {
		return false, &exitError{code: exitRuntime, msg: "vault.json is damaged but vault.json.bak is usable; run 'pandora doctor --fix' to restore it"}
	}
	question := fmt.Sprintf("Replace vault.json with vault.json.bak (%d secrets, saved %s)? Changes made after that are lost.", rep.Backup.Entries, rep.Backup.Modified.Format("2006-01-02 15:04"))
	if err := a.requireConfirmation(yes, question); err != nil {
		return false, err
	}
	if err := store.RestoreVaultBackup(pw); err != nil {
		return false, err
	}
	fmt.Fprintf(a.out, "Restored vault.json from vault.json.bak (%d secrets). A damaged file, if there was one, is kept as vault.json.corrupt.\n", rep.Backup.Entries)
	return true, store.Unlock(pw)
}

// describeFile words a FileReport for the doctor output.
func describeFile(r store.FileReport) string {
	switch r.Health {
	case store.FileOK:
		return fmt.Sprintf("OK (%d secrets, saved %s)", r.Entries, r.Modified.Format("2006-01-02 15:04"))
	case store.FileMissing:
		return "missing"
	case store.FileNewer:
		return fmt.Sprintf("written by a newer Pandora (%v)", r.Err)
	case store.FileWrongPassword:
		return "does not open with this password"
	default:
		return fmt.Sprintf("DAMAGED (%v)", r.Err)
	}
}

// inspectWithPassword asks for the master password like unlock does (the
// environment variable, or up to three prompts on a terminal) but checks it
// against both vault files, so it also works when vault.json is damaged. A
// password is accepted when either file opens with it.
func (a *app) inspectWithPassword() (string, store.VaultReport, error) {
	accepted := func(r store.VaultReport) bool {
		return r.Vault.Health == store.FileOK || r.Backup.Health == store.FileOK
	}
	// Wrong only when some file could have tested the password and none opened.
	wrong := func(r store.VaultReport) bool {
		return !accepted(r) && (r.Vault.Health == store.FileWrongPassword || r.Backup.Health == store.FileWrongPassword)
	}

	if pw := a.getenv(masterEnv); pw != "" {
		a.warnEnv(masterEnv)
		rep, err := store.InspectVault(pw)
		switch {
		case err != nil:
			return "", rep, err
		case wrong(rep):
			return "", rep, fmt.Errorf("wrong master password in %s", masterEnv)
		}
		return pw, rep, nil
	}
	if !a.isTTY() {
		return "", store.VaultReport{}, fmt.Errorf("no master password: set %s or run in a terminal", masterEnv)
	}
	for attempt := 1; attempt <= maxPasswordAttempts; attempt++ {
		pw, err := a.readHidden("Master password: ")
		if err != nil {
			return "", store.VaultReport{}, err
		}
		rep, err := store.InspectVault(pw)
		if err != nil {
			return "", rep, err
		}
		if !wrong(rep) {
			return pw, rep, nil
		}
		if attempt < maxPasswordAttempts {
			fmt.Fprintln(a.err, "Wrong password, try again.")
		}
	}
	return "", store.VaultReport{}, errors.New("wrong master password")
}
