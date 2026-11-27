# Scripting and CI

Pandora's command line is built for scripts: data goes to stdout, messages and errors to stderr, and exit codes are stable.

## Exit codes

| Code | Meaning |
| ---- | ------- |
| `0` | Success |
| `1` | Runtime error, including a refused confirmation |
| `2` | Usage error (bad flag, missing argument) |
| `3` | Only from `pandora expiring`: something is expired or expiring (not an error; stdout still carries the list) |

## Unlocking without a terminal

Commands that read or write secrets need the master password. On a terminal Pandora asks for it with a hidden prompt (three attempts). With no terminal, it reads `PANDORA_MASTER_PASSWORD` and prints a one-time warning that environment variables can be visible to other processes of your user. With neither, the command exits with `1`.

!!! warning "Be deliberate in CI"
    A master password in an environment variable is only as safe as the runner. Use your CI system's masked secrets, keep the variable out of logs, and prefer a dedicated vault that holds only the secrets that job needs. `pandora run` removes both password variables from the child's environment.

## Inject secrets into one command

```sh
pandora run -s github-token=GITHUB_TOKEN -s aws-key=AWS_SECRET_ACCESS_KEY -- ./deploy.sh
```

Each secret becomes an environment variable for that process and its children only. `ENV_NAME` defaults to the title upper-cased with other characters replaced by `_`. `run` fails before starting anything if a secret is missing and exits with the command's exit code.

## Read a value

```sh
TOKEN="$(pandora get -q github-token)"
```

`-q` omits the trailing newline. Without it `get` prints the value followed by a newline, so `add` then `get` reproduces the file you piped in.

## Store a secret from a pipeline

```sh
printf '%s' "$NEW_TOKEN" | pandora add github-token -c Tokens --expires 2027-01-31
pandora add deploy-key < ~/.ssh/id_ed25519      # multi-line values work
```

Empty values, values over 32 KB, duplicate titles, invalid dates and invalid tags are rejected.

## Expiry checks

`pandora expiring` lists secrets that have expired or expire within a window (default 14 days, or `expiry_warn_days` from the config) and exits `3` when the list is not empty:

```sh
# crontab: notify on Mondays when something is due
0 9 * * 1  pandora expiring --within 30d || notify-send "Pandora: secrets expiring"
```

`--json` prints `{title, category, expires, days, status}` objects, with `days` negative once expired.

## Machine-readable output

`pandora list --json`, `history --json`, `log --json`, `peer list --json` and `expiring --json` print JSON for tools such as `jq`. They never include values.

## Dotenv files

`pandora env -s a=VAR_A -s b=VAR_B > .env` writes dotenv lines. It refuses when stdout is a terminal unless `--force`. Delete the file when you are done; it holds plain text.

## Shell completion

`pandora completion bash|zsh|fish|powershell` prints a completion script. Titles and categories are offered only when `PANDORA_MASTER_PASSWORD` is exported, because they are encrypted. See the [command-line guide](../reference/cli.md#shell-completion).
