# My-gather build targets.
#
# Honours Constitution Principle I (Single Static Binary): CGO_ENABLED=0
# for every build; cross-compiles for the four supported targets.

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

.PHONY: all build test vet lint release clean help

help:
	@echo "Targets:"
	@echo "  build     Build local binary into ./bin/$(BIN_NAME)"
	@echo "  test      Run go test ./..."
	@echo "  vet       Run go vet ./..."
	@echo "  release   Cross-compile for $(TARGETS) into $(DIST)/"
	@echo "  clean     Remove build artefacts"

all: vet test build

build:
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BIN_NAME) ./cmd/$(BIN_NAME)

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

release: clean
	@mkdir -p $(DIST)
	@for target in $(TARGETS); do \
	  os=$${target%%/*}; arch=$${target##*/}; \
	  out=$(DIST)/$(BIN_NAME)-$$os-$$arch; \
	  echo "=> $$out"; \
	  CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch \
	    $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $$out ./cmd/$(BIN_NAME) \
	    || exit 1; \
	done

clean:
	rm -rf bin $(DIST)
