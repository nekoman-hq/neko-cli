VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -X github.com/nekoman-hq/neko-cli/internal/version.Version=$(VERSION) \
           -X github.com/nekoman-hq/neko-cli/internal/version.Commit=$(COMMIT) \
           -X github.com/nekoman-hq/neko-cli/internal/version.Date=$(DATE) \
           -X github.com/nekoman-hq/neko-cli/internal/version.BuiltBy=make

.PHONY: build
build:
	go build -ldflags "$(LDFLAGS)" -o neko

.PHONY: install
install:
	go install -ldflags "$(LDFLAGS)"