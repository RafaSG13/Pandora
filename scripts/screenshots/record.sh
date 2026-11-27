#!/usr/bin/env bash
# Regenerates assets/demo.gif and assets/screenshots/*.png from a throwaway
# vault with an in-memory keyring. Never touches your real vault or keyring.
# Needs vhs (brew install vhs) and Go.
set -euo pipefail
cd "$(dirname "$0")/../.."
command -v vhs >/dev/null || { echo "record.sh: vhs not found (brew install vhs)" >&2; exit 1; }
bin="$(mktemp -d)"
trap 'rm -rf "$bin"' EXIT
go build -o "$bin/pandora-demo" ./scripts/screenshots/main.go
mkdir -p assets/screenshots
# NO_COLOR, TERM=dumb or CI (termenv treats CI as "not a terminal") would make
# Pandora draw without colour.
env -u NO_COLOR -u CI TERM=xterm-256color COLORTERM=truecolor LANG=en_US.UTF-8 \
  PATH="$bin:$PATH" vhs scripts/screenshots/pandora.tape
