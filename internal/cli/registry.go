package cli

import (
	"fmt"
	"strings"

	"github.com/RafaSG13/Pandora/internal/config"
	"github.com/RafaSG13/Pandora/internal/gen"
)

// argKind says what a flag value or a positional argument looks like, which
// is what shell completion needs to know.
type argKind string

const (
	argNone     argKind = ""         // no argument (a boolean flag, or no positional)
	argText     argKind = "text"     // free text, nothing to complete
	argFile     argKind = "file"     // a file name
	argTitle    argKind = "title"    // the title of a stored secret
	argCategory argKind = "category" // a category already in use
	argChoice   argKind = "choice"   // one of argSpec.choices
	argCommand  argKind = "command"  // the remainder is another command line
)

type argSpec struct {
	kind    argKind
	choices []string // only for argChoice
}

// flagSpec describes one flag. names are spelled without dashes; a
// single-letter name is a short flag (-c), anything longer a long one (--category).
type flagSpec struct {
	names  []string
	help   string
	arg    argSpec // argNone for boolean flags
	repeat bool    // may be given more than once
}

// command is one entry of the command table. The table drives the usage
// text, the dispatch in run and the generated shell completions, so the
// three cannot drift apart. A test checks the flags against the flag sets the
// commands really define.
type command struct {
	name    string
	aliases []string // extra names accepted by the dispatcher only
	summary string
	hidden  bool // dispatched but not shown in usage or completions
	flags   []flagSpec
	args    argSpec // what the positional arguments complete to
	run     func(a *app, args []string) error
}

// shells lists the shells `pandora completion` can generate for.
var shells = []string{"bash", "zsh", "fish", "powershell"}

