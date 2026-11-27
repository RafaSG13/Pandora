# Configuration

Optional settings live in `config.json` next to `vault.json` (`pandora config path` prints the location; on macOS `~/Library/Application Support/pandora/`, on Linux `~/.config/pandora/`, on Windows `%AppData%\pandora\`). Every key is optional; the file never contains secrets. **Precedence: environment variable, then the file, then the built-in default.** A broken file never stops Pandora: invalid values, unknown keys and a malformed file are reported once (a toast after unlocking in the TUI, `pandora: warning:` on stderr in the CLI) and the defaults stay in force.

```json
{
  "lock_minutes": 10,
  "ascii": false,
  "clipboard_seconds": 15,
  "reveal_seconds": 10,
  "expiry_warn_days": 14,
  "gen_length": 24,
  "gen_charset": "alnum",
  "sort": "created",
  "compact": false,
  "history_depth": 3,
  "audit_log": false,
  "background": "auto",
  "keys": { "copy": ["enter", "c"], "quit": ["q"] }
}
```

| Key                 | Default   | Valid values                                   | Effect |
| ------------------- | --------- | ---------------------------------------------- | ------ |
| `lock_minutes`      | `5`       | `0`–`10080` (`0` disables)                     | Idle minutes before the screen locks (env `PANDORA_LOCK_MINUTES`) |
| `ascii`             | by locale | `true`, `false`                                | Plain ASCII drawing; `false` also overrides a non-UTF-8 locale (env `PANDORA_ASCII=1` forces it on) |
| `clipboard_seconds` | `15`      | `1`–`3600`                                     | Seconds before a copied secret is cleared from the clipboard |
| `reveal_seconds`    | `10`      | `1`–`3600`                                     | Seconds before a revealed value hides again |
| `expiry_warn_days`  | `14`      | `0`–`3650`                                     | How many days ahead an expiry counts as "soon" |
| `gen_length`        | `24`      | `8`–`128`                                      | Default for `pandora gen --length`; the form's generator starts at the nearest preset (16, 24, 32, 48, 64) |
| `gen_charset`       | `alnum`   | `alnum`, `symbols`, `hex`                      | Default for `pandora gen --charset` and the form's generator |
| `sort`              | `created` | `created`, `name`, `category`                  | Initial list order |
| `compact`           | `false`   | `true`, `false`                                | Start with one-line rows |
| `history_depth`     | `3`       | `0`–`20` (`0` disables)                        | Previous values kept per secret when its value changes (see [Value history](../guides/history-and-audit.md#value-history)) |
| `audit_log`         | `false`   | `true`, `false`                                | Record actions in the encrypted audit log (see [Audit log](../guides/history-and-audit.md#audit-log)) |
| `background`        | `auto`    | `auto`, `light`, `dark`                        | Which colours to use. `auto` asks the terminal once at start; set `light` or `dark` to skip the question (a terminal that never answers delays the start by several seconds) |
| `keys`              | see below | object: action → list of keys                  | Rebind the list-screen keys (see [Rebinding keys]#rebinding-keys) |

The environment variables below remain available and win over the file.

| Variable                 | Effect                                                              |
| ------------------------ | ------------------------------------------------------------------- |
| `PANDORA_LOCK_MINUTES`   | Idle minutes before the screen locks (default `5`, `0` disables)    |
| `PANDORA_MASTER_PASSWORD` | CLI only: the master password when there is no terminal (visible to your user's other processes; a warning is printed). Removed from the environment of `pandora run` children |
| `PANDORA_ASCII=1`        | Draw with plain ASCII; also used automatically when the locale is not UTF-8 |
| `NO_COLOR`               | Any non-empty value turns colour off (also `TERM=dumb`); the selected row is then marked with `▌` (`>` in ASCII mode) and expired secrets show `◷!` in compact rows |

## Rebinding keys

`keys` maps an action to the keys that trigger it; actions you leave out keep their default. The actions are `copy` (`enter`, `c`), `copy_username` (`u`), `reveal` (`v`), `view` (`V`), `new` (`n`), `edit` (`e`), `delete` (`d`), `reload` (`r`), `sort` (`s`), `tag` (`T`), `filters` (`f`), `compact` (`t`), `history` (`H`), `clear_clipboard` (`x`), `lock` (`L`), `help` (`?`) and `quit` (`q`). A key is `enter`, one printable character, or `ctrl+` and a letter. Digits, `/`, `j`, `k`, `g`, `G`, `esc`, the arrows and `ctrl+c` cannot be rebound, and a key cannot be bound to two actions. The help overlay, the footer and the hints in the detail card show the keys you chose. Only the list screen is affected: the form, the unlock screen, the filter prompt, the history list, the viewer, the picker and the delete confirmation keep their keys. The whole object is accepted or rejected as one unit (one warning); `pandora config get keys` prints the effective bindings.

```json
{ "keys": { "copy": ["y"], "quit": ["Q"] } }
```
