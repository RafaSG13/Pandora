package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"filippo.io/age"
	"filippo.io/age/armor"
	"github.com/zalando/go-keyring"

	"github.com/RafaSG13/Pandora/internal/store"
)

const (
	exportFormat  = "pandora-export"
	exportVersion = 1

	armorHeader  = "-----BEGIN AGE ENCRYPTED FILE-----"
	binaryHeader = "age-encryption.org/"
)

// exportFile is the export document. Without --values it is written as plain
// JSON and no secret carries a value. With --values the same document,
// values included, is only ever written encrypted (age, scrypt passphrase).
type exportFile struct {
	Format         string         `json:"format"`
	Version        int            `json:"version"`
	Exported       string         `json:"exported"`
	ValuesIncluded bool           `json:"values_included"`
	Secrets        []exportSecret `json:"secrets"`
}

type exportSecret struct {
	store.SecretMeta
	Value *string `json:"value,omitempty"`
}

func (a *app) cmdExport(args []string) error {
	const usage = "export [--values] [-o FILE] [--force]"
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	var withValues, force bool
	var output string
	fs.BoolVar(&withValues, "values", false, "include secret values; the output is then encrypted with a passphrase (prompted twice, or from "+passphraseEnv+")")
	fs.StringVar(&output, "o", "", "write to `FILE` instead of stdout")
	fs.StringVar(&output, "output", "", "write to `FILE` instead of stdout")
	fs.BoolVar(&force, "force", false, "overwrite an existing output file")
	if _, err := a.parse("export", usage, fs, args, 0, 0); err != nil {
		return err
	}
	toStdout := output == "" || output == "-"
	if !toStdout && !force {
		if _, err := os.Lstat(output); err == nil {
			return fmt.Errorf("%s already exists (use --force to overwrite)", output)
		}
	}

	if err := a.unlock(); err != nil {
		return err
	}
	items, err := store.LoadMetadata()
	if err != nil {
		return err
	}
	doc := exportFile{
		Format:         exportFormat,
		Version:        exportVersion,
		Exported:       time.Now().UTC().Format(time.RFC3339),
		ValuesIncluded: withValues,
		Secrets:        make([]exportSecret, 0, len(items)),
	}
	var missing []string
	for _, m := range items {
		s := exportSecret{SecretMeta: m}
		if withValues {
			v, err := store.GetSecretValue(m.Title)
			switch {
			case errors.Is(err, keyring.ErrNotFound):
				missing = append(missing, m.Title)
				continue
			case err != nil:
				return fmt.Errorf("reading %q from the keyring: %w", m.Title, err)
			}
			s.Value = &v
		}
		doc.Secrets = append(doc.Secrets, s)
	}
	if len(missing) > 0 {
		return fmt.Errorf("no keyring value for: %s (run 'pandora doctor')", strings.Join(missing, ", "))
	}

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if withValues {
		pass, err := a.passphrase(true)
		if err != nil {
			return err
		}
		if data, err = a.encrypt(data, pass); err != nil {
			return err
		}
	}

	recordExport := func() {
		if !withValues {
			a.audit("export", "") // titles are written in plain text; no value was read
			return
		}
		for _, s := range doc.Secrets {
			a.audit("export", s.Title)
		}
	}
	if toStdout {
		if _, err = a.out.Write(data); err != nil {
			return err
		}
		recordExport()
		return nil
	}
	if err := writeFile(output, data, force); err != nil {
		return err
	}
	recordExport()
	kind := "plain-text metadata (titles are not encrypted)"
	if withValues {
		kind = "encrypted metadata and values"
	}
	fmt.Fprintf(a.err, "Exported %d secrets (%s) to %s.\n", len(doc.Secrets), kind, output)
	return nil
}

