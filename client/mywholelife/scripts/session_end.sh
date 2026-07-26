#!/usr/bin/env bash
set -uo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
OUTBOX="$HOME/.mywholelife/outbox"

[ -d "$OUTBOX" ] || exit 0

for dir in "$OUTBOX"/*/; do
  [ -d "$dir" ] || continue
  project=$(basename "$dir")
  "$SCRIPT_DIR/push.sh" "$project" || echo "session_end: push failed for $project" >&2
done

exit 0
