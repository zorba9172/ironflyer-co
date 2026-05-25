#!/usr/bin/env bash
# scripts/build-docker.sh — build all Ironflyer dev images with
# consistent tags from the repo root. Uses BuildKit for the cache
# mounts the Dockerfiles depend on. Final step prints sizes so a
# regression in image weight is visible at a glance.
#
# Usage:
#   ./scripts/build-docker.sh                 # tag :dev
#   TAG=staging ./scripts/build-docker.sh     # tag :staging
#   NEXT_PUBLIC_IRONFLYER_API_URL=https://api.example.com ./scripts/build-docker.sh
set -euo pipefail

# Resolve repo root (the script lives in scripts/, so root is one up).
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

TAG="${TAG:-dev}"
NEXT_PUBLIC_IRONFLYER_API_URL="${NEXT_PUBLIC_IRONFLYER_API_URL:-http://localhost:8080}"

export DOCKER_BUILDKIT=1

build() {
  local name="$1"
  local dockerfile="$2"
  shift 2
  echo
  echo "==> building ironflyer/${name}:${TAG}"
  docker build \
    -f "${dockerfile}" \
    -t "ironflyer/${name}:${TAG}" \
    "$@" \
    .
}

build orchestrator infra/docker/orchestrator.Dockerfile
build runtime      infra/docker/runtime.Dockerfile
build web          infra/docker/web.Dockerfile \
  --build-arg "NEXT_PUBLIC_IRONFLYER_API_URL=${NEXT_PUBLIC_IRONFLYER_API_URL}"
build openvscode   infra/docker/ironflyer-openvscode.Dockerfile
build code         infra/docker/ironflyer-code.Dockerfile

echo
echo "==> built images"
docker images \
  --format 'table {{.Repository}}:{{.Tag}}\t{{.Size}}\t{{.CreatedSince}}' \
  | (read -r header; echo "$header"; grep -E "^ironflyer/(orchestrator|runtime|web|openvscode|code):${TAG}\b" | sort)
