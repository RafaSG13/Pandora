# Keybindings

## Browsing

| Key               | Action                                                 |
| ----------------- | ------------------------------------------------------ |
| `↑` `↓` / `j` `k` | Move through the list                                  |
| `g` / `G`, `pgup` / `pgdn` | First / last, previous / next page            |
| `/`               | Filter by name, user name, category, tags or notes (`esc` clears the filter). `tag:x`, `cat:x` and `user:x` keep only secrets whose tag, category or user name contains `x`; other words are fuzzy-matched (`tag:prod db`). A qualifier with no value is ignored |
| `1`–`9`, `0`      | Show only the n-th category (same key again, or `0`, shows all) |
| `f`               | Pick a category or tag from a list with counts (reaches categories past the ninth; `enter` applies, picking the active one again clears it) |
| `T`               | Cycle the tag filter: each tag in use, then all secrets (combines with the category filter) |
| `s`               | Cycle sort: date added, name, category                 |
| `t`               | Toggle compact one-line rows                           |
| `enter` / `c`     | Copy the selected secret to the clipboard (cleared after 15 s) |
| `u`               | Copy the selected secret's user name (plain clipboard, not cleared automatically) |
| `x`               | Clear the clipboard now                                |
| `v`               | Reveal / hide the selected secret's value (hides after 10 s or when the selection changes) |
| `V`               | Open the whole value in a scrollable viewer (`↑` `↓` `pgup` `pgdn` `g` `G` scroll, `c` copies, `esc` closes); it stays open while you press keys and closes 10 s after the last one |
| `H`               | Previous values of the selected secret: `↑` `↓` choose one, `r` reveal it (hides like `v`), `enter` make it current again, `esc` close |
| `n`               | New secret                                             |
| `e`               | Edit the selected secret                               |
| `d`               | Delete the selected secret (asks for confirmation)     |
| `r`               | Reload the list from disk and re-check the keyring     |
| `?`               | Show the full key list; on small terminals it scrolls (`↑/↓`, `PgUp/PgDn`, `Home/End`), `esc` / `?` / `q` closes it |
| `L`               | Lock now: masks everything and asks for the master password |
| `q` / `ctrl+c`    | Quit                                                   |

## Mouse

| Input        | Action                                      |
| ------------ | ------------------------------------------- |
| Click        | Select the secret under the pointer         |
| Double-click | Copy the secret under the pointer (same as `c`) |
| Click on the value box | Reveal / hide the value (same as `v`) |
| Wheel        | Scroll the list without changing the selection (arrow keys scroll back to it) |

Mouse input is ignored while a dialog, the form or the filter prompt is open. Because Pandora captures the mouse, select terminal text with `Shift` held (`Alt` in some terminals).

## Forms

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
| `esc`                 | Cancel; with unsaved changes it asks first: press `esc` again to discard (`ctrl+c` works the same way to quit) |

Fields: **Name**, **Category**, **Username** (optional, e.g. a login or e-mail), **Secret**, **Expires** (`YYYY-MM-DD`, optional), **Notes** (optional, e.g. a URL) and **Tags** (optional, comma-separated, e.g. `prod, db`). Tags are stored lowercase, trimmed, unique and sorted; a tag cannot contain spaces or commas, is at most 24 characters, and a secret has at most 10. The **Secret** can have several lines (SSH keys, certificates, JSON credentials; up to 32 KB): paste it, or type `enter` for a new line. It is shown as bullets until you press `ctrl+r`; `↑`/`↓` move through its lines and leave it from the first or last line, and `tab` moves on. When editing, leave **Secret** empty to keep the current value. Renaming a secret keeps its value. Names must be unique. A secret that expires within 14 days, or already has, shows a `◷` badge in the list, the detail card and the header. Revealing a long value in the detail card shows its first lines and says how many more there are; `c` always copies all of it, and `V` opens all of it.

## Terminal size

The interface needs at least 60×18 characters and shows `Terminal too small` below that.

## Delete confirmation

`y` deletes; any other key cancels.
