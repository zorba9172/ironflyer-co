# syntax=docker/dockerfile:1.7
# scrcpy-bridge — Go service that wraps scrcpy + Pion WebRTC. Built as
# a static-ish binary in the alpine stage and shipped on debian-slim
# because that's where the scrcpy + adb apt packages live.

# ---- build stage ----------------------------------------------------------
FROM golang:1.25-alpine AS build
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
WORKDIR /src

# Cache deps separately from sources for faster iterative builds.
COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN go build -trimpath -ldflags='-s -w' -o /out/scrcpy-bridge ./cmd/scrcpy-bridge

# ---- runtime stage --------------------------------------------------------
FROM debian:bookworm-slim
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update \
  && apt-get install -y --no-install-recommends \
       scrcpy \
       adb \
       ca-certificates \
       tini \
  && rm -rf /var/lib/apt/lists/*

COPY --from=build /out/scrcpy-bridge /scrcpy-bridge

ENV BRIDGE_PORT=9100 \
    ADB_SERVER=localhost:5037 \
    SCRCPY_PATH=/usr/bin/scrcpy

EXPOSE 9100
USER nobody
ENTRYPOINT ["/usr/bin/tini", "--", "/scrcpy-bridge"]
