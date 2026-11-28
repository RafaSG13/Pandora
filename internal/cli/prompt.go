package cli

import (
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"golang.org/x/term"
)

// readPasswordRestoring reads a line without echo like term.ReadPassword, but
// puts the terminal back if Ctrl-C or SIGTERM arrives while it waits: the
// default action would kill the process with echo still switched off. After
// restoring, the signal is sent again with its default action, so Run still
// never calls os.Exit.
func readPasswordRestoring(fd int) ([]byte, error) {
	if runtime.GOOS == "windows" {
		return term.ReadPassword(fd)
	}
	state, err := term.GetState(fd)
	if err != nil {
		return nil, err
	}
	sigs := make(chan os.Signal, 1)
	done := make(chan struct{})
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case sig := <-sigs:
			_ = term.Restore(fd, state)
			signal.Reset(os.Interrupt, syscall.SIGTERM)
			if p, err := os.FindProcess(os.Getpid()); err == nil {
				_ = p.Signal(sig)
			}
		case <-done:
		}
	}()
	defer func() {
		signal.Stop(sigs)
		close(done)
	}()
	return term.ReadPassword(fd)
}
