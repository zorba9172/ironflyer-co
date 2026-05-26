# Ironflyer orchestrator — build targets.
#
# Production binaries MUST be linked via `make build` so the /version
# endpoint reports the real commit + tag + build timestamp. Dev runs
# (`go run`, `go build`) leave the defaults from cmd/orchestrator/main.go
# intact ("dev" / "unknown"), which is the correct signal that the
# binary did not come out of the release pipeline.

BIN          ?= ./bin/orchestrator
PKG          := ./cmd/orchestrator
BUILD_VERSION := $(shell git describe --tags --always 2>/dev/null || echo dev)
BUILD_COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_TIME    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

GO_LDFLAGS = -X main.buildVersion=$(BUILD_VERSION) \
             -X main.buildCommit=$(BUILD_COMMIT) \
             -X main.buildTime=$(BUILD_TIME)

.PHONY: build vet run clean

build:
	@mkdir -p $(dir $(BIN))
	go build -trimpath -ldflags '$(GO_LDFLAGS)' -o $(BIN) $(PKG)

vet:
	go vet ./...

run:
	go run -ldflags '$(GO_LDFLAGS)' $(PKG)

clean:
	rm -rf $(dir $(BIN))
