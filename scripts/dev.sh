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
#   scripts/dev.sh --ide       # also build the branded Theia IDE image (clients/ide)

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

COMPOSE_FILE="infra/compose/docker-compose.dev.yml"
IN_DOCKER=0
TAIL_LOGS=0
BUILD_IDE=0
IDE_IMAGE="ironflyer/theia-ide:latest"

for arg in "$@"; do
  case "$arg" in
    --in-docker) IN_DOCKER=1 ;;
    --logs)      TAIL_LOGS=1 ;;
    --ide)       BUILD_IDE=1 ;;
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

# Branded Theia IDE image (clients/ide). Built on demand with --ide, or
# automatically if it's referenced but missing. The runtime's docker driver
# spins one container per workspace from this image; it is not a compose service.
if [ "$BUILD_IDE" -eq 1 ]; then
  if docker image inspect "$IDE_IMAGE" >/dev/null 2>&1; then
    bold "→ branded Theia IDE image already built ($IDE_IMAGE) — skipping"
  else
    bold "→ building branded Theia IDE image ($IDE_IMAGE) — first build is heavy"
    docker build -t "$IDE_IMAGE" clients/ide
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
  (cd core/orchestrator && go run ./cmd/orchestrator)

  # runtime — branded Theia IDE per workspace (build it first with: scripts/dev.sh --ide)
  (cd core/runtime && \\
    IRONFLYER_RUNTIME_DRIVER=docker \\
    IRONFLYER_IDE_IMAGE=$IDE_IMAGE \\
    IRONFLYER_IDE_CONTAINER_PORT=3030 \\
    IRONFLYER_RUNTIME_DEV_AUTOCREATE=1 \\
    go run ./cmd/runtime)
  # (drop the env overrides to fall back to the mock driver / no embedded IDE)

  # studio (the cockpit — new primary surface; proxies /api/runtime to :8090)
  (cd clients/studio && pnpm install && pnpm dev)

Browse:
  - studio:        http://localhost:3000  (Code tab embeds the branded IDE)
  - orchestrator:  http://localhost:8080/healthz
  - runtime:       http://localhost:8090/healthz
  - temporal-ui:   http://localhost:8233
  - minio console: http://localhost:9001 (ironflyer / ironflyer-dev)

EOF
fi

if [ "$TAIL_LOGS" -eq 1 ]; then
  docker compose -f "$COMPOSE_FILE" logs -f --tail=200
fi
