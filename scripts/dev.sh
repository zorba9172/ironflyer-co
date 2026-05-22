#!/usr/bin/env bash
# Ironflyer dev bootstrap. One-command "everything up":
#   1. Verify host tooling (go, node, docker).
#   2. Start the infra layer (postgres / redis / surreal / temporal / minio / code-server).
#   3. Print quick-start commands for orchestrator/runtime/web on the host,
#      OR bring those up inside compose with --in-docker.
#
# Usage:
#   scripts/dev.sh             # storage only, run apps on host
#   scripts/dev.sh --in-docker # also bring orchestrator/runtime/web up in compose
#   scripts/dev.sh --logs      # tail compose logs after starting

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

COMPOSE_FILE="infra/compose/docker-compose.dev.yml"
IN_DOCKER=0
TAIL_LOGS=0

for arg in "$@"; do
  case "$arg" in
    --in-docker) IN_DOCKER=1 ;;
    --logs)      TAIL_LOGS=1 ;;
    -h|--help)
      sed -n '2,12p' "$0"
      exit 0
      ;;
  esac
done

bold() { printf "\033[1m%s\033[0m\n" "$*"; }
warn() { printf "\033[33m%s\033[0m\n" "$*" >&2; }
die()  { printf "\033[31m%s\033[0m\n" "$*" >&2; exit 1; }

require() {
  command -v "$1" >/dev/null 2>&1 || die "missing dependency: $1 — install before running scripts/dev.sh"
}

bold "→ checking dependencies"
require docker
require go
require node
require npm

# Docker daemon up?
docker info >/dev/null 2>&1 || die "docker daemon not reachable — start Docker Desktop / colima first"

# .env scaffold
if [ ! -f .env ] && [ ! -f .env.local ]; then
  if [ -f .env.example ]; then
    cp .env.example .env.local
    bold "→ created .env.local from .env.example (edit secrets before going to prod)"
  fi
fi

bold "→ starting compose stack ($COMPOSE_FILE)"
if [ "$IN_DOCKER" -eq 1 ]; then
  docker compose -f "$COMPOSE_FILE" --profile apps up -d --build
else
  docker compose -f "$COMPOSE_FILE" up -d
fi

bold "→ stack is up"
docker compose -f "$COMPOSE_FILE" ps

if [ "$IN_DOCKER" -eq 0 ]; then
  cat <<EOF

Run the apps on the host (faster iteration):

  # orchestrator
  (cd apps/orchestrator && go run ./cmd/orchestrator)

  # runtime
  (cd apps/runtime && go run ./cmd/runtime)

  # web
  (cd apps/web && npm install && npm run dev)

Browse:
  - web:           http://localhost:3000
  - orchestrator:  http://localhost:8080/healthz
  - runtime:       http://localhost:8090/healthz
  - temporal-ui:   http://localhost:8233
  - minio console: http://localhost:9001 (ironflyer / ironflyer-dev)
  - code-server:   http://localhost:8443 (password: ironflyer-dev)

EOF
fi

if [ "$TAIL_LOGS" -eq 1 ]; then
  docker compose -f "$COMPOSE_FILE" logs -f --tail=200
fi
