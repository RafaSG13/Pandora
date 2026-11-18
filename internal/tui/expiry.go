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
	expiryFar                        // beyond the warning window
	expirySoon                       // within the warning window
	expiryExpired                    // already past
)

// expiryInfo describes meta's expiry relative to now, e.g. "expires in 5d";
// an expiry within soonDays days counts as expirySoon.
func expiryInfo(meta store.SecretMeta, now time.Time, soonDays int) (label string, level expiryLevel) {
	days, ok := meta.DaysUntilExpiry(now)
	switch {
	case !ok:
		return "", expiryNone
	case days < 0:
		return fmt.Sprintf("expired %dd ago", -days), expiryExpired
	case days == 0:
		return "expires today", expirySoon
	case days <= soonDays:
		return fmt.Sprintf("expires in %dd", days), expirySoon
	default:
		return fmt.Sprintf("expires in %dd", days), expiryFar
	}
}
