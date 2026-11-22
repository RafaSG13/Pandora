package cli

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// parseDotenv is a minimal dotenv reader (bare and double-quoted values) used
// to prove that dotenvValue output round-trips.
func parseDotenv(t *testing.T, text string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for text != "" {
		eq := strings.IndexByte(text, '=')
		if eq < 0 {
			t.Fatalf("no '=' in %q", text)
		}
		key, rest := text[:eq], text[eq+1:]
		var val string
		if strings.HasPrefix(rest, `"`) {
			var b strings.Builder
			i := 1
			for ; i < len(rest) && rest[i] != '"'; i++ {
				if rest[i] != '\\' {
					b.WriteByte(rest[i])
					continue
				}
				i++
				switch rest[i] {
				case 'n':
					b.WriteByte('\n')
				case 'r':
					b.WriteByte('\r')
				default: // \\, \" and \$
					b.WriteByte(rest[i])
				}
			}
			if i >= len(rest) || rest[i] != '"' {
				t.Fatalf("unterminated quote in %q", rest)
			}
			val, rest = b.String(), rest[i+1:]
			if !strings.HasPrefix(rest, "\n") {
				t.Fatalf("junk after closing quote: %q", rest)
			}
			text = rest[1:]
		} else {
			nl := strings.IndexByte(rest, '\n')
			if nl < 0 {
				t.Fatalf("unterminated line %q", rest)
			}
			val, text = rest[:nl], rest[nl+1:]
		}
		out[key] = val
	}
	return out
}

func TestEnvQuotingRoundTrips(t *testing.T) {
	isolate(t)
	values := []string{
		"plain-Value_1.2/x:y@z%+",
		"",
		"a \"b\"\n$c",
		`back\slash`,
		"line1\r\nline2\n",
		"it's $HOME `x` #not-a-comment",
		"ünï ✓ with spaces",
		`ends with \`,
	}
	args := []string{"env"}
	for i, v := range values {
		mustSave(t, fmt.Sprintf("s%d", i), "", v)
		args = append(args, "-s", fmt.Sprintf("s%d=K%d", i, i))
	}
	code, stdout, stderr := harness{}.run(t, args...)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	got := parseDotenv(t, stdout)
	for i, v := range values {
		if got[fmt.Sprintf("K%d", i)] != v {
			t.Errorf("K%d = %q, want %q (output %q)", i, got[fmt.Sprintf("K%d", i)], v, stdout)
		}
	}
	if len(got) != len(values) {
		t.Errorf("got %d variables, want %d", len(got), len(values))
	}
}

func TestEnvDefaultNameMatchesRun(t *testing.T) {
	isolate(t)
	mustSave(t, "my api-key", "", "v1")
	code, stdout, stderr := harness{}.run(t, "env", "-s", "my api-key")
	if code != 0 || stdout != "MY_API_KEY=v1\n" {
		t.Fatalf("exit %d stdout %q stderr %q", code, stdout, stderr)
	}
}

func TestEnvRefusesTerminalUnlessForced(t *testing.T) {
	isolate(t)
	mustSave(t, "tok", "", "sup3r secret")
	code, stdout, stderr := harness{outTTY: true}.run(t, "env", "-s", "tok")
	if code != exitUsage {
		t.Fatalf("exit %d, want %d", code, exitUsage)
	}
	if stdout != "" || strings.Contains(stderr, "sup3r") {
		t.Errorf("leaked: stdout %q stderr %q", stdout, stderr)
	}
	code, stdout, stderr = harness{outTTY: true}.run(t, "env", "-s", "tok", "--force")
	if code != 0 || stdout != "TOK=\"sup3r secret\"\n" {
		t.Fatalf("forced: exit %d stdout %q stderr %q", code, stdout, stderr)
	}
	if strings.Contains(stderr, "sup3r") {
		t.Errorf("value in stderr: %q", stderr)
	}
}

func TestEnvMissingSecretPrintsNothing(t *testing.T) {
	isolate(t)
	mustSave(t, "have", "", "hv")
	code, stdout, stderr := harness{}.run(t, "env", "-s", "have", "-s", "nope")
	if code != 1 || stdout != "" {
		t.Fatalf("exit %d stdout %q", code, stdout)
	}
	if !strings.Contains(stderr, `"nope"`) || strings.Contains(stderr, "hv") {
		t.Errorf("stderr %q", stderr)
	}
}

func TestEnvUsageErrors(t *testing.T) {
	isolate(t)
	mustSave(t, "a", "", "1")
	mustSave(t, "b", "", "2")
	for _, args := range [][]string{
		{"env"},
		{"env", "-s", "a=X", "-s", "b=X"},
		{"env", "-s", "a=BAD NAME"},
		{"env", "-s", "a=X#Y"},
	} {
		if code, stdout, _ := (harness{}).run(t, args...); code != exitUsage || stdout != "" {
			t.Errorf("%v: exit %d stdout %q, want usage error", args, code, stdout)
		}
	}
}

func TestGetNewlineControl(t *testing.T) {
	isolate(t)
	mustSave(t, "tok", "", "abc")
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"get", "tok"}, "abc\n"},
		{[]string{"get", "tok", "-q"}, "abc"},
		{[]string{"get", "-q", "tok"}, "abc"},
		{[]string{"get", "tok", "--newline=false"}, "abc"},
		{[]string{"get", "tok", "--newline=true"}, "abc\n"},
	} {
		code, stdout, stderr := harness{}.run(t, tc.args...)
		if code != 0 || stdout != tc.want {
			t.Errorf("%v: exit %d stdout %q (stderr %q), want %q", tc.args, code, stdout, stderr, tc.want)
		}
	}
}

func TestGetCopyKeepsValueOffStreams(t *testing.T) {
	isolate(t)
	mustSave(t, "tok", "", "clip-me")
	var copied string
	code, stdout, stderr := harness{clip: func(s string) error { copied = s; return nil }}.run(t, "get", "tok", "--copy")
	if code != 0 || copied != "clip-me" {
		t.Fatalf("exit %d copied %q stderr %q", code, copied, stderr)
	}
	if stdout != "" || strings.Contains(stderr, "clip-me") || !strings.Contains(stderr, "TUI") {
		t.Errorf("stdout %q stderr %q", stdout, stderr)
	}

	code, stdout, stderr = harness{clip: func(string) error { return errors.New("no clipboard") }}.run(t, "get", "tok", "--copy")
	if code != 1 || stdout != "" || strings.Contains(stderr, "clip-me") {
		t.Errorf("failing clipboard: exit %d stdout %q stderr %q", code, stdout, stderr)
	}
}
