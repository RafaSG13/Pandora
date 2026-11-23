package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"text/tabwriter"

	"github.com/zalando/go-keyring"

	"github.com/RafaSG13/Pandora/internal/store"
)

// versionEntry is one element of `history --json`. Version is -1 for the
// previous value, -2 for the one before, and so on.
type versionEntry struct {
	Version  int    `json:"version"`
	Replaced string `json:"replaced"`
}

// checkVersionFlag validates a --version value: 0 is the current value and a
// negative number counts back from it. It returns how many steps back.
func checkVersionFlag(cmd string, v int) (back int, err error) {
	if v > 0 {
		return 0, usageErrorf("%s: --version counts back from the current value: use -1 for the previous one", cmd)
	}
	return -v, nil
}

// versionError words the store's history errors for the command line.
func versionError(title string, err error) error {
	switch {
	case errors.Is(err, store.ErrNotFound):
		return fmt.Errorf("secret %q not found", title)
	case errors.Is(err, store.ErrNoSuchVersion):
		return fmt.Errorf("%w (see 'pandora history %s')", err, title)
	case errors.Is(err, keyring.ErrNotFound):
		return fmt.Errorf("that value of %q is missing from the keyring ('pandora doctor' lists it)", title)
	}
	return err
}

// cmdHistory lists the previous values of a secret: when each stopped being
// current, never the values.
func (a *app) cmdHistory(args []string) error {
	const usage = "history <title> [--json]"
	fs := flag.NewFlagSet("history", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "print JSON instead of a table")
	pos, err := a.parse("history", usage, fs, args, 1, 1)
	if err != nil {
		return err
	}
	if err := a.unlock(); err != nil {
		return err
	}
	infos, err := store.Versions(pos[0])
	if err != nil {
		return versionError(pos[0], err)
	}
	entries := make([]versionEntry, len(infos))
	for i, v := range infos {
		entries[i] = versionEntry{Version: -(i + 1), Replaced: v.Replaced}
	}
	if *asJSON {
		enc := json.NewEncoder(a.out)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)
	}
	if len(entries) == 0 {
		fmt.Fprintf(a.err, "pandora: %q has no previous values\n", pos[0])
		return nil
	}
	tw := tabwriter.NewWriter(a.out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "VERSION\tREPLACED")
	for _, e := range entries {
		fmt.Fprintf(tw, "%d\t%s\n", e.Version, e.Replaced)
	}
	return tw.Flush()
}

// cmdRestore makes a previous value current again. The value it replaces
// becomes the newest previous one, so the restore can be undone.
func (a *app) cmdRestore(args []string) error {
	const usage = "restore <title> [--version -1]"
	fs := flag.NewFlagSet("restore", flag.ContinueOnError)
	version := fs.Int("version", -1, "which previous value: -1 is the latest, -2 the one before")
	pos, err := a.parse("restore", usage, fs, args, 1, 1)
	if err != nil {
		return err
	}
	back, err := checkVersionFlag("restore", *version)
	if err != nil {
		return err
	}
	if back == 0 {
		return usageErrorf("restore: --version 0 is the current value; use -1 or lower")
	}
	if err := a.unlock(); err != nil {
		return err
	}
	if err := store.RestoreVersion(pos[0], back); err != nil {
		return versionError(pos[0], err)
	}
	a.audit("restore", pos[0])
	fmt.Fprintf(a.err, "Restored version %d of %q; the value it replaced is now version -1.\n", -back, pos[0])
	return nil
}
