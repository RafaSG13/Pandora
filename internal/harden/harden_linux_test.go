//go:build linux

package harden

import (
	"os"
	"testing"

	"golang.org/x/sys/unix"
)

func TestProcessIsNonDumpableWithNoCoreFiles(t *testing.T) {
	Process()
	if got, err := unix.PrctlRetInt(unix.PR_GET_DUMPABLE, 0, 0, 0, 0); err != nil || got != 0 {
		t.Errorf("PR_GET_DUMPABLE = %d, %v; want 0", got, err)
	}
	var l unix.Rlimit
	if err := unix.Getrlimit(unix.RLIMIT_CORE, &l); err != nil || l.Cur != 0 {
		t.Errorf("RLIMIT_CORE soft = %d, %v; want 0", l.Cur, err)
	}
	if _, err := os.Executable(); err != nil {
		t.Errorf("os.Executable after hardening: %v", err)
	}
}
