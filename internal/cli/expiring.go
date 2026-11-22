package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/RafaSG13/Pandora/internal/config"
	"github.com/RafaSG13/Pandora/internal/store"
)

// expiringEntry is one element of `expiring --json`. Days is the whole
// calendar days until Expires, negative once it has passed.
type expiringEntry struct {
	Title    string `json:"title"`
	Category string `json:"category"`
	Expires  string `json:"expires"`
	Days     int    `json:"days"`
	Status   string `json:"status"` // "expired", or "expiring" for today and the days ahead
}

// parseWithin reads a --within value: a whole number of days, optionally
// followed by "d" (14 or 14d).
func parseWithin(s string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimSpace(s), "d"))
	if err != nil || n < 0 || n > config.MaxExpiryWarnDays {
		return 0, fmt.Errorf("--within must be a number of days between 0 and %d (for example 14d)", config.MaxExpiryWarnDays)
	}
	return n, nil
}

// cmdExpiring lists the secrets that have expired or expire within the
// window. It exits with exitExpiring when the list is not empty, so cron and
// CI can act on it, and never reads a value.
func (a *app) cmdExpiring(args []string) error {
	const usage = "expiring [--within 14d] [--json]"
	fs := flag.NewFlagSet("expiring", flag.ContinueOnError)
	var within string
	var asJSON bool
	fs.StringVar(&within, "within", "", "window in days, e.g. 14d (default expiry_warn_days from the config file, 14)")
	fs.BoolVar(&asJSON, "json", false, "print JSON instead of a table")
	if _, err := a.parse("expiring", usage, fs, args, 0, 0); err != nil {
		return err
	}
	var window int
	var withinSet bool
	fs.Visit(func(f *flag.Flag) { withinSet = withinSet || f.Name == "within" })
	if withinSet {
		n, err := parseWithin(within)
		if err != nil {
			return usageErrorf("expiring: %v", err)
		}
		window = n
	} else {
		window = a.loadConfig().ExpiryWarnDays
	}

	if err := a.unlock(); err != nil {
		return err
	}
	items, err := store.LoadMetadata()
	if err != nil {
		return err
	}
	now := a.now()
	entries := []expiringEntry{}
	for _, m := range items {
		days, ok := m.DaysUntilExpiry(now)
		if !ok || days > window {
			continue
		}
		status := "expiring"
		if days < 0 {
			status = "expired"
		}
		entries = append(entries, expiringEntry{m.Title, m.Category, m.Expires, days, status})
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Days < entries[j].Days })

	if asJSON {
		enc := json.NewEncoder(a.out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(entries); err != nil {
			return err
		}
	} else if len(entries) > 0 {
		tw := tabwriter.NewWriter(a.out, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "TITLE\tCATEGORY\tEXPIRES\tSTATUS")
		for _, e := range entries {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", e.Title, e.Category, e.Expires, expiryText(e.Days))
		}
		if err := tw.Flush(); err != nil {
			return err
		}
	}
	if len(entries) > 0 {
		return &exitError{code: exitExpiring}
	}
	return nil
}

// expiryText words the days until an expiry the way the TUI does.
func expiryText(days int) string {
	switch {
	case days < 0:
		return fmt.Sprintf("expired %dd ago", -days)
	case days == 0:
		return "expires today"
	default:
		return fmt.Sprintf("expires in %dd", days)
	}
}
