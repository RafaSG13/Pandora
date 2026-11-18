//go:build darwin

package tui

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/atotto/clipboard"
)

// osascript runs concealedCopyJXA. Using it keeps the binary CGO-free and
// dependency-free at the cost of ~100 ms per copy.
const osascript = "/usr/bin/osascript"

// concealedCopyJXA reads the value from stdin (never argv or the
// environment, which other processes can see) and writes it together with
// org.nspasteboard.ConcealedType, the nspasteboard.org marker that clipboard
// managers honour by not recording the entry.
const concealedCopyJXA = `ObjC.import('AppKit');
var d = $.NSFileHandle.fileHandleWithStandardInput.readDataToEndOfFile;
var s = $.NSString.alloc.initWithDataEncoding(d, $.NSUTF8StringEncoding);
var pb = $.NSPasteboard.generalPasteboard;
pb.clearContents;
pb.setStringForType(s, 'public.utf8-plain-text');
pb.setStringForType('', 'org.nspasteboard.ConcealedType');
undefined;`

func platformWriteSecret(s string) error { return writeSecret(osascript, s) }

// writeSecret copies s concealed via the osascript binary bin, falling back
// to a plain write if that fails so copying never stops working.
func writeSecret(bin, s string) error {
	if err := writeConcealed(bin, s); err != nil {
		return clipboard.WriteAll(s)
	}
	return nil
}

func writeConcealed(bin, s string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "-l", "JavaScript", "-e", concealedCopyJXA)
	cmd.Stdin = strings.NewReader(s)
	// Discard osascript's output: it must not reach the TUI's terminal.
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("concealed copy failed: %w", err)
	}
	return nil
}
