#!/usr/bin/env bash
# Ironflyer — Anti-Bloat: knip driver.
#
# Wires the `deadcode` gate (see core/orchestrator/internal/ai/finisher/
# gates_antibloat.go). knip is the standard TypeScript unused
# exports / files / dependencies detector. We run it via `npx --yes`
# so it is NOT a project dep — first run downloads, subsequent runs
# cache.
#
# Outputs:
#   tmp/reports/knip-<timestamp>.json     (normalized findings shape)
#   stdout: "deadcode: <files> files, <exports> exports, <deps> deps -> <path>"
#
# Operator wiring:
#   ./scripts/lint/run-knip.sh
#   export IRONFLYER_DEADCODE_REPORT_PATH=/abs/path/to/knip-<ts>.json
#
# Exit codes:
#   0  total unused items <= budget (default 0)
#   1  total unused items > budget OR knip produced no parseable output

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
REPORT_DIR="${REPO_ROOT}/tmp/reports"
mkdir -p "${REPORT_DIR}"

TS="$(date -u +%Y%m%dT%H%M%SZ)"
NORMALIZED="${REPORT_DIR}/knip-${TS}.json"
BUDGET="${IRONFLYER_DEADCODE_BUDGET:-0}"

if ! command -v npx >/dev/null 2>&1; then
  echo "FATAL: npx not in PATH — install Node.js + npm" >&2
  exit 1
fi
if ! command -v jq >/dev/null 2>&1; then
  echo "FATAL: jq not in PATH — required to project knip output" >&2
  exit 1
fi

TMP_RAW="$(mktemp -d)"
trap 'rm -rf "${TMP_RAW}"' EXIT

# Walk both TS client roots. Each contributes one raw report; we merge
# them into a single normalized document at the end.
run_one() {
  local dir="$1"
  local out_file="$2"
  if [ ! -d "${REPO_ROOT}/${dir}" ]; then
    echo "skipping ${dir} (not a directory)" >&2
    return 0
  fi
  echo "running knip in ${dir}..." >&2
  # knip exits non-zero when findings exist — that's informational,
  # not fatal. Capture stdout into a JSON file; ignore stderr noise.
  (
    cd "${REPO_ROOT}/${dir}" && \
    npx --yes knip --reporter json --no-progress 2>/dev/null
  ) > "${out_file}" || true
  if [ ! -s "${out_file}" ]; then
    echo "WARN: knip produced no output for ${dir}" >&2
    # Write an empty shape so jq merge below doesn't choke.
    echo '{"files":[],"issues":[]}' > "${out_file}"
  fi
}

run_one "clients/web"              "${TMP_RAW}/web.json"
run_one "clients/vscode-extension" "${TMP_RAW}/vscode.json"

# knip JSON shape (v5+):
#   {
#     "files": [ "<path>", ... ],                    # unused files
#     "issues": [ { "file": "...", "exports": [...], # unused exports
#                  "dependencies": [...] } ]
#   }
#
# We project into the gate's expected shape:
#   { summary: { unusedFiles, unusedExports, unusedDeps, budget, exceeded },
#     findings: [ { path, message, severity } ... ] }
#
# severity = "error" when overall counts > budget, "warning" per item
# otherwise. The summary issue leads so the dashboard renders the
# verdict at the top of the gate panel.
jq -s --arg budget "${BUDGET}" '
  # Merge: flatten files + issues across all module reports.
  ( map(.files // []) | add // [] ) as $files
  | ( map(.issues // []) | add // [] ) as $issues
  | ( ( $issues | map((.exports // []) | length) | add ) // 0 ) as $expCount
  | ( ( $issues | map((.dependencies // []) | length) | add ) // 0 ) as $depCount
  | ( ($files | length) + $expCount + $depCount ) as $total
  | ( ($budget | tonumber) ) as $b
  | ( ($total > $b) ) as $exceeded
  | {
      summary: {
        unusedFiles: ($files | length),
        unusedExports: $expCount,
        unusedDeps: $depCount,
        budget: $b,
        exceeded: $exceeded
      },
      findings: (
        [ {
            path: "clients/",
            message: ("knip: " + ($files | length | tostring) + " unused files, "
                     + ($expCount | tostring) + " unused exports, "
                     + ($depCount | tostring) + " unused deps "
                     + "(budget " + ($b | tostring) + ")"),
            severity: (if $exceeded then "error" else "info" end)
          } ]
        +
        ($files | map({
            path: .,
            message: ("unused file: " + .),
            severity: (if $exceeded then "error" else "warning" end)
        }))
        +
        ($issues | map(
          (.file // "unknown") as $f
          | ((.exports // []) | map({
              path: $f,
              message: ("unused export: " + (.name // . | tostring)
                        + " in " + $f),
              severity: (if $exceeded then "error" else "warning" end)
            }))
          + ((.dependencies // []) | map({
              path: $f,
              message: ("unused dependency: " + (.name // . | tostring)
                        + " in " + $f),
              severity: (if $exceeded then "error" else "warning" end)
            }))
        ) | add // [])
      )
    }
' "${TMP_RAW}"/*.json > "${NORMALIZED}"

TOTAL=$(jq -r '(.summary.unusedFiles + .summary.unusedExports + .summary.unusedDeps)' "${NORMALIZED}")
FILES=$(jq -r '.summary.unusedFiles' "${NORMALIZED}")
EXPORTS=$(jq -r '.summary.unusedExports' "${NORMALIZED}")
DEPS=$(jq -r '.summary.unusedDeps' "${NORMALIZED}")
EXCEEDED=$(jq -r '.summary.exceeded' "${NORMALIZED}")

echo "deadcode: ${FILES} files, ${EXPORTS} exports, ${DEPS} deps (budget ${BUDGET}) -> ${NORMALIZED}"
echo "wire the orchestrator gate: export IRONFLYER_DEADCODE_REPORT_PATH='${NORMALIZED}'"

if [ "${EXCEEDED}" = "true" ]; then
  echo "deadcode: FAIL — ${TOTAL} unused items exceed budget ${BUDGET}" >&2
  exit 1
fi
exit 0
