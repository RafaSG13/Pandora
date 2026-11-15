package tui

import (
	"fmt"
	"time"

	"github.com/RafaSG13/Pandora/internal/store"
)

// expiryLevel says how urgent a secret's expiry is.
type expiryLevel int

const (
	expiryNone    expiryLevel = iota // no (valid) expiry set
	expiryFar                        // more than expirySoonDays away
	expirySoon                       // within expirySoonDays
	expiryExpired                    // already past
)

// expirySoonDays is how far ahead an expiry starts to warn.
const expirySoonDays = 14

// expiryInfo describes meta's expiry relative to now, e.g. "expires in 5d".
func expiryInfo(meta store.SecretMeta, now time.Time) (label string, level expiryLevel) {
	days, ok := meta.DaysUntilExpiry(now)
	switch {
	case !ok:
		return "", expiryNone
	case days < 0:
		return fmt.Sprintf("expired %dd ago", -days), expiryExpired
	case days == 0:
		return "expires today", expirySoon
	case days <= expirySoonDays:
		return fmt.Sprintf("expires in %dd", days), expirySoon
	default:
		return fmt.Sprintf("expires in %dd", days), expiryFar
	}
}
