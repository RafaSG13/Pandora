package store

import (
	"errors"
	"strings"
	"time"

	"github.com/zalando/go-keyring"
)

// DateLayout is the format of every date in the metadata file.
const DateLayout = "2006-01-02"

// ErrBadDate is returned for an expiry that is not a YYYY-MM-DD date.
var ErrBadDate = errors.New("date must be YYYY-MM-DD")

// ParseDate parses a metadata date. An empty string yields the zero time and no error.
func ParseDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}
	t, err := time.ParseInLocation(DateLayout, s, time.Local)
	if err != nil {
		return time.Time{}, ErrBadDate
	}
	return t, nil
}

// DaysUntilExpiry returns the whole calendar days from now until the expiry
// date (negative once expired) and whether an expiry is set and valid.
func (m SecretMeta) DaysUntilExpiry(now time.Time) (days int, ok bool) {
	exp, err := ParseDate(m.Expires)
	if err != nil || exp.IsZero() {
		return 0, false
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	return int(exp.Sub(today).Hours() / 24), true
}

// IsValueMissing reports whether err means the secret's value is not in the
// keyring (as opposed to the keyring being unreachable).
func IsValueMissing(err error) bool { return errors.Is(err, keyring.ErrNotFound) }
