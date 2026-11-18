
<p align="center">
  <img src="assets/pandora-logo.svg" alt="Pandora - The Mythological Vault">
</p>


<p align="center">
  A terminal user interface vault for your secrets, backed by your operating system's keyring.
</p>

<p align="center">
  <a href="LICENSE"><img alt="License: MIT" src="https://img.shields.io/badge/license-MIT-blue.svg"></a>
  <img alt="Go" src="https://img.shields.io/badge/go-1.27-00ADD8.svg">
  <img alt="Platforms" src="https://img.shields.io/badge/platforms-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey.svg">
</p>

<p align="center">
  <video src="assets/demo.mp4" autoplay loop muted playsinline width="100%"></video>
</p>


Pandora keeps API keys, tokens and passwords out of dotfiles, shell history and plain-text notes. Secret **values** are stored by the OS (macOS Keychain, Windows Credential Manager, Linux Secret Service); Pandora only adds a fast interface for browsing, copying and managing them. It makes no network connections.

## Features

- **Encrypted storage** — a master password protects everything: names, notes and values are encrypted, and values also live in the OS keyring under random IDs.
- **Full management** — add, edit (including rename), delete with confirmation, and reload.
- **Rich metadata** — category (with autocomplete), notes or URL, and an optional expiry date with warning badges.
- **Fast lookup** — fuzzy filter over name and category, quick category filters (`1`–`9`), and sorting by name, category or date.
- **Safe copying** — copy to the clipboard, cleared automatically after 15 seconds (or right away with `x`), with a visible countdown; on macOS and Windows the copy is marked so clipboard history skips it.
- **Masked by default** — the value is revealed one secret at a time and hides again after 10 seconds or when you move.
- **Secret generator** — `ctrl+g` in the form fills a random password or token (length and charset selectable); `pandora gen` does the same from the shell.
- **Idle lock** — after 5 idle minutes everything is masked, the secrets are dropped from memory, a pending clipboard is cleared, and the master password is required again.
- **Orphan detection** — secrets whose keyring value is missing are flagged `!`.
- **Polished interface** — per-category colours (or none, with `NO_COLOR`), full-row selection, bordered detail card, toasts, compact mode (`t`), mouse support, and an ASCII fallback.
- **Command-line mode** — `list`, `get`, `add`, `rm`, `run`, `gen`, `passwd`, encrypted `export`/`import`, `doctor` and shell completions.
- **Safe vault handling** — atomic writes with a `vault.json.bak` backup; a corrupt or tampered file is reported, never overwritten.
- **Single binary, no network, no telemetry.**

## Installation

**Homebrew** (tested on macOS; the formula also has Linux entries):

```sh
brew install RafaSG13/tap/pandora
```

