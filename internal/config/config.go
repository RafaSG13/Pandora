// Package config loads Pandora's optional settings file, config.json, from
// the same directory as the vault.
//
// The file is untrusted input: it is size-limited, every key is checked on
// its own, and a bad value, an unknown key or an unreadable file never stops
// Pandora from starting. Each problem is reported as a warning and the
// built-in default stays in place. Precedence per key: environment variable
// (PANDORA_LOCK_MINUTES, PANDORA_ASCII), then the file, then the default.
// The file never holds secrets or secret names.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/RafaSG13/Pandora/internal/gen"
)

// FileName is the settings file inside the Pandora config directory.
const FileName = "config.json"

// maxFileBytes caps what is read: a settings file is a few hundred bytes.
const maxFileBytes = 64 << 10

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

// maxLockMinutes (one year) keeps the idle-lock duration far from overflow.
const maxLockMinutes = 525600

// SortNames are the values of the sort setting, in the order the TUI cycles.
var SortNames = []string{"created", "name", "category"}

// Config is the effective set of settings.
type Config struct {
	LockMinutes      int    // idle lock; 0 disables it
	ASCII            bool   // force the plain-ASCII look
	ClipboardSeconds int    // how long a copied secret stays on the clipboard
	RevealSeconds    int    // how long a revealed value stays visible
	ExpiryWarnDays   int    // how far ahead an expiry starts to warn
	GenLength        int    // default generated length, one of gen.Lengths
	GenCharset       string // default generator charset, a gen.Charsets key
	Sort             string // default sort, one of SortNames
	Compact          bool   // start with compact rows
}

// Defaults returns the built-in settings.
func Defaults() Config {
	return Config{
		LockMinutes:      DefaultLockMinutes,
		ClipboardSeconds: DefaultClipboardSeconds,
		RevealSeconds:    DefaultRevealSeconds,
		ExpiryWarnDays:   DefaultExpiryWarnDays,
		GenLength:        DefaultGenLength,
		GenCharset:       DefaultGenCharset,
		Sort:             DefaultSort,
	}
}

// setting describes one key of the file: how to read it and how to print it.
type setting struct {
	key    string
	set    func(c *Config, raw json.RawMessage) error
	format func(c Config) string
}

func intSetting(key string, lo, hi int, field func(*Config) *int) setting {
	return setting{
		key: key,
		set: func(c *Config, raw json.RawMessage) error {
			var n int
			if err := json.Unmarshal(raw, &n); err != nil {
				return errors.New("must be a whole number")
			}
			if n < lo || n > hi {
				return fmt.Errorf("must be between %d and %d", lo, hi)
			}
			*field(c) = n
			return nil
		},
		format: func(c Config) string { return strconv.Itoa(*field(&c)) },
	}
}

func boolSetting(key string, field func(*Config) *bool) setting {
	return setting{
		key: key,
		set: func(c *Config, raw json.RawMessage) error {
			var b bool
			if err := json.Unmarshal(raw, &b); err != nil {
				return errors.New("must be true or false")
			}
			*field(c) = b
			return nil
		},
		format: func(c Config) string { return strconv.FormatBool(*field(&c)) },
	}
}

func choiceSetting(key string, choices func() []string, field func(*Config) *string) setting {
	return setting{
		key: key,
		set: func(c *Config, raw json.RawMessage) error {
			var s string
			if err := json.Unmarshal(raw, &s); err != nil || !slices.Contains(choices(), s) {
				return fmt.Errorf("must be one of %s", strings.Join(choices(), ", "))
			}
			*field(c) = s
			return nil
		},
		format: func(c Config) string { return *field(&c) },
	}
}

func charsetKeys() []string {
	keys := make([]string, len(gen.Charsets))
	for i, cs := range gen.Charsets {
		keys[i] = cs.Key
	}
	return keys
}

