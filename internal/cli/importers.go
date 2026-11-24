package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/RafaSG13/Pandora/internal/store"
)

// formatPandora is Pandora's own export, the default of `import --format`.
const formatPandora = "pandora"

// importFormats lists the values accepted by `import --format`.
var importFormats = []string{formatPandora, "dotenv", "bitwarden-json"}

// importOptions are the `import` flags a parser may use.
type importOptions struct {
	category string // dotenv: category given to every imported secret
}

// importBatch is what a parser found. Everything in it has been validated, so
// it can be handed to store.RestoreSecrets as is.
type importBatch struct {
	secrets []exportSecret
	skipped []string // items that were not imported, each with its reason (never a value)
	notes   []string // things imported only partly, e.g. a dropped TOTP seed
}

// externalParsers holds one pure parser per foreign format. A parser fails on
// anything that makes the whole file untrustworthy (syntax, wrong kind of
// file) and skips single entries it cannot use, saying why.
var externalParsers = map[string]func(data []byte, opts importOptions) (*importBatch, error){
	"dotenv":         parseDotenv,
	"bitwarden-json": parseBitwardenJSON,
}

// skip records an entry that cannot be imported. Reasons never contain
// values: they come from normalizeSecret and fixed texts.
func (b *importBatch) skip(label string, why error) {
	b.skipped = append(b.skipped, fmt.Sprintf("%s: %v", label, why))
}

// add normalises s and appends it, or records why it cannot be imported.
func (b *importBatch) add(s exportSecret, today string) bool {
	if err := normalizeSecret(&s, today); err != nil {
		b.skip(fmt.Sprintf("%q", s.Title), err)
		return false
	}
	b.secrets = append(b.secrets, s)
	return true
}

// parseDotenv reads KEY=value lines, the inverse of what `pandora env`
// prints. Accepted: blank lines, # comments, an optional `export ` prefix,
// bare values (a trailing ` # comment` is dropped), 'single-quoted' literals
// and "double-quoted" values with \\ \" \n \r \t \$ escapes; quoted values may
// span lines. Any other backslash sequence is kept as written. Syntax
// errors abort the whole file and name only the line, never its content.
func parseDotenv(data []byte, opts importOptions) (*importBatch, error) {
	if !utf8.Valid(data) {
		return nil, errors.New("not a text file: invalid UTF-8")
	}
	text := strings.ReplaceAll(strings.TrimPrefix(string(data), "\ufeff"), "\r\n", "\n")
	today := time.Now().Format(store.DateLayout)
	batch := &importBatch{}
	firstSeen := map[string]int{}
	skipBlanks := func(i int) int {
		for i < len(text) && (text[i] == ' ' || text[i] == '\t') {
			i++
		}
		return i
	}

	line := 1
	for i := 0; i < len(text); {
		i = skipBlanks(i)
		switch {
		case i >= len(text):
			continue
		case text[i] == '\n':
			i++
			line++
			continue
		case text[i] == '#':
			for i < len(text) && text[i] != '\n' {
				i++
			}
			continue
		}
		start := line
		if rest := text[i:]; strings.HasPrefix(rest, "export") && len(rest) > 6 && (rest[6] == ' ' || rest[6] == '\t') {
			i = skipBlanks(i + 6)
		}
		eq := i
		for eq < len(text) && text[eq] != '=' && text[eq] != '\n' {
			eq++
		}
		if eq >= len(text) || text[eq] != '=' {
			return nil, fmt.Errorf("line %d: expected KEY=value", start)
		}
		key := strings.TrimRight(text[i:eq], " \t")
		if !dotenvKey.MatchString(key) {
			return nil, fmt.Errorf("line %d: not a usable variable name", start)
		}
		if first, dup := firstSeen[key]; dup {
			return nil, fmt.Errorf("line %d: %s is already defined on line %d", start, key, first)
		}
		firstSeen[key] = start

		value, next, err := readDotenvValue(text, skipBlanks(eq+1))
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", start, err)
		}
		line += strings.Count(text[i:next], "\n")
		i = next

		if value == "" {
			batch.skip(key, errors.New("empty value"))
			continue
		}
		v := value
		batch.add(exportSecret{SecretMeta: store.SecretMeta{Title: key, Category: opts.category}, Value: &v}, today)
	}
	return batch, nil
}

// readDotenvValue reads one value starting at text[i] and returns it with the
// index just past the end of its line.
func readDotenvValue(text string, i int) (value string, next int, err error) {
	if i < len(text) && (text[i] == '"' || text[i] == '\'') {
		quote := text[i]
		var b strings.Builder
		p := i + 1
		for ; p < len(text) && text[p] != quote; p++ {
			if quote == '"' && text[p] == '\\' && p+1 < len(text) {
				p++
				switch text[p] {
				case 'n':
					b.WriteByte('\n')
				case 'r':
					b.WriteByte('\r')
				case 't':
					b.WriteByte('\t')
				case '\\', '"', '$':
					b.WriteByte(text[p])
				default:
					b.WriteByte('\\')
					b.WriteByte(text[p])
				}
				continue
			}
			b.WriteByte(text[p])
		}
		if p >= len(text) {
			return "", 0, errors.New("unterminated quote")
		}
		p++ // closing quote
		for p < len(text) && (text[p] == ' ' || text[p] == '\t') {
			p++
		}
		switch {
		case p >= len(text):
		case text[p] == '\n':
			p++
		case text[p] == '#':
			for p < len(text) && text[p] != '\n' {
				p++
			}
			if p < len(text) {
				p++
			}
		default:
			return "", 0, errors.New("unexpected text after the closing quote")
		}
		return b.String(), p, nil
	}

	end := i
	for end < len(text) && text[end] != '\n' {
		end++
	}
	raw := text[i:end]
	if c := strings.Index(raw, " #"); c >= 0 {
		raw = raw[:c]
	} else if c := strings.Index(raw, "\t#"); c >= 0 {
		raw = raw[:c]
	}
	if end < len(text) {
		end++
	}
	return strings.TrimSpace(raw), end, nil
}

