# syntax=docker/dockerfile:1.7
#
# Runtime (workspace sandbox) image.
#
# IMPORTANT: when running with IRONFLYER_RUNTIME_DRIVER=docker the container
# must have access to a Docker socket so it can spawn sibling workspace
# containers. In compose/k8s mount /var/run/docker.sock and add the iron
# user to the docker group, OR use sysbox / Firecracker. Running as root
# is intentionally avoided — DinD via socket-passthrough is the supported
# pattern.
FROM golang:1.25-alpine AS build
WORKDIR /src
RUN apk add --no-cache git
COPY core/runtime/go.mod core/runtime/go.sum ./
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go mod download
COPY core/runtime/ ./
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' \
      -o /out/runtime ./cmd/runtime

# Final image keeps `git` (for workspace clones), `curl` (for HEALTHCHECK),
# and `docker-cli` so IRONFLYER_RUNTIME_DRIVER=docker can spawn sibling
# workspace containers through the mounted host socket.
FROM alpine:3.20
RUN apk add --no-cache git ca-certificates curl docker-cli tzdata \
    && adduser -D -u 10001 iron
USER iron
WORKDIR /home/iron
COPY --from=build /out/runtime /usr/local/bin/runtime
EXPOSE 8090
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD curl -fsS http://127.0.0.1:8090/healthz || exit 1
ENTRYPOINT ["/usr/local/bin/runtime"]
