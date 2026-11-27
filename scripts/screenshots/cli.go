//go:build ignore

// Command pandora-cli-demo is a tiny pretend shell for recording the
// command-line demo (assets/demo-cli.gif). It runs `pandora ...` lines
// in-process against throwaway vaults and an in-memory keyring, so nothing
// touches your real vault or OS keyring. A real keyring cannot be shared
// between separate processes in a recording, hence the in-process shell.
//
// Two fake machines, "laptop" and "server", each have their own vault and
// directory. `ssh server` switches to the server, `exit` comes back, and
// `scp FILE host:` copies a file into that machine's directory. Any other
// line goes to /bin/sh. `pandora ... > FILE` redirects stdout.
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/zalando/go-keyring"

	"github.com/RafaSG13/Pandora/internal/cli"
	"github.com/RafaSG13/Pandora/internal/store"
)

const password = "demo-master-password"

type seed struct {
	title, category, value string
	createdAgo             int
}

var (
	root     string
	machines = map[string][]seed{
		"laptop": {
			{"github-token", "Tokens", "demo-github-7Fq2Lw9Xk3Rt", 40},
			{"stripe-api-key", "Payments", "demo-stripe-Hn4Vb8Qz1Mc6", 30},
			{"openai-api-key", "Tokens", "demo-openai-Zr6Tb2Hx9Ue5", 12},
		},
		"server": {
			{"prod-db-url", "Databases", "demo-postgres-Wm3Jc8Nv1Qa4", 90},
		},
	}
	stack = []string{"laptop"}
)

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "pandora-cli-demo: %v\n", err)
	if root != "" {
		os.RemoveAll(root)
	}
	os.Exit(1)
}

func dir(machine string) string { return filepath.Join(root, machine) }

func current() string { return stack[len(stack)-1] }

// enter makes machine the active one: its directory is HOME and the cwd, and
// its vault is the unlocked one.
func enter(machine string) error {
	store.Lock()
	d := dir(machine)
	os.Setenv("HOME", d)
	os.Setenv("XDG_CONFIG_HOME", filepath.Join(d, ".config"))
	os.Setenv("AppData", d)
	if err := os.Chdir(d); err != nil {
		return err
	}
	return store.Unlock(password)
}

func setup() error {
	keyring.MockInit()
	for _, name := range []string{"laptop", "server"} {
		d := dir(name)
		if err := os.MkdirAll(d, 0o700); err != nil {
			return err
		}
		os.Setenv("HOME", d)
		os.Setenv("XDG_CONFIG_HOME", filepath.Join(d, ".config"))
		os.Setenv("AppData", d)
		store.Lock()
		if err := store.Create(password); err != nil {
			return err
		}
		for _, sd := range machines[name] {
			meta := store.SecretMeta{
				Title:    sd.title,
				Category: sd.category,
				Created:  time.Now().AddDate(0, 0, -sd.createdAgo).Format(store.DateLayout),
			}
			if err := store.SaveSecret(meta, sd.value); err != nil {
				return err
			}
		}
	}
	return enter("laptop")
}

// split breaks a line into words, honouring single and double quotes.
func split(line string) []string {
	var words []string
	var cur strings.Builder
	var quote rune
	inWord := false
	for _, r := range line {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote, inWord = r, true
		case r == ' ' || r == '\t':
			if inWord {
				words = append(words, cur.String())
				cur.Reset()
				inWord = false
			}
		default:
			cur.WriteRune(r)
			inWord = true
		}
	}
	if inWord {
		words = append(words, cur.String())
	}
	return words
}

func runPandora(words []string) {
	var out io.Writer = os.Stdout
	if n := len(words); n >= 2 && words[n-2] == ">" {
		f, err := os.Create(words[n-1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "sh: %v\n", err)
			return
		}
		defer f.Close()
		out = f
		words = words[:n-2]
	}
	cli.Run(words[1:], os.Stdin, out, os.Stderr)
}

func scp(words []string) {
	if len(words) != 3 || !strings.HasSuffix(words[2], ":") {
		fmt.Fprintln(os.Stderr, "usage: scp FILE host:")
		return
	}
	host := strings.TrimSuffix(words[2], ":")
	if _, ok := machines[host]; !ok {
		fmt.Fprintf(os.Stderr, "ssh: Could not resolve hostname %s\n", host)
		return
	}
	b, err := os.ReadFile(words[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "scp: %v\n", err)
		return
	}
	if err := os.WriteFile(filepath.Join(dir(host), filepath.Base(words[1])), b, 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "scp: %v\n", err)
		return
	}
	fmt.Printf("%-24s 100%% %5d     1.2MB/s   00:00\n", filepath.Base(words[1]), len(b))
}

// readLine reads one line a byte at a time so nothing is buffered ahead of the
// confirmation prompts that pandora itself reads from stdin.
func readLine() (string, bool) {
	var sb strings.Builder
	buf := make([]byte, 1)
	for {
		n, err := os.Stdin.Read(buf)
		if n == 1 {
			if buf[0] == '\n' {
				return sb.String(), true
			}
			sb.WriteByte(buf[0])
		}
		if err != nil {
			return sb.String(), sb.Len() > 0
		}
	}
}

func main() {
	var err error
	if root, err = os.MkdirTemp("", "pandora-cli-demo-"); err != nil {
		fatal(err)
	}
	defer os.RemoveAll(root)
	if err := setup(); err != nil {
		fatal(err)
	}
	for {
		fmt.Printf("\x1b[1;35m%s\x1b[0m:\x1b[34m~\x1b[0m$ ", current())
		line, ok := readLine()
		if !ok {
			return
		}
		words := split(strings.TrimSpace(line))
		if len(words) == 0 {
			continue
		}
		switch words[0] {
		case "pandora":
			runPandora(words)
		case "scp":
			scp(words)
		case "ssh":
			if len(words) != 2 || machines[words[1]] == nil {
				fmt.Fprintln(os.Stderr, "usage: ssh server")
				continue
			}
			stack = append(stack, words[1])
			if err := enter(words[1]); err != nil {
				fatal(err)
			}
		case "exit":
			if len(stack) == 1 {
				return
			}
			stack = stack[:len(stack)-1]
			if err := enter(current()); err != nil {
				fatal(err)
			}
		default:
			c := exec.Command("/bin/sh", "-c", line)
			c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
			_ = c.Run()
		}
	}
}
