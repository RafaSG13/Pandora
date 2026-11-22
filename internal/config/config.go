// Package config reads Pandora's optional settings file (config.json, next to
// vault.json) and merges it with the environment. Precedence is environment,
// then file, then built-in default. A broken file never stops Pandora: every
// problem becomes a warning and the default stays in force.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/RafaSG13/Pandora/internal/gen"
)

// Built-in defaults.
const (
	DefaultLockMinutes      = 5
	DefaultClipboardSeconds = 15
	DefaultRevealSeconds    = 10
	DefaultExpiryWarnDays   = 14
	DefaultGenLength        = 24
	DefaultGenCharset       = "alnum"
	DefaultSort             = "created"
)

// Upper bounds of the numeric settings.
const (
	maxLockMinutes = 7 * 24 * 60
	maxSeconds     = 3600
	// MaxExpiryWarnDays is the largest expiry_warn_days; `expiring --within` shares it.
	MaxExpiryWarnDays = 3650
)

// maxFileBytes caps what is read from the file.
const maxFileBytes = 64 << 10

// Keys lists the settings in display order.
var Keys = []string{
	"lock_minutes", "ascii", "clipboard_seconds", "reveal_seconds",
	"expiry_warn_days", "gen_length", "gen_charset", "sort", "compact",
}

var sortNames = []string{"created", "name", "category"}

// Settings are the effective values after merging default, file and environment.
type Settings struct {
	LockAfter      time.Duration // idle lock; 0 disables it
	ClipboardTTL   time.Duration
	RevealTTL      time.Duration
	ASCII          *bool // nil: decided by the locale
	ExpiryWarnDays int
	GenLength      int
	GenCharset     string
	Sort           string // one of created, name, category
	Compact        bool
}

func defaults() Settings {
	return Settings{
		LockAfter:      DefaultLockMinutes * time.Minute,
		ClipboardTTL:   DefaultClipboardSeconds * time.Second,
		RevealTTL:      DefaultRevealSeconds * time.Second,
		ExpiryWarnDays: DefaultExpiryWarnDays,
		GenLength:      DefaultGenLength,
		GenCharset:     DefaultGenCharset,
		Sort:           DefaultSort,
	}
}

// Path returns where the config file lives. It does not create anything.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "pandora", "config.json"), nil
}

// Load returns the effective settings and one warning per problem found in
// the file (each prefixed "config.json: "). A missing file is not a problem.
func Load(getenv func(string) string) (Settings, []string) {
	s := defaults()
	var warns []string
	warn := func(format string, args ...any) {
		warns = append(warns, "config.json: "+fmt.Sprintf(format, args...))
	}

	raw, tooBig, err := readFile()
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		warn("%v", err)
	case tooBig:
		warn("file larger than %d KB, ignored", maxFileBytes>>10)
	default:
		applyFile(&s, raw, warn)
	}

	applyEnv(&s, getenv)
	return s, warns
}

// readFile returns the file's bytes, or tooBig when it exceeds maxFileBytes.
func readFile() (data []byte, tooBig bool, err error) {
	path, err := Path()
	if err != nil {
		return nil, false, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxFileBytes+1))
	if err != nil {
		return nil, false, err
	}
	return b, len(b) > maxFileBytes, nil
}

func applyFile(s *Settings, raw []byte, warn func(string, ...any)) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		warn("not a valid JSON object")
		return
	}
	names := make([]string, 0, len(fields))
	for k := range fields {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		if err := set(s, k, fields[k]); err != nil {
			if errors.Is(err, errUnknownKey) {
				warn("unknown key %q ignored", k)
			} else {
				warn("%s: %v", k, err)
			}
		}
	}
}

var errUnknownKey = errors.New("unknown key")

