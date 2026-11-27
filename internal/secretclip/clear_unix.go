//go:build !windows

package secretclip

import "syscall"

// detachAttr gives the helper its own session so it survives the terminal
// closing.
func detachAttr() *syscall.SysProcAttr { return &syscall.SysProcAttr{Setsid: true} }
