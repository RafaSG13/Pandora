package cli

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

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
	// The real import parser must read back what env printed; the empty
	// value is the one thing it declines to import.
	batch, err := parseDotenv([]byte(stdout), importOptions{})
	if err != nil {
		t.Fatalf("%v (output %q)", err, stdout)
	}
	got := map[string]string{}
	for _, s := range batch.secrets {
		got[s.Title] = *s.Value
	}
	for i, v := range values {
		if v != "" && got[fmt.Sprintf("K%d", i)] != v {
			t.Errorf("K%d = %q, want %q (output %q)", i, got[fmt.Sprintf("K%d", i)], v, stdout)
		}
	}
	if len(got) != len(values)-1 || len(batch.skipped) != 1 {
		t.Errorf("got %d variables and %d skipped, want %d and 1", len(got), len(batch.skipped), len(values)-1)
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
