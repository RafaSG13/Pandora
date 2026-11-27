//go:build darwin

package harden

import (
	"testing"

	"golang.org/x/sys/unix"
)

func TestProcessDisablesCoreFiles(t *testing.T) {
	Process()
	var l unix.Rlimit
	if err := unix.Getrlimit(unix.RLIMIT_CORE, &l); err != nil || l.Cur != 0 {
		t.Errorf("RLIMIT_CORE soft = %d, %v; want 0", l.Cur, err)
	}
}
