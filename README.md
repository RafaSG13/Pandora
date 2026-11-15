<h1 align="center">Pandora</h1>

<p align="center">
  A keyboard-driven terminal vault for your secrets, backed by your operating system's keyring.
</p>

<p align="center">
  <a href="LICENSE"><img alt="License: MIT" src="https://img.shields.io/badge/license-MIT-blue.svg"></a>
  <img alt="Go" src="https://img.shields.io/badge/go-1.27-00ADD8.svg">
  <img alt="Platforms" src="https://img.shields.io/badge/platforms-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey.svg">
</p>

```
▌PANDORA · OS keychain vault                                               3 secrets
────────────────────────────────────────────────────────────────────────────────────
╭──────────────────────────────────────╮  ╭────────────────────────────────────────╮
│                                      │  │                                        │
│  Secrets                             │  │  stripe-live                           │
│                                      │  │                                        │
│    githubToken                       │  │  Category   INFRA                      │
│    TOKENS · 2026-11-15               │  │  Created    2026-11-16                 │
│  │ stripe-live                       │  │  Value      ••••••••••••               │
│  │ INFRA · 2026-11-16                │  │                                        │
│    npm-publish                       │  │                                        │
│    TOKENS · 2026-11-20               │  │                                        │
╰──────────────────────────────────────╯  ╰────────────────────────────────────────╯
✓ Copied to clipboard (clears in 15s)
↑/↓ move · / filter · enter/c copy · v reveal · n new · e edit · d delete · r reload · ? help · q quit
```

Pandora keeps API keys, tokens and passwords out of dotfiles, shell history and plain-text notes. Secret **values** are stored by the OS (macOS Keychain, Windows Credential Manager, Linux Secret Service); Pandora only adds a fast interface for browsing, copying and managing them. It makes no network connections.

## Features

- **Native secure storage** — values never touch disk through Pandora; they live in the OS keyring.
- **Full management** — add, edit (including rename), delete with confirmation, and reload.
- **Fast lookup** — fuzzy filter over name and category.
- **Safe copying** — copy to the clipboard, which is cleared automatically after 15 seconds.
- **Masked by default** — values are hidden until you explicitly reveal them.
- **Responsive layout** — adapts to the terminal size, light and dark palettes.
- **Safe metadata handling** — atomic writes; a corrupt metadata file is reported, never overwritten.
- **Single binary, no network, no telemetry.**

## Installation

Pandora needs Go (the version in the `go` directive of [`go.mod`](go.mod)).

```sh
git clone <repository-url> pandora
cd pandora
make build          # builds bin/pandora
```

Put the binary on your `PATH`, for example:

```sh
install -m 0755 bin/pandora /usr/local/bin/pandora
```

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

1. Press `n`, fill in a name, a category and the secret value, and press `enter` on the last field to save.
2. Move with `↑`/`↓` (or `j`/`k`), press `v` to reveal the value, `c` or `enter` to copy it.
3. Press `/` to filter, `e` to edit, `d` to delete, `q` to quit.

## Keybindings

### Browsing

| Key               | Action                                                 |
| ----------------- | ------------------------------------------------------ |
| `↑` `↓` / `j` `k` | Move through the list                                  |
| `/`               | Filter by name or category (`esc` clears the filter)   |
| `enter` / `c`     | Copy the selected secret to the clipboard (cleared after 15 s) |
| `v`               | Toggle revealing secret values                         |
| `n`               | New secret                                             |
| `e`               | Edit the selected secret                               |
| `d`               | Delete the selected secret (asks for confirmation)     |
| `r`               | Reload the list from disk                              |
| `?`               | Show the full key list (any key closes it)             |
| `q` / `ctrl+c`    | Quit                                                   |

### Mouse

| Input        | Action                                      |
| ------------ | ------------------------------------------- |
| Click        | Select the secret under the pointer         |
| Wheel        | Move the selection up or down               |

Mouse input is ignored while a dialog, the form or the filter prompt is open. Because Pandora captures the mouse, select terminal text with `Shift` held (`Alt` in some terminals).

### Forms

| Key                   | Action                                                      |
| --------------------- | ----------------------------------------------------------- |
| `tab` / `↓`           | Next field                                                  |
| `shift+tab` / `↑`     | Previous field                                              |
| `enter`               | Next field; on the last field, save                         |
| `esc`                 | Cancel                                                      |

When editing, leave **Secret** empty to keep the current value. Renaming a secret moves its value to the new name. Names must be unique.

### Delete confirmation

`y` deletes; any other key cancels.

## How your data is stored

| Data                            | Where                                                              |
| ------------------------------- | ------------------------------------------------------------------ |
| Secret values                   | OS keyring, service `pandora-cli`, account = the secret's name     |
| Name, category, creation date   | `<user config dir>/pandora/metadata.json` (file `0600`, directory `0700`) |

The user config dir is `~/Library/Application Support` on macOS, `$XDG_CONFIG_HOME` or `~/.config` on Linux and `%AppData%` on Windows. The metadata file never contains secret values, but it does reveal the **names** of your secrets.

Secrets are ordinary keyring entries, so you can also inspect them with the system tools, e.g. on macOS:

```sh
security find-generic-password -s pandora-cli -a "<name>" -w
```

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the metadata schema and data flows.

## Security model

Pandora is designed to reduce accidental exposure, not to defend against a compromised account.

**It does**

- keep values in the OS keyring instead of files;
- mask values on screen until revealed, and clear the clipboard 15 seconds after copying;
- never log, print or transmit values, and never open network connections;
- write metadata atomically with owner-only permissions (`0600`/`0700` on macOS and Linux; Windows relies on the per-user profile directory's ACLs).

**It does not**

- protect against malware or other processes running as your user — they can read the keyring subject to your OS's prompts and policies;
- control clipboard managers or other tools that record clipboard history;
- hide secret names (they are stored in the metadata file) or wipe memory after use;
- sync, back up or encrypt exports — there are none yet.

To report a vulnerability, see [SECURITY.md](SECURITY.md).

## Project layout

```
cmd/pandora/     program entry point
internal/store/  keyring access and metadata persistence
internal/tui/    Bubble Tea model, update logic, views and styles
docs/            architecture notes and roadmap
```

## Documentation

- [Architecture](docs/ARCHITECTURE.md) — packages, data flow, persisted schema
- [Roadmap](docs/ROADMAP.md) — planned features and UI improvements
- [Contributing](CONTRIBUTING.md) — development setup and guidelines
- [Changelog](CHANGELOG.md)
- [Security policy](SECURITY.md)

## Development

```sh
make build   # build bin/pandora
make run     # run without building a binary
make vet     # go vet ./...
make test    # go test ./...
```

Tests use an in-memory keyring and a temporary config directory; they never touch your real secrets. See [CONTRIBUTING.md](CONTRIBUTING.md).

## Built with

[Bubble Tea](https://github.com/charmbracelet/bubbletea), [Bubbles](https://github.com/charmbracelet/bubbles), [Lip Gloss](https://github.com/charmbracelet/lipgloss), [go-keyring](https://github.com/zalando/go-keyring) and [atotto/clipboard](https://github.com/atotto/clipboard).

## License

[MIT](LICENSE)
