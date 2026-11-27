//go:build unix

package harden

import "golang.org/x/sys/unix"

// setSoftCoreLimit sets the soft core-file size limit to 0. Only the soft
// limit is lowered: children started by `pandora run` inherit it but can raise
// it back.
func setSoftCoreLimit() {
	var l unix.Rlimit
	if unix.Getrlimit(unix.RLIMIT_CORE, &l) != nil {
		return
	}
	l.Cur = 0
	_ = unix.Setrlimit(unix.RLIMIT_CORE, &l)
}
