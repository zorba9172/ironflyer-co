#!/usr/bin/env bash
# Ironflyer — Anti-Bloat: Lighthouse CI driver.
#
# Wires the `perf_budget` gate (see core/orchestrator/internal/ai/finisher/
# gates_antibloat.go). @lhci/cli is Google's reference harness for
# Lighthouse — we run it via `npx --yes` so it is NOT a project dep.
#
# Inputs:
#   IRONFLYER_LH_URL              URL to audit (default http://localhost:3000)
#   IRONFLYER_LH_RUNS             number of Lighthouse runs (default 3)
#   IRONFLYER_LH_PERF_MIN         performance threshold (default 80; <60 fails)
#   IRONFLYER_LH_A11Y_MIN         accessibility threshold (default 95; <90 fails)
#   IRONFLYER_LH_BP_MIN           best-practices threshold (default 85)
#   IRONFLYER_LH_SEO_MIN          seo threshold (default 85)
#
# Outputs:
#   tmp/reports/lighthouse-<ts>.json   (normalized findings shape)
#   tmp/reports/lighthouse-<ts>/       (raw lhr-*.json files from lhci)
#   stdout: "perf_budget: perf=<N> a11y=<N> bp=<N> seo=<N> -> <path>"
#
# Operator wiring:
#   IRONFLYER_LH_URL=https://preview.example.com ./scripts/lint/run-lighthouse.sh
#   export IRONFLYER_PERF_REPORT_PATH=/abs/path/to/lighthouse-<ts>.json
#
# Exit codes:
#   0  every category meets its threshold (or only warning-level dips)
#   1  any category below its ERROR threshold (perf<60, a11y<90)

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
REPORT_DIR="${REPO_ROOT}/tmp/reports"
mkdir -p "${REPORT_DIR}"

TS="$(date -u +%Y%m%dT%H%M%SZ)"
NORMALIZED="${REPORT_DIR}/lighthouse-${TS}.json"
LH_OUT_DIR="${REPORT_DIR}/lighthouse-${TS}"

URL="${IRONFLYER_LH_URL:-http://localhost:3000}"
RUNS="${IRONFLYER_LH_RUNS:-3}"

# Thresholds — overridable via env.
PERF_MIN="${IRONFLYER_LH_PERF_MIN:-80}"
PERF_FAIL=60
A11Y_MIN="${IRONFLYER_LH_A11Y_MIN:-95}"
A11Y_FAIL=90
BP_MIN="${IRONFLYER_LH_BP_MIN:-85}"
SEO_MIN="${IRONFLYER_LH_SEO_MIN:-85}"

if ! command -v npx >/dev/null 2>&1; then
  echo "FATAL: npx not in PATH — install Node.js + npm" >&2
  exit 1
fi
if ! command -v jq >/dev/null 2>&1; then
  echo "FATAL: jq not in PATH — required to project Lighthouse output" >&2
  exit 1
fi

mkdir -p "${LH_OUT_DIR}"

echo "running lighthouse against ${URL} (runs=${RUNS})..." >&2

# lhci collect writes to .lighthouseci/ by default. We point it at our
# per-run dir via the canonical config-less invocation; --numberOfRuns
# is the supported way to average out variability.
COLLECT_DIR="${REPORT_DIR}/.lhci-${TS}"
mkdir -p "${COLLECT_DIR}"

(
  cd "${REPO_ROOT}" && \
  npx --yes @lhci/cli@0.13 collect \
    --url="${URL}" \
    --numberOfRuns="${RUNS}" \
    >&2
) || {
  echo "WARN: lhci collect non-zero — capturing whatever landed in .lighthouseci/" >&2
}

# `lhci collect` always writes to .lighthouseci/ in the cwd. Now upload
# to filesystem so the lhr-*.json files land in a deterministic path.
(
  cd "${REPO_ROOT}" && \
  npx --yes @lhci/cli@0.13 upload \
    --target=filesystem \
    --outputDir="${LH_OUT_DIR}" \
    >&2
) || {
  echo "WARN: lhci upload non-zero — falling back to direct .lighthouseci/ scrape" >&2
}

