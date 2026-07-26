#!/usr/bin/env bash
set -euo pipefail

SKILL_SRC=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

GLOBAL=0
[ "${1:-}" = "--global" ] && GLOBAL=1

if [ "$GLOBAL" -eq 1 ]; then
  SKILLS_DIR="$HOME/.claude/skills"
  SETTINGS_DIR="$HOME/.claude"
  CMD="$HOME/.claude/skills/mywholelife/scripts/session_end.sh"
else
  SKILLS_DIR=".claude/skills"
  SETTINGS_DIR=".claude"
  CMD="\${CLAUDE_PROJECT_DIR}/.claude/skills/mywholelife/scripts/session_end.sh"
fi

DEST="$SKILLS_DIR/mywholelife"
mkdir -p "$SKILLS_DIR"
cp -R "$SKILL_SRC/." "$DEST/"
chmod +x "$DEST"/scripts/*.sh

SETTINGS="$SETTINGS_DIR/settings.json"
mkdir -p "$SETTINGS_DIR"

write_fresh() {
  printf '{\n  "hooks": {\n    "SessionEnd": [\n      {\n        "matcher": "*",\n        "hooks": [\n          { "type": "command", "command": "%s" }\n        ]\n      }\n    ]\n  }\n}\n' "$CMD" > "$SETTINGS"
}

if [ -f "$SETTINGS" ] && grep -q "skills/mywholelife/scripts/session_end.sh" "$SETTINGS"; then
  echo "install: SessionEnd hook already registered in $SETTINGS"
elif command -v jq >/dev/null 2>&1 && [ -f "$SETTINGS" ]; then
  tmp=$(mktemp)
  jq --arg cmd "$CMD" '.hooks.SessionEnd = ((.hooks.SessionEnd // []) + [{"matcher":"*","hooks":[{"type":"command","command":$cmd}]}])' \
     "$SETTINGS" > "$tmp" && mv "$tmp" "$SETTINGS"
  echo "install: appended SessionEnd hook to $SETTINGS (jq)"
elif [ ! -f "$SETTINGS" ]; then
  write_fresh
  echo "install: created $SETTINGS with SessionEnd hook"
else
  echo "install: $SETTINGS exists and jq is unavailable — merge the block from hook/settings.snippet.json manually." >&2
  echo "install: use this command path: $CMD" >&2
fi

echo "install: copied skill to $DEST"
echo "install: if $SKILLS_DIR was newly created, restart Claude Code for the skill to load."
