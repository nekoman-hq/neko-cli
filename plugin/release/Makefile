PLUGIN_NAME := release
BINARY := plugin-$(PLUGIN_NAME)
INSTALL_DIR := $(HOME)/.neko/plugins/$(PLUGIN_NAME)

# Read metadata from root .plugin.release.neko.json
VERSION ?= $(shell jq -r '.plugins.release' ../../.plugin.release.neko.json 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -X github.com/nekoman-hq/neko-cli/plugin/release/pkg/metadata.Version=$(VERSION) \
           -X github.com/nekoman-hq/neko-cli/plugin/release/pkg/metadata.GitCommit=$(COMMIT) \
           -X github.com/nekoman-hq/neko-cli/plugin/release/pkg/metadata.BuildDate=$(DATE)

.PHONY: build install clean update-manifest

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) main.go

update-manifest:
	@jq '.version = "$(VERSION)"' manifest.json > manifest.json.tmp && mv manifest.json.tmp manifest.json

install: update-manifest build
	mkdir -p $(INSTALL_DIR)
	cp $(BINARY) $(INSTALL_DIR)/
	cp manifest.json $(INSTALL_DIR)/

clean:
	rm -f $(BINARY)
	rm -rf $(INSTALL_DIR)