**Pre-built binary.** Download the archive for your OS and CPU from the [Releases page](https://github.com/RafaSG13/Pandora/releases), check it against `checksums.txt`, and put `pandora` on your `PATH`:

```sh
shasum -a 256 -c checksums.txt --ignore-missing
tar -xzf pandora_<version>_<os>_<arch>.tar.gz
install -m 0755 pandora /usr/local/bin/pandora
```

Binaries are not code-signed or notarised yet. On macOS, Gatekeeper may block a downloaded binary: run `xattr -d com.apple.quarantine pandora` once, or build from source.

**With Go** (the version in the `go` directive of [`go.mod`](go.mod)):

```sh
go install github.com/RafaSG13/Pandora/cmd/pandora@latest
```

**From a checkout:**

```sh
git clone https://github.com/RafaSG13/Pandora.git
cd Pandora
make build          # builds bin/pandora with the version embedded
```

Check the install with `pandora --version`.

### Platform requirements

| Platform | Secret storage                                   | Clipboard                                   |
| -------- | ------------------------------------------------ | ------------------------------------------- |
| macOS    | Keychain (built in)                              | `pbcopy` / `pbpaste` (built in)             |
| Windows  | Credential Manager (built in)                    | built in                                    |
| Linux    | A running Secret Service provider (GNOME Keyring, KWallet, KeePassXC) reachable over D-Bus | `xclip`, `xsel` or `wl-clipboard` |

## Quick start

```sh
pandora
```

0. The first time, Pandora asks you to create a **master password** (at least 10 characters). It cannot be recovered. If you used an older version, your secrets are migrated to the encrypted vault at this point.
1. Press `n`, fill in a name, a category and the secret value (or press `ctrl+g` to generate one), and press `enter` through the optional **Expires** and **Notes** fields to save.
2. Move with `↑`/`↓` (or `j`/`k`), press `v` to reveal the value, `c` or `enter` to copy it.
3. Press `/` to filter, `s` to sort, `e` to edit, `d` to delete, `?` for every key, `q` to quit.

## Keybindings

### Browsing

| Key               | Action                                                 |
| ----------------- | ------------------------------------------------------ |
| `↑` `↓` / `j` `k` | Move through the list                                  |
| `g` / `G`, `pgup` / `pgdn` | First / last, previous / next page            |
| `/`               | Filter by name or category (`esc` clears the filter)   |
| `1`–`9`, `0`      | Show only the n-th category (same key again, or `0`, shows all) |
| `s`               | Cycle sort: date added, name, category                 |
| `t`               | Toggle compact one-line rows                           |
| `enter` / `c`     | Copy the selected secret to the clipboard (cleared after 15 s) |
| `x`               | Clear the clipboard now                                |
| `v`               | Reveal / hide the selected secret's value (hides after 10 s or when the selection changes) |
| `n`               | New secret                                             |
| `e`               | Edit the selected secret                               |
| `d`               | Delete the selected secret (asks for confirmation)     |
| `r`               | Reload the list from disk and re-check the keyring     |
| `?`               | Show the full key list; on small terminals it scrolls (`↑/↓`, `PgUp/PgDn`, `Home/End`), `esc` / `?` / `q` closes it |
| `q` / `ctrl+c`    | Quit                                                   |

### Mouse

| Input        | Action                                      |
| ------------ | ------------------------------------------- |
| Click        | Select the secret under the pointer         |
| Wheel        | Scroll the list without changing the selection (arrow keys scroll back to it) |

Mouse input is ignored while a dialog, the form or the filter prompt is open. Because Pandora captures the mouse, select terminal text with `Shift` held (`Alt` in some terminals).

### Forms

| Key                   | Action                                                      |
| --------------------- | ----------------------------------------------------------- |
| `tab` / `↓`           | Next field                                                  |
| `shift+tab` / `↑`     | Previous field                                              |
| `enter`               | Next field; on the last field, save. In **Secret**: new line |
| `ctrl+s`              | Save from any field                                         |
| `ctrl+r`              | Show or hide the secret you are typing                      |
| `→`                   | Accept the suggested category                               |
| `ctrl+g`              | Generate a random secret (shown so you can see it)          |
| `ctrl+l` / `ctrl+t`   | Cycle the generated length (16–64) / charset (letters+digits, +symbols, hex) |
| `esc`                 | Cancel                                                      |

Fields: **Name**, **Category**, **Secret**, **Expires** (`YYYY-MM-DD`, optional) and **Notes** (optional, e.g. a URL). The **Secret** can have several lines (SSH keys, certificates, JSON credentials; up to 32 KB): paste it, or type `enter` for a new line. It is shown as bullets until you press `ctrl+r`; `↑`/`↓` move through its lines and leave it from the first or last line, and `tab` moves on. When editing, leave **Secret** empty to keep the current value. Renaming a secret keeps its value. Names must be unique. A secret that expires within 14 days, or already has, shows a `◷` badge in the list, the detail card and the header. Revealing a long value in the detail card shows its first lines and says how many more there are; `c` always copies all of it.

### Configuration

| Variable                 | Effect                                                              |
| ------------------------ | ------------------------------------------------------------------- |
| `PANDORA_LOCK_MINUTES`   | Idle minutes before the screen locks (default `5`, `0` disables)    |
| `PANDORA_MASTER_PASSWORD` | CLI only: the master password when there is no terminal (visible to your user's other processes; a warning is printed). Removed from the environment of `pandora run` children |
| `PANDORA_ASCII=1`        | Draw with plain ASCII; also used automatically when the locale is not UTF-8 |
| `NO_COLOR`               | Any non-empty value turns colour off (also `TERM=dumb`); the selected row is then marked with `▌` (`>` in ASCII mode) and expired secrets show `◷!` in compact rows |

### Terminal size

The interface needs at least 60×18 characters and shows `Terminal too small` below that.

### Delete confirmation

`y` deletes; any other key cancels.

## How your data is stored

| Data                            | Where                                                              |
| ------------------------------- | ------------------------------------------------------------------ |
| Secret values                   | OS keyring, service `pandora-cli`, account = a random 32-hex ID; the stored value is itself encrypted with the vault key. Values over about 1.5 KB are split across several entries (`<id>`, `<id>.1`, …) to stay under the OS limits |
| Name, category, dates, expiry, notes | `<user config dir>/pandora/vault.json` (file `0600`, directory `0700`): XChaCha20-Poly1305 under a random data key that is wrapped with your master password (Argon2id). The previous version is kept as `vault.json.bak`, also ciphertext |

The user config dir is `~/Library/Application Support` on macOS, `$XDG_CONFIG_HOME` or `~/.config` on Linux and `%AppData%` on Windows. Neither the vault nor the keyring reveals the **names** or values of your secrets without the master password. The first run asks you to create one (at least 10 characters, cannot be recovered). Older plain-text stores (`metadata.json` and keyring entries named after the secrets) are migrated automatically on first start and the old data is removed.

Because values are encrypted, the system keyring tools show only opaque entries, e.g. on macOS `security find-generic-password -s pandora-cli -w` prints ciphertext. Use `pandora get` or `pandora export --values`.

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the vault format and data flows.

## Command-line mode

`pandora` with no arguments opens the interface. With a command it runs non-interactively, which suits scripts and CI. Data goes to stdout, messages and errors to stderr. Exit codes: `0` success, `1` runtime error (including a refused confirmation), `2` usage error. `pandora help` and `pandora <command> -h` list the flags. Flags may come before or after the arguments.

| Command | What it does |
| ------- | ------------ |
| `pandora list [--category C] [--json]` | Titles, categories, creation and expiry dates. Never shows values. |
| `pandora get <title>` | Prints the value followed by a newline, so `add` then `get` reproduces the file you piped in. Exit `1` if the secret does not exist. |
| `pandora add <title> [-c CATEGORY] [--notes TEXT] [--expires YYYY-MM-DD]` | Stores a new secret. The value is read from stdin when piped (any number of lines, one trailing newline removed; `pandora add key < id_ed25519`) or typed at a hidden prompt on a terminal (one line). Empty values, values over 32 KB, duplicate titles and invalid dates are rejected. |
| `pandora rm <title> [-y]` | Deletes a secret. Asks on a terminal; without a terminal it refuses unless `-y` is given. |
| `pandora run -s TITLE[=ENV_NAME] [-s ...] -- cmd args...` | Runs `cmd` with each secret in an environment variable that exists only for that process. `ENV_NAME` defaults to the title upper-cased with other characters replaced by `_`. Fails before running anything if a secret is missing; exits with the command's exit code. |
| `pandora export [--values] [-o FILE] [--force]` | Writes the metadata as JSON. With `--values` the values are included and the whole file is encrypted. |
| `pandora import FILE [--values] [--overwrite]` | Reads an export (`-` for stdin). Existing titles are skipped unless `--overwrite`; values are restored only with `--values`. The whole file is validated before anything is written. |
| `pandora doctor [--fix] [-y]` | Lists vault entries whose value is missing from the keyring; `--fix` removes them (asks unless `-y`). Exits `1` while problems remain. |
| `pandora passwd` | Changes the master password (needs a terminal). Values stay valid. |
| `pandora version` | Prints the version, commit and build date (also `pandora --version`). |
| `pandora gen [--length 8-128] [--charset alnum\|symbols\|hex] [--count 1-100]` | Prints random secrets (default: one of 24 letters and digits) using `crypto/rand`. Stores nothing. |
| `pandora completion bash\|zsh\|fish\|powershell` | Prints a shell completion script (see below). |

```sh
printf '%s' "$TOKEN" | pandora add github-token -c Tokens --expires 2027-01-31
pandora list --category tokens --json
pandora run -s github-token=GITHUB_TOKEN -- gh api user
pandora export -o names.json                # metadata only, plain JSON
pandora export --values -o backup.age       # prompts twice for a passphrase
PANDORA_PASSPHRASE=... pandora import backup.age --values --overwrite
```

**Encrypted exports.** `export --values` never writes plaintext values. The document, titles included, is encrypted with [age](https://age-encryption.org) using a scrypt passphrase recipient and ASCII-armored; `age -d backup.age` can decrypt it too. The passphrase must be at least 10 characters when exporting; it is prompted for twice without echo, or read from `PANDORA_PASSPHRASE` when there is no terminal (environment variables can be visible to other processes of your user, so prefer the prompt where possible; a warning is printed and `pandora run` removes the variable from the child's environment). Export files are created with mode `0600`, and an existing file is only replaced with `--force`. Export without `--values` is **plain text**: names, categories, notes and dates, but no values.

**Doctor limitation.** `doctor` can only find vault entries without a value. The keyring API Pandora uses cannot list entries, so values that exist in the keyring without a vault entry cannot be detected.

**Shell completion.** Completion offers commands, flags, secret titles (`get`, `rm`, `run -s`), categories (`-c`) and file names. Names are encrypted, so titles and categories are offered only when `PANDORA_MASTER_PASSWORD` is exported in your shell; otherwise completion stays silent about them and never prompts. Values are never read.

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

## Security model

Pandora is designed to reduce accidental exposure, not to defend against a compromised account.

**It does**

- require a master password to see anything, and keep values in the OS keyring instead of files;
- mask values on screen until revealed, and clear the clipboard 15 seconds after copying — but only if the clipboard still holds what Pandora copied (it compares a SHA-256 and keeps no plaintext), so something you copied meanwhile is left alone;
- mark copied values as concealed: `org.nspasteboard.ConcealedType` on macOS, and excluded from clipboard history (`Win+V`) and cloud clipboard sync on Windows;
- never log, print or transmit values, and never open network connections;
- write the vault atomically with owner-only permissions (`0600`/`0700` on macOS and Linux; Windows relies on the per-user profile directory's ACLs);
- encrypt names, categories, notes, expiry dates and values under a key that only your master password unlocks, and store the keyring entries under random IDs, so neither `security dump-keychain` nor the config directory reveals your titles.

**It does not**

- protect against malware or other processes running as your user — they can read the keyring subject to your OS's prompts and policies;
- control clipboard managers or other tools that record clipboard history — a tool may ignore the concealed marker, and Linux has no such convention at all;
- wipe memory reliably after use, or stop someone who can read your keyring and guess a weak master password offline (use a long, unique one; the key derivation is Argon2id with 64 MiB, 3 passes);
- sync or back up for you — [`pandora export`](#command-line-mode) is manual, and an export made with `--values` is only as safe as its passphrase.

**Things that surprise people**

- **`pandora run` uses the environment.** Values reach the child process (and everything it starts) as environment variables; on some systems other processes of the same user can read them (`ps eww`, `/proc/<pid>/environ`). Use it for commands you trust.
- **The idle lock asks for your master password.** After the idle timeout (default 5 minutes) the data key is dropped from memory (best effort: Go does not guarantee wiped memory), the list is emptied and the unlock screen needs the password again. An unsaved form is discarded.
- **Lose the master password, lose the vault.** Nothing can recover it. `pandora export --values` makes an encrypted backup under a separate passphrase.
- **`PANDORA_MASTER_PASSWORD` is a trade-off.** It lets scripts and shell completion work without a terminal, but environment variables can be visible to other processes of your user. Without it, completion offers no titles or categories because they are encrypted.
- **`pandora export` without `--values` is plain text.** It writes names, categories, notes and dates, unencrypted, to the file you choose. Never put a secret in the notes.
- **The threat model** is accidental exposure: shoulder surfing, screen sharing, shell history, dotfiles and notes apps. Malware running as you is out of scope.

To report a vulnerability, see [SECURITY.md](SECURITY.md).

## Project layout

```
cmd/pandora/     program entry point
internal/cli/    command-line mode (list, get, add, rm, run, gen, export, import, doctor, completion)
internal/store/  keyring access and metadata persistence
internal/gen/    random secret generator (crypto/rand)
internal/tui/    Bubble Tea model, update logic, views and styles
internal/version/ build version, commit and date
docs/            architecture notes and roadmap
```

## Documentation

- [Architecture](docs/ARCHITECTURE.md) — packages, data flow, persisted schema
- [Roadmap](docs/ROADMAP.md) — what comes next, with the decisions each item needs
- [Contributing](CONTRIBUTING.md) — development setup and guidelines
- [Changelog](CHANGELOG.md)
- [Security policy](SECURITY.md)

## Development

```sh
make build   # build bin/pandora with the version embedded
make run     # run without building a binary
make vet     # go vet ./...
make test    # go test ./...
```

Tests use an in-memory keyring and a temporary config directory; they never touch your real secrets. See [CONTRIBUTING.md](CONTRIBUTING.md).

## Built with

[Bubble Tea](https://github.com/charmbracelet/bubbletea), [Bubbles](https://github.com/charmbracelet/bubbles), [Lip Gloss](https://github.com/charmbracelet/lipgloss), [go-keyring](https://github.com/zalando/go-keyring), [age](https://github.com/FiloSottile/age) and [atotto/clipboard](https://github.com/atotto/clipboard).

## License

[MIT](LICENSE)
