# syntax=docker/dockerfile:1.7
#
# Orchestrator image. Multi-stage: alpine + Go build, alpine runtime
# (small + has curl for healthcheck). Runs as a non-root user.
#
# Templates are baked into the image at /app/templates so the scaffold
# step works without a sidecar PVC; dev iterations can still bind-mount
# the repo's templates/ via docker-compose.dev.yml.
#
# CGO is required because the production build opts in to the
# tree-sitter AST adapters (`-tags treesitter`). The smacker/go-tree-sitter
# Go bindings ship the per-grammar C code inline and compile it via cgo,
# so we need gcc + musl-dev in the build stage. The `treesitter` tag is
# documented in docs/PATCHES.md.
FROM golang:1.25-alpine AS build
WORKDIR /src
RUN apk add --no-cache git gcc g++ musl-dev
# Build metadata — pinned via --build-arg so /version returns the real
# release tag + commit + ISO build time instead of dev/unknown. The CI
# release workflow passes these; local docker build falls back to
# dev/unknown/now so the build still succeeds.
ARG BUILD_VERSION=dev
ARG BUILD_COMMIT=unknown
ARG BUILD_TIME=unknown
COPY core/orchestrator/go.mod core/orchestrator/go.sum ./
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go mod download
COPY core/orchestrator/ ./
# Production builds opt in to the tree-sitter AST adapters so symbol-level
# patches resolve through real parsers instead of the no-op fallback. The
# tree-sitter Go bindings require CGO, so CGO_ENABLED=1 is mandatory here.
# ldflags inject build metadata into main.buildVersion/buildCommit/buildTime
# so /version, /healthz, and the startup banner stop returning "dev/unknown".
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=1 GOOS=linux go build -trimpath -tags treesitter \
      -ldflags="-s -w -linkmode external -extldflags \"-static\" \
        -X main.buildVersion=${BUILD_VERSION} \
        -X main.buildCommit=${BUILD_COMMIT} \
        -X main.buildTime=${BUILD_TIME}" \
      -o /out/orchestrator ./cmd/orchestrator

FROM alpine:3.20
RUN apk add --no-cache ca-certificates curl tzdata \
    && adduser -D -u 10001 iron \
    && mkdir -p /app/templates \
    && chown -R iron:iron /app
USER iron
WORKDIR /app
COPY --from=build /out/orchestrator /usr/local/bin/orchestrator
# Baked-in scaffold templates. Dev compose overlays this with a bind-mount
# so contributors can edit templates/ without rebuilding the image.
COPY --chown=iron:iron templates/ /app/templates/
ENV IRONFLYER_SCAFFOLD_ROOT=/app/templates
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD curl -fsS http://127.0.0.1:8080/livez || exit 1
ENTRYPOINT ["/usr/local/bin/orchestrator"]
