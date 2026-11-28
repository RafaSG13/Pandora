// Package termsafe makes untrusted text safe to draw on a terminal.
package termsafe

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Sanitize makes s safe to write to a terminal: C0 controls other than
// tab and newline, and DEL, become caret notation (ESC → "^[", CR → "^M",
// DEL → "^?"); C1 controls (U+0080–U+009F) become `\xNN`; invalid UTF-8
// becomes U+FFFD. Everything else is unchanged.
//
// Secret values come from imports and from peers' share files, so they can
// carry escape sequences (window-title or clipboard OSC, cursor moves) that
// the terminal would run when the value is revealed.
func Sanitize(s string) string {
	if !needsWork(s) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 8)
	for i := 0; i < len(s); {
		r, n := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && n == 1:
			b.WriteRune(utf8.RuneError)
		case r == '\t' || r == '\n':
			b.WriteRune(r)
		case r < 0x20:
			b.WriteByte('^')
			b.WriteByte(byte(r) + 0x40)
		case r == 0x7f:
			b.WriteString("^?")
		case r >= 0x80 && r <= 0x9f:
			fmt.Fprintf(&b, `\x%02x`, r)
		default:
			b.WriteString(s[i : i+n])
		}
		i += n
	}
	return b.String()
}

// needsWork reports whether Sanitize would change s.
func needsWork(s string) bool {
	for i := 0; i < len(s); {
		r, n := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && n == 1:
			return true
		case r == '\t' || r == '\n':
		case r < 0x20 || r == 0x7f || r >= 0x80 && r <= 0x9f:
			return true
		}
		i += n
	}
	return false
}
