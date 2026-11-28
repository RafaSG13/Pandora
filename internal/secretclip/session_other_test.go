//go:build !darwin && !windows

package secretclip

import "testing"

func TestOnX11(t *testing.T) {
	cases := []struct {
		name                     string
		sessionType, wayland, dp string
		want                     bool
	}{
		{"x11 session", "x11", "", ":0", true},
		{"wayland session with XWayland", "wayland", "wayland-0", ":0", false},
		{"unknown type, only DISPLAY", "", "", ":0", true},
		{"unknown type, Wayland display", "", "wayland-0", ":0", false},
		{"headless", "", "", "", false},
	}
	for _, c := range cases {
		if got := onX11(c.sessionType, c.wayland, c.dp); got != c.want {
			t.Errorf("%s: onX11 = %v, want %v", c.name, got, c.want)
		}
	}
}
