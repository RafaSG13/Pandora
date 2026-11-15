package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/zalando/go-keyring"

	"github.com/RafaSG13/Pandora/internal/store"
)

type listEntry struct {
	Title    string `json:"title"`
	Category string `json:"category"`
	Created  string `json:"created"`
	Expires  string `json:"expires"`
}

func (a *app) cmdList(args []string) error {
	const usage = "list [--category C] [--json]"
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	var category string
	var asJSON bool
	fs.StringVar(&category, "category", "", "only secrets in this category (case-insensitive)")
	fs.StringVar(&category, "c", "", "shorthand for --category")
	fs.BoolVar(&asJSON, "json", false, "print JSON instead of a table")
	if _, err := a.parse("list", usage, fs, args, 0, 0); err != nil {
		return err
	}

	items, err := store.LoadMetadata()
	if err != nil {
		return err
	}
	entries := make([]listEntry, 0, len(items))
	for _, m := range items {
		if category != "" && !strings.EqualFold(m.Category, strings.TrimSpace(category)) {
			continue
		}
		entries = append(entries, listEntry{m.Title, m.Category, m.Created, m.Expires})
	}

	if asJSON {
		enc := json.NewEncoder(a.out)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)
	}
	if len(entries) == 0 {
		return nil
	}
	tw := tabwriter.NewWriter(a.out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "TITLE\tCATEGORY\tCREATED\tEXPIRES")
	for _, e := range entries {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", e.Title, e.Category, e.Created, e.Expires)
	}
	return tw.Flush()
}

func (a *app) cmdGet(args []string) error {
	fs := flag.NewFlagSet("get", flag.ContinueOnError)
	pos, err := a.parse("get", "get <title>", fs, args, 1, 1)
	if err != nil {
		return err
	}
	value, err := secretValue(pos[0])
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(a.out, value)
	return err
}

// secretValue reads a value from the keyring with a friendly not-found error.
func secretValue(title string) (string, error) {
	v, err := store.GetSecretValue(title)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", fmt.Errorf("secret %q not found", title)
	}
	return v, err
}

func (a *app) cmdAdd(args []string) error {
	const usage = "add <title> [-c CATEGORY] [--notes TEXT] [--expires YYYY-MM-DD]"
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	var category, notes, expires string
	fs.StringVar(&category, "c", "", "category")
	fs.StringVar(&category, "category", "", "category")
	fs.StringVar(&notes, "notes", "", "free-form notes")
	fs.StringVar(&expires, "expires", "", "expiry date, YYYY-MM-DD")
	pos, err := a.parse("add", usage, fs, args, 1, 1)
	if err != nil {
		return err
	}

	title := strings.TrimSpace(pos[0])
	if title == "" {
		return usageErrorf("add: title must not be empty")
	}
	expires = strings.TrimSpace(expires)
	if _, err := store.ParseDate(expires); err != nil {
		return usageErrorf("add: invalid --expires %q: %v", expires, err)
	}

	value, err := a.readValue(title)
	if err != nil {
		return err
	}
	if value == "" {
		return errors.New("refusing to store an empty value")
	}

	meta := store.SecretMeta{
		Title:    title,
		Category: strings.TrimSpace(category),
		Created:  time.Now().Format(store.DateLayout),
		Expires:  expires,
		Notes:    notes,
	}
	if err := store.SaveSecret(meta, value); err != nil {
		if errors.Is(err, store.ErrExists) {
			return fmt.Errorf("secret %q already exists", title)
		}
		return err
	}
	fmt.Fprintf(a.err, "Saved %q.\n", title)
	return nil
}

// readValue takes the secret from a hidden terminal prompt, or from piped
// stdin with one trailing newline removed.
func (a *app) readValue(title string) (string, error) {
	if a.isTTY() {
		return a.readHidden(fmt.Sprintf("Value for %q (hidden): ", title))
	}
	data, err := io.ReadAll(a.in)
	if err != nil {
		return "", err
	}
	v := string(data)
	if s, ok := strings.CutSuffix(v, "\n"); ok {
		v = strings.TrimSuffix(s, "\r")
	}
	return v, nil
}

func (a *app) cmdRm(args []string) error {
	fs := flag.NewFlagSet("rm", flag.ContinueOnError)
	var yes bool
	fs.BoolVar(&yes, "y", false, "delete without asking")
	fs.BoolVar(&yes, "yes", false, "delete without asking")
	pos, err := a.parse("rm", "rm <title> [-y]", fs, args, 1, 1)
	if err != nil {
		return err
	}
	title := pos[0]

	items, err := store.LoadMetadata()
	if err != nil {
		return err
	}
	if !hasTitle(items, title) {
		return fmt.Errorf("secret %q not found", title)
	}
	if err := a.requireConfirmation(yes, fmt.Sprintf("Delete %q?", title)); err != nil {
		return err
	}
	if err := store.DeleteSecret(title); err != nil {
		return err
	}
	fmt.Fprintf(a.err, "Deleted %q.\n", title)
	return nil
}

func hasTitle(items []store.SecretMeta, title string) bool {
	for _, m := range items {
		if m.Title == title {
			return true
		}
	}
	return false
}

func (a *app) cmdDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	var fix, yes bool
	fs.BoolVar(&fix, "fix", false, "remove metadata entries whose value is missing")
	fs.BoolVar(&yes, "y", false, "with --fix, remove without asking")
	fs.BoolVar(&yes, "yes", false, "with --fix, remove without asking")
	if _, err := a.parse("doctor", "doctor [--fix] [-y]", fs, args, 0, 0); err != nil {
		return err
	}

	items, err := store.LoadMetadata()
	if err != nil {
		return err
	}
	missing, err := store.MissingValues()
	if err != nil {
		return err
	}

	fmt.Fprintf(a.out, "Checked %d metadata entries.\n", len(items))
	if len(missing) == 0 {
		fmt.Fprintln(a.out, "OK: every entry has a value in the keyring.")
	} else {
		fmt.Fprintf(a.out, "%d entries have no value in the keyring:\n", len(missing))
		for _, t := range missing {
			fmt.Fprintf(a.out, "  %s\n", t)
		}
	}
	fmt.Fprintln(a.out, "Note: keyring entries without metadata cannot be detected; the keyring API cannot list entries.")

	if len(missing) == 0 {
		return nil
	}
	if !fix {
		return &exitError{code: exitRuntime, msg: "inconsistencies found; run 'pandora doctor --fix' to remove the orphaned metadata entries"}
	}
	if err := a.requireConfirmation(yes, fmt.Sprintf("Remove %d metadata entries without a value?", len(missing))); err != nil {
		return err
	}
	for _, t := range missing {
		if err := store.DeleteSecret(t); err != nil {
			return fmt.Errorf("removing %q: %w", t, err)
		}
	}
	fmt.Fprintf(a.out, "Removed %d metadata entries.\n", len(missing))
	return nil
}