// settings lists every key, in the order `pandora config get` prints them.
func settings() []setting {
	lengths := setting{
		key: "gen_length",
		set: func(c *Config, raw json.RawMessage) error {
			var n int
			if err := json.Unmarshal(raw, &n); err != nil || !slices.Contains(gen.Lengths, n) {
				return fmt.Errorf("must be one of %v", gen.Lengths)
			}
			c.GenLength = n
			return nil
		},
		format: func(c Config) string { return strconv.Itoa(c.GenLength) },
	}
	return []setting{
		intSetting("lock_minutes", 0, maxLockMinutes, func(c *Config) *int { return &c.LockMinutes }),
		boolSetting("ascii", func(c *Config) *bool { return &c.ASCII }),
		intSetting("clipboard_seconds", 1, 600, func(c *Config) *int { return &c.ClipboardSeconds }),
		intSetting("reveal_seconds", 1, 300, func(c *Config) *int { return &c.RevealSeconds }),
		intSetting("expiry_warn_days", 1, 365, func(c *Config) *int { return &c.ExpiryWarnDays }),
		lengths,
		choiceSetting("gen_charset", charsetKeys, func(c *Config) *string { return &c.GenCharset }),
		choiceSetting("sort", func() []string { return SortNames }, func(c *Config) *string { return &c.Sort }),
		boolSetting("compact", func(c *Config) *bool { return &c.Compact }),
	}
}

// Keys returns the setting names in display order.
func Keys() []string {
	var keys []string
	for _, s := range settings() {
		keys = append(keys, s.key)
	}
	return keys
}

// Value returns the effective value of key as text, and false for an
// unknown key.
func (c Config) Value(key string) (string, bool) {
	for _, s := range settings() {
		if s.key == key {
			return s.format(c), true
		}
	}
	return "", false
}

// Path is where the settings file lives inside dir.
func Path(dir string) string { return filepath.Join(dir, FileName) }

// Load returns the effective settings and a warning for every problem found.
// dir is the Pandora config directory; "" means there is none, so only the
// environment is read. A missing file is not a problem.
func Load(dir string, getenv func(string) string) (Config, []string) {
	c := Defaults()
	var warns []string
	if dir != "" {
		warns = c.loadFile(Path(dir))
	}
	return c, append(warns, c.applyEnv(getenv)...)
}

func (c *Config) loadFile(path string) []string {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		reason := err.Error()
		var pe *fs.PathError
		if errors.As(err, &pe) {
			reason = pe.Err.Error() // without the path
		}
		return []string{fmt.Sprintf("%s: cannot read (%s); using the defaults", FileName, reason)}
	}
	defer f.Close()
	if st, err := f.Stat(); err != nil || !st.Mode().IsRegular() {
		return []string{FileName + ": not a regular file; using the defaults"}
	}
	data, err := io.ReadAll(io.LimitReader(f, maxFileBytes+1))
	if err != nil {
		return []string{fmt.Sprintf("%s: cannot read (%v); using the defaults", FileName, err)}
	}
	if len(data) > maxFileBytes {
		return []string{fmt.Sprintf("%s: larger than %d KB; using the defaults", FileName, maxFileBytes>>10)}
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return []string{FileName + ": not a JSON object; using the defaults"}
	}
	var warns []string
	for _, s := range settings() {
		v, ok := raw[s.key]
		if !ok {
			continue
		}
		delete(raw, s.key)
		// null would decode to the zero value; for lock_minutes that silently
		// disables the idle lock, so it is rejected for every key.
		if string(bytes.TrimSpace(v)) == "null" {
			warns = append(warns, fmt.Sprintf("%s: %s must not be null; using the default", FileName, s.key))
			continue
		}
		if err := s.set(c, v); err != nil {
			warns = append(warns, fmt.Sprintf("%s: %s %v; using the default", FileName, s.key, err))
		}
	}
	for _, k := range slices.Sorted(maps.Keys(raw)) {
		warns = append(warns, fmt.Sprintf("%s: unknown key %q ignored", FileName, k))
	}
	return warns
}

// applyEnv lets the environment variables win over the file.
func (c *Config) applyEnv(getenv func(string) string) []string {
	var warns []string
	if v := strings.TrimSpace(getenv("PANDORA_LOCK_MINUTES")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 && n <= maxLockMinutes {
			c.LockMinutes = n
		} else {
			warns = append(warns, "PANDORA_LOCK_MINUTES must be a whole number of minutes (0 disables the lock); ignored")
		}
	}
	if getenv("PANDORA_ASCII") == "1" {
		c.ASCII = true
	}
	return warns
}
