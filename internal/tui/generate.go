package tui

import (
	"crypto/rand"
	"errors"
	"math/big"
	"strings"
)

var errNoSecret = errors.New("could not generate a secret")

// genCharset is a named alphabet for generated secrets. Each class must
// appear at least once in a generated value.
type genCharset struct {
	name    string
	classes []string
}

var (
	genLengths  = []int{16, 24, 32, 48, 64}
	genCharsets = []genCharset{
		{"letters+digits", []string{"abcdefghijklmnopqrstuvwxyz", "ABCDEFGHIJKLMNOPQRSTUVWXYZ", "0123456789"}},
		{"letters+digits+symbols", []string{"abcdefghijklmnopqrstuvwxyz", "ABCDEFGHIJKLMNOPQRSTUVWXYZ", "0123456789", "!@#$%^&*()-_=+[]{}:,.?"}},
		{"hex", []string{"0123456789abcdef"}},
	}
)

// generateSecret returns a random string of length n drawn uniformly from the
// charset using crypto/rand. When n allows it, every class is represented.
func generateSecret(cs genCharset, n int) (string, error) {
	alphabet := strings.Join(cs.classes, "")
	for range 1000 {
		out := make([]byte, n)
		for i := range out {
			idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
			if err != nil {
				return "", err
			}
			out[i] = alphabet[idx.Int64()]
		}
		if n < len(cs.classes) || hasEveryClass(string(out), cs.classes) {
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
