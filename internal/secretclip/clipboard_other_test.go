//go:build linux

package secretclip

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeWlCopy writes a stand-in for wl-copy that records its arguments and
// stdin, then exits with code.
func fakeWlCopy(t *testing.T, code string) (bin, args, stdin string) {
	t.Helper()
	dir := t.TempDir()
	bin, args, stdin = filepath.Join(dir, "wl-copy"), filepath.Join(dir, "args"), filepath.Join(dir, "stdin")
	script := "#!/bin/sh\necho \"$@\" > " + args + "\ncat > " + stdin + "\nexit " + code + "\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin, args, stdin
}

func TestWriteSecretUsesWlCopySensitiveOnWayland(t *testing.T) {
	bin, args, stdin := fakeWlCopy(t, "0")
	if err := writeSecret("wayland-0", bin, "v"); err != nil {
		t.Fatal(err)
	}
	gotArgs, _ := os.ReadFile(args)
	if strings.TrimSpace(string(gotArgs)) != "--sensitive" {
		t.Errorf("argv = %q, want --sensitive only (the value must not be an argument)", gotArgs)
	}
	if gotIn, _ := os.ReadFile(stdin); string(gotIn) != "v" {
		t.Errorf("stdin = %q, want the value", gotIn)
	}
}

func TestWriteSecretSkipsWlCopyOutsideWayland(t *testing.T) {
	bin, args, _ := fakeWlCopy(t, "1")
	_ = writeSecret("", bin, "v") // falls through to the real clipboard, whose error does not matter here
	if _, err := os.Stat(args); err == nil {
		t.Error("wl-copy ran although WAYLAND_DISPLAY is unset")
	}
}
