# Command-line guide

`pandora` with no arguments opens the interface. With a command it runs non-interactively, which suits scripts and CI. Data goes to stdout, messages and errors to stderr. Exit codes: `0` success, `1` runtime error (including a refused confirmation), `2` usage error, `3` only from `pandora expiring` (something is expired or expiring). `pandora help` and `pandora <command> -h` list the flags. Flags may come before or after the arguments.

| Command | What it does |
| ------- | ------------ |
| `pandora list [--category C] [--tag T] [--json]` | Titles, categories, creation and expiry dates, and tags. Never shows values. `--tag` (`-t`) keeps secrets that have the tag; repeat it or separate with commas to require several. `--json` always includes `tags` (an empty list when there are none) and includes `username` when the secret has one. |
| `pandora expiring [--within 14d] [--json]` | Lists secrets that have expired or expire within the window, soonest first (never values). The window is `--within` (days, `14` or `14d`) or else `expiry_warn_days` from the config file (default 14). Exits `3` when the list is not empty and `0` when nothing is due, so cron and CI can act on it. `--json` prints an array of `{title, category, expires, days, status}`: `days` is negative once expired and `status` is `expired` or `expiring` (today counts as expiring); an empty result is `[]`. |
| `pandora get <title> [-q] [--newline=false] [--copy]` | Prints the value followed by a newline, so `add` then `get` reproduces the file you piped in. `-q` or `--newline=false` omits the newline (for `$(...)` and pipes). `--copy` puts the value on the clipboard instead of printing it, concealed like the TUI copy; a detached helper clears it after `clipboard_seconds` if it still holds the value, even though the command has already exited. Exit `1` if the secret does not exist. |
| `pandora history <title> [--json]` | Lists the previous values of a secret, newest first: `-1` is the latest, with the date it stopped being current. Never prints values. `--json` prints `[{version, replaced}]`; a secret without history gives `[]`. |
| `pandora get <title> --version -1` | `get` also reads a previous value: `0` is the current one (default), `-1` the one before, `-2` the one before that. Exit `1` if there is no such version. |
| `pandora restore <title> [--version -1]` | Makes a previous value current again. The value it replaces becomes `-1`, so a restore can be undone with another `restore`. Only the vault changes; nothing is rewritten in the keyring. |
| `pandora log [--since 7d\|12h\|YYYY-MM-DD] [--json]` | Prints the audit log, oldest first: time (local), action and secret title, never values. Needs the master password. `--json` prints `[{time, action, title}]` with `time` in RFC 3339 UTC; an `export` of metadata only has an empty title. If `audit_log` is off and nothing was ever recorded it says so on stderr. Lines that cannot be read are counted in a warning. |
| `pandora add <title> [-c CATEGORY] [-u USERNAME] [--tag TAG] [--notes TEXT] [--expires YYYY-MM-DD]` | Stores a new secret. The value is read from stdin when piped (any number of lines, one trailing newline removed; `pandora add key < id_ed25519`) or typed at a hidden prompt on a terminal (one line). `--username` (`-u`) records a login or e-mail. `--tag` (`-t`) may be repeated or comma-separated. Empty values, values over 32 KB, duplicate titles, invalid dates and invalid tags are rejected. |
| `pandora rm <title> [-y]` | Deletes a secret. Asks on a terminal; without a terminal it refuses unless `-y` is given. |
| `pandora run -s TITLE[=ENV_NAME] [-s ...] -- cmd args...` | Runs `cmd` with each secret in an environment variable that exists only for that process. `ENV_NAME` defaults to the title upper-cased with other characters replaced by `_`. Fails before running anything if a secret is missing; exits with the command's exit code. |
| `pandora env -s TITLE[=ENV_NAME] [-s ...] [--force]` | Prints one `KEY=value` line per secret, in dotenv format (`pandora env -s github-token > .env`). Names default as in `run`. Values made only of letters, digits and `_./:@%+-` are bare; anything else is double-quoted with `\\`, `\"`, `\n`, `\r` and `\$` escaped, so multi-line values stay on one line. Because it prints values, it refuses (exit `2`) when stdout is a terminal unless you pass `--force`. Fails before printing anything if a secret is missing. |
| `pandora export [--values] [-o FILE] [--force]` | Writes the metadata as JSON. With `--values` the values are included and the whole file is encrypted. |
| `pandora import FILE [--format FORMAT] [--values] [--overwrite] [--dry-run] [--category C]` | Reads a file (`-` for stdin). `--format` is `pandora` (default, Pandora's own export), `dotenv` or `bitwarden-json` (see [Importing from other tools](../guides/importing.md)). Existing titles are skipped unless `--overwrite`; with the Pandora format values are restored only with `--values`. The whole file is validated before anything is written. `--dry-run` lists what would be imported, replaced and skipped and writes nothing. |
| `pandora doctor [--fix] [-y]` | Checks `vault.json` and `vault.json.bak` against your master password, then lists vault entries whose value is missing from the keyring. `--fix` restores a damaged or missing `vault.json` from the backup and removes entries without a value (each asks unless `-y`). Exits `1` while problems remain. |
| `pandora id` | Prints this vault's share key (stdout) and its fingerprint (stderr). |
| `pandora peer add NAME KEY\|KEYFILE [-y]` / `peer list [--json]` / `peer rm NAME [-y]` | Manage the vaults you trust to exchange secrets with. `add` shows the key's fingerprint and asks you to confirm it matches `pandora id` on the other machine. |
| `pandora export --values --to PEER [-s TITLE ...]` | Encrypts (and signs) just those secrets for a trusted peer; `pandora import FILE --values` on the other machine opens it with no passphrase and rejects unsigned, altered or foreign files. |
| `pandora passwd` | Changes the master password (needs a terminal). Values stay valid. |
| `pandora version` | Prints the version, commit and build date (also `pandora --version`). |
| `pandora gen [--length 8-128] [--charset alnum\|symbols\|hex] [--count 1-100]` | Prints random secrets (default: one of 24 letters and digits) using `crypto/rand`. Stores nothing. |
| `pandora config path` / `config get [KEY]` | Prints where `config.json` lives (the file need not exist), or the effective settings after merging environment, file and defaults (one `key = value` per line, or just the value for one key). Read-only: edit the file by hand. |
| `pandora completion bash\|zsh\|fish\|powershell` | Prints a shell completion script (see below). |

## Examples

```sh
printf '%s' "$TOKEN" | pandora add github-token -c Tokens --expires 2027-01-31
pandora list --category tokens --json
pandora list --tag prod --tag db            # secrets with both tags
pandora run -s github-token=GITHUB_TOKEN -- gh api user
pandora env -s github-token=GITHUB_TOKEN > .env   # dotenv file; delete it when done
pandora export -o names.json                # metadata only, plain JSON
pandora export --values -o backup.age       # prompts twice for a passphrase
PANDORA_PASSPHRASE=... pandora import backup.age --values --overwrite
```


## Shell completion

Completion offers commands, flags, secret titles (`get`, `rm`, `run -s`), categories (`-c`) and file names. Names are encrypted, so titles and categories are offered only when `PANDORA_MASTER_PASSWORD` is exported in your shell; otherwise completion stays silent about them and never prompts. Values are never read.

```sh
# bash    (add to ~/.bashrc)
source <(pandora completion bash)
# zsh     (add to ~/.zshrc, after compinit)
source <(pandora completion zsh)
# fish
pandora completion fish > ~/.config/fish/completions/pandora.fish
# PowerShell
pandora completion powershell | Out-String | Invoke-Expression
```

For the flags of every command exactly as the program prints them, see [Command flags](cli-flags.md).
