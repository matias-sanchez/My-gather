# My-gather build targets.
#
# Honours Constitution Principle I (Single Static Binary): CGO_ENABLED=0
# for every build; cross-compiles for the four supported targets.

# Force static builds everywhere this Makefile runs. Any stray cgo import
# therefore fails locally too, not just in CI cross-compile.
export CGO_ENABLED := 0

GO ?= go
MODULE := github.com/matias-sanchez/My-gather
BIN_NAME := my-gather
DIST := dist

# Version metadata injected via -ldflags at link time.
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "v0.0.0-dev")
COMMIT   ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILDAT  ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS  := -s -w \
            -X main.version=$(VERSION) \
            -X main.commit=$(COMMIT) \
            -X main.builtAt=$(BUILDAT)

TARGETS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64

# Prefer sha256sum (GNU coreutils, Linux CI default); fall back to
# `shasum -a 256` on macOS where sha256sum is usually absent.
SHA256 := $(shell command -v sha256sum 2>/dev/null || echo "shasum -a 256")

.PHONY: all build test vet lint release clean help

help:
	@echo "Targets:"
	@echo "  build     Build local binary into ./bin/$(BIN_NAME)"
	@echo "  test      Run go test ./..."
	@echo "  vet       Run go vet ./..."
	@echo "  lint      Run gofmt check + go vet"
	@echo "  release   Cross-compile + package tarballs + SHA256SUMS into $(DIST)/"
	@echo "  clean     Remove build artefacts"

all: vet test build

build:
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BIN_NAME) ./cmd/$(BIN_NAME)

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

lint:
	@out=$$(gofmt -l .); \
	  if [ -n "$$out" ]; then \
	    echo "Files need formatting -- run 'gofmt -w .':"; \
	    echo "$$out"; \
	    exit 1; \
	  fi
	$(GO) vet ./...

# release: cross-compile for every target, bundle each into a tarball,
# and emit a single SHA256SUMS file beside them. Does NOT depend on
# `clean` so the local ./bin/ binary survives a release build.
release: $(DIST)
	@rm -f $(DIST)/my-gather_*.tar.gz $(DIST)/SHA256SUMS
	@for target in $(TARGETS); do \
	  os=$${target%%/*}; arch=$${target##*/}; \
	  stage="$(DIST)/my-gather_$(VERSION)_$${os}_$${arch}"; \
	  echo "=> $$stage"; \
	  mkdir -p "$$stage"; \
	  GOOS=$$os GOARCH=$$arch \
	    $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o "$$stage/$(BIN_NAME)" ./cmd/$(BIN_NAME) \
	    || exit 1; \
	  cp README.md CHANGELOG.md "$$stage/" 2>/dev/null || true; \
	  ( cd $(DIST) && tar -czf "my-gather_$(VERSION)_$${os}_$${arch}.tar.gz" "my-gather_$(VERSION)_$${os}_$${arch}" ) || exit 1; \
	  rm -rf "$$stage"; \
	done
	@( cd $(DIST) && $(SHA256) my-gather_*.tar.gz > SHA256SUMS )
	@echo "Release artifacts in $(DIST)/:"
	@ls -lh $(DIST)/my-gather_*.tar.gz $(DIST)/SHA256SUMS

$(DIST):
	@mkdir -p $(DIST)

clean:
	rm -rf bin $(DIST)
