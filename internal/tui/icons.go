package tui

import (
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
)

// glyphSet is every non-letter symbol the UI draws, so a terminal without
// UTF-8 (or PANDORA_ASCII=1) can get a plain-ASCII look.
type glyphSet struct {
	ascii bool

	dot, mask, ellipsis, rule, sep string // list dot, masked-value char, "…", "─", "·"
	lock, unlock, clock, orphan    string
	marker, logo                   string // focused-field bar, header logo
	barFull, barEmpty              string // countdown bars
	thumb, track                   string // list scrollbar
	border                         lipgloss.Border
	spinner                        spinner.Spinner
}

// detectASCII reports whether the environment asks for ASCII output:
// PANDORA_ASCII=1, or a locale that is set but not UTF-8.
func detectASCII(getenv func(string) string) bool {
	if getenv("PANDORA_ASCII") == "1" {
		return true
	}
	for _, name := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		if v := getenv(name); v != "" {
			l := strings.ToLower(v)
			return !strings.Contains(l, "utf-8") && !strings.Contains(l, "utf8")
		}
	}
	return false
}

func newGlyphs(ascii bool) glyphSet {
	if ascii {
		return glyphSet{
			ascii: true,
			dot:   "*", mask: "*", ellipsis: "...", rule: "-", sep: "-",
			lock: "[x]", unlock: "[ ]", clock: "~", orphan: "!",
			marker: ">", logo: "#",
			barFull: "#", barEmpty: "-",
			thumb: "#", track: "|",
			border: lipgloss.Border{
				Top: "-", Bottom: "-", Left: "|", Right: "|",
				TopLeft: "+", TopRight: "+", BottomLeft: "+", BottomRight: "+",
			},
			spinner: spinner.Line,
		}
	}
	return glyphSet{
		dot: "●", mask: "•", ellipsis: "…", rule: "─", sep: "·",
		lock: "🔒", unlock: "🔓", clock: "◷", orphan: "!",
		marker: "▌", logo: "◆",
		barFull: "█", barEmpty: "░",
		thumb: "┃", track: "┆",
		border:  lipgloss.RoundedBorder(),
		spinner: spinner.Dot,
	}
}

// g is the active glyph set.
var g = newGlyphs(detectASCII(os.Getenv))
