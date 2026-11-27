//go:build unix && !linux

package harden

// Process disables core dumps for this process.
func Process() { setSoftCoreLimit() }