// set applies one file value to s. On error s is unchanged.
func set(s *Settings, key string, raw json.RawMessage) error {
	switch key {
	case "lock_minutes":
		n, err := intIn(raw, 0, maxLockMinutes)
		if err != nil {
			return err
		}
		s.LockAfter = time.Duration(n) * time.Minute
	case "ascii":
		var b bool
		if err := decode(raw, &b, "a boolean"); err != nil {
			return err
		}
		s.ASCII = &b
	case "clipboard_seconds":
		n, err := intIn(raw, 1, maxSeconds)
		if err != nil {
			return err
		}
		s.ClipboardTTL = time.Duration(n) * time.Second
	case "reveal_seconds":
		n, err := intIn(raw, 1, maxSeconds)
		if err != nil {
			return err
		}
		s.RevealTTL = time.Duration(n) * time.Second
	case "expiry_warn_days":
		n, err := intIn(raw, 0, MaxExpiryWarnDays)
		if err != nil {
			return err
		}
		s.ExpiryWarnDays = n
	case "gen_length":
		n, err := intIn(raw, gen.MinLength, gen.MaxLength)
		if err != nil {
			return err
		}
		s.GenLength = n
	case "gen_charset":
		var v string
		if err := decode(raw, &v, "a string"); err != nil {
			return err
		}
		if _, ok := gen.Lookup(v); !ok {
			return fmt.Errorf("must be one of %s", charsetKeys())
		}
		s.GenCharset = v
	case "sort":
		var v string
		if err := decode(raw, &v, "a string"); err != nil {
			return err
		}
		if !contains(sortNames, v) {
			return fmt.Errorf("must be one of %s", strings.Join(sortNames, ", "))
		}
		s.Sort = v
	case "compact":
		var b bool
		if err := decode(raw, &b, "a boolean"); err != nil {
			return err
		}
		s.Compact = b
	default:
		return errUnknownKey
	}
	return nil
}

func decode(raw json.RawMessage, into any, want string) error {
	// null would decode silently to the zero value (0 minutes disables the lock).
	if string(raw) == "null" || json.Unmarshal(raw, into) != nil {
		return fmt.Errorf("must be %s", want)
	}
	return nil
}

func intIn(raw json.RawMessage, lo, hi int) (int, error) {
	var n int
	if err := decode(raw, &n, "an integer"); err != nil {
		return 0, err
	}
	if n < lo || n > hi {
		return 0, fmt.Errorf("must be between %d and %d", lo, hi)
	}
	return n, nil
}

func charsetKeys() string {
	keys := make([]string, len(gen.Charsets))
	for i, cs := range gen.Charsets {
		keys[i] = cs.Key
	}
	return strings.Join(keys, ", ")
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// applyEnv lets the environment override the file. Invalid values are ignored
// silently, as they always were.
func applyEnv(s *Settings, getenv func(string) string) {
	if v := strings.TrimSpace(getenv("PANDORA_LOCK_MINUTES")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 && n <= maxLockMinutes {
			s.LockAfter = time.Duration(n) * time.Minute
		}
	}
	if getenv("PANDORA_ASCII") == "1" {
		t := true
		s.ASCII = &t
	}
}

// Get returns the effective value of key as text, as shown by `pandora config get`.
func (s Settings) Get(key string) (string, bool) {
	switch key {
	case "lock_minutes":
		return strconv.Itoa(int(s.LockAfter / time.Minute)), true
	case "ascii":
		if s.ASCII == nil {
			return "auto", true
		}
		return strconv.FormatBool(*s.ASCII), true
	case "clipboard_seconds":
		return strconv.Itoa(int(s.ClipboardTTL / time.Second)), true
	case "reveal_seconds":
		return strconv.Itoa(int(s.RevealTTL / time.Second)), true
	case "expiry_warn_days":
		return strconv.Itoa(s.ExpiryWarnDays), true
	case "gen_length":
		return strconv.Itoa(s.GenLength), true
	case "gen_charset":
		return s.GenCharset, true
	case "sort":
		return s.Sort, true
	case "compact":
		return strconv.FormatBool(s.Compact), true
	}
	return "", false
}
