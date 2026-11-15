package cli

import (
	"flag"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/RafaSG13/Pandora/internal/store"
)

// cmdCompletion prints the completion script of a shell.
func (a *app) cmdCompletion(args []string) error {
	usage := "completion " + strings.Join(shells, "|")
	pos, err := a.parse("completion", usage, flag.NewFlagSet("completion", flag.ContinueOnError), args, 0, 1)
	if err != nil {
		return err
	}
	valid := strings.Join(shells, ", ")
	if len(pos) == 0 {
		return usageErrorf("completion: missing shell (valid: %s)", valid)
	}
	script, ok := completionScript(pos[0], visibleCommands())
	if !ok {
		return usageErrorf("completion: unknown shell %q (valid: %s)", pos[0], valid)
	}
	_, err = io.WriteString(a.out, script)
	return err
}

// cmdComplete is the hidden helper behind the generated scripts: it prints
// completion candidates, one per line. It prints secret titles (or
// categories) and never values. It must not prompt and must not fail loudly,
// so every problem ends in empty output and exit code 0. Names are encrypted,
// so candidates exist only when PANDORA_MASTER_PASSWORD is exported.
func (a *app) cmdComplete(args []string) error {
	if len(args) == 0 {
		return nil
	}
	if !store.IsUnlocked() {
		pw := a.getenv(masterEnv)
		if pw == "" || !store.Exists() || store.Unlock(pw) != nil {
			return nil
		}
	}
	items, err := store.LoadMetadata()
	if err != nil {
		return nil
	}
	var lines []string
	switch args[0] {
	case "titles":
		for _, m := range items {
			lines = append(lines, m.Title)
		}
	case "categories":
		seen := map[string]bool{}
		for _, m := range items {
			if k := strings.ToLower(m.Category); m.Category != "" && !seen[k] {
				seen[k] = true
				lines = append(lines, m.Category)
			}
		}
		slices.Sort(lines)
	}
	for _, l := range lines {
		if strings.ContainsAny(l, "\r\n") {
			continue // cannot be represented one per line
		}
		fmt.Fprintln(a.out, l)
	}
	return nil
}

// completionScript renders the completion script of shell for cmds.
func completionScript(shell string, cmds []command) (string, bool) {
	var b strings.Builder
	switch shell {
	case "bash":
		writeBash(&b, cmds)
	case "zsh":
		writeZsh(&b, cmds)
	case "fish":
		writeFish(&b, cmds)
	case "powershell":
		writePowerShell(&b, cmds)
	default:
		return "", false
	}
	return b.String(), true
}

func commandNames(cmds []command) []string {
	names := make([]string, 0, len(cmds))
	for _, c := range cmds {
		names = append(names, c.name)
	}
	return names
}

func flagTokens(f flagSpec) []string {
	toks := make([]string, 0, len(f.names))
	for _, n := range f.names {
		toks = append(toks, flagToken(n))
	}
	return toks
}

// allFlagTokens lists every flag spelling of a command.
func allFlagTokens(c command) []string {
	var toks []string
	for _, f := range c.flags {
		toks = append(toks, flagTokens(f)...)
	}
	return toks
}

// completeKind is the argument of `pandora __complete` for a dynamic kind.
func completeKind(k argKind) string {
	if k == argCategory {
		return "categories"
	}
	return "titles"
}

// shQuote quotes s for a POSIX shell (also valid for zsh).
func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// ---- bash ----

