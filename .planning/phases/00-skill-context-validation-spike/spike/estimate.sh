#!/usr/bin/env bash
# Throwaway spike tooling (Phase 0). Estimates the resident context cost of each
# payload shape (naive / proxy / merged) and writes the per-shape, per-N cost curve
# to spike/curve.csv. Token estimate is the rough chars/4 heuristic (D-01); Plan 02
# calibrates it against real `/context` anchors. Separates always-on catalog cost
# (name+description, re-injected each turn per D-02 / PITFALLS.md Pitfall 2) from
# full-body-load cost (name+description+body, the cost if bug #14882 reproduces).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GEN="$SCRIPT_DIR/gen-skills.sh"
CURVE="$SCRIPT_DIR/curve.csv"
BRIEF="$SCRIPT_DIR/recent-brief.md"
SKILLS_DIR="$HOME/.claude/skills"
NS=(1 5 10 20 30 50)

chars() { printf '%s' "$1" | wc -c | tr -d ' '; }
tokens() { echo $(( $1 / 4 )); }  # chars/4 heuristic (D-01)

cleanup() { bash "$GEN" --clean >/dev/null 2>&1 || true; }
trap cleanup EXIT

# Measure a skill shape: install one unit, split name+description (catalog) from body.
measure_skill() {
  local shape="$1" f name_val desc_val body cat_chars body_chars
  bash "$GEN" "$shape" 1 >/dev/null
  f="$(ls -d "$SKILLS_DIR/spike-dummy-$shape-"*/ | head -1)SKILL.md"
  name_val="$(grep '^name:' "$f" | head -1 | sed 's/^name: //')"
  desc_val="$(grep '^description:' "$f" | head -1 | sed 's/^description: //')"
  body="$(awk 'f{print} /^---$/{c++; if(c==2) f=1}' "$f")"
  cat_chars=$(( $(chars "$name_val") + $(chars "$desc_val") ))
  body_chars="$(chars "$body")"
  PU_CAT=$(tokens "$cat_chars")
  PU_FULL=$(tokens $(( cat_chars + body_chars )))
  bash "$GEN" --clean >/dev/null
}

# Merged shape bypasses the skill mechanism: per-unit is one brief's own token count.
measure_merged() {
  bash "$GEN" merged 1 >/dev/null
  local brief_chars
  brief_chars="$(chars "$(cat "$BRIEF")")"
  PU_CAT=$(tokens "$brief_chars")
  PU_FULL=$PU_CAT
  bash "$GEN" --clean >/dev/null
}

declare -A CAT FULL
measure_skill naive;  CAT[naive]=$PU_CAT;  FULL[naive]=$PU_FULL
measure_skill proxy;  CAT[proxy]=$PU_CAT;  FULL[proxy]=$PU_FULL
measure_merged;       CAT[merged]=$PU_CAT; FULL[merged]=$PU_FULL

echo "shape,N,catalog_tokens,fullload_tokens" > "$CURVE"
for shape in naive proxy merged; do
  for n in "${NS[@]}"; do
    echo "$shape,$n,$(( CAT[$shape] * n )),$(( FULL[$shape] * n ))" >> "$CURVE"
  done
done

echo "Per-unit token estimates (chars/4 heuristic, CC 2.1.211):"
printf '  %-7s catalog=%-5s fullload=%s\n' \
  naive  "${CAT[naive]}"  "${FULL[naive]}" \
  proxy  "${CAT[proxy]}"  "${FULL[proxy]}" \
  merged "${CAT[merged]}" "${FULL[merged]}"
echo "Curve written to $CURVE ($(grep -vc '^#' "$CURVE" | awk '{print $1-1}') data rows)."
