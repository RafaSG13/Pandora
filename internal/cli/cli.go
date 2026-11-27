// Package cli implements Pandora's non-interactive command-line mode
// (list, get, add, rm, run, env, export, import, doctor, gen, version, completion).
// The commands are declared once in the table returned by commands; usage,
// dispatch and shell completions all derive from it. Run never calls os.Exit
// and takes its streams as arguments so it can be tested.
package cli

import (
	"bufio"
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/RafaSG13/Pandora/internal/config"
	"github.com/RafaSG13/Pandora/internal/secretclip"
	"github.com/RafaSG13/Pandora/internal/store"
	"github.com/RafaSG13/Pandora/internal/version"
)

// Exit codes.
const (
	exitOK       = 0
	exitRuntime  = 1
	exitUsage    = 2
	exitExpiring = 3 // `expiring` found expired or expiring secrets
)

// passphraseEnv names the environment variable that supplies the export
// passphrase when no terminal is available.
const passphraseEnv = "PANDORA_PASSPHRASE"

// masterEnv names the environment variable that supplies the master password
// when no terminal is available.
const masterEnv = "PANDORA_MASTER_PASSWORD"

// exitError carries the exit code for Run. An empty msg means the message was
// already printed.
type exitError struct {
	code int
	msg  string
}

func (e *exitError) Error() string { return e.msg }

func usageErrorf(format string, args ...any) error {
	return &exitError{code: exitUsage, msg: fmt.Sprintf(format, args...)}
}

// app holds the streams and the environment hooks that tests replace.
type app struct {
	in       io.Reader
	out, err io.Writer

	getenv  func(string) string
	environ func() []string
	// isTTY reports whether stdin is an interactive terminal.
	isTTY func() bool
	// isOutTTY reports whether stdout is an interactive terminal.
	isOutTTY func() bool
	// copySecret puts a value on the system clipboard (concealed where the OS allows).
	copySecret func(string) error
	// clearLater schedules a detached helper that clears the clipboard after a delay if it still holds the value.
	clearLater func(value string, after time.Duration) error
	// clearIfHolds empties the clipboard when it still holds the value with this SHA-256.
	clearIfHolds func([sha256.Size]byte) (bool, error)
	// sleep waits; tests replace it.
	sleep func(time.Duration)
	// readHidden prompts on stderr and reads a line from the terminal without echo.
	readHidden func(prompt string) (string, error)
	// now returns the current time; tests replace it.
	now func() time.Time
	// scryptWorkFactor overrides the age scrypt cost (log2 N) when non-zero.
	scryptWorkFactor int
	// warnedEnv records which environment-secret warnings were already printed.
	warnedEnv map[string]bool
	// cfg is the settings loaded from config.json, kept for the process.
	cfg *config.Settings
	// auditWarned is set once a failure to write the audit log was reported.
	auditWarned bool
}

// Run executes the command line (without the program name) and returns the
// process exit code: 0 success, 1 runtime error, 2 usage error, 3 `expiring`
// found something.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return newApp(stdin, stdout, stderr).run(args)
}

func newApp(stdin io.Reader, stdout, stderr io.Writer) *app {
	a := &app{in: stdin, out: stdout, err: stderr, getenv: os.Getenv, environ: os.Environ, now: time.Now}
	a.isTTY = func() bool {
		f, ok := a.in.(*os.File)
		return ok && term.IsTerminal(int(f.Fd()))
	}
	a.isOutTTY = func() bool {
		f, ok := a.out.(*os.File)
		return ok && term.IsTerminal(int(f.Fd()))
	}
	a.copySecret = secretclip.Write
	a.clearLater = secretclip.ClearLater
	a.clearIfHolds = secretclip.ClearIfHolds
	a.sleep = time.Sleep
	a.readHidden = func(prompt string) (string, error) {
		f, ok := a.in.(*os.File)
		if !ok || !term.IsTerminal(int(f.Fd())) {
			return "", errors.New("stdin is not a terminal")
		}
		fmt.Fprint(a.err, prompt)
		b, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(a.err)
		return string(b), err
	}
	return a
}

