.DEFAULT_GOAL := help

# ============================================================================
# Plugin Configuration
# ============================================================================
PLUGIN_NAME := release
BINARY := plugin-$(PLUGIN_NAME)
INSTALL_DIR := $(HOME)/.neko/plugins/$(PLUGIN_NAME)

# ============================================================================
# Version Management
# ============================================================================
# Read metadata from root V2 release state.
VERSION ?= $(shell jq -r '.units["plugin-release"].version' ../../.neko/release.state.json 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -X github.com/nekoman-hq/neko-cli/plugin/release/pkg/metadata.Version=$(VERSION) \
           -X github.com/nekoman-hq/neko-cli/plugin/release/pkg/metadata.GitCommit=$(COMMIT) \
           -X github.com/nekoman-hq/neko-cli/plugin/release/pkg/metadata.BuildDate=$(DATE)

# ============================================================================
# Colors
# ============================================================================
COLOR_RESET  := \033[0m
COLOR_BOLD   := \033[1m
COLOR_GREEN  := \033[32m
COLOR_YELLOW := \033[33m
COLOR_BLUE   := \033[34m
COLOR_CYAN   := \033[36m

# ============================================================================
# Targets
# ============================================================================

.PHONY: help
help: ## Show this help message
	@echo "$(COLOR_BOLD)$(COLOR_CYAN)Plugin: $(PLUGIN_NAME)$(COLOR_RESET)"
	@echo ""
	@echo "$(COLOR_BOLD)Available Targets:$(COLOR_RESET)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  $(COLOR_GREEN)%-20s$(COLOR_RESET) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(COLOR_BOLD)Current Version:$(COLOR_RESET) $(COLOR_CYAN)$(VERSION)$(COLOR_RESET)"

.PHONY: build
build: ## Build the plugin binary
	@echo "$(COLOR_BLUE)Building $(PLUGIN_NAME) plugin...$(COLOR_RESET)"
	@go build -ldflags "$(LDFLAGS)" -o $(BINARY) main.go
	@echo "$(COLOR_GREEN)✓ Build complete: $(BINARY)$(COLOR_RESET)"

.PHONY: update-manifest
update-manifest: ## Update manifest.json with current version
	@echo "$(COLOR_BLUE)Updating manifest.json to version $(VERSION)...$(COLOR_RESET)"
	@jq '.version = "$(VERSION)"' manifest.json > manifest.json.tmp && mv manifest.json.tmp manifest.json
	@echo "$(COLOR_GREEN)✓ Manifest updated$(COLOR_RESET)"

.PHONY: install
install: update-manifest build ## Install the plugin
	@echo "$(COLOR_BLUE)Installing $(PLUGIN_NAME) plugin...$(COLOR_RESET)"
	@mkdir -p $(INSTALL_DIR)
	@cp $(BINARY) $(INSTALL_DIR)/
	@cp manifest.json $(INSTALL_DIR)/
	@echo "$(COLOR_GREEN)✓ Plugin installed to $(INSTALL_DIR)$(COLOR_RESET)"

.PHONY: clean
clean: ## Remove build artifacts
	@echo "$(COLOR_BLUE)Cleaning $(PLUGIN_NAME) plugin...$(COLOR_RESET)"
	@rm -f $(BINARY)
	@rm -rf $(INSTALL_DIR)
	@echo "$(COLOR_GREEN)✓ Cleanup complete$(COLOR_RESET)"

.PHONY: test
test: ## Run plugin tests
	@echo "$(COLOR_BLUE)Running tests for $(PLUGIN_NAME) plugin...$(COLOR_RESET)"
	@go test ./...
	@echo "$(COLOR_GREEN)✓ Tests passed$(COLOR_RESET)"

.PHONY: lint
lint: ## Run linter
	@echo "$(COLOR_BLUE)Linting $(PLUGIN_NAME) plugin...$(COLOR_RESET)"
	@golangci-lint run ./...
	@echo "$(COLOR_GREEN)✓ Linting complete$(COLOR_RESET)"

.PHONY: fmt
fmt: ## Format code
	@echo "$(COLOR_BLUE)Formatting code...$(COLOR_RESET)"
	@go fmt ./...
	@echo "$(COLOR_GREEN)✓ Formatting complete$(COLOR_RESET)"
