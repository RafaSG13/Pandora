#!/bin/sh
# Print the CHANGELOG.md section for a version (e.g. `scripts/release-notes.sh 0.1.0`).
# Exits 1 when the section is missing or empty, so a release cannot go out
# without notes.
set -eu

version="${1:?usage: release-notes.sh VERSION}"
version="${version#v}"
changelog="${2:-$(dirname "$0")/../CHANGELOG.md}"

notes=$(awk -v v="$version" '
  /^## \[/ {
    if (found) exit
    if (index($0, "## [" v "]") == 1) { found = 1; next }
  }
  found { print }
' "$changelog")

# Trim leading/trailing blank lines.
notes=$(printf '%s\n' "$notes" | awk 'NF { p = 1 } p' | awk '{ l[NR] = $0 } END { n = NR; while (n > 0 && l[n] == "") n--; for (i = 1; i <= n; i++) print l[i] }')

if [ -z "$notes" ]; then
  echo "release-notes: no CHANGELOG.md section for $version" >&2
  exit 1
fi
printf '%s\n' "$notes"
