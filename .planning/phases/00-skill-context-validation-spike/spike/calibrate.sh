#!/usr/bin/env bash
# Throwaway spike tooling (Phase 0, Plan 03). Reconciles the estimated cost curve
# (spike/curve.csv, chars/4 heuristic from Plan 01) against the ground-truth
# `/context` resident-token anchors (spike/readings.csv, CC 2.1.211 from Plan 02),
# and writes the calibrated curve to spike/curve-calibrated.csv.
#
# Data contract (D-01 / Plan 03 Task 1):
#   estimated_tokens = curve.csv catalog_tokens  (the always-on catalog cost is the
#   go/no-go basis D-05's 10% line judges). The SAME catalog column drives both the
#   correction-factor derivation and every row-fill. fullload_tokens is read only for
#   the catalog-vs-catalog+body #14882 diagnostic printed to stdout; it is NOT written.
#
#   per_skill_resident (naive) = (naive_N20_resident - baseline_resident) / 20,
#   cross-checked against (naive_N50_resident - baseline_resident) / 50.
#   correction_factor = per_skill_resident / naive_est_per_unit_catalog.
#   naive/proxy are catalog-loaded skills -> both take this catalog factor (proxy has
#   no live anchor of its own; see Plan 02 -- naive already clears the bar). merged
#   bypasses the skill mechanism entirely (a flat markdown file, no catalog re-injection),
#   so its calibrated cost is its own estimated token count.
#
#   pct_of_window is expressed as a PERCENT (calibrated / total_window_tokens * 100)
#   so it compares directly against the D-05 10% line.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CURVE="$SCRIPT_DIR/curve.csv"
READINGS="$SCRIPT_DIR/readings.csv"
OUT="$SCRIPT_DIR/curve-calibrated.csv"
PASS_LINE=10   # D-05: recent-skill set must stay <= 10% of the context window

awk -F, -v pass_line="$PASS_LINE" '
  # ---- readings.csv (ground-truth /context anchors) ----
  FNR==NR {
    if (FNR==1) next
    if ($1=="baseline")            { baseline=$3 }
    if ($1=="naive" && $2==20 && $3!="") { n20=$3; window=$4 }
    if ($1=="naive" && $2==50 && $3!="") { n50=$3 }
    next
  }
  # ---- curve.csv (estimated chars/4 curve) ----
  FNR==1 { next }
  {
    shape=$1; n=$2; cat=$3; full=$4
    key=shape SUBSEP n
    catalog[key]=cat; fullload[key]=full
    if (!(shape in seen_shape)) { order[++nshapes]=shape; seen_shape[shape]=1 }
    ns[shape,++cnt[shape]]=n
    if (n==1) { est_unit[shape]=cat; full_unit[shape]=full }
  }
  END {
    # --- derive the catalog correction factor from the naive anchor ---
    per_skill_naive_20 = (n20 - baseline) / 20
    per_skill_naive_50 = (n50 - baseline) / 50
    factor = per_skill_naive_20 / est_unit["naive"]

    # --- write the calibrated curve ---
    printf "shape,N,estimated_tokens,calibrated_tokens,pct_of_window\n" > OUTFILE
    for (i=1; i<=nshapes; i++) {
      shape=order[i]
      for (j=1; j<=cnt[shape]; j++) {
        n=ns[shape,j]
        est=catalog[shape,n]
        if (shape=="merged") cal=est
        else                 cal=est * factor
        cal_r = sprintf("%.0f", cal)
        pct   = sprintf("%.4f", (cal / window) * 100)
        printf "%s,%s,%s,%s,%s\n", shape, n, est, cal_r, pct >> OUTFILE
      }
    }

    # --- stdout diagnostics ---
    printf "=== Calibration (CC 2.1.211) ===\n"
    printf "baseline resident      : %d tok (window %d)\n", baseline, window
    printf "naive per-skill resident: %.1f tok  (N=20 anchor)  cross-check %.1f tok (N=50 anchor)\n", per_skill_naive_20, per_skill_naive_50
    printf "naive est catalog/unit  : %d tok   -> correction factor %.4f\n", est_unit["naive"], factor
    printf "naive est fullload/unit : %d tok\n", full_unit["naive"]
    printf "\n-- per-skill calibrated cost --\n"
    printf "naive : %.0f tok/skill\n", est_unit["naive"] * factor
    printf "proxy : %.0f tok/skill (no live anchor; naive catalog factor applied)\n", est_unit["proxy"] * factor
    printf "merged: %d tok/brief (flat file, un-calibrated -- bypasses skill mechanism)\n", est_unit["merged"]

    # --- #14882 verdict: does the measured per-skill track catalog-only or catalog+body? ---
    d_cat  = (per_skill_naive_20 > est_unit["naive"])  ? per_skill_naive_20 - est_unit["naive"]  : est_unit["naive"]  - per_skill_naive_20
    d_full = (per_skill_naive_20 > full_unit["naive"]) ? per_skill_naive_20 - full_unit["naive"] : full_unit["naive"] - per_skill_naive_20
    printf "\n-- #14882 diagnostic --\n"
    printf "measured %.0f tok/skill vs catalog-only est %d (|d|=%.0f) vs fullload est %d (|d|=%.0f)\n", per_skill_naive_20, est_unit["naive"], d_cat, full_unit["naive"], d_full
    if (d_cat < d_full)
      printf "VERDICT: tracks CATALOG-ONLY -> progressive disclosure HOLDS, bug #14882 does NOT reproduce on 2.1.211.\n"
    else
      printf "VERDICT: tracks CATALOG+BODY -> bug #14882 REPRODUCES; naive fails.\n"

    # --- pass/fail at N=20 against the 10% line + safe ceiling for the winner ---
    printf "\n-- go/no-go at N=20 (D-05 %d%% line) --\n", pass_line
    winner=""
    for (i=1; i<=nshapes; i++) {
      shape=order[i]
      if (shape=="merged") cal20=catalog[shape,20]
      else                 cal20=catalog[shape,20] * factor
      pct20=(cal20/window)*100
      verdict=(pct20 <= pass_line) ? "PASS" : "FAIL"
      printf "%-7s N=20: %.0f tok = %.4f%% -> %s\n", shape, cal20, pct20, verdict
      if (winner=="" && verdict=="PASS") winner=shape
    }
    # adopted shape = first PASS on the D-07 ladder (naive -> proxy -> merged)
    if (winner=="naive")  per_win = est_unit["naive"] * factor
    else if (winner=="proxy") per_win = est_unit["proxy"] * factor
    else per_win = est_unit["merged"]
    ceiling = int((window * pass_line/100) / per_win)
    printf "\nadopted shape (D-07 ladder): %s\n", winner
    printf "safe ceiling N (largest recent-set <= %d%% of %d-tok window): %d skills (%.0f tok/skill)\n", pass_line, window, ceiling, per_win
    # conservative-window headroom note (a real agent session may run a smaller window)
    printf "  headroom check @200000-tok window: safe ceiling = %d skills\n", int((200000 * pass_line/100) / per_win)
  }
' OUTFILE="$OUT" "$READINGS" "$CURVE"

echo
echo "Calibrated curve written to $OUT ($(( $(grep -vc '^#' "$OUT") - 1 )) data rows)."