func (a *app) run(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(a.out, topUsage())
		return exitOK
	}
	cmd, ok := lookup(args[0])
	if !ok {
		fmt.Fprintf(a.err, "pandora: unknown command %q\n\n%s", args[0], topUsage())
		return exitUsage
	}
	err := cmd.run(a, args[1:])

	var ee *exitError
	switch {
	case err == nil:
		return exitOK
	case errors.As(err, &ee):
		if ee.msg != "" {
			fmt.Fprintf(a.err, "pandora: %s\n", ee.msg)
		}
		return ee.code
	default:
		fmt.Fprintf(a.err, "pandora: %v\n", err)
		if errors.Is(err, store.ErrVaultCorrupt) && cmd.name != "doctor" {
			fmt.Fprintln(a.err, "pandora: run 'pandora doctor' to check vault.json and its backup")
		}
		return exitRuntime
	}
}

func (a *app) cmdHelp([]string) error {
	_, err := fmt.Fprint(a.out, topUsage())
	return err
}

func (a *app) cmdVersion(args []string) error {
	if _, err := a.parse("version", "version", flag.NewFlagSet("version", flag.ContinueOnError), args, 0, 0); err != nil {
		return err
	}
	_, err := fmt.Fprintln(a.out, version.String())
	return err
}

// parse parses flags that may appear before, between or after positional
// arguments (flag.FlagSet alone stops at the first positional) and checks the
// number of positionals. -h prints usage to stdout and ends the command with
// exit code 0; other problems print usage to stderr and exit with code 2.
func (a *app) parse(name, usage string, fs *flag.FlagSet, args []string, minPos, maxPos int) ([]string, error) {
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	fail := func(msg string) error {
		fmt.Fprintf(a.err, "pandora %s: %s\n", name, msg)
		a.printUsage(a.err, usage, fs)
		return &exitError{code: exitUsage}
	}

	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				a.printUsage(a.out, usage, fs)
				return nil, &exitError{code: exitOK}
			}
			return nil, fail(err.Error())
		}
		rest := fs.Args()
		if len(rest) == 0 {
			break
		}
		if n := len(args) - len(rest); n > 0 && args[n-1] == "--" {
			pos = append(pos, rest...) // everything after "--" is positional
			break
		}
		pos = append(pos, rest[0])
		args = rest[1:]
	}
	if len(pos) < minPos || len(pos) > maxPos {
		return nil, fail(fmt.Sprintf("expected %s, got %d", plural(minPos, maxPos), len(pos)))
	}
	return pos, nil
}

func plural(minPos, maxPos int) string {
	switch {
	case minPos == maxPos && minPos == 1:
		return "1 argument"
	case minPos == maxPos:
		return fmt.Sprintf("%d arguments", minPos)
	case maxPos == 0:
		return "no arguments"
	default:
		return fmt.Sprintf("%d to %d arguments", minPos, maxPos)
	}
}

func (a *app) printUsage(w io.Writer, usage string, fs *flag.FlagSet) {
	fmt.Fprintf(w, "Usage: pandora %s\n", usage)
	fs.SetOutput(w)
	fs.PrintDefaults()
	fs.SetOutput(io.Discard)
}

// confirm asks a yes/no question on the terminal. It must only be called when
// isTTY reports true.
func (a *app) confirm(question string) (bool, error) {
	fmt.Fprintf(a.err, "%s [y/N] ", question)
	line, err := bufio.NewReader(a.in).ReadString('\n')
	if err != nil && line == "" {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	}
	return false, nil
}

// requireConfirmation implements the shared "-y or a terminal" rule of
// destructive commands. It returns nil when the action may proceed.
func (a *app) requireConfirmation(yes bool, question string) error {
	if yes {
		return nil
	}
	if !a.isTTY() {
		return errors.New("refusing to continue without confirmation: stdin is not a terminal (use -y to confirm)")
	}
	ok, err := a.confirm(question)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("cancelled")
	}
	return nil
}
