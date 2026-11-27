//go:build !darwin && !windows

package secretclip

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/atotto/clipboard"
)

// platformWrite copies s. On Wayland it uses `wl-copy --sensitive`, which sets
// the x-kde-passwordManagerHint marker so clipboard history skips the entry;
// X11 has no such marker and gets a plain write.
func platformWrite(s string) error { return writeSecret(os.Getenv("WAYLAND_DISPLAY"), "wl-copy", s) }

// writeSecret is platformWrite with the Wayland display and the wl-copy
// binary injected. Copying never stops working: when wl-copy is missing or
// too old to know --sensitive, it falls back to the plain write.
func writeSecret(wayland, bin, s string) error {
	if wayland != "" && writeSensitive(bin, s) == nil {
		return nil
	}
	return clipboard.WriteAll(s)
}

// writeSensitive runs `bin --sensitive` with s on stdin. Stdout and stderr stay
// nil: wl-copy forks a background server that keeps serving the selection, and
// a pipe would make Run wait for it.
func writeSensitive(bin, s string) error {
	path, err := exec.LookPath(bin)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "--sensitive")
	cmd.Stdin = strings.NewReader(s)
	return cmd.Run()
}
