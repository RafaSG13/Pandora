package cli

import (
	"flag"
	"fmt"
	"strings"

	"github.com/RafaSG13/Pandora/internal/config"
)

// loadConfig reads config.json merged with the environment. Problems in the
// file are reported on stderr and never change the exit code.
func (a *app) loadConfig() config.Settings {
	s, warns := config.Load(a.getenv)
	for _, w := range warns {
		fmt.Fprintf(a.err, "pandora: warning: %s\n", w)
	}
	return s
}

// cmdConfig shows where the config file lives or what is in effect. It is
// read-only: the file is edited by hand. It does not unlock the vault.
func (a *app) cmdConfig(args []string) error {
	const usage = "config path | config get [KEY]"
	pos, err := a.parse("config", usage, flag.NewFlagSet("config", flag.ContinueOnError), args, 1, 2)
	if err != nil {
		return err
	}

	switch pos[0] {
	case "path":
		if len(pos) != 1 {
			return usageErrorf("config: path takes no argument")
		}
		path, err := config.Path()
		if err != nil {
			return fmt.Errorf("locating the config file: %w", err)
		}
		_, err = fmt.Fprintln(a.out, path)
		return err

	case "get":
		s := a.loadConfig()
		if len(pos) == 2 {
			v, ok := s.Get(pos[1])
			if !ok {
				return usageErrorf("config: unknown key %q (valid: %s)", pos[1], strings.Join(config.Keys, ", "))
			}
			_, err := fmt.Fprintln(a.out, v)
			return err
		}
		var b strings.Builder
		for _, k := range config.Keys {
			v, _ := s.Get(k)
			fmt.Fprintf(&b, "%s = %s\n", k, v)
		}
		_, err := fmt.Fprint(a.out, b.String())
		return err
	}
	return usageErrorf("config: unknown subcommand %q (valid: path, get)", pos[0])
}
