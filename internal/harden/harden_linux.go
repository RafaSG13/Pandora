//go:build linux

package harden

import "golang.org/x/sys/unix"

// Process disables core dumps and makes the process non-dumpable: no core
// file, /proc/<pid>/{mem,environ,maps} become root-owned, and same-user ptrace
// is refused. The process can still read its own /proc/self/exe, so
// os.Executable keeps working.
func Process() {
	_ = unix.Prctl(unix.PR_SET_DUMPABLE, 0, 0, 0, 0)
	setSoftCoreLimit()
}
