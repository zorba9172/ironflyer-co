# Ironflyer CLI Makefile. The CLI is a standalone Go module — these
# targets only operate inside core/cli/.

BINARY    := ironflyer
PKG       := ./cmd/ironflyer
VERSION   := $(shell cat version.txt 2>/dev/null || echo v0.0.0-dev)
LDFLAGS   := -s -w -X 'ironflyer/core/cli/internal/commands.Version=$(VERSION)'
DIST      := dist

.PHONY: all
all: build

.PHONY: build
build:
	@mkdir -p $(DIST)
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST)/$(BINARY) $(PKG)

.PHONY: install
install:
	go install -trimpath -ldflags "$(LDFLAGS)" $(PKG)

.PHONY: vet
vet:
	go vet ./...

.PHONY: release
release:
	@mkdir -p $(DIST)
	@for os in darwin linux windows; do \
		for arch in amd64 arm64; do \
			ext=""; \
			if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
			out="$(DIST)/$(BINARY)-$(VERSION)-$$os-$$arch$$ext"; \
			echo "→ $$out"; \
			GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 \
				go build -trimpath -ldflags "$(LDFLAGS)" -o "$$out" $(PKG) || exit 1; \
		done; \
	done
	@cd $(DIST) && shasum -a 256 $(BINARY)-$(VERSION)-* > SHA256SUMS

.PHONY: clean
clean:
	rm -rf $(DIST)
