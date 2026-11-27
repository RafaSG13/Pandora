// Package secretclip copies secret values to the system clipboard. It is
// shared by the TUI and the CLI so both conceal the entry the same way.
package secretclip

// Write puts a secret value on the system clipboard, marked (where the OS has
// a convention for it) so clipboard-history managers and cloud clipboard sync
// skip it: org.nspasteboard.ConcealedType on macOS, the history/monitor/cloud
// exclusion formats on Windows, `wl-copy --sensitive` (which sets
// x-kde-passwordManagerHint) on Wayland. X11 has no marker and gets a plain
// write. platformWrite lives in clipboard_<os>.go.
func Write(s string) error { return platformWrite(s) }
