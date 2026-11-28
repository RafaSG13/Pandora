package termsafe

import "testing"

func TestSanitizeNeutralisesTerminalControls(t *testing.T) {
	in := "a\x1b]52;c;QQ==\x07b\n\tc\u009b\x7f"
	want := "a^[]52;c;QQ==^Gb\n\tc\\x9b^?"
	if got := Sanitize(in); got != want {
		t.Errorf("Sanitize = %q, want %q", got, want)
	}
	if got := Sanitize("x\r\ny"); got != "x^M\ny" {
		t.Errorf("CR = %q", got)
	}
	if got := Sanitize("bad\xffbyte"); got != "bad\ufffdbyte" {
		t.Errorf("invalid UTF-8 = %q", got)
	}
}

func TestSanitizeKeepsSafeTextUnchanged(t *testing.T) {
	for _, s := range []string{"", "plain", "línea\n\ttab", "日本語 ✓ 👨\u200d👩", "keep \ufffd as is"} {
		if got := Sanitize(s); got != s {
			t.Errorf("Sanitize(%q) = %q, want it unchanged", s, got)
		}
	}
}
