# Importing from other tools

`pandora import --format FORMAT FILE` reads files from other tools. Add `--dry-run` first to see what would be imported, replaced or skipped; nothing is written. Existing titles are skipped unless you pass `--overwrite`. These files hold your secrets in plain text: delete them once you have checked the import.

```sh
pandora import --format dotenv .env --category ci --dry-run
pandora import --format bitwarden-json bitwarden_export.json
```

| Format | How it maps |
| ------ | ----------- |
| `dotenv` | The inverse of `pandora env`. Title = variable name, value = its value. Accepts blank lines, `#` comments, an optional `export ` prefix, bare values (a trailing ` # comment` is dropped), `'literal'` values and `"double-quoted"` values with `\\ \" \n \r \t \$` escapes, which may span lines. `--category` gives every secret a category. A syntax error or a repeated variable aborts the whole import and names the line; an empty value is skipped. |
| `bitwarden-json` | An **unencrypted** Bitwarden export (Settings, Export vault, `.json`; encrypted exports are rejected). A login becomes a secret whose value is its password; its user name becomes the secret's username, and its notes and first URL are kept in the (single-line) notes. A secure note becomes a secret whose value is the note. The folder becomes the category and the original dates are kept. Items with the same name get ` (2)`, ` (3)`. Cards, identities, logins without a password, **TOTP seeds** and custom fields are not imported; each skipped item is listed with its reason on stderr. |

The mapping follows Bitwarden's documented export layout and was tested with synthetic files only; if a real export imports oddly, `--dry-run` shows the plan before anything changes.