func writeBash(b *strings.Builder, cmds []command) {
	b.WriteString(`# bash completion for pandora
# Load it with: source <(pandora completion bash)

_pandora_complete() {
    # $1: titles or categories; matches against $cur of the caller
    local line
    while IFS= read -r line; do
        if [[ $line == "$cur"* ]]; then
            COMPREPLY+=("$(printf '%q' "$line")")
        fi
    done < <("${COMP_WORDS[0]}" __complete "$1" 2>/dev/null)
}

_pandora_files() {
    local IFS=$'\n'
    compopt -o filenames 2>/dev/null
    COMPREPLY=($(compgen -f -- "$cur"))
}

_pandora() {
    local cur prev cmd i
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"

    if (( COMP_CWORD == 1 )); then
        COMPREPLY=($(compgen -W ` + shQuote(strings.Join(commandNames(cmds), " ")) + ` -- "$cur"))
        return 0
    fi
    cmd="${COMP_WORDS[1]}"

    # After "--" the rest is another command line.
    for (( i = 2; i < COMP_CWORD; i++ )); do
        if [[ ${COMP_WORDS[i]} == -- ]]; then
            _pandora_files
            return 0
        fi
    done

    # The value of a flag.
    case "$cmd:$prev" in
`)
	for _, c := range cmds {
		for _, f := range c.flags {
			if f.arg.kind == argNone {
				continue
			}
			var labels []string
			for _, t := range flagTokens(f) {
				labels = append(labels, c.name+":"+t)
			}
			fmt.Fprintf(b, "    %s) %s; return 0 ;;\n", strings.Join(labels, "|"), bashAction(f.arg))
		}
	}
	b.WriteString(`    esac

    case "$cmd" in
`)
	for _, c := range cmds {
		flags := "COMPREPLY=($(compgen -W " + shQuote(strings.Join(allFlagTokens(c), " ")) + ` -- "$cur"))`
		switch c.args.kind {
		case argTitle, argFile, argChoice:
			fmt.Fprintf(b, "    %s)\n        if [[ $cur == -* ]]; then\n            %s\n        else\n            %s\n        fi ;;\n",
				c.name, flags, bashAction(c.args))
		default:
			fmt.Fprintf(b, "    %s) %s ;;\n", c.name, flags)
		}
	}
	b.WriteString(`    esac
    return 0
}

complete -F _pandora pandora
`)
}

func bashAction(a argSpec) string {
	switch a.kind {
	case argTitle:
		return "_pandora_complete titles"
	case argCategory:
		return "_pandora_complete categories"
	case argFile:
		return "_pandora_files"
	case argChoice:
		return `COMPREPLY=($(compgen -W ` + shQuote(strings.Join(a.choices, " ")) + ` -- "$cur"))`
	}
	return ":" // free text: nothing to offer
}

// ---- zsh ----

func writeZsh(b *strings.Builder, cmds []command) {
	b.WriteString(`#compdef pandora
# zsh completion for pandora
# Load it with: source <(pandora completion zsh)

_pandora_complete() {
    # $1: titles or categories, $2: description
    local -a items
    items=(${(f)"$("$_pandora_bin" __complete "$1" 2>/dev/null)"})
    _wanted "$1" expl "$2" compadd -a items
}

_pandora_titles() { _pandora_complete titles 'secret title' }

_pandora_categories() { _pandora_complete categories category }

_pandora() {
    local curcontext="$curcontext" state line
    local _pandora_bin=$words[1]
    typeset -A opt_args

    _arguments -C \
        '1:command:->command' \
        '*::argument:->args'

    case $state in
    command)
        local -a commands
        commands=(
`)
	for _, c := range cmds {
		fmt.Fprintf(b, "            %s\n", shQuote(c.name+":"+c.summary))
	}
	b.WriteString(`        )
        _describe -t commands 'pandora command' commands
        ;;
    args)
        case $line[1] in
`)
	for _, c := range cmds {
		specs := zshSpecs(c)
		if len(specs) == 0 {
			fmt.Fprintf(b, "        %s) _nothing ;;\n", c.name)
			continue
		}
		fmt.Fprintf(b, "        %s)\n            _arguments -S \\\n", c.name)
		for i, s := range specs {
			sep := " \\"
			if i == len(specs)-1 {
				sep = ""
			}
			fmt.Fprintf(b, "                %s%s\n", s, sep)
		}
		b.WriteString("            ;;\n")
	}
	b.WriteString(`        esac
        ;;
    esac
}

if [ "$funcstack[1]" = "_pandora" ]; then
    _pandora "$@"
else
    compdef _pandora pandora
fi
`)
}

