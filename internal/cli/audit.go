package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/RafaSG13/Pandora/internal/store"
)

// audit records an action in the audit log when audit_log is on (store
// decides; it does nothing otherwise). A failure is reported once on stderr
// and never changes what the command does or its exit code.
func (a *app) audit(action, title string) {
	if err := store.RecordAudit(action, title); err != nil && !a.auditWarned {
		a.auditWarned = true
		fmt.Fprintf(a.err, "pandora: warning: could not write the audit log: %v\n", err)
	}
}

// parseSince reads a `log --since` value: a number of days ("7d"), hours
// ("12h") or a date (YYYY-MM-DD, local midnight). now is the reference for the
// relative forms.
func parseSince(s string, now time.Time) (time.Time, error) {
	s = strings.TrimSpace(s)
	if unit := s[max(0, len(s)-1):]; unit == "d" || unit == "h" {
		n, err := strconv.Atoi(s[:len(s)-1])
		if err == nil && n >= 0 {
			d := 24 * time.Hour
			if unit == "h" {
				d = time.Hour
			}
			return now.Add(-time.Duration(n) * d), nil
		}
	}
	t, err := store.ParseDate(s)
	if err != nil || t.IsZero() {
		return time.Time{}, fmt.Errorf("--since must be a duration like 7d or 12h, or a date YYYY-MM-DD")
	}
	return t, nil
}

// logEntry is one element of `log --json`.
type logEntry struct {
	Time   string `json:"time"` // RFC 3339, UTC
	Action string `json:"action"`
	Title  string `json:"title"`
}

// cmdLog prints the audit log, oldest first: when, what, and which secret,
// never a value. It needs the master password because the log is encrypted.
func (a *app) cmdLog(args []string) error {
	const usage = "log [--since 7d|12h|YYYY-MM-DD] [--json]"
	fs := flag.NewFlagSet("log", flag.ContinueOnError)
	var since string
	var asJSON bool
	fs.StringVar(&since, "since", "", "only entries from this far back (7d, 12h) or from this date (YYYY-MM-DD)")
	fs.BoolVar(&asJSON, "json", false, "print JSON instead of a table")
	if _, err := a.parse("log", usage, fs, args, 0, 0); err != nil {
		return err
	}
	var from time.Time
	var sinceSet bool
	fs.Visit(func(f *flag.Flag) { sinceSet = sinceSet || f.Name == "since" })
	if sinceSet {
		t, err := parseSince(since, a.now())
		if err != nil {
			return usageErrorf("log: %v", err)
		}
		from = t
	}

	if err := a.unlock(); err != nil {
		return err
	}
	entries, unreadable, err := store.ReadAudit(from)
	if err != nil {
		return err
	}
	if unreadable > 0 {
		fmt.Fprintf(a.err, "pandora: warning: %d audit log lines could not be read (damaged, or written under another key)\n", unreadable)
	}
	if len(entries) == 0 && !store.AuditExists() && !a.loadConfig().AuditLog {
		fmt.Fprintln(a.err, "pandora: the audit log is off; set \"audit_log\": true in config.json to record actions")
	}

	if asJSON {
		out := make([]logEntry, len(entries))
		for i, e := range entries {
			out[i] = logEntry{e.Time.UTC().Format(time.RFC3339), e.Action, e.Title}
		}
		enc := json.NewEncoder(a.out)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}
	if len(entries) == 0 {
		return nil
	}
	tw := tabwriter.NewWriter(a.out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "TIME\tACTION\tTITLE")
	for _, e := range entries {
		title := e.Title
		if title == "" {
			title = "-"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", e.Time.Local().Format("2006-01-02 15:04:05"), e.Action, title)
	}
	return tw.Flush()
}
