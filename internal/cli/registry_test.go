package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/RafaSG13/Pandora/internal/config"
	"github.com/RafaSG13/Pandora/internal/gen"
	"github.com/RafaSG13/Pandora/internal/version"
)

func TestVersionAndDashDashVersionAgree(t *testing.T) {
	var outputs []string
	for _, arg := range []string{"version", "--version"} {
		code, stdout, stderr := harness{}.run(t, arg)
		if code != 0 || stderr != "" {
			t.Fatalf("%s: exit %d, stderr %q", arg, code, stderr)
		}
		outputs = append(outputs, stdout)
	}
	if outputs[0] != outputs[1] || outputs[0] != version.String()+"\n" {
		t.Errorf("version = %q, --version = %q, want %q", outputs[0], outputs[1], version.String()+"\n")
	}
}

func TestVersionRejectsExtraArguments(t *testing.T) {
	for _, args := range [][]string{{"version", "x"}, {"--version", "x"}, {"version", "--bogus"}} {
		if code, stdout, _ := (harness{}).run(t, args...); code != 2 || stdout != "" {
			t.Errorf("%v = %d %q, want exit 2 and no output", args, code, stdout)
		}
	}
}

func TestHelpListsEveryVisibleCommandAndHidesInternalOnes(t *testing.T) {
	_, stdout, _ := harness{}.run(t, "help")
	for _, c := range commands() {
		if listed := regexp.MustCompile(`(?m)^  ` + regexp.QuoteMeta(c.name) + ` `).MatchString(stdout); listed == c.hidden {
			t.Errorf("command %q: listed in help = %v, hidden = %v", c.name, listed, c.hidden)
		}
	}
	for _, name := range []string{"gen", "version", "completion"} {
		if !strings.Contains(stdout, "  "+name+" ") {
			t.Errorf("help does not list %q", name)
		}
	}
}

func TestGenDefaults(t *testing.T) {
	isolate(t) // gen reads config.json: never the developer's real one
	code, stdout, stderr := harness{}.run(t, "gen")
	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	if code != 0 || stderr != "" || len(lines) != 1 || len(lines[0]) != config.DefaultGenLength {
		t.Fatalf("gen = %d %q (stderr %q), want one %d-character line", code, stdout, stderr, config.DefaultGenLength)
	}
}

func TestGenLengthCharsetAndCount(t *testing.T) {
	isolate(t)
	for _, cs := range gen.Charsets {
		for _, n := range []int{gen.MinLength, 12, gen.MaxLength} {
			code, stdout, stderr := harness{}.run(t, "gen", "--charset", cs.Key, "--length", strconv.Itoa(n), "--count", "3")
			if code != 0 || stderr != "" {
				t.Fatalf("gen %s/%d: exit %d, stderr %q", cs.Key, n, code, stderr)
			}
			lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
			if len(lines) != 3 {
				t.Fatalf("gen %s/%d printed %d lines, want 3", cs.Key, n, len(lines))
			}
			alphabet := strings.Join(cs.Classes, "")
			for _, line := range lines {
				if len(line) != n {
					t.Errorf("gen %s/%d: line has length %d", cs.Key, n, len(line))
				}
				for _, r := range line {
					if !strings.ContainsRune(alphabet, r) {
						t.Errorf("gen %s: character %q outside the charset", cs.Key, r)
					}
				}
			}
		}
	}
}

func TestGenCountBounds(t *testing.T) {
	isolate(t)
	code, stdout, _ := harness{}.run(t, "gen", "--count", strconv.Itoa(maxGenCount))
	if code != 0 || strings.Count(stdout, "\n") != maxGenCount {
		t.Errorf("--count %d: exit %d, %d lines", maxGenCount, code, strings.Count(stdout, "\n"))
	}
}

func TestGenRejectsBadFlagsAsUsageErrors(t *testing.T) {
	isolate(t)
	tests := []struct {
		name string
		args []string
		want string // text the message must contain
	}{
		{"length below range", []string{"--length", strconv.Itoa(gen.MinLength - 1)}, strconv.Itoa(gen.MinLength) + " and " + strconv.Itoa(gen.MaxLength)},
		{"length above range", []string{"--length", strconv.Itoa(gen.MaxLength + 1)}, "--length"},
		{"length zero", []string{"--length", "0"}, "--length"},
		{"length not a number", []string{"--length", "abc"}, "length"},
		{"unknown charset", []string{"--charset", "emoji"}, "alnum, symbols, hex"},
		{"count zero", []string{"--count", "0"}, "1 and " + strconv.Itoa(maxGenCount)},
		{"count above range", []string{"--count", strconv.Itoa(maxGenCount + 1)}, "--count"},
		{"count negative", []string{"--count", "-3"}, "--count"},
		{"positional", []string{"extra"}, "pandora gen:"},
	}
	for _, tc := range tests {
		code, stdout, stderr := harness{}.run(t, append([]string{"gen"}, tc.args...)...)
		if code != 2 || stdout != "" || !strings.Contains(stderr, tc.want) {
			t.Errorf("%s: exit %d, stdout %q, stderr %q; want exit 2, no stdout, stderr containing %q", tc.name, code, stdout, stderr, tc.want)
		}
	}
}

