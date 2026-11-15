// Package gen generates random secrets with crypto/rand.
package gen

import (
	"crypto/rand"
	"errors"
	"math/big"
	"strings"
)

// Length bounds accepted by the command line. The TUI cycles through Lengths.
const (
	MinLength = 8
	MaxLength = 128
)

// ErrLength is returned when a requested length is outside MinLength..MaxLength.
var ErrLength = errors.New("length out of range")

var errNoSecret = errors.New("could not generate a secret")

// Charset is a named alphabet. Every class appears at least once in a
// generated value when the length allows it.
type Charset struct {
	Key     string // stable identifier used on the command line
	Name    string // human-readable label shown in the TUI
	Classes []string
}

var (
	// Lengths are the presets the TUI cycles through.
	Lengths = []int{16, 24, 32, 48, 64}

	Charsets = []Charset{
		{"alnum", "letters+digits", []string{"abcdefghijklmnopqrstuvwxyz", "ABCDEFGHIJKLMNOPQRSTUVWXYZ", "0123456789"}},
		{"symbols", "letters+digits+symbols", []string{"abcdefghijklmnopqrstuvwxyz", "ABCDEFGHIJKLMNOPQRSTUVWXYZ", "0123456789", "!@#$%^&*()-_=+[]{}:,.?"}},
		{"hex", "hex", []string{"0123456789abcdef"}},
	}
)

// Lookup returns the charset with the given Key.
func Lookup(key string) (Charset, bool) {
	for _, cs := range Charsets {
		if cs.Key == key {
			return cs, true
		}
	}
	return Charset{}, false
}

// Generate returns a random string of length n drawn uniformly from the
// charset. n must be within MinLength..MaxLength.
func Generate(cs Charset, n int) (string, error) {
	if n < MinLength || n > MaxLength {
		return "", ErrLength
	}
	alphabet := strings.Join(cs.Classes, "")
	for range 1000 {
		out := make([]byte, n)
		for i := range out {
			idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
			if err != nil {
				return "", err
			}
			out[i] = alphabet[idx.Int64()]
		}
		if hasEveryClass(string(out), cs.Classes) {
			return string(out), nil
		}
	}
	return "", errNoSecret
}

func hasEveryClass(s string, classes []string) bool {
	for _, c := range classes {
		if !strings.ContainsAny(s, c) {
			return false
		}
	}
	return true
}