// bitwardenFile is the part of an unencrypted Bitwarden JSON export that is
// read; everything else is ignored.
type bitwardenFile struct {
	Encrypted bool `json:"encrypted"`
	Folders   []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"folders"`
	Items []bitwardenItem `json:"items"`
}

type bitwardenItem struct {
	FolderID     *string `json:"folderId"`
	Type         int     `json:"type"`
	Name         string  `json:"name"`
	Notes        *string `json:"notes"`
	RevisionDate string  `json:"revisionDate"`
	CreationDate string  `json:"creationDate"`
	Login        *struct {
		Username *string `json:"username"`
		Password *string `json:"password"`
		Totp     *string `json:"totp"`
		URIs     []struct {
			URI *string `json:"uri"`
		} `json:"uris"`
	} `json:"login"`
}

// Bitwarden item types.
const (
	bwLogin      = 1
	bwSecureNote = 2
	bwCard       = 3
	bwIdentity   = 4
)

// parseBitwardenJSON reads an unencrypted Bitwarden export. A login becomes a
// secret whose value is its password; a secure note becomes one whose value is
// the note. The folder is the category; the notes keep the user name and the
// first URL (Pandora's notes are one line, so line breaks become spaces).
// Cards, identities and logins without a password are skipped, and TOTP seeds
// and custom fields are never imported. Items with the same name get a
// " (2)", " (3)" suffix instead of overwriting each other.
func parseBitwardenJSON(data []byte, _ importOptions) (*importBatch, error) {
	var f bitwardenFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("not a Bitwarden JSON export: %w", err)
	}
	if f.Encrypted {
		return nil, errors.New("this Bitwarden export is encrypted; export it again as unencrypted .json")
	}
	if f.Items == nil {
		return nil, errors.New("not a Bitwarden JSON export: no items list")
	}
	folders := make(map[string]string, len(f.Folders))
	for _, fo := range f.Folders {
		folders[fo.ID] = strings.TrimSpace(fo.Name)
	}

	today := time.Now().Format(store.DateLayout)
	batch := &importBatch{}
	used := map[string]bool{}
	for n, it := range f.Items {
		title := strings.TrimSpace(it.Name)
		label := fmt.Sprintf("%q", title)
		if title == "" {
			label = fmt.Sprintf("item #%d", n+1)
		}

		var value string
		var noteParts []string
		var username string
		switch it.Type {
		case bwLogin:
			if it.Login == nil || deref(it.Login.Password) == "" {
				batch.skip(label, errors.New("login without a password"))
				continue
			}
			value = *it.Login.Password
			if oneLine(deref(it.Notes)) != "" {
				noteParts = append(noteParts, oneLine(deref(it.Notes)))
			}
			username = oneLine(deref(it.Login.Username))
			for _, uri := range it.Login.URIs {
				if u := oneLine(deref(uri.URI)); u != "" {
					noteParts = append(noteParts, "url: "+u)
					break
				}
			}
			if deref(it.Login.Totp) != "" {
				batch.notes = append(batch.notes, label+": TOTP seed not imported")
			}
		case bwSecureNote:
			value = deref(it.Notes)
			if strings.TrimSpace(value) == "" {
				batch.skip(label, errors.New("empty secure note"))
				continue
			}
		case bwCard:
			batch.skip(label, errors.New("cards are not supported"))
			continue
		case bwIdentity:
			batch.skip(label, errors.New("identities are not supported"))
			continue
		default:
			batch.skip(label, fmt.Errorf("unsupported item type %d", it.Type))
			continue
		}

		category := ""
		if it.FolderID != nil {
			category = folders[*it.FolderID]
		}
		s := exportSecret{
			SecretMeta: store.SecretMeta{
				Title:    title,
				Category: category,
				Username: username,
				Created:  bitwardenDate(it.CreationDate),
				Updated:  bitwardenDate(it.RevisionDate),
				Notes:    strings.Join(noteParts, " | "),
			},
			Value: &value,
		}
		if err := normalizeSecret(&s, today); err != nil {
			batch.skip(label, err)
			continue
		}
		unique := title
		for k := 2; used[unique]; k++ {
			unique = fmt.Sprintf("%s (%d)", title, k)
		}
		if unique != title {
			batch.notes = append(batch.notes, fmt.Sprintf("%s: duplicate name, imported as %q", label, unique))
			s.Title = unique
		}
		used[unique] = true
		batch.secrets = append(batch.secrets, s)
	}
	return batch, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// oneLine turns text into a single line of printable characters: whitespace
// runs (line breaks included) become one space and other control characters
// are dropped, which is what store.ValidateText accepts.
func oneLine(s string) string {
	s = strings.Map(func(r rune) rune {
		switch {
		case unicode.IsSpace(r):
			return ' '
		case unicode.IsControl(r):
			return -1
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

// bitwardenDate keeps the YYYY-MM-DD part of a Bitwarden timestamp, or
// returns "" when there is none.
func bitwardenDate(ts string) string {
	if len(ts) < len(store.DateLayout) {
		return ""
	}
	d := ts[:len(store.DateLayout)]
	if _, err := store.ParseDate(d); err != nil {
		return ""
	}
	return d
}
