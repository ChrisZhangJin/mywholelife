#!/usr/bin/env bash
set -euo pipefail

if [ $# -lt 1 ]; then
  echo "usage: push.sh <project> [scope]" >&2
  exit 2
fi

PROJECT="$1"
SCOPE="${2:-project}"

MWL_HOME="$HOME/.mywholelife"
AGENT_JSON="$MWL_HOME/agent.json"
OUTBOX="$MWL_HOME/outbox"

if [ ! -f "$AGENT_JSON" ]; then
  echo "push: $AGENT_JSON missing — run init.sh first" >&2
  exit 1
fi

read_field() {
  grep -oE "\"$1\":\"[^\"]*\"" "$AGENT_JSON" | head -1 | sed "s/.*\"$1\":\"//;s/\"$//"
}

ID=$(read_field id)
SERVICE_URL="${MWL_SERVICE_URL:-$(read_field service_url)}"

if [ ! -d "$OUTBOX/$PROJECT" ]; then
  echo "push: nothing staged at $OUTBOX/$PROJECT" >&2
  exit 0
fi

tar -C "$OUTBOX/$PROJECT" -cf - . \
  | curl -sf --data-binary @- \
      -H 'Content-Type: application/x-tar' \
      "$SERVICE_URL/agent/$ID/memory?scope=$SCOPE&project=$PROJECT"

rm -rf "${OUTBOX:?}/$PROJECT"
echo "push: sent $PROJECT (scope=$SCOPE) and cleared outbox"
