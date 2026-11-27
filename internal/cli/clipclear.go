package cli

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/RafaSG13/Pandora/internal/config"
	"github.com/RafaSG13/Pandora/internal/secretclip"
)

// cmdClearClipboard is the hidden helper started by secretclip.ClearLater. It
// reads the hex SHA-256 of a copied value from stdin, waits the given number
// of seconds and then clears the clipboard if it still holds that value. It
// never opens the vault and never sees the value itself.
func (a *app) cmdClearClipboard(args []string) error {
	pos, err := a.parse(secretclip.ClearCommand, secretclip.ClearCommand+" <seconds>",
		flag.NewFlagSet(secretclip.ClearCommand, flag.ContinueOnError), args, 1, 1)
	if err != nil {
		return err
	}
	n, err := strconv.Atoi(pos[0])
	if err != nil || n < 1 || n > config.MaxSeconds {
		return usageErrorf("seconds must be 1 to %d", config.MaxSeconds)
	}
	line, err := bufio.NewReader(a.in).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	want, derr := hex.DecodeString(strings.TrimSpace(line))
	if derr != nil || len(want) != sha256.Size {
		return usageErrorf("expected the SHA-256 of the copied value, in hex, on stdin")
	}
	a.sleep(time.Duration(n) * time.Second)
	_, err = a.clearIfHolds([sha256.Size]byte(want))
	return err
}
