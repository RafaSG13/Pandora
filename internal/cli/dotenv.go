package cli

import (
	"flag"
	"regexp"
	"strings"

	"github.com/RafaSG13/Pandora/internal/termsafe"
)

// dotenvBare matches values that need no quoting in a dotenv file.
var dotenvBare = regexp.MustCompile(`^[A-Za-z0-9_./:@%+-]*$`)

// dotenvKey matches the variable names `env` is willing to print.
var dotenvKey = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

// dotenvEscaper escapes what is special inside a double-quoted dotenv value.
var dotenvEscaper = strings.NewReplacer(
	`\`, `\\`,
	`"`, `\"`,
	"\n", `\n`,
	"\r", `\r`,
	"$", `\$`,
	"`", "\\`",
)

// dotenvValue renders a value for a KEY=value line: bare when it only holds
// safe characters, otherwise double-quoted with \\, \", \n, \r, \$ and \` escaped
// (no parser or shell then expands variables, runs commands or splits the line).
func dotenvValue(v string) string {
	if dotenvBare.MatchString(v) {
		return v
	}
	return `"` + dotenvEscaper.Replace(v) + `"`
}

func (a *app) cmdEnv(args []string) error {
	const usage = "env -s TITLE[=ENV_NAME] [-s ...] [--force]"
	fs := flag.NewFlagSet("env", flag.ContinueOnError)
	var specs repeatFlag
	fs.Var(&specs, "s", "secret to print, `TITLE[=ENV_NAME]`; ENV_NAME defaults to the title upper-cased with other characters as _ (repeatable)")
	fs.Var(&specs, "secret", "same as -s")
	force := fs.Bool("force", false, "print values even when stdout is a terminal")
	if _, err := a.parse("env", usage, fs, args, 0, 0); err != nil {
		return err
	}
	if len(specs) == 0 {
		return usageErrorf("env: at least one -s is required\nusage: pandora %s", usage)
	}
	injections, err := parseInjections("env", specs)
	if err != nil {
		return err
	}
	for _, inj := range injections {
		if !dotenvKey.MatchString(inj.env) {
			return usageErrorf("env: %q is not a usable dotenv variable name; use TITLE=ENV_NAME", inj.env)
		}
	}
	if a.isOutTTY() && !*force {
		return usageErrorf("env: refusing to print secret values to a terminal; redirect the output or pass --force")
	}

	if err := a.unlock(); err != nil {
		return err
	}
	// Resolve every secret before printing anything.
	var b strings.Builder
	for _, inj := range injections {
		v, err := secretValue(inj.title)
		if err != nil {
			return err
		}
		b.WriteString(inj.env + "=" + dotenvValue(v) + "\n")
	}
	for _, inj := range injections {
		a.audit("env", inj.title)
	}
	out := b.String()
	if a.isOutTTY() { // only possible with --force
		out = termsafe.Sanitize(out)
	}
	_, err = a.out.Write([]byte(out))
	return err
}
