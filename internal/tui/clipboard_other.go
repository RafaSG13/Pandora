//go:build !darwin && !windows

package tui

import "github.com/atotto/clipboard"

// platformWriteSecret is a plain clipboard write: X11/Wayland clipboard
// managers have no standard "do not record" marker.
func platformWriteSecret(s string) error { return clipboard.WriteAll(s) }
