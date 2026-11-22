//go:build !darwin && !windows

package secretclip

import "github.com/atotto/clipboard"

// platformWrite is a plain clipboard write: X11/Wayland clipboard
// managers have no standard "do not record" marker.
func platformWrite(s string) error { return clipboard.WriteAll(s) }