// commands returns the command table in the order shown by `pandora help`.
// It is a function, not a variable, so the run funcs can refer back to it.
func commands() []command {
	charsets := charsetNames()
	yes := func(help string) flagSpec { return flagSpec{names: []string{"y", "yes"}, help: help} }
	title := argSpec{kind: argTitle}

	return []command{
		{
			name: "list", summary: "List secrets (metadata only)",
			flags: []flagSpec{
				{names: []string{"c", "category"}, help: "only secrets in this category", arg: argSpec{kind: argCategory}},
				{names: []string{"t", "tag"}, help: "only secrets with this tag (repeat or comma-separate to require several)", arg: argSpec{kind: argText}, repeat: true},
				{names: []string{"json"}, help: "print JSON instead of a table"},
			},
			run: (*app).cmdList,
		},
		{
			name: "expiring", summary: "List expired and expiring secrets (exit 3 if any)",
			flags: []flagSpec{
				{names: []string{"within"}, help: "window in days, e.g. 14d (default: expiry_warn_days from the config file)", arg: argSpec{kind: argText}},
				{names: []string{"json"}, help: "print JSON instead of a table"},
			},
			run: (*app).cmdExpiring,
		},
		{
			name: "get", summary: "Print a secret's value",
			flags: []flagSpec{
				{names: []string{"q"}, help: "no trailing newline (same as --newline=false)"},
				{names: []string{"newline"}, help: "print a trailing newline (default true)"},
				{names: []string{"copy"}, help: "copy to the clipboard instead of printing"},
				{names: []string{"version"}, help: "0 is the current value; -1 the previous one, -2 the one before", arg: argSpec{kind: argText}},
			},
			args: title,
			run:  (*app).cmdGet,
		},
		{
			name: "history", summary: "List the previous values of a secret (dates only, never values)",
			flags: []flagSpec{
				{names: []string{"json"}, help: "print JSON instead of a table"},
			},
			args: title,
			run:  (*app).cmdHistory,
		},
		{
			name: "restore", summary: "Make a previous value of a secret current again",
			flags: []flagSpec{
				{names: []string{"version"}, help: "which previous value: -1 is the latest (default), -2 the one before", arg: argSpec{kind: argText}},
			},
			args: title,
			run:  (*app).cmdRestore,
		},
		{
			name: "add", summary: "Add a secret (value from stdin or a hidden prompt)",
			flags: []flagSpec{
				{names: []string{"c", "category"}, help: "category", arg: argSpec{kind: argCategory}},
				{names: []string{"t", "tag"}, help: "tag (repeat or comma-separate for several)", arg: argSpec{kind: argText}, repeat: true},
				{names: []string{"notes"}, help: "free-form notes", arg: argSpec{kind: argText}},
				{names: []string{"expires"}, help: "expiry date, YYYY-MM-DD", arg: argSpec{kind: argText}},
			},
			args: argSpec{kind: argText},
			run:  (*app).cmdAdd,
		},
		{
			name: "rm", summary: "Delete a secret",
			flags: []flagSpec{yes("delete without asking")},
			args:  title,
			run:   (*app).cmdRm,
		},
		{
			name: "run", summary: "Run a command with secrets injected as environment variables",
			flags: []flagSpec{
				{names: []string{"s", "secret"}, help: "secret to inject, TITLE or TITLE=ENV_NAME", arg: title, repeat: true},
			},
			args: argSpec{kind: argCommand},
			run:  (*app).cmdRun,
		},
		{
			name: "env", summary: "Print secrets as KEY=value dotenv lines",
			flags: []flagSpec{
				{names: []string{"s", "secret"}, help: "secret to print, TITLE or TITLE=ENV_NAME", arg: title, repeat: true},
				{names: []string{"force"}, help: "print even when stdout is a terminal"},
			},
			run: (*app).cmdEnv,
		},
		{
			name: "export", summary: "Export metadata, or encrypted metadata and values",
			flags: []flagSpec{
				{names: []string{"values"}, help: "include values, encrypted with a passphrase"},
				{names: []string{"o", "output"}, help: "write to FILE instead of stdout", arg: argSpec{kind: argFile}},
				{names: []string{"force"}, help: "overwrite an existing output file"},
			},
			run: (*app).cmdExport,
		},
		{
			name: "import", summary: "Import a Pandora export, a dotenv file or a Bitwarden JSON export",
			flags: []flagSpec{
				{names: []string{"format"}, help: "format of FILE (default pandora)", arg: argSpec{kind: argChoice, choices: importFormats}},
				{names: []string{"values"}, help: "restore the values in an encrypted Pandora export"},
				{names: []string{"overwrite"}, help: "replace secrets that already exist"},
				{names: []string{"dry-run"}, help: "list what would change and write nothing"},
				{names: []string{"category"}, help: "with --format dotenv: category for every imported secret", arg: argSpec{kind: argCategory}},
			},
			args: argSpec{kind: argFile},
			run:  (*app).cmdImport,
		},
		{
			name: "doctor", summary: "Check the vault files and the keyring values",
			flags: []flagSpec{
				{names: []string{"fix"}, help: "repair: restore a damaged vault.json from its backup, remove entries whose value is missing"},
				yes("with --fix, repair without asking"),
			},
			run: (*app).cmdDoctor,
		},
		{name: "passwd", summary: "Change the master password", run: (*app).cmdPasswd},
		{
			name: "gen", summary: "Generate random secrets (printed to stdout, not stored)",
			flags: []flagSpec{
				{names: []string{"length"}, help: fmt.Sprintf("length, %d to %d (default %d, or gen_length from the config file)", gen.MinLength, gen.MaxLength, config.DefaultGenLength), arg: argSpec{kind: argText}},
				{names: []string{"charset"}, help: "alphabet (default alnum, or gen_charset from the config file)", arg: argSpec{kind: argChoice, choices: charsets}},
				{names: []string{"count"}, help: fmt.Sprintf("how many, 1 to %d (default 1)", maxGenCount), arg: argSpec{kind: argText}},
			},
			run: (*app).cmdGen,
		},
		{
			name: "config", summary: "Show the config file path or the effective settings",
			args: argSpec{kind: argChoice, choices: []string{"path", "get"}},
			run:  (*app).cmdConfig,
		},
		{name: "version", aliases: []string{"--version"}, summary: "Print version information (also --version)", run: (*app).cmdVersion},
		{
			name: "completion", summary: "Print a shell completion script (" + strings.Join(shells, ", ") + ")",
			args: argSpec{kind: argChoice, choices: shells},
			run:  (*app).cmdCompletion,
		},
		{name: "help", aliases: []string{"-h", "-help", "--help"}, summary: "Show this help", run: (*app).cmdHelp},
		{name: "__complete", hidden: true, summary: "Print completion candidates for the shell scripts", run: (*app).cmdComplete},
	}
}

// lookup finds a command by name or alias.
func lookup(name string) (command, bool) {
	for _, c := range commands() {
		if c.name == name {
			return c, true
		}
		for _, alias := range c.aliases {
			if alias == name {
				return c, true
			}
		}
	}
	return command{}, false
}

// visibleCommands returns the commands that appear in usage and completions.
func visibleCommands() []command {
	var out []command
	for _, c := range commands() {
		if !c.hidden {
			out = append(out, c)
		}
	}
	return out
}

// flagToken spells a flag name as typed on the command line.
func flagToken(name string) string {
	if len(name) == 1 {
		return "-" + name
	}
	return "--" + name
}

// topUsage renders the top-level help from the command table.
func topUsage() string {
	cmds := visibleCommands()
	width := 0
	for _, c := range cmds {
		width = max(width, len(c.name))
	}
	var b strings.Builder
	b.WriteString("Usage: pandora [command]\n\nWithout a command Pandora opens the interactive interface.\n\nCommands:\n")
	for _, c := range cmds {
		fmt.Fprintf(&b, "  %-*s  %s\n", width, c.name, c.summary)
	}
	b.WriteString("\nRun 'pandora <command> -h' for the flags of a command.\n")
	return b.String()
}
