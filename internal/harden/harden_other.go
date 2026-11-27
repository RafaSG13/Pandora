//go:build !unix

package harden

// Process does nothing on this platform.
func Process() {}