# Find the lhr JSON reports (lhci's filesystem target emits lhr-*.json).
shopt -s nullglob
LHR_FILES=("${LH_OUT_DIR}"/lhr-*.json)
if [ ${#LHR_FILES[@]} -eq 0 ]; then
  # Fallback: raw .lighthouseci/ output.
  LHR_FILES=("${REPO_ROOT}"/.lighthouseci/lhr-*.json)
fi

if [ ${#LHR_FILES[@]} -eq 0 ]; then
  echo "FATAL: no lhr-*.json reports produced — is the URL reachable?" >&2
  cat > "${NORMALIZED}" <<EOF
{
  "summary": {
    "performance": 0,
    "accessibility": 0,
    "bestPractices": 0,
    "seo": 0,
    "passed": false,
    "thresholds": { "performance": ${PERF_MIN}, "accessibility": ${A11Y_MIN}, "bestPractices": ${BP_MIN}, "seo": ${SEO_MIN} }
  },
  "findings": [
    { "category": "infra", "audit": "collect", "score": 0, "severity": "warning",
      "message": "lhci collect produced no lhr-*.json reports against ${URL}" }
  ]
}
EOF
  exit 1
fi

# Project the median run per category into 0-100 scores, then collect
# every audit whose score < 0.9 across the runs (deduped by audit id)
# and emit them as findings whose severity follows the audit's category
# threshold. The jq pipeline:
#   - reduces N lhr JSONs into a stream of {category, score}
#   - computes per-category median (sort + middle element)
#   - collects audit-level failures (score < 0.9) as findings
jq -s --argjson perfMin "${PERF_MIN}" \
      --argjson perfFail "${PERF_FAIL}" \
      --argjson a11yMin "${A11Y_MIN}" \
      --argjson a11yFail "${A11Y_FAIL}" \
      --argjson bpMin "${BP_MIN}" \
      --argjson seoMin "${SEO_MIN}" '
  # Helper: median of an array of numbers.
  def median: sort | if length == 0 then 0 elif length % 2 == 1 then .[length/2|floor] else (.[length/2-1] + .[length/2]) / 2 end;

  # Per-run extraction → { perf, a11y, bp, seo, audits: [...] }
  ( map({
      perf: ((.categories.performance.score // 0) * 100),
      a11y: ((.categories.accessibility.score // 0) * 100),
      bp:   ((.categories["best-practices"].score // 0) * 100),
      seo:  ((.categories.seo.score // 0) * 100),
      audits: ([ .audits | to_entries[] | {
                  id: .key,
                  title: (.value.title // .key),
                  score: ((.value.score // 1) * 100),
                  displayValue: (.value.displayValue // "")
                } | select(.score < 90) ])
    }) ) as $runs

  | ( $runs | map(.perf) | median ) as $perf
  | ( $runs | map(.a11y) | median ) as $a11y
  | ( $runs | map(.bp)   | median ) as $bp
  | ( $runs | map(.seo)  | median ) as $seo

  | ( [ $runs[].audits[] ] | group_by(.id) | map(.[0]) ) as $audits

  # Pass = all medians ≥ their min; Fail only when any median < its fail floor.
  | ( ($perf >= $perfMin) and ($a11y >= $a11yMin) and ($bp >= $bpMin) and ($seo >= $seoMin) ) as $passed
  | ( ($perf < $perfFail) or ($a11y < $a11yFail) ) as $hardFail

  | {
      summary: {
        performance:   ($perf | round),
        accessibility: ($a11y | round),
        bestPractices: ($bp   | round),
        seo:           ($seo  | round),
        passed:        $passed,
        hardFail:      $hardFail,
        thresholds: {
          performance:   $perfMin,
          accessibility: $a11yMin,
          bestPractices: $bpMin,
          seo:           $seoMin
        }
      },
      findings: (
        [
          # Per-category summary finding.
          { category: "performance",  audit: "category-score", score: ($perf | round),
            severity: (if $perf < $perfFail then "error" elif $perf < $perfMin then "warning" else "info" end),
            message: ("performance score " + (($perf | round) | tostring) + " (min " + ($perfMin | tostring) + ", fail < " + ($perfFail | tostring) + ")") },
          { category: "accessibility", audit: "category-score", score: ($a11y | round),
            severity: (if $a11y < $a11yFail then "error" elif $a11y < $a11yMin then "warning" else "info" end),
            message: ("accessibility score " + (($a11y | round) | tostring) + " (min " + ($a11yMin | tostring) + ", fail < " + ($a11yFail | tostring) + ")") },
          { category: "best-practices", audit: "category-score", score: ($bp | round),
            severity: (if $bp < $bpMin then "warning" else "info" end),
            message: ("best-practices score " + (($bp | round) | tostring) + " (min " + ($bpMin | tostring) + ")") },
          { category: "seo", audit: "category-score", score: ($seo | round),
            severity: (if $seo < $seoMin then "warning" else "info" end),
            message: ("seo score " + (($seo | round) | tostring) + " (min " + ($seoMin | tostring) + ")") }
        ]
        +
        ( $audits | map({
            category: "audit",
            audit: .id,
            score: (.score | round),
            severity: (if .score < 50 then "warning" else "info" end),
            message: (.title + ": score " + ((.score | round) | tostring) + (if (.displayValue // "") == "" then "" else " (" + .displayValue + ")" end)),
            path: ("lighthouse://" + .id)
          })
        )
      )
    }
' "${LHR_FILES[@]}" > "${NORMALIZED}"

PERF=$(jq -r '.summary.performance'   "${NORMALIZED}")
A11Y=$(jq -r '.summary.accessibility' "${NORMALIZED}")
BP=$(jq   -r '.summary.bestPractices' "${NORMALIZED}")
SEO=$(jq  -r '.summary.seo'           "${NORMALIZED}")
HARDFAIL=$(jq -r '.summary.hardFail'  "${NORMALIZED}")

echo "perf_budget: perf=${PERF} a11y=${A11Y} bp=${BP} seo=${SEO} -> ${NORMALIZED}"
echo "wire the orchestrator gate: export IRONFLYER_PERF_REPORT_PATH='${NORMALIZED}'"

if [ "${HARDFAIL}" = "true" ]; then
  echo "perf_budget: FAIL — perf<${PERF_FAIL} or a11y<${A11Y_FAIL}" >&2
  exit 1
fi
exit 0
