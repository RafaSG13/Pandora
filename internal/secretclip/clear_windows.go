//go:build windows

package secretclip

import (
	"syscall"

	"golang.org/x/sys/windows"
)

// detachAttr runs the helper without a console, outside the caller's process
// group, so it survives the terminal closing.
func detachAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP,
		HideWindow:    true,
	}
}
