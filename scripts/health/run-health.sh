#!/usr/bin/env bash
# Ironflyer — pre-deploy health pass.
#
# Runs the Anti-Bloat tool drivers (govulncheck + jscpd) and surfaces
# the report paths so an operator can export them into the
# orchestrator's gate environment. Non-blocking by default — the
# scripts print a summary and exit codes propagate so CI gating is a
# one-line flip when ready.
#
# Operators run this BEFORE `pulumi up` as part of the closeout
# checklist (docs/CLOSEOUT_CHECKLIST.md).

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "${REPO_ROOT}"

bold=$(printf '\033[1m'); reset=$(printf '\033[0m')
green=$(printf '\033[32m'); red=$(printf '\033[31m'); yellow=$(printf '\033[33m')

section() { printf "\n%s== %s ==%s\n" "${bold}" "$1" "${reset}"; }
ok()      { printf "  %s[OK]%s   %s\n" "${green}" "${reset}" "$1"; }
warn()    { printf "  %s[WARN]%s %s\n" "${yellow}" "${reset}" "$1"; }
fail()    { printf "  %s[FAIL]%s %s\n" "${red}" "${reset}" "$1"; }

VULN_RC=0
DEDUP_RC=0

section "govulncheck (Go vuln scan)"
if ./scripts/lint/run-govulncheck.sh; then
  ok "govulncheck completed"
else
  VULN_RC=$?
  warn "govulncheck exit=${VULN_RC} (gate may surface findings)"
fi

section "jscpd (TypeScript dup)"
if ./scripts/lint/run-jscpd.sh; then
  ok "jscpd within budget"
else
  DEDUP_RC=$?
  warn "jscpd exit=${DEDUP_RC} (over budget — investigate before deploy)"
fi

section "summary"
echo "  reports under: tmp/reports/"
echo "  wire gates in orchestrator env:"
echo "    IRONFLYER_VULN_REPORT_PATH=<path printed above>"
echo "    IRONFLYER_DEDUP_REPORT_PATH=<path printed above>"

# By design we do NOT propagate non-zero from the dup/vuln tools yet —
# CI uploads artifacts, operators iterate. Flip this to `exit
# $((VULN_RC | DEDUP_RC))` when the team is ready to block deploys.
exit 0
