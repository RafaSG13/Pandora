# Release QA checklist

Automated tests use an in-memory keyring and a stubbed clipboard, so they cannot catch problems with the **real** OS keyring, the real clipboard or real terminals. Run this checklist on at least macOS before every tagged release, and record the result in the release pull request (date, OS version, terminal, anything that failed).

Use a throwaway test vault: start with a clean `HOME`-level config (move `~/Library/Application Support/pandora` / `~/.config/pandora` aside first) and delete the `pandora-cli` entries you created afterwards. **Never test with real secrets.**

## 1. Keyring (real backend)

| Check | Expect |
| --- | --- |
| First `pandora add` / `n` in the TUI | The OS may ask permission to use the keychain; after allowing it, the secret saves. |
| Add, edit (change value), rename, delete in the TUI | Each works; the old keyring entry disappears after a rename or delete (check with Keychain Access / `secret-tool` / Credential Manager). |
| Quit and restart | The list and values are still there; no secret is flagged `!`. |
| `pandora get NAME`, `pandora run -s NAME=VAR -- env` | Value printed / present only in the child's environment. |
| `pandora export --values -o f`, then import into an empty vault | Round-trips; the file is not readable without the passphrase. |
| Delete an entry in the OS keychain tool, restart Pandora | The secret is marked `!` in the list, and `pandora doctor` reports it. |
| Locked keychain (macOS: lock the login keychain) | A clear error, no crash, no data loss. |
| Linux without a running Secret Service (e.g. a bare SSH session) | The error screen / CLI message says the keyring is unavailable. |

## 2. Clipboard

| Check | Expect |
| --- | --- |
| `c` copies; paste elsewhere | Value pasted; countdown bar runs; clipboard is empty after 15 s. |
| Copy, then copy something else yourself, wait 15 s | **Your other text is untouched.** |
| `x` right after copying | Clipboard cleared immediately. |
| Copy, then let the idle lock trigger | Clipboard cleared. |
| Linux without `xclip`/`xsel`/`wl-clipboard` | `✗ Could not copy` toast, not `✓ Copied`. |

## 3. Terminals

Run the TUI in each terminal you can reach: Terminal.app, iTerm2, Windows Terminal, GNOME Terminal (or Konsole), and inside tmux and over SSH.

| Check | Expect |
| --- | --- |
| Colours on a dark and on a light background | Text and selection readable; category colours distinct. |
| Click a row; wheel over the list | Click selects the clicked row; wheel scrolls without moving the selection. |
| Select text with `Shift` (or `Alt`) + drag | Terminal selection works (mouse capture is bypassed). |
| Resize down to 60×18 and up again | No garbled layout; below the minimum the "Terminal too small" screen shows. |
| `?` help at 60×18 | Every line is reachable by scrolling. |
| `PANDORA_ASCII=1` and `NO_COLOR=1` | ASCII glyphs only / no colour; the selected row is still distinguishable. |
| `q` / `ctrl+c` | Terminal restored (cursor visible, mouse reporting off, no leftover alternate screen). |

## 4. Install artefacts

| Check | Expect |
| --- | --- |
| Download the archive for your OS/arch from the draft release, verify it against `checksums.txt` | Checksum matches. |
| `pandora --version` | Prints the tag's version, a commit and a date, not `dev`. |
| macOS: run the downloaded binary | Gatekeeper may warn (binaries are not notarised yet); `xattr -d com.apple.quarantine pandora` or right-click → Open works. |
| `go install github.com/RafaSG13/Pandora/cmd/pandora@vX.Y.Z` | Installs; `pandora --version` reports `X.Y.Z`. |
| Shell completions (`pandora completion zsh`/`bash`/`fish`) | Subcommands and secret titles complete; no values are ever offered. |

## Recording results

Paste this into the release PR and fill it in:

```
QA vX.Y.Z — <date>
- macOS <version>, <terminal>: keyring ✅/❌  clipboard ✅/❌  terminal ✅/❌  artefacts ✅/❌
- Linux <distro>, <terminal>: …
- Windows <version>, Windows Terminal: …
- Not tested: …
- Findings: …
```

Anything that fails and needs code becomes a new item in [ROADMAP.md](ROADMAP.md).