// The command table must describe the flags the commands really define.
func TestCommandTableMatchesFlagSets(t *testing.T) {
	isolate(t) // `gen -h` prints the defaults, which come from config.json
	flagLine := regexp.MustCompile(`(?m)^  -(\S+)`)
	for _, c := range visibleCommands() {
		code, stdout, stderr := harness{}.run(t, c.name, "-h")
		if code != 0 {
			t.Fatalf("%s -h: exit %d, stderr %q", c.name, code, stderr)
		}
		var defined []string
		for _, m := range flagLine.FindAllStringSubmatch(stdout, -1) {
			defined = append(defined, m[1])
		}
		var declared []string
		for _, f := range c.flags {
			declared = append(declared, f.names...)
		}
		slices.Sort(defined)
		slices.Sort(declared)
		if !slices.Equal(defined, declared) {
			t.Errorf("%s: table flags %v, flag set defines %v", c.name, declared, defined)
		}
	}
}

func TestCompletionScriptsMentionEveryCommandAndFlag(t *testing.T) {
	// How each shell's script spells a flag name.
	spell := map[string]func(name string) string{
		"bash":       flagToken,
		"zsh":        flagToken,
		"powershell": flagToken,
		"fish": func(name string) string {
			if len(name) == 1 {
				return "-s " + name
			}
			return "-l " + name
		},
	}
	for _, shell := range shells {
		code, script, stderr := harness{}.run(t, "completion", shell)
		if code != 0 || stderr != "" || script == "" {
			t.Fatalf("completion %s: exit %d, stderr %q", shell, code, stderr)
		}
		for _, c := range visibleCommands() {
			if !strings.Contains(script, c.name) {
				t.Errorf("%s: command %q missing", shell, c.name)
			}
			for _, f := range c.flags {
				for _, n := range f.names {
					if want := spell[shell](n); !strings.Contains(script, want) {
						t.Errorf("%s: flag %q of %q missing (looked for %q)", shell, n, c.name, want)
					}
				}
			}
		}
		if !strings.Contains(script, "__complete") {
			t.Errorf("%s: script never calls the __complete helper", shell)
		}
	}
}

func TestCompletionUsageErrors(t *testing.T) {
	for _, args := range [][]string{{"completion"}, {"completion", "tcsh"}, {"completion", "bash", "zsh"}} {
		code, stdout, stderr := harness{}.run(t, args...)
		if code != 2 || stdout != "" {
			t.Errorf("%v = %d %q, want exit 2 and no output", args, code, stdout)
		}
		if len(args) < 3 && !strings.Contains(stderr, "bash, zsh, fish, powershell") {
			t.Errorf("%v: stderr %q does not list the valid shells", args, stderr)
		}
	}
}

func TestCompleteTitlesPrintsTitlesOnly(t *testing.T) {
	isolate(t)
	mustSave(t, "github", "Tokens", "value-one-9f3")
	mustSave(t, "my db", "Databases", "value-two-7c1")
	mustSave(t, "stripe", "Tokens", "value-three-2b8")

	code, stdout, stderr := harness{}.run(t, "__complete", "titles")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	if want := []string{"github", "my db", "stripe"}; !slices.Equal(strings.Split(strings.TrimSuffix(stdout, "\n"), "\n"), want) {
		t.Errorf("titles = %q, want %v", stdout, want)
	}
	for _, leak := range []string{"value-", "Tokens", "Databases", "note "} {
		if strings.Contains(stdout, leak) {
			t.Errorf("output leaks %q: %q", leak, stdout)
		}
	}
}

func TestCompleteCategoriesAreUniqueAndSorted(t *testing.T) {
	isolate(t)
	mustSave(t, "a", "Tokens", "v")
	mustSave(t, "b", "databases", "v")
	mustSave(t, "c", "tokens", "v")
	mustSave(t, "d", "", "v")

	_, stdout, _ := harness{}.run(t, "__complete", "categories")
	if want := "Tokens\ndatabases\n"; stdout != want {
		t.Errorf("categories = %q, want %q", stdout, want)
	}
}

func TestCompleteNeverFailsOrPrompts(t *testing.T) {
	isolate(t)
	mustSave(t, "github", "Tokens", "v")

	// No kind and an unknown kind print nothing; neither may prompt or complain.
	for _, args := range [][]string{{"__complete"}, {"__complete", "bogus"}} {
		code, stdout, stderr := harness{tty: true}.run(t, args...)
		if code != 0 || stdout != "" || stderr != "" {
			t.Errorf("%v: exit %d, stdout %q, stderr %q; want silent success", args, code, stdout, stderr)
		}
	}

	// A store error ends quietly too.
	dir, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "pandora", "vault.json")
	if err := os.WriteFile(file, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := harness{tty: true}.run(t, "__complete", "titles")
	if code != 0 || stdout != "" || stderr != "" {
		t.Errorf("broken vault: exit %d, stdout %q, stderr %q; want silent success", code, stdout, stderr)
	}
}
