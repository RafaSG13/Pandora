# Installation

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

**With Go** (the version in the `go` directive of [`go.mod`](https://github.com/RafaSG13/Pandora/blob/main/go.mod)):

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
