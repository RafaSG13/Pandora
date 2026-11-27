package secretclip

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/atotto/clipboard"
)

// ClearCommand is the hidden CLI command that waits and then clears the
// clipboard if it still holds a copied secret. ClearLater starts it detached.
const ClearCommand = "__clear-clipboard"

// Holds reports whether the clipboard text got is the value whose SHA-256 is
// want. Terminals and clipboard tools sometimes append a trailing newline,
// which is ignored.
func Holds(got string, want [sha256.Size]byte) bool {
	for _, s := range []string{got, strings.TrimRight(got, "\r\n")} {
		if sha256.Sum256([]byte(s)) == want {
			return true
		}
	}
	return false
}

// ClearIfHoldsUsing empties the clipboard through write unless read shows it
// no longer holds the value whose SHA-256 is want. An unreadable clipboard is
// cleared anyway. cleared is false only when the content had changed.
func ClearIfHoldsUsing(read func() (string, error), write func(string) error, want [sha256.Size]byte) (cleared bool, err error) {
	if got, rerr := read(); rerr == nil && !Holds(got, want) {
		return false, nil
	}
	return true, write("")
}

// ClearIfHolds is ClearIfHoldsUsing on the system clipboard.
func ClearIfHolds(want [sha256.Size]byte) (bool, error) {
	return ClearIfHoldsUsing(clipboard.ReadAll, clipboard.WriteAll, want)
}

// ClearLater starts a detached copy of this executable that, after the given
// delay, clears the clipboard if it still holds value. It outlives the caller
// (the TUI quitting, a one-shot CLI command exiting). The value itself never
// leaves this process: the helper receives only its SHA-256, over a pipe so it
// does not show up in the process list. Pandora settings, including both
// password variables, are removed from the helper's environment.
func ClearLater(value string, after time.Duration) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, ClearCommand, strconv.Itoa(int(after/time.Second)))
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "PANDORA_") {
			cmd.Env = append(cmd.Env, kv)
		}
	}
	if cmd.Env == nil {
		cmd.Env = []string{}
	}
	cmd.SysProcAttr = detachAttr()
	r, w, err := os.Pipe()
	if err != nil {
		return err
	}
	cmd.Stdin = r
	if err := cmd.Start(); err != nil {
		r.Close()
		w.Close()
		return err
	}
	r.Close()
	sum := sha256.Sum256([]byte(value))
	_, werr := w.WriteString(hex.EncodeToString(sum[:]) + "\n")
	if cerr := w.Close(); werr == nil {
		werr = cerr
	}
	if rerr := cmd.Process.Release(); werr == nil {
		werr = rerr
	}
	return werr
}
