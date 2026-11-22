package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/zalando/go-keyring"

	"github.com/RafaSG13/Pandora/internal/store"
)

type listEntry struct {
	Title    string   `json:"title"`
	Category string   `json:"category"`
	Created  string   `json:"created"`
	Expires  string   `json:"expires"`
	Tags     []string `json:"tags"`
}

func (a *app) cmdList(args []string) error {
	const usage = "list [--category C] [--tag T] [--json]"
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	var category string
	var tagArgs repeatFlag
	var asJSON bool
	fs.StringVar(&category, "category", "", "only secrets in this category (case-insensitive)")
	fs.StringVar(&category, "c", "", "shorthand for --category")
	fs.Var(&tagArgs, "tag", "only secrets with this tag; repeat or comma-separate to require several")
	fs.Var(&tagArgs, "t", "shorthand for --tag")
	fs.BoolVar(&asJSON, "json", false, "print JSON instead of a table")
	if _, err := a.parse("list", usage, fs, args, 0, 0); err != nil {
		return err
	}
	want, err := store.ParseTags(strings.Join(tagArgs, ","))
	if err != nil {
		return usageErrorf("list: invalid --tag: %v", err)
	}

	if err := a.unlock(); err != nil {
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
		if !hasAllTags(m.Tags, want) {
			continue
		}
		tags := m.Tags
		if tags == nil {
			tags = []string{}
		}
		entries = append(entries, listEntry{m.Title, m.Category, m.Created, m.Expires, tags})
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
	fmt.Fprintln(tw, "TITLE\tCATEGORY\tCREATED\tEXPIRES\tTAGS")
	for _, e := range entries {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", e.Title, e.Category, e.Created, e.Expires, strings.Join(e.Tags, ","))
	}
	return tw.Flush()
}

// hasAllTags reports whether have contains every tag in want (both are in
// normalised form).
func hasAllTags(have, want []string) bool {
	for _, w := range want {
		if !slices.Contains(have, w) {
			return false
		}
	}
	return true
}

func (a *app) cmdGet(args []string) error {
	const usage = "get <title> [-q] [--newline=false] [--copy] [--version -1]"
	fs := flag.NewFlagSet("get", flag.ContinueOnError)
	quiet := fs.Bool("q", false, "no trailing newline (same as --newline=false)")
	newline := fs.Bool("newline", true, "print a trailing newline")
	cp := fs.Bool("copy", false, "copy the value to the clipboard instead of printing it")
	version := fs.Int("version", 0, "0 is the current value; -1 the previous one, -2 the one before (see 'pandora history')")
	pos, err := a.parse("get", usage, fs, args, 1, 1)
	if err != nil {
		return err
	}
	back, err := checkVersionFlag("get", *version)
	if err != nil {
		return err
	}
	if err := a.unlock(); err != nil {
		return err
	}
	var value string
	if back == 0 {
		value, err = secretValue(pos[0])
	} else if value, err = store.GetVersion(pos[0], back); err != nil {
		err = versionError(pos[0], err)
	}
	if err != nil {
		return err
	}
	if *cp {
		if err := a.copySecret(value); err != nil {
			return fmt.Errorf("copy to clipboard: %w", err)
		}
		fmt.Fprintln(a.err, "pandora: copied to the clipboard; it is not cleared automatically (only the TUI clears it after its deadline)")
		return nil
	}
	if *quiet || !*newline {
		_, err = io.WriteString(a.out, value)
	} else {
		_, err = fmt.Fprintln(a.out, value)
	}
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
	const usage = "add <title> [-c CATEGORY] [--tag TAG] [--notes TEXT] [--expires YYYY-MM-DD]"
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	var category, notes, expires string
	var tagArgs repeatFlag
	fs.StringVar(&category, "c", "", "category")
	fs.StringVar(&category, "category", "", "category")
	fs.Var(&tagArgs, "t", "tag; repeat or comma-separate for several")
	fs.Var(&tagArgs, "tag", "tag; repeat or comma-separate for several")
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
	tags, err := store.ParseTags(strings.Join(tagArgs, ","))
	if err != nil {
		return usageErrorf("add: invalid --tag: %v", err)
	}

	if err := a.unlock(); err != nil {
		return err
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
		Tags:     tags,
	}
	if err := store.SaveSecret(meta, value); err != nil {
		switch {
		case errors.Is(err, store.ErrExists):
			return fmt.Errorf("secret %q already exists", title)
		case errors.Is(err, store.ErrInvalidText):
			return usageErrorf("add: %v", err)
		}
		return err
	}
	fmt.Fprintf(a.err, "Saved %q.\n", title)
	return nil
}

// readValue takes the secret from a hidden terminal prompt (one line), or from
// piped stdin (any number of lines, e.g. a private key) with one trailing
// newline removed. At most store.MaxValueBytes are accepted.
func (a *app) readValue(title string) (string, error) {
	if a.isTTY() {
		return a.readHidden(fmt.Sprintf("Value for %q (hidden): ", title))
	}
	// Room for a trailing "\r\n" plus one byte, so an oversized value is detected.
	data, err := io.ReadAll(io.LimitReader(a.in, store.MaxValueBytes+3))
	if err != nil {
		return "", err
	}
	v := string(data)
	if s, ok := strings.CutSuffix(v, "\n"); ok {
		v = strings.TrimSuffix(s, "\r")
	}
	if len(v) > store.MaxValueBytes {
		return "", store.ErrValueTooLarge
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

	if err := a.unlock(); err != nil {
		return err
	}
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