// writeFile writes data with mode 0600. Without force an existing file is an
// error; with force it is replaced atomically.
func writeFile(path string, data []byte, force bool) error {
	if !force {
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		if _, err := f.Write(data); err != nil {
			f.Close()
			os.Remove(path)
			return err
		}
		if err := f.Close(); err != nil {
			os.Remove(path)
			return err
		}
		return nil
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".pandora-export-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path) // CreateTemp files are 0600
}

// passphrase returns the export passphrase from the environment or, on a
// terminal, from a hidden prompt (asked twice when confirm is set).
func (a *app) passphrase(confirm bool) (string, error) {
	if p := a.getenv(passphraseEnv); p != "" {
		if confirm {
			if err := checkPassphraseLen(p); err != nil {
				return "", err
			}
			a.warnEnv(passphraseEnv)
		}
		return p, nil
	}
	if !a.isTTY() {
		return "", fmt.Errorf("no passphrase: set %s or run in a terminal", passphraseEnv)
	}
	p, err := a.readHidden("Passphrase: ")
	if err != nil {
		return "", err
	}
	if p == "" {
		return "", errors.New("the passphrase must not be empty")
	}
	if confirm {
		if err := checkPassphraseLen(p); err != nil {
			return "", err
		}
		again, err := a.readHidden("Confirm passphrase: ")
		if err != nil {
			return "", err
		}
		if again != p {
			return "", errors.New("passphrases do not match")
		}
	}
	return p, nil
}

// checkPassphraseLen enforces the minimum length of a new export passphrase.
func checkPassphraseLen(p string) error {
	if utf8.RuneCountInString(p) < store.MinPasswordLen {
		return fmt.Errorf("passphrase must be at least %d characters", store.MinPasswordLen)
	}
	return nil
}

// encrypt returns the ASCII-armored age file for data, protected by pass.
func (a *app) encrypt(data []byte, pass string) ([]byte, error) {
	rcpt, err := age.NewScryptRecipient(pass)
	if err != nil {
		return nil, err
	}
	if a.scryptWorkFactor != 0 {
		rcpt.SetWorkFactor(a.scryptWorkFactor)
	}
	var buf bytes.Buffer
	armored := armor.NewWriter(&buf)
	w, err := age.Encrypt(armored, rcpt)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	if err := armored.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func isEncrypted(data []byte) bool {
	data = bytes.TrimLeft(data, " \t\r\n")
	return bytes.HasPrefix(data, []byte(armorHeader)) || bytes.HasPrefix(data, []byte(binaryHeader))
}

func decrypt(data []byte, pass string) ([]byte, error) {
	id, err := age.NewScryptIdentity(pass)
	if err != nil {
		return nil, err
	}
	var src io.Reader = bytes.NewReader(data)
	if bytes.HasPrefix(bytes.TrimLeft(data, " \t\r\n"), []byte(armorHeader)) {
		src = armor.NewReader(src)
	}
	r, err := age.Decrypt(src, id)
	if err != nil {
		var noMatch *age.NoIdentityMatchError
		if errors.As(err, &noMatch) {
			return nil, errors.New("cannot decrypt: wrong passphrase, or not a passphrase-encrypted Pandora export")
		}
		return nil, fmt.Errorf("cannot decrypt: %w", err)
	}
	plain, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("cannot decrypt: file is corrupt or truncated: %w", err)
	}
	return plain, nil
}

func (a *app) cmdImport(args []string) error {
	const usage = "import FILE [--format FORMAT] [--values] [--overwrite] [--dry-run] [--category C]"
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	var withValues, overwrite, dryRun bool
	var format, category string
	fs.StringVar(&format, "format", formatPandora, "format of FILE: "+strings.Join(importFormats, ", "))
	fs.BoolVar(&withValues, "values", false, "restore the secret values contained in the file (needs an encrypted Pandora export made with --values)")
	fs.BoolVar(&overwrite, "overwrite", false, "replace secrets that already exist instead of skipping them")
	fs.BoolVar(&dryRun, "dry-run", false, "list what would be imported, replaced and skipped, and change nothing")
	fs.StringVar(&category, "category", "", "with --format dotenv: category for every imported secret")
	pos, err := a.parse("import", usage, fs, args, 1, 1)
	if err != nil {
		return err
	}
	if !slices.Contains(importFormats, format) {
		return usageErrorf("import: unknown --format %q (valid: %s)", format, strings.Join(importFormats, ", "))
	}
	external := format != formatPandora
	if external && withValues {
		return usageErrorf("import: --values only applies to --format %s; %s files always carry their values", formatPandora, format)
	}
	if category != "" && format != "dotenv" {
		return usageErrorf("import: --category only applies to --format dotenv")
	}
	if err := store.ValidateText(category); err != nil {
		return usageErrorf("import: invalid --category: %v", err)
	}
	if err := a.unlock(); err != nil {
		return err
	}

	var data []byte
	if pos[0] == "-" {
		data, err = io.ReadAll(a.in)
	} else {
		data, err = os.ReadFile(pos[0])
	}
	if err != nil {
		return err
	}

	var doc *exportFile
	var batch *importBatch
	if external {
		if isEncrypted(data) {
			return fmt.Errorf("%s is an age-encrypted file; --format %s reads plain-text exports", pos[0], format)
		}
		if batch, err = externalParsers[format](data, importOptions{category: category}); err != nil {
			return fmt.Errorf("%s: %w", pos[0], err)
		}
		doc = &exportFile{Secrets: batch.secrets}
		withValues = true
	} else {
		if isEncrypted(data) {
			pass, err := a.passphrase(false)
			if err != nil {
				return err
			}
			if data, err = decrypt(data, pass); err != nil {
				return err
			}
		}
		if doc, err = parseExport(data); err != nil {
			return fmt.Errorf("%s: %w", pos[0], err)
		}
		hasValues := false
		for _, s := range doc.Secrets {
			hasValues = hasValues || s.Value != nil
		}
		switch {
		case withValues && !hasValues:
			return errors.New("--values given but the file contains no values (was it exported with --values?)")
		case !withValues && hasValues:
			fmt.Fprintln(a.err, "Note: the file contains values; pass --values to restore them. Importing metadata only.")
		}
	}

	existing, err := store.LoadMetadata()
	if err != nil {
		return err
	}
	var restore []store.Restore
	var plan []string
	var imported, replaced, skipped, withoutValue int
	for _, s := range doc.Secrets {
		exists := hasTitle(existing, s.Title)
		if exists && !overwrite {
			skipped++
			plan = append(plan, "  skip     "+s.Title+" (already exists)")
			continue
		}
		r := store.Restore{Meta: s.SecretMeta}
		if withValues {
			r.Value = s.Value
		}
		switch {
		case exists:
			replaced++
			plan = append(plan, "  replace  "+s.Title)
		case r.Value == nil:
			imported++
			withoutValue++
			plan = append(plan, "  import   "+s.Title+" (no value)")
		default:
			imported++
			plan = append(plan, "  import   "+s.Title)
		}
		restore = append(restore, r)
	}

	if batch != nil {
		for _, n := range batch.notes {
			fmt.Fprintf(a.err, "Note: %s\n", n)
		}
		if len(batch.skipped) > 0 {
			fmt.Fprintf(a.err, "Not imported (%d):\n", len(batch.skipped))
			for _, s := range batch.skipped {
				fmt.Fprintf(a.err, "  %s\n", s)
			}
		}
	}
	if dryRun {
		for _, line := range plan {
			fmt.Fprintln(a.out, line)
		}
		fmt.Fprintf(a.out, "Dry run, nothing written. Would import %d, replace %d, skip %d (already exist).\n", imported, replaced, skipped)
		return nil
	}
	if len(restore) > 0 {
		if err := store.RestoreSecrets(restore); err != nil {
			return err
		}
	}

	for _, r := range restore {
		a.audit("import", r.Meta.Title)
	}
	fmt.Fprintf(a.out, "Imported %d, replaced %d, skipped %d (already exist).\n", imported, replaced, skipped)
	if withoutValue > 0 {
		fmt.Fprintf(a.out, "%d imported entries have no value; 'pandora doctor' lists them.\n", withoutValue)
	}
	if external && pos[0] != "-" {
		fmt.Fprintf(a.err, "Note: %s holds your secrets in plain text; delete it once you have checked the import.\n", pos[0])
	}
	return nil
}

// parseExport decodes and validates a whole export document before anything
// is written.
func parseExport(data []byte) (*exportFile, error) {
	var doc exportFile
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("not a Pandora export: %w", err)
	}
	if doc.Format != exportFormat {
		return nil, errors.New("not a Pandora export (missing or unknown format)")
	}
	if doc.Version != exportVersion {
		return nil, fmt.Errorf("unsupported export version %d (this build reads version %d)", doc.Version, exportVersion)
	}
	today := time.Now().Format(store.DateLayout)
	seen := make(map[string]bool, len(doc.Secrets))
	for i := range doc.Secrets {
		s := &doc.Secrets[i]
		if err := normalizeSecret(s, today); err != nil {
			return nil, fmt.Errorf("secret #%d (%q): %w", i+1, s.Title, err)
		}
		if seen[s.Title] {
			return nil, fmt.Errorf("secret %q appears more than once", s.Title)
		}
		seen[s.Title] = true
	}
	return &doc, nil
}

// normalizeSecret validates one secret read from a file and normalises it in
// place: tags are cleaned and a missing creation date becomes today. Its
// errors never contain the value.
func normalizeSecret(s *exportSecret, today string) error {
	if strings.TrimSpace(s.Title) == "" {
		return errors.New("empty title")
	}
	for _, text := range []string{s.Title, s.Category, s.Notes} {
		if err := store.ValidateText(text); err != nil {
			return err
		}
	}
	tags, err := store.NormalizeTags(s.Tags)
	if err != nil {
		return err
	}
	s.Tags = tags
	for _, f := range []struct{ name, v string }{{"created", s.Created}, {"updated", s.Updated}, {"expires", s.Expires}} {
		if _, err := store.ParseDate(f.v); err != nil {
			return fmt.Errorf("invalid %s %q: %v", f.name, f.v, err)
		}
	}
	if s.Value != nil && *s.Value == "" {
		return errors.New("empty value")
	}
	if s.Value != nil && len(*s.Value) > store.MaxValueBytes {
		return store.ErrValueTooLarge
	}
	if s.Created == "" {
		s.Created = today
	}
	return nil
}
