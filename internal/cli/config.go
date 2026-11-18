package cli

import (
	"flag"
	"fmt"
	"strings"

	"github.com/RafaSG13/Pandora/internal/config"
	"github.com/RafaSG13/Pandora/internal/store"
)

// cmdConfig shows where the settings file is and the effective settings. It
// reads only config.json and the environment: no master password, no vault.
func (a *app) cmdConfig(args []string) error {
	const usage = "config path | config get [KEY]"
	pos, err := a.parse("config", usage, flag.NewFlagSet("config", flag.ContinueOnError), args, 1, 2)
	if err != nil {
		return err
	}
	dir, _ := store.ConfigDir() // "" when there is no config directory

	switch {
	case pos[0] == "path" && len(pos) == 1:
		if dir == "" {
			return fmt.Errorf("no user config directory on this system")
		}
		_, err := fmt.Fprintln(a.out, config.Path(dir))
		return err
	case pos[0] == "get":
		cfg, warns := config.Load(dir, a.getenv)
		for _, w := range warns {
			fmt.Fprintf(a.err, "pandora: warning: %s\n", w)
		}
		if len(pos) == 2 {
			v, ok := cfg.Value(pos[1])
			if !ok {
				return usageErrorf("config: unknown key %q (keys: %s)", pos[1], strings.Join(config.Keys(), ", "))
			}
			_, err := fmt.Fprintln(a.out, v)
			return err
		}
		for _, k := range config.Keys() {
			v, _ := cfg.Value(k)
			fmt.Fprintf(a.out, "%s = %s\n", k, v)
		}
		return nil
	}
	return usageErrorf("config: expected \"path\" or \"get [KEY]\"")
}
