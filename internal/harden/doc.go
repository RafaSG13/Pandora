// Package harden applies best-effort hardening of the running process against
// memory disclosure (core dumps, same-user debuggers). Errors are ignored: a
// platform that refuses simply keeps its defaults.
package harden
