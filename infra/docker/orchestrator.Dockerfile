# syntax=docker/dockerfile:1.7
#
# Orchestrator image. Multi-stage: alpine + Go build, distroless-style
# minimal runtime (still alpine for /healthz curl + zoneinfo). Runs as a
# non-root user.
#
# Templates are baked into the image at /app/templates so the scaffold
# step works without a sidecar PVC; dev iterations can still bind-mount
# the repo's templates/ via docker-compose.dev.yml.
FROM golang:1.25-alpine AS build
WORKDIR /src
RUN apk add --no-cache git
COPY apps/orchestrator/go.mod apps/orchestrator/go.sum* ./
RUN go mod download || true
COPY apps/orchestrator/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' \
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
HEALTHCHECK --interval=15s --timeout=3s --start-period=10s --retries=3 \
    CMD curl -fsS http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["/usr/local/bin/orchestrator"]
