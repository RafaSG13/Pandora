package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
)

// repeatFlag collects every occurrence of a repeatable flag, in order.
type repeatFlag []string

func (s *repeatFlag) String() string     { return strings.Join(*s, ",") }
func (s *repeatFlag) Set(v string) error { *s = append(*s, v); return nil }

type injection struct{ title, env string }

func (a *app) cmdRun(args []string) error {
	const usage = "run -s TITLE[=ENV_NAME] [-s ...] -- command [args...]"
	// Everything after the first "--" is the child command, never flags.
	var command []string
	for i, arg := range args {
		if arg == "--" {
			args, command = args[:i], args[i+1:]
			break
		}
	}

	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	var specs repeatFlag
	fs.Var(&specs, "s", "secret to inject, `TITLE[=ENV_NAME]`; ENV_NAME defaults to the title upper-cased with other characters as _ (repeatable)")
	fs.Var(&specs, "secret", "same as -s")
	if _, err := a.parse("run", usage, fs, args, 0, 0); err != nil {
		return err
	}
	if len(specs) == 0 {
		return usageErrorf("run: at least one -s is required\nusage: pandora %s", usage)
	}
	if len(command) == 0 {
		return usageErrorf("run: missing command after --\nusage: pandora %s", usage)
	}

	injections, err := parseInjections("run", specs)
	if err != nil {
		return err
	}

	if err := a.unlock(); err != nil {
		return err
	}

	// Resolve every secret before starting anything.
	env := make([]string, 0, len(injections))
	for _, inj := range injections {
		v, err := secretValue(inj.title)
		if err != nil {
			return err
		}
		env = append(env, inj.env+"="+v)
	}

	for _, inj := range injections {
		a.audit("run", inj.title)
	}
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Env = append(childBase(a.environ()), env...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = a.in, a.out, a.err
	return a.start(cmd)
}

// parseInjections turns -s specs into title/env pairs and rejects duplicate
// environment variable names. cmd names the command in error messages.
func parseInjections(cmd string, specs []string) ([]injection, error) {
	seen := map[string]string{}
	out := make([]injection, 0, len(specs))
	for _, spec := range specs {
		title, env := spec, ""
		if i := strings.LastIndex(spec, "="); i >= 0 {
			title, env = spec[:i], spec[i+1:]
			if env == "" {
				return nil, usageErrorf("%s: empty ENV_NAME in %q", cmd, spec)
			}
		}
		if title == "" {
			return nil, usageErrorf("%s: empty title in %q", cmd, spec)
		}
		if env == "" {
			env = EnvName(title)
		}
		if env == "" {
			return nil, usageErrorf("%s: cannot derive an environment variable name from %q; use TITLE=ENV_NAME", cmd, title)
		}
		if strings.ContainsAny(env, "=\x00") {
			return nil, usageErrorf("%s: invalid environment variable name %q", cmd, env)
		}
		if prev, dup := seen[env]; dup {
			return nil, usageErrorf("%s: %q and %q both map to %s", cmd, prev, title, env)
		}
		seen[env] = title
		out = append(out, injection{title, env})
	}
	return out, nil
}

// EnvName derives an environment variable name from a secret title: ASCII
// letters (upper-cased) and digits are kept, everything else becomes "_". It
// returns "" when the title has no letter or digit at all.
func EnvName(title string) string {
	var b strings.Builder
	alnum := false
	for _, r := range strings.ToUpper(title) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			alnum = true
		} else {
			b.WriteByte('_')
		}
	}
	if !alnum {
		return ""
	}
	return b.String()
}

// childBase is the parent's environment minus Pandora's own password
// variables, which the child has no business seeing.
func childBase(environ []string) []string {
	out := make([]string, 0, len(environ))
	for _, kv := range environ {
		if !strings.HasPrefix(kv, passphraseEnv+"=") && !strings.HasPrefix(kv, masterEnv+"=") {
			out = append(out, kv)
		}
	}
	return out
}

// start runs cmd to completion, forwarding interrupt/terminate signals to it,
// and returns an error carrying the child's exit code.
func (a *app) start(cmd *exec.Cmd) error {
	sigs := make(chan os.Signal, 4)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigs)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting %s: %w", cmd.Args[0], err)
	}
	done := make(chan struct{})
	go func() {
		for {
			select {
			case s := <-sigs:
				_ = cmd.Process.Signal(s)
			case <-done:
				return
			}
		}
	}()
	err := cmd.Wait()
	close(done)

	if err == nil {
		return nil
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code := ee.ExitCode()
		if ws, ok := ee.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			code = 128 + int(ws.Signal())
		}
		if code < 0 {
			code = exitRuntime
		}
		return &exitError{code: code}
	}
	return err
}