func zshSpecs(c command) []string {
	var specs []string
	for _, f := range c.flags {
		toks := flagTokens(f)
		tail := "[" + zshBracket(f.help) + "]" + zshAction(f.arg, "value")
		switch {
		case len(toks) == 1 && f.repeat:
			specs = append(specs, shQuote("*"+toks[0]+tail))
		case len(toks) == 1:
			specs = append(specs, shQuote(toks[0]+tail))
		case f.repeat:
			specs = append(specs, "'*'{"+strings.Join(toks, ",")+"}"+shQuote(tail))
		default:
			specs = append(specs, shQuote("("+strings.Join(toks, " ")+")")+"{"+strings.Join(toks, ",")+"}"+shQuote(tail))
		}
	}
	switch c.args.kind {
	case argTitle, argFile, argChoice, argText:
		specs = append(specs, shQuote("1"+zshAction(c.args, "argument")))
	case argCommand:
		specs = append(specs, `'*:: :_normal'`)
	}
	return specs
}

// zshAction is the ":message:action" tail of an _arguments spec.
func zshAction(a argSpec, message string) string {
	switch a.kind {
	case argTitle:
		return ":secret title:_pandora_titles"
	case argCategory:
		return ":category:_pandora_categories"
	case argFile:
		return ":file:_files"
	case argChoice:
		return ":" + message + ":(" + strings.Join(a.choices, " ") + ")"
	case argText:
		return ":" + message + ": " // a lone space: show the message, offer nothing
	}
	return ""
}

