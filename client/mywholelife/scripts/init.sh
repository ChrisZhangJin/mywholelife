#!/usr/bin/env bash
set -euo pipefail

SKILLS_DIR=".claude/skills"
NAME=""
SERVICE_URL="${MWL_SERVICE_URL:-}"

while [ $# -gt 0 ]; do
  case "$1" in
    --global) SKILLS_DIR="$HOME/.claude/skills"; shift ;;
    --url) SERVICE_URL="$2"; shift 2 ;;
    --name) NAME="$2"; shift 2 ;;
    *) NAME="$1"; shift ;;
  esac
done

MWL_HOME="$HOME/.mywholelife"
AGENT_JSON="$MWL_HOME/agent.json"
MEMORY_DIR="$MWL_HOME/memory"
mkdir -p "$MWL_HOME"

read_field() {
  grep -oE "\"$1\":\"[^\"]*\"" "$AGENT_JSON" | head -1 | sed "s/.*\"$1\":\"//;s/\"$//"
}

if [ -f "$AGENT_JSON" ]; then
  ID=$(read_field id)
  [ -z "$SERVICE_URL" ] && SERVICE_URL=$(read_field service_url)
  [ -z "$NAME" ] && NAME=$(read_field name)
else
  [ -z "$SERVICE_URL" ] && SERVICE_URL="http://106.53.207.230:8088"
  [ -z "$NAME" ] && NAME="${MWL_AGENT_NAME:-$(hostname)}"
  NAME=$(printf '%s' "$NAME" | tr -c 'a-zA-Z0-9_-' '-')
  NAME=${NAME:0:64}
  REG_OUT=$(mktemp); REG_CODE=$(curl -sS -o "$REG_OUT" -w '%{http_code}' \
      -X POST -H "X-Agent-Name: $NAME" "$SERVICE_URL/agent/register" || echo 000)
  ID=$(cat "$REG_OUT"); rm -f "$REG_OUT"
  if [ "$REG_CODE" = "200" ]; then
    : # fresh registration
  elif [ "$REG_CODE" = "409" ]; then
    LOOK_OUT=$(mktemp)
    LOOK_CODE=$(curl -sS -o "$LOOK_OUT" -w '%{http_code}' \
        "$SERVICE_URL/agent/lookup?name=$NAME" || echo 000)
    if [ "$LOOK_CODE" != "200" ]; then
      rm -f "$LOOK_OUT"
      echo "init: name '$NAME' already registered but lookup failed (HTTP $LOOK_CODE)" >&2
      exit 1
    fi
    ID=$(cat "$LOOK_OUT"); rm -f "$LOOK_OUT"
    echo "init: name '$NAME' already registered — adopting existing id $ID" >&2
  else
    echo "init: register failed (HTTP $REG_CODE)" >&2
    exit 1
  fi
  if [ -z "$ID" ] || [ "${#ID}" -lt 32 ]; then
    echo "init: registration returned bad id from $SERVICE_URL" >&2
    exit 1
  fi
  printf '{"id":"%s","name":"%s","service_url":"%s"}\n' "$ID" "$NAME" "$SERVICE_URL" > "$AGENT_JSON"
fi

if [ -z "$ID" ]; then
  echo "init: no agent id resolved" >&2
  exit 1
fi

NEW_SKILLS_DIR=0
[ -d "$SKILLS_DIR" ] || NEW_SKILLS_DIR=1

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
curl -sf "$SERVICE_URL/agent/$ID/init" -o "$tmp/init.zip"
unzip -o "$tmp/init.zip" -d "$tmp/x" >/dev/null

mkdir -p "$MEMORY_DIR" "$SKILLS_DIR"
[ -d "$tmp/x/memory" ] && cp -R "$tmp/x/memory/." "$MEMORY_DIR/"
[ -d "$tmp/x/skills" ] && cp -R "$tmp/x/skills/." "$SKILLS_DIR/"

echo "init: reloaded memory into $MEMORY_DIR and skills into $SKILLS_DIR"
if [ "$NEW_SKILLS_DIR" -eq 1 ]; then
  echo "init: $SKILLS_DIR was newly created — restart Claude Code for these skills to load." >&2
fi
