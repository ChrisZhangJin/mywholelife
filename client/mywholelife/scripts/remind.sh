#!/usr/bin/env bash
set -euo pipefail

SKILLS_DIR=".claude/skills"
SERVICE_URL="${MWL_SERVICE_URL:-}"
MEM=""

while [ $# -gt 0 ]; do
  case "$1" in
    --global) SKILLS_DIR="$HOME/.claude/skills"; shift ;;
    --url) SERVICE_URL="$2"; shift 2 ;;
    *) MEM="$1"; shift ;;
  esac
done

if [ -z "$MEM" ]; then
  echo "usage: remind.sh [--global] [--url <url>] <memId>" >&2
  exit 2
fi

MEM=$(printf '%s' "$MEM" | tr -c 'a-zA-Z0-9_-' '-')
MEM=${MEM:0:64}

MWL_HOME="$HOME/.mywholelife"
AGENT_JSON="$MWL_HOME/agent.json"

if [ ! -f "$AGENT_JSON" ]; then
  echo "remind: $AGENT_JSON missing — run init.sh first" >&2
  exit 1
fi

read_field() {
  grep -oE "\"$1\":\"[^\"]*\"" "$AGENT_JSON" | head -1 | sed "s/.*\"$1\":\"//;s/\"$//"
}

ID=$(read_field id)
[ -z "$SERVICE_URL" ] && SERVICE_URL=$(read_field service_url)

if [ -z "$ID" ]; then
  echo "remind: no agent id resolved from $AGENT_JSON" >&2
  exit 1
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
curl -sf "$SERVICE_URL/agent/$ID/remind?mem=$MEM" -o "$tmp/mem.tar"

mkdir -p "$SKILLS_DIR/$MEM"
tar -C "$SKILLS_DIR/$MEM" -xf "$tmp/mem.tar"

# Load-bearing mid-session fallback (RECALL-03/D-05): a newly-created skill
# subdirectory is not reliably auto-activated mid-session on CC 2.1.211
# (anthropics/claude-code#31559), so echo the reinstalled body to stdout —
# this is the version-independent guarantee, unlike the reload hint below.
echo "remind: reinstalled $SKILLS_DIR/$MEM/SKILL.md"
echo "----- BEGIN RECALLED SKILL.md -----"
cat "$SKILLS_DIR/$MEM/SKILL.md"
echo "----- END RECALLED SKILL.md -----"
echo "remind: run /reload-skills (CC 2.1.152+) or restart Claude Code to make $MEM model-invocable this session; it loads normally next session from $SKILLS_DIR/$MEM/." >&2
