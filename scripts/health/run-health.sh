#!/usr/bin/env bash
# Ironflyer — pre-deploy health pass.
#
# Runs every functional Anti-Bloat tool driver (govulncheck + jscpd +
# knip + gocognit + goleak smoke + size-limit) and surfaces the report
# paths so an operator can export them into the orchestrator's gate
# environment. Each tool writes into tmp/reports/. A health-summary.json
# is written alongside the per-tool reports stamping the failing tool's
# name + exit code so CI artifacts can be diffed across runs.
#
# Operators run this BEFORE `pulumi up` as part of the closeout
# checklist (docs/CLOSEOUT_CHECKLIST.md).
#
# Exit codes:
#   0  every driver exited 0
#   1  one or more drivers exited non-zero (summary in tmp/reports/health-summary.json)

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "${REPO_ROOT}"

REPORT_DIR="${REPO_ROOT}/tmp/reports"
mkdir -p "${REPORT_DIR}"
SUMMARY="${REPORT_DIR}/health-summary.json"

bold=$(printf '\033[1m'); reset=$(printf '\033[0m')
green=$(printf '\033[32m'); red=$(printf '\033[31m'); yellow=$(printf '\033[33m')

section() { printf "\n%s== %s ==%s\n" "${bold}" "$1" "${reset}"; }
ok()      { printf "  %s[OK]%s   %s\n" "${green}" "${reset}" "$1"; }
warn()    { printf "  %s[WARN]%s %s\n" "${yellow}" "${reset}" "$1"; }
fail()    { printf "  %s[FAIL]%s %s\n" "${red}" "${reset}" "$1"; }

# Per-tool exit codes. 0 = success; non-zero = the tool's exit.
declare -i VULN_RC=0 DEDUP_RC=0 DEAD_RC=0 COMPLEX_RC=0 LEAK_RC=0 BUNDLE_RC=0 LH_RC=0

run_tool() {
  local label="$1"
  local script="$2"
  section "${label}"
  if [ ! -x "${script}" ]; then
    warn "${script} missing or not executable — skipped"
    return 99
  fi
  if "${script}"; then
    ok "${label} OK"
    return 0
  fi
  local rc=$?
  fail "${label} exit=${rc}"
  return ${rc}
}

run_tool "govulncheck (Go vuln scan)"    "./scripts/lint/run-govulncheck.sh" || VULN_RC=$?
run_tool "jscpd (TS dup)"                "./scripts/lint/run-jscpd.sh"       || DEDUP_RC=$?
run_tool "knip (TS deadcode)"            "./scripts/lint/run-knip.sh"        || DEAD_RC=$?
run_tool "gocognit (Go complexity)"      "./scripts/lint/run-gocognit.sh"    || COMPLEX_RC=$?
# goleak smoke requires a running orchestrator + IRONFLYER_LEAK_PROBE_TOKEN.
# When either is missing the script exits 2; we treat that as "skipped" so
# the health pass still completes for local dev.
if [ -n "${IRONFLYER_LEAK_PROBE_TOKEN:-}" ]; then
  run_tool "goleak smoke (goroutine snapshot)" "./scripts/lint/run-goleak-smoke.sh" || LEAK_RC=$?
else
  section "goleak smoke (goroutine snapshot)"
  warn "IRONFLYER_LEAK_PROBE_TOKEN unset — skipping (gate stays SeverityInfo)"
fi
run_tool "size-limit (web bundle budgets)" "./scripts/lint/run-size-limit.sh" || BUNDLE_RC=$?
# Lighthouse smoke requires a running web (or a deploy preview URL) —
# same pattern as goleak: only run when IRONFLYER_LH_URL is set so the
# health pass still completes for local dev without a server.
if [ -n "${IRONFLYER_LH_URL:-}" ]; then
  run_tool "lighthouse (perf/a11y/bp/seo budgets)" "./scripts/lint/run-lighthouse.sh" || LH_RC=$?
else
  section "lighthouse (perf/a11y/bp/seo budgets)"
  warn "IRONFLYER_LH_URL unset — skipping (gate stays SeverityInfo)"
fi

# Stamp the summary so CI artifacts capture what failed without parsing
# stdout. Each entry: { tool, rc, reportGlob }.
TS="$(date -u +%Y%m%dT%H%M%SZ)"
cat > "${SUMMARY}" <<EOF
{
  "ts": "${TS}",
  "results": [
    { "tool": "govulncheck", "rc": ${VULN_RC},    "reportGlob": "tmp/reports/govulncheck-*.json" },
    { "tool": "jscpd",       "rc": ${DEDUP_RC},   "reportGlob": "tmp/reports/jscpd-*.json" },
    { "tool": "knip",        "rc": ${DEAD_RC},    "reportGlob": "tmp/reports/knip-*.json" },
    { "tool": "gocognit",    "rc": ${COMPLEX_RC}, "reportGlob": "tmp/reports/gocognit-*.json" },
    { "tool": "goleak",      "rc": ${LEAK_RC},    "reportGlob": "tmp/reports/goleak-*.json" },
    { "tool": "size-limit",  "rc": ${BUNDLE_RC},  "reportGlob": "tmp/reports/size-limit-*.json" },
    { "tool": "lighthouse",  "rc": ${LH_RC},      "reportGlob": "tmp/reports/lighthouse-*.json" }
  ],
  "anyFailed": $([ $((VULN_RC | DEDUP_RC | DEAD_RC | COMPLEX_RC | LEAK_RC | BUNDLE_RC | LH_RC)) -ne 0 ] && echo true || echo false)
}
EOF

section "summary"
echo "  reports under: tmp/reports/"
echo "  health summary: ${SUMMARY}"
echo "  wire gates in orchestrator env:"
echo "    IRONFLYER_VULN_REPORT_PATH=<latest tmp/reports/govulncheck-*.json>"
echo "    IRONFLYER_DEDUP_REPORT_PATH=<latest tmp/reports/jscpd-*.json>"
echo "    IRONFLYER_DEADCODE_REPORT_PATH=<latest tmp/reports/knip-*.json>"
echo "    IRONFLYER_COMPLEXITY_REPORT_PATH=<latest tmp/reports/gocognit-*.json>"
echo "    IRONFLYER_MEMLEAK_REPORT_PATH=<latest tmp/reports/goleak-normalized-*.json>"
echo "    IRONFLYER_BUNDLE_REPORT_PATH=<latest tmp/reports/size-limit-*.json>"
echo "    IRONFLYER_PERF_REPORT_PATH=<latest tmp/reports/lighthouse-*.json>"

declare -i AGG=$((VULN_RC | DEDUP_RC | DEAD_RC | COMPLEX_RC | LEAK_RC | BUNDLE_RC | LH_RC))
if [ "${AGG}" -ne 0 ]; then
  echo
  fail "one or more drivers failed — see ${SUMMARY}"
  exit 1
fi
exit 0
