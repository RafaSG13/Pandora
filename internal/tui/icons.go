package tui

import (
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
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

// detectNoColor reports whether the environment asks for output without
// colour: NO_COLOR set to anything non-empty (https://no-color.org), or a
// dumb terminal.
func detectNoColor(getenv func(string) string) bool {
	return getenv("NO_COLOR") != "" || getenv("TERM") == "dumb"
}

// noColor is true when the UI draws no colour at all; see applyColorMode.
var noColor = detectNoColor(os.Getenv)

// applyColorMode switches the whole UI to colourless output when off is
// true. lipgloss already picks the Ascii profile for NO_COLOR, but not for
// TERM=dumb together with COLORTERM, so it is forced here. The Ascii profile
// drops every escape code, bold and reverse video included, which is why the
// selected row also carries a marker (see rowDelegate.Render). The dark
// background is assumed so no terminal query is sent just to pick colours
// that are never drawn.
func applyColorMode(off bool) {
	noColor = off
	if off {
		lipgloss.SetColorProfile(termenv.Ascii)
		lipgloss.SetHasDarkBackground(true)
	}
}
