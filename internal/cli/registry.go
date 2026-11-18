package cli

import (
	"fmt"
	"strings"

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
		{name: "get", summary: "Print a secret's value", args: title, run: (*app).cmdGet},
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
			name: "export", summary: "Export metadata, or encrypted metadata and values",
			flags: []flagSpec{
				{names: []string{"values"}, help: "include values, encrypted with a passphrase"},
				{names: []string{"o", "output"}, help: "write to FILE instead of stdout", arg: argSpec{kind: argFile}},
				{names: []string{"force"}, help: "overwrite an existing output file"},
			},
			run: (*app).cmdExport,
		},
		{
			name: "import", summary: "Import an export file",
			flags: []flagSpec{
				{names: []string{"values"}, help: "restore the values in an encrypted export"},
				{names: []string{"overwrite"}, help: "replace secrets that already exist"},
			},
			args: argSpec{kind: argFile},
			run:  (*app).cmdImport,
		},
		{
			name: "doctor", summary: "Check metadata against the keyring",
			flags: []flagSpec{
				{names: []string{"fix"}, help: "remove metadata entries whose value is missing"},
				yes("with --fix, remove without asking"),
			},
			run: (*app).cmdDoctor,
		},
		{name: "passwd", summary: "Change the master password", run: (*app).cmdPasswd},
		{
			name: "config", summary: "Show the settings file path and the effective settings",
			args: argSpec{kind: argChoice, choices: []string{"path", "get"}},
			run:  (*app).cmdConfig,
		},
		{
			name: "gen", summary: "Generate random secrets (printed to stdout, not stored)",
			flags: []flagSpec{
				{names: []string{"length"}, help: fmt.Sprintf("length, %d to %d (default %d)", gen.MinLength, gen.MaxLength, defaultGenLength), arg: argSpec{kind: argText}},
				{names: []string{"charset"}, help: "alphabet (default alnum)", arg: argSpec{kind: argChoice, choices: charsets}},
				{names: []string{"count"}, help: fmt.Sprintf("how many, 1 to %d (default 1)", maxGenCount), arg: argSpec{kind: argText}},
			},
			run: (*app).cmdGen,
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
