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
	"strings"
	"time"

	"filippo.io/age"
	"filippo.io/age/armor"
	"github.com/zalando/go-keyring"

	"pandora/internal/store"
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

	if toStdout {
		_, err = a.out.Write(data)
		return err
	}
	if err := writeFile(output, data, force); err != nil {
		return err
	}
	kind := "metadata"
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
	const usage = "import FILE [--values] [--overwrite]"
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	var withValues, overwrite bool
	fs.BoolVar(&withValues, "values", false, "restore the secret values contained in the file (needs an encrypted export made with --values)")
	fs.BoolVar(&overwrite, "overwrite", false, "replace secrets that already exist instead of skipping them")
	pos, err := a.parse("import", usage, fs, args, 1, 1)
	if err != nil {
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
	if isEncrypted(data) {
		pass, err := a.passphrase(false)
		if err != nil {
			return err
		}
		if data, err = decrypt(data, pass); err != nil {
			return err
		}
	}
	doc, err := parseExport(data)
	if err != nil {
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

	existing, err := store.LoadMetadata()
	if err != nil {
		return err
	}
	var restore []store.Restore
	var imported, replaced, skipped, withoutValue int
	for _, s := range doc.Secrets {
		exists := hasTitle(existing, s.Title)
		if exists && !overwrite {
			skipped++
			continue
		}
		r := store.Restore{Meta: s.SecretMeta}
		if withValues {
			r.Value = s.Value
		}
		switch {
		case exists:
			replaced++
		case r.Value == nil:
			imported++
			withoutValue++
		default:
			imported++
		}
		restore = append(restore, r)
	}
	if len(restore) > 0 {
		if err := store.RestoreSecrets(restore); err != nil {
			return err
		}
	}

	fmt.Fprintf(a.out, "Imported %d, replaced %d, skipped %d (already exist).\n", imported, replaced, skipped)
	if withoutValue > 0 {
		fmt.Fprintf(a.out, "%d imported entries have no value; 'pandora doctor' lists them.\n", withoutValue)
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
		if strings.TrimSpace(s.Title) == "" {
			return nil, fmt.Errorf("secret #%d has an empty title", i+1)
		}
		if seen[s.Title] {
			return nil, fmt.Errorf("secret %q appears more than once", s.Title)
		}
		seen[s.Title] = true
		for _, f := range []struct{ name, v string }{{"created", s.Created}, {"updated", s.Updated}, {"expires", s.Expires}} {
			if _, err := store.ParseDate(f.v); err != nil {
				return nil, fmt.Errorf("secret %q: invalid %s %q: %v", s.Title, f.name, f.v, err)
			}
		}
		if s.Value != nil && *s.Value == "" {
			return nil, fmt.Errorf("secret %q has an empty value", s.Title)
		}
		if s.Created == "" {
			s.Created = today
		}
	}
	return &doc, nil
}
