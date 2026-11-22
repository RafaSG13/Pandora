//go:build windows

package secretclip

import (
	"runtime"
	"time"
	"unsafe"

	"github.com/atotto/clipboard"
	"golang.org/x/sys/windows"
)

var (
	user32                     = windows.NewLazySystemDLL("user32.dll")
	kernel32                   = windows.NewLazySystemDLL("kernel32.dll")
	procOpenClipboard          = user32.NewProc("OpenClipboard")
	procCloseClipboard         = user32.NewProc("CloseClipboard")
	procEmptyClipboard         = user32.NewProc("EmptyClipboard")
	procSetClipboardData       = user32.NewProc("SetClipboardData")
	procRegisterClipboardFormW = user32.NewProc("RegisterClipboardFormatW")
	procGlobalAlloc            = kernel32.NewProc("GlobalAlloc")
	procGlobalFree             = kernel32.NewProc("GlobalFree")
	procGlobalLock             = kernel32.NewProc("GlobalLock")
	procGlobalUnlock           = kernel32.NewProc("GlobalUnlock")
	procRtlMoveMemory          = kernel32.NewProc("RtlMoveMemory")
)

const (
	cfUnicodeText = 13
	gmemMoveable  = 0x0002
)

// concealFormats are the registered clipboard formats Windows checks
// (with a DWORD 0 payload) to keep an entry out of clipboard monitors,
// Win+V history and cloud clipboard sync.
var concealFormats = []string{
	"ExcludeClipboardContentFromMonitorProcessing",
	"CanIncludeInClipboardHistory",
	"CanUploadToCloudClipboard",
}

// platformWrite sets the value as CF_UNICODETEXT plus concealFormats,
// falling back to a plain write if the text could not be set so copying
// never stops working.
func platformWrite(s string) error {
	if err := writeConcealed(s); err != nil {
		return clipboard.WriteAll(s)
	}
	return nil
}

func writeConcealed(s string) error {
	text, err := windows.UTF16FromString(s)
	if err != nil {
		return err
	}
	// OpenClipboard and CloseClipboard must run on the same OS thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := openClipboard(); err != nil {
		return err
	}
	defer procCloseClipboard.Call()
	if r, _, err := procEmptyClipboard.Call(); r == 0 {
		return err
	}
	if err := setClipboard(cfUnicodeText, unsafe.Pointer(&text[0]), uintptr(len(text))*2); err != nil {
		return err
	}
	// The text is on the clipboard; markers are best effort from here on.
	// History and monitors read them on CloseClipboard, so order is fine.
	zero := uint32(0)
	for _, name := range concealFormats {
		p, err := windows.UTF16PtrFromString(name)
		if err != nil {
			continue
		}
		if id, _, _ := procRegisterClipboardFormW.Call(uintptr(unsafe.Pointer(p))); id != 0 {
			_ = setClipboard(id, unsafe.Pointer(&zero), unsafe.Sizeof(zero))
		}
	}
	return nil
}

// openClipboard retries briefly: another process may hold the clipboard.
func openClipboard() error {
	var err error
	for range 5 {
		var r uintptr
		if r, _, err = procOpenClipboard.Call(0); r != 0 {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return err
}

// setClipboard copies n bytes at src into a movable global block and hands
// it to the clipboard as format; the OS owns the block only on success.
func setClipboard(format uintptr, src unsafe.Pointer, n uintptr) error {
	h, _, err := procGlobalAlloc.Call(gmemMoveable, n)
	if h == 0 {
		return err
	}
	dst, _, err := procGlobalLock.Call(h)
	if dst == 0 {
		procGlobalFree.Call(h)
		return err
	}
	procRtlMoveMemory.Call(dst, uintptr(src), n)
	procGlobalUnlock.Call(h)
	if r, _, err := procSetClipboardData.Call(format, h); r == 0 {
		procGlobalFree.Call(h)
		return err
	}
	return nil
}
