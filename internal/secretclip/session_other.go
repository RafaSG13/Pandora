//go:build !darwin && !windows

package secretclip

import "os"

// OnX11 reports whether the clipboard lives in an X11 session. X11 has no
// concealment marker, and any program connected to the same X server can read
// the clipboard at any time.
func OnX11() bool {
	return onX11(os.Getenv("XDG_SESSION_TYPE"), os.Getenv("WAYLAND_DISPLAY"), os.Getenv("DISPLAY"))
}

func onX11(sessionType, wayland, display string) bool {
	switch sessionType {
	case "x11":
		return true
	case "wayland":
		return false
	}
	return wayland == "" && display != ""
}
