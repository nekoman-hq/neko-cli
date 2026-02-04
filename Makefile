VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -X github.com/nekoman-hq/neko-cli/pkg/version.Version=$(VERSION) \
           -X github.com/nekoman-hq/neko-cli/pkg/version.Commit=$(COMMIT) \
           -X github.com/nekoman-hq/neko-cli/pkg/version.Date=$(DATE) \
           -X github.com/nekoman-hq/neko-cli/pkg/version.BuiltBy=make

.PHONY: build install clean install-plugins test versions release release-snapshot

build:
	go build -ldflags "$(LDFLAGS)" -o neko

install:
	go install -ldflags "$(LDFLAGS)"

install-plugins:
	cd plugin/release && $(MAKE) install

all: build install-plugins

clean:
	rm -f neko
	cd plugin/release && $(MAKE) clean || true
	cd plugin/core && $(MAKE) clean || true

test:
	go test ./...

versions:
	@echo "CLI:            $(VERSION)"
	@echo "Plugin Release: $$(jq -r '.plugins.release' .plugin.release.neko.json)"

release:
	PLUGIN_RELEASE_VERSION=$$(jq -r '.plugins.release' .plugin.release.neko.json) goreleaser release

release-snapshot:
	PLUGIN_RELEASE_VERSION=$$(jq -r '.plugins.release' .plugin.release.neko.json) goreleaser release --snapshot --clean