func zshBracket(s string) string {
	return strings.NewReplacer(`\`, `\\`, "[", `\[`, "]", `\]`).Replace(s)
}

// ---- fish ----

func writeFish(b *strings.Builder, cmds []command) {
	b.WriteString(`# fish completion for pandora
# Load it with: pandora completion fish | source

function __pandora_complete
    set -l bin (commandline -opc)[1]
    command $bin __complete $argv[1] 2>/dev/null
end

complete -c pandora -f
`)
	for _, c := range cmds {
		fmt.Fprintf(b, "complete -c pandora -n __fish_use_subcommand -a %s -d %s\n", c.name, fishQuote(c.summary))
	}
	for _, c := range cmds {
		cond := "-n " + fishQuote("__fish_seen_subcommand_from "+c.name)
		for _, f := range c.flags {
			var names []string
			for _, n := range f.names {
				if len(n) == 1 {
					names = append(names, "-s "+n)
				} else {
					names = append(names, "-l "+n)
				}
			}
			fmt.Fprintf(b, "complete -c pandora %s %s%s -d %s\n", cond, strings.Join(names, " "), fishArg(f.arg), fishQuote(f.help))
		}
		switch c.args.kind {
		case argTitle, argFile, argChoice:
			fmt.Fprintf(b, "complete -c pandora %s%s\n", cond, fishArg(c.args))
		}
	}
}

// fishArg is the option tail that makes fish expect and complete a value.
func fishArg(a argSpec) string {
	switch a.kind {
	case argTitle, argCategory:
		return " -x -a " + fishQuote("("+"__pandora_complete "+completeKind(a.kind)+")")
	case argFile:
		return " -r -F"
	case argChoice:
		return " -x -a " + fishQuote(strings.Join(a.choices, " "))
	case argText:
		return " -x"
	}
	return ""
}

func fishQuote(s string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, "'", `\'`).Replace(s) + "'"
}

// ---- PowerShell ----

func writePowerShell(b *strings.Builder, cmds []command) {
	b.WriteString(`# PowerShell completion for pandora
# Load it with: pandora completion powershell | Out-String | Invoke-Expression

$script:PandoraCommands = [ordered]@{
`)
	for _, c := range cmds {
		fmt.Fprintf(b, "    %s = %s\n", psQuote(c.name), psQuote(c.summary))
	}
	b.WriteString("}\n\n$script:PandoraSpec = @{\n")
	for _, c := range cmds {
		fmt.Fprintf(b, "    %s = @{\n        Positional = %s\n        Flags = [ordered]@{\n", psQuote(c.name), psArg(c.args))
		for _, f := range c.flags {
			for _, t := range flagTokens(f) {
				fmt.Fprintf(b, "            %s = %s\n", psQuote(t), psArgHelp(f.arg, f.help))
			}
		}
		b.WriteString("        }\n    }\n")
	}
	b.WriteString(`}

function script:Get-PandoraCandidates([string]$Exe, [string]$Kind) {
    try { $out = & $Exe __complete $Kind 2>$null } catch { return @() }
    return @($out | Where-Object { $_ })
}

function script:New-PandoraResult([string]$Text, [string]$Tooltip) {
    $completion = $Text
    if ($Text -match '[^\w.\-/:=@+,]') { $completion = "'" + $Text.Replace("'", "''") + "'" }
    if (-not $Tooltip) { $Tooltip = $Text }
    return [System.Management.Automation.CompletionResult]::new($completion, $Text, 'ParameterValue', $Tooltip)
}

Register-ArgumentCompleter -Native -CommandName 'pandora' -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)

    $exe = $commandAst.CommandElements[0].ToString()
    $typed = @()
    foreach ($element in ($commandAst.CommandElements | Select-Object -Skip 1)) { $typed += $element.ToString() }
    $limit = $typed.Count
    if ($wordToComplete -ne '') { $limit-- }
    $before = @()
    for ($i = 0; $i -lt $limit; $i++) { $before += $typed[$i] }

    $results = @()
    if ($before.Count -eq 0) {
        foreach ($name in $script:PandoraCommands.Keys) {
            if ($name.StartsWith($wordToComplete, [System.StringComparison]::OrdinalIgnoreCase)) {
                $results += New-PandoraResult $name $script:PandoraCommands[$name]
            }
        }
        return $results
    }

    $cmd = $before[0]
    if (-not $script:PandoraSpec.Contains($cmd)) { return }
    if ($before -contains '--') { return }
    $spec = $script:PandoraSpec[$cmd]

    $arg = $null
    if ($before.Count -gt 1 -and $spec.Flags.Contains($before[$before.Count - 1]) -and $spec.Flags[$before[$before.Count - 1]].Kind -ne '') {
        $arg = $spec.Flags[$before[$before.Count - 1]]
    } elseif (-not $wordToComplete.StartsWith('-') -and @('title', 'file', 'choice') -contains $spec.Positional.Kind) {
        $arg = $spec.Positional
    }

    if ($null -eq $arg) {
        foreach ($flag in $spec.Flags.Keys) {
            if ($flag.StartsWith($wordToComplete, [System.StringComparison]::OrdinalIgnoreCase)) {
                $results += New-PandoraResult $flag $spec.Flags[$flag].Help
            }
        }
        return $results
    }

    $candidates = @()
    switch ($arg.Kind) {
        'title'    { $candidates = Get-PandoraCandidates $exe 'titles' }
        'category' { $candidates = Get-PandoraCandidates $exe 'categories' }
        'choice'   { $candidates = $arg.Choices }
        default    { return }   # files and free text: fall back to PowerShell's own completion
    }
    foreach ($candidate in $candidates) {
        if ($candidate.StartsWith($wordToComplete, [System.StringComparison]::OrdinalIgnoreCase)) {
            $results += New-PandoraResult $candidate $null
        }
    }
    return $results
}
`)
}

func psArg(a argSpec) string { return psArgHelp(a, "") }

func psArgHelp(a argSpec, help string) string {
	choices := make([]string, 0, len(a.choices))
	for _, c := range a.choices {
		choices = append(choices, psQuote(c))
	}
	return fmt.Sprintf("@{ Kind = %s; Choices = @(%s); Help = %s }", psQuote(string(a.kind)), strings.Join(choices, ", "), psQuote(help))
}

func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
