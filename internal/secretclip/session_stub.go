//go:build darwin || windows

package secretclip

// OnX11 is always false here: macOS and Windows do not use X11 for the clipboard.
func OnX11() bool { return false }
