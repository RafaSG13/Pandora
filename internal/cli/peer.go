package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/RafaSG13/Pandora/internal/store"
)

func (a *app) cmdID(args []string) error {
	if _, err := a.parse("id", "id", flag.NewFlagSet("id", flag.ContinueOnError), args, 0, 0); err != nil {
		return err
	}
	if err := a.unlock(); err != nil {
		return err
	}
	key, err := store.ShareKey()
	if err != nil {
		return err
	}
	fmt.Fprintln(a.out, key)
	fmt.Fprintf(a.err, "Fingerprint: %s\n", store.Fingerprint(key))
	return nil
}

func (a *app) cmdPeer(args []string) error {
	const usage = "peer add NAME KEY|KEYFILE [-y] | peer list [--json] | peer rm NAME [-y]"
	fs := flag.NewFlagSet("peer", flag.ContinueOnError)
	var yes, asJSON bool
	fs.BoolVar(&yes, "y", false, "with add or rm, do not ask")
	fs.BoolVar(&yes, "yes", false, "with add or rm, do not ask")
	fs.BoolVar(&asJSON, "json", false, "with list, print JSON")
	pos, err := a.parse("peer", usage, fs, args, 1, 3)
	if err != nil {
		return err
	}
	switch pos[0] {
	case "add":
		if len(pos) != 3 {
			return usageErrorf("peer: add takes NAME and KEY (the output of 'pandora id', or a file containing it)")
		}
		if asJSON {
			return usageErrorf("peer: --json only applies to list")
		}
		name, key := pos[1], pos[2]
		if !strings.HasPrefix(strings.TrimSpace(key), store.ShareKeyPrefix) {
			b, err := os.ReadFile(key)
			if err != nil {
				return fmt.Errorf("reading the key file: %w", err)
			}
			key = string(b)
		}
		key = strings.TrimSpace(key)
		if err := a.unlock(); err != nil {
			return err
		}
		if err := store.CheckPeer(name, key); err != nil {
			if errors.Is(err, store.ErrInvalidPeerName) || errors.Is(err, store.ErrInvalidShareKey) {
				return usageErrorf("peer add: %v", err)
			}
			return err
		}
		fmt.Fprintf(a.err, "Peer %q fingerprint: %s\n", name, store.Fingerprint(key))
		if err := a.requireConfirmation(yes, "Does it match the fingerprint 'pandora id' prints on that machine?"); err != nil {
			return err
		}
		if err := store.AddPeer(name, key); err != nil {
			return err
		}
		fmt.Fprintf(a.err, "Trusted peer %q.\n", name)
		return nil

	case "list":
		if len(pos) != 1 {
			return usageErrorf("peer: list takes no argument")
		}
		if yes {
			return usageErrorf("peer: -y only applies to add and rm")
		}
		if err := a.unlock(); err != nil {
			return err
		}
		peers, err := store.Peers()
		if err != nil {
			return err
		}
		if asJSON {
			type row struct {
				Name        string `json:"name"`
				Fingerprint string `json:"fingerprint"`
			}
			rows := make([]row, 0, len(peers))
			for _, p := range peers {
				rows = append(rows, row{p.Name, store.Fingerprint(p.Key)})
			}
			b, err := json.Marshal(rows)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(a.out, string(b))
			return err
		}
		if len(peers) == 0 {
			fmt.Fprintln(a.err, "No peers. Add one with 'pandora peer add NAME KEY'.")
			return nil
		}
		width := 0
		for _, p := range peers {
			width = max(width, len(p.Name))
		}
		for _, p := range peers {
			fmt.Fprintf(a.out, "%-*s  %s\n", width, p.Name, store.Fingerprint(p.Key))
		}
		return nil

	case "rm":
		if len(pos) != 2 {
			return usageErrorf("peer: rm takes a NAME")
		}
		if asJSON {
			return usageErrorf("peer: --json only applies to list")
		}
		name := pos[1]
		if err := a.unlock(); err != nil {
			return err
		}
		peers, err := store.Peers()
		if err != nil {
			return err
		}
		found := false
		for _, p := range peers {
			found = found || p.Name == name
		}
		if !found {
			return fmt.Errorf("peer %q not found", name)
		}
		if err := a.requireConfirmation(yes, fmt.Sprintf("Stop trusting peer %q?", name)); err != nil {
			return err
		}
		if err := store.RemovePeer(name); err != nil {
			return err
		}
		fmt.Fprintf(a.err, "Removed peer %q.\n", name)
		return nil
	}
	return usageErrorf("peer: unknown subcommand %q (valid: add, list, rm)", pos[0])
}
