.DEFAULT_GOAL := help

# ============================================================================
# Version Management
# ============================================================================
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -X github.com/nekoman-hq/neko-cli/pkg/version.Version=$(VERSION) \
           -X github.com/nekoman-hq/neko-cli/pkg/version.Commit=$(COMMIT) \
           -X github.com/nekoman-hq/neko-cli/pkg/version.Date=$(DATE) \
           -X github.com/nekoman-hq/neko-cli/pkg/version.BuiltBy=make

# ============================================================================
# Plugin Configuration
# ============================================================================
RELEASE_CONFIG := .neko/release.config.json
RELEASE_STATE := .neko/release.state.json
PLUGIN_DIR := plugin
PLUGIN_INSTALL_DIR := $(HOME)/.neko/plugins

# Dynamically read plugin directories from V2 release units.
PLUGINS := $(shell test -f "$(RELEASE_CONFIG)" && jq -r '.units[].id | select(startswith("plugin-")) | sub("^plugin-"; "")' $(RELEASE_CONFIG) 2>/dev/null || echo "")

# ============================================================================
# Colors for help output
# ============================================================================
COLOR_RESET   := \033[0m
COLOR_BOLD    := \033[1m
COLOR_GREEN   := \033[32m
COLOR_YELLOW  := \033[33m
COLOR_BLUE    := \033[34m
COLOR_CYAN    := \033[36m

# ============================================================================
# Main Targets
# ============================================================================

.PHONY: help
help: ## Show this help message
	@echo "$(COLOR_BOLD)$(COLOR_CYAN)Neko CLI - Makefile Commands$(COLOR_RESET)"
	@echo ""
	@echo "$(COLOR_BOLD)Usage:$(COLOR_RESET)"
	@echo "  make $(COLOR_GREEN)<target>$(COLOR_RESET)"
	@echo ""
	@echo "$(COLOR_BOLD)Available Targets:$(COLOR_RESET)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  $(COLOR_GREEN)%-20s$(COLOR_RESET) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(COLOR_BOLD)Plugin Targets:$(COLOR_RESET)"
	@echo "  $(COLOR_GREEN)plugin-%-build$(COLOR_RESET)       Build specific plugin (e.g., make plugin-release-build)"
	@echo "  $(COLOR_GREEN)plugin-%-install$(COLOR_RESET)     Install specific plugin"
	@echo "  $(COLOR_GREEN)plugin-%-clean$(COLOR_RESET)       Clean specific plugin"
	@echo "  $(COLOR_GREEN)plugin-%-test$(COLOR_RESET)        Test specific plugin"
	@echo ""
	@echo "$(COLOR_BOLD)Note:$(COLOR_RESET) Plugins are read from $(COLOR_CYAN)$(RELEASE_CONFIG)$(COLOR_RESET)"
	@echo "      Use $(COLOR_GREEN)make check-plugin-config$(COLOR_RESET) to validate configuration"
	@echo ""
	@$(MAKE) --no-print-directory versions

# ============================================================================
# Build Targets
# ============================================================================

.PHONY: build
build: ## Build the neko CLI binary
	@echo "$(COLOR_BLUE)Building neko CLI...$(COLOR_RESET)"
	@go build -ldflags "$(LDFLAGS)" -o neko
	@echo "$(COLOR_GREEN)✓ Build complete$(COLOR_RESET)"

.PHONY: install
install: ## Install the neko CLI binary
	@echo "$(COLOR_BLUE)Installing neko CLI...$(COLOR_RESET)"
	@go install -ldflags "$(LDFLAGS)"
	@echo "$(COLOR_GREEN)✓ Installation complete$(COLOR_RESET)"

.PHONY: all
all: build install-plugins ## Build CLI and install all plugins
	@echo "$(COLOR_GREEN)✓ Full build complete$(COLOR_RESET)"

# ============================================================================
# Plugin Management
# ============================================================================

.PHONY: install-plugins
install-plugins: ## Install all plugins (compile binaries and copy to ~/.neko/plugins/<plugin_name>/)
	@if [ ! -f "$(RELEASE_CONFIG)" ]; then \
		echo "$(COLOR_YELLOW)⚠ $(RELEASE_CONFIG) not found$(COLOR_RESET)"; \
		exit 1; \
	fi
	@if [ ! -f "$(RELEASE_STATE)" ]; then \
		echo "$(COLOR_YELLOW)⚠ $(RELEASE_STATE) not found$(COLOR_RESET)"; \
		exit 1; \
	fi
	@if [ -z "$(PLUGINS)" ]; then \
		echo "$(COLOR_YELLOW)⚠ No plugin units defined in $(RELEASE_CONFIG)$(COLOR_RESET)"; \
		exit 1; \
	fi
	@echo "$(COLOR_BLUE)Installing all plugins...$(COLOR_RESET)"
	@for plugin in $(PLUGINS); do \
		if [ -d "$(PLUGIN_DIR)/$$plugin" ]; then \
			echo "$(COLOR_CYAN)  → Installing plugin: $$plugin$(COLOR_RESET)"; \
			$(MAKE) --no-print-directory plugin-$$plugin-install || exit 1; \
		else \
			echo "$(COLOR_YELLOW)  ⚠ Plugin directory not found: $(PLUGIN_DIR)/$$plugin$(COLOR_RESET)"; \
		fi; \
	done
	@echo "$(COLOR_GREEN)✓ All plugins installed$(COLOR_RESET)"

.PHONY: build-plugins
build-plugins: ## Build all plugins
	@echo "$(COLOR_BLUE)Building all plugins...$(COLOR_RESET)"
	@for plugin in $(PLUGINS); do \
		if [ -d "$(PLUGIN_DIR)/$$plugin" ]; then \
			echo "$(COLOR_CYAN)  → Building plugin: $$plugin$(COLOR_RESET)"; \
			cd $(PLUGIN_DIR)/$$plugin && $(MAKE) build || exit 1; \
			cd ../..; \
		fi; \
	done
	@echo "$(COLOR_GREEN)✓ All plugins built$(COLOR_RESET)"

.PHONY: clean-plugins
clean-plugins: ## Clean all plugin builds
	@echo "$(COLOR_BLUE)Cleaning all plugins...$(COLOR_RESET)"
	@for plugin in $(PLUGINS); do \
		if [ -d "$(PLUGIN_DIR)/$$plugin" ]; then \
			echo "$(COLOR_CYAN)  → Cleaning plugin: $$plugin$(COLOR_RESET)"; \
			cd $(PLUGIN_DIR)/$$plugin && $(MAKE) clean || true; \
			cd ../..; \
		fi; \
	done
	@echo "$(COLOR_GREEN)✓ All plugins cleaned$(COLOR_RESET)"

.PHONY: test-plugins
test-plugins: ## Run tests for all plugins
	@echo "$(COLOR_BLUE)Testing all plugins...$(COLOR_RESET)"
	@for plugin in $(PLUGINS); do \
		if [ -d "$(PLUGIN_DIR)/$$plugin" ]; then \
			echo "$(COLOR_CYAN)  → Testing plugin: $$plugin$(COLOR_RESET)"; \
			cd $(PLUGIN_DIR)/$$plugin && $(MAKE) test || exit 1; \
			cd ../..; \
		fi; \
	done
	@echo "$(COLOR_GREEN)✓ All plugin tests passed$(COLOR_RESET)"

# ============================================================================
# Individual Plugin Targets
# ============================================================================

.PHONY: plugin-%-build
plugin-%-build: ## Build a specific plugin
	@if [ -d "$(PLUGIN_DIR)/$*" ]; then \
		echo "$(COLOR_BLUE)Building plugin: $*$(COLOR_RESET)"; \
		cd $(PLUGIN_DIR)/$* && $(MAKE) build; \
		echo "$(COLOR_GREEN)✓ Plugin $* built$(COLOR_RESET)"; \
	else \
		echo "$(COLOR_YELLOW)⚠ Plugin $* not found$(COLOR_RESET)"; \
		exit 1; \
	fi

.PHONY: plugin-%-install
plugin-%-install: ## Install a specific plugin (compile binary and copy to ~/.neko/plugins/<plugin_name>/)
	@if [ -d "$(PLUGIN_DIR)/$*" ]; then \
		echo "$(COLOR_BLUE)Installing plugin: $*$(COLOR_RESET)"; \
		\
		echo "$(COLOR_CYAN)  → Building plugin binary...$(COLOR_RESET)"; \
		cd $(PLUGIN_DIR)/$* && $(MAKE) build || exit 1; \
		cd ../..; \
		\
		echo "$(COLOR_CYAN)  → Creating plugin directory...$(COLOR_RESET)"; \
		mkdir -p $(PLUGIN_INSTALL_DIR)/$*; \
		\
		echo "$(COLOR_CYAN)  → Copying plugin binary...$(COLOR_RESET)"; \
		if [ -f "$(PLUGIN_DIR)/$*/plugin-$*" ]; then \
			cp $(PLUGIN_DIR)/$*/plugin-$* $(PLUGIN_INSTALL_DIR)/$*/; \
			chmod +x $(PLUGIN_INSTALL_DIR)/$*/plugin-$*; \
		else \
			echo "$(COLOR_YELLOW)⚠ Binary plugin-$* not found$(COLOR_RESET)"; \
			exit 1; \
		fi; \
		\
		echo "$(COLOR_CYAN)  → Copying plugin manifest...$(COLOR_RESET)"; \
		if [ -f "$(PLUGIN_DIR)/$*/manifest.json" ]; then \
			cp $(PLUGIN_DIR)/$*/manifest.json $(PLUGIN_INSTALL_DIR)/$*/; \
		else \
			echo "$(COLOR_YELLOW)⚠ manifest.json not found for plugin $*$(COLOR_RESET)"; \
		fi; \
		\
		echo "$(COLOR_GREEN)✓ Plugin $* installed to $(PLUGIN_INSTALL_DIR)/$*$(COLOR_RESET)"; \
	else \
		echo "$(COLOR_YELLOW)⚠ Plugin $* not found$(COLOR_RESET)"; \
		exit 1; \
	fi

.PHONY: plugin-%-clean
plugin-%-clean: ## Clean a specific plugin
	@if [ -d "$(PLUGIN_DIR)/$*" ]; then \
		echo "$(COLOR_BLUE)Cleaning plugin: $*$(COLOR_RESET)"; \
		cd $(PLUGIN_DIR)/$* && $(MAKE) clean || true; \
		echo "$(COLOR_GREEN)✓ Plugin $* cleaned$(COLOR_RESET)"; \
	else \
		echo "$(COLOR_YELLOW)⚠ Plugin $* not found$(COLOR_RESET)"; \
		exit 1; \
	fi

.PHONY: plugin-%-uninstall
plugin-%-uninstall: ## Uninstall a specific plugin from ~/.neko/plugins/<plugin_name>/
	@echo "$(COLOR_BLUE)Uninstalling plugin: $*$(COLOR_RESET)"
	@if [ -d "$(PLUGIN_INSTALL_DIR)/$*" ]; then \
		rm -rf $(PLUGIN_INSTALL_DIR)/$*; \
		echo "$(COLOR_GREEN)✓ Plugin $* uninstalled$(COLOR_RESET)"; \
	else \
		echo "$(COLOR_YELLOW)⚠ Plugin $* not installed$(COLOR_RESET)"; \
	fi

.PHONY: plugin-%-test
plugin-%-test: ## Test a specific plugin
	@if [ -d "$(PLUGIN_DIR)/$*" ]; then \
		echo "$(COLOR_BLUE)Testing plugin: $*$(COLOR_RESET)"; \
		cd $(PLUGIN_DIR)/$* && $(MAKE) test; \
		echo "$(COLOR_GREEN)✓ Plugin $* tests passed$(COLOR_RESET)"; \
	else \
		echo "$(COLOR_YELLOW)⚠ Plugin $* not found$(COLOR_RESET)"; \
		exit 1; \
	fi

.PHONY: list-installed-plugins
list-installed-plugins: ## List all installed plugins
	@echo "$(COLOR_BOLD)Installed Plugins:$(COLOR_RESET)"
	@if [ -d "$(PLUGIN_INSTALL_DIR)" ]; then \
		for plugin_path in $(PLUGIN_INSTALL_DIR)/*; do \
			if [ -d "$$plugin_path" ]; then \
				plugin=$$(basename $$plugin_path); \
				if [ -f "$$plugin_path/manifest.json" ]; then \
					version=$$(jq -r '.version // "unknown"' $$plugin_path/manifest.json 2>/dev/null || echo "unknown"); \
					printf "  $(COLOR_GREEN)%-15s$(COLOR_RESET) v%-10s $(COLOR_CYAN)%s$(COLOR_RESET)\n" \
						"$$plugin" "$$version" "$$plugin_path"; \
				else \
					printf "  $(COLOR_GREEN)%-15s$(COLOR_RESET) $(COLOR_YELLOW)%-11s$(COLOR_RESET) $(COLOR_CYAN)%s$(COLOR_RESET)\n" \
						"$$plugin" "(no manifest)" "$$plugin_path"; \
				fi; \
			fi; \
		done; \
	else \
		echo "  $(COLOR_YELLOW)No plugins installed$(COLOR_RESET)"; \
	fi

# ============================================================================
# Manifest Management
# ============================================================================

.PHONY: update-manifests
update-manifests: ## Update all plugin manifests with versions from V2 release state
	@echo "$(COLOR_BLUE)Updating all plugin manifests...$(COLOR_RESET)"
	@if [ ! -f "$(RELEASE_CONFIG)" ]; then \
		echo "$(COLOR_YELLOW)⚠ $(RELEASE_CONFIG) not found$(COLOR_RESET)"; \
		exit 1; \
	fi
	@if [ ! -f "$(RELEASE_STATE)" ]; then \
		echo "$(COLOR_YELLOW)⚠ $(RELEASE_STATE) not found$(COLOR_RESET)"; \
		exit 1; \
	fi
	@for plugin in $(PLUGINS); do \
		if [ -d "$(PLUGIN_DIR)/$$plugin" ]; then \
			unit="plugin-$$plugin"; \
			version=$$(jq -r --arg unit "$$unit" '.units[$$unit].version // "unknown"' $(RELEASE_STATE)); \
			if [ "$$version" != "unknown" ] && [ "$$version" != "null" ]; then \
				echo "$(COLOR_CYAN)  → Updating $$plugin manifest to version $$version$(COLOR_RESET)"; \
				cd $(PLUGIN_DIR)/$$plugin && $(MAKE) update-manifest VERSION=$$version || exit 1; \
				cd ../..; \
			else \
				echo "$(COLOR_YELLOW)  ⚠ No version found for $$unit in $(RELEASE_STATE)$(COLOR_RESET)"; \
			fi; \
		fi; \
	done
	@echo "$(COLOR_GREEN)✓ All manifests updated$(COLOR_RESET)"

# ============================================================================
# Testing
# ============================================================================

.PHONY: test
test: ## Run tests for the main CLI
	@echo "$(COLOR_BLUE)Running CLI tests...$(COLOR_RESET)"
	@go test ./...
	@echo "$(COLOR_GREEN)✓ Tests passed$(COLOR_RESET)"

.PHONY: test-all
test-all: test test-plugins ## Run all tests (CLI + plugins)
	@echo "$(COLOR_GREEN)✓ All tests completed$(COLOR_RESET)"

# ============================================================================
# Cleanup
# ============================================================================

.PHONY: clean
clean: ## Clean all build artifacts
	@echo "$(COLOR_BLUE)Cleaning build artifacts...$(COLOR_RESET)"
	@rm -f neko
	@$(MAKE) --no-print-directory clean-plugins
	@echo "$(COLOR_GREEN)✓ Cleanup complete$(COLOR_RESET)"

.PHONY: uninstall-plugins
uninstall-plugins: ## Uninstall all plugins from ~/.neko/plugins/
	@echo "$(COLOR_BLUE)Uninstalling all plugins...$(COLOR_RESET)"
	@if [ -d "$(PLUGIN_INSTALL_DIR)" ]; then \
		rm -rf $(PLUGIN_INSTALL_DIR); \
		echo "$(COLOR_GREEN)✓ All plugins uninstalled$(COLOR_RESET)"; \
	else \
		echo "$(COLOR_YELLOW)⚠ No plugins installed$(COLOR_RESET)"; \
	fi

# ============================================================================
# Version Information
# ============================================================================

.PHONY: versions
versions: ## Display version information for CLI and all plugins
	@echo "$(COLOR_BOLD)Version Information:$(COLOR_RESET)"
	@echo "  CLI:            $(COLOR_CYAN)$(VERSION)$(COLOR_RESET)"
	@if [ -f "$(RELEASE_STATE)" ]; then \
		for plugin in $(PLUGINS); do \
			unit="plugin-$$plugin"; \
			version=$$(jq -r --arg unit "$$unit" '.units[$$unit].version // "unknown"' $(RELEASE_STATE) 2>/dev/null); \
			printf "  Plugin %-8s $(COLOR_CYAN)%s$(COLOR_RESET)\n" "$$plugin:" "$$version"; \
		done; \
	else \
		echo "  $(COLOR_YELLOW)⚠ Release state not found$(COLOR_RESET)"; \
	fi

# ============================================================================
# Release Support
# ============================================================================

.PHONY: release-patch
release-patch: ## Create a patch release (x.y.Z)
	@echo "$(COLOR_BLUE)Creating patch release...$(COLOR_RESET)"
	@./neko release patch --describe --verbose
	@echo "$(COLOR_GREEN)✓ Patch release complete$(COLOR_RESET)"

.PHONY: release-minor
release-minor: ## Create a minor release (x.Y.0)
	@echo "$(COLOR_BLUE)Creating minor release...$(COLOR_RESET)"
	@./neko release minor --describe --verbose
	@echo "$(COLOR_GREEN)✓ Minor release complete$(COLOR_RESET)"

.PHONY: release-major
release-major: ## Create a major release (X.0.0)
	@echo "$(COLOR_BLUE)Creating major release...$(COLOR_RESET)"
	@./neko release major --describe --verbose
	@echo "$(COLOR_GREEN)✓ Major release complete$(COLOR_RESET)"

.PHONY: release-validate
release-validate: ## Validate release configuration
	@echo "$(COLOR_BLUE)Validating release configuration...$(COLOR_RESET)"
	@./neko release validate
	@echo "$(COLOR_GREEN)✓ Validation complete$(COLOR_RESET)"

.PHONY: release-history
release-history: ## Show release history
	@./neko release history

.PHONY: release-contributors
release-contributors: ## Show repository contributors
	@./neko release contributors

.PHONY: release
release: ## Interactive release (prompts for type)
	@echo "$(COLOR_BOLD)$(COLOR_CYAN)Select release type:$(COLOR_RESET)"
	@echo "  $(COLOR_GREEN)1$(COLOR_RESET) - Patch (x.y.Z)"
	@echo "  $(COLOR_GREEN)2$(COLOR_RESET) - Minor (x.Y.0)"
	@echo "  $(COLOR_GREEN)3$(COLOR_RESET) - Major (X.0.0)"
	@read -p "Enter choice [1-3]: " choice; \
	case $$choice in \
		1) $(MAKE) release-patch --describe --verbose ;; \
		2) $(MAKE) release-minor --describe --verbose ;; \
		3) $(MAKE) release-major --describe --verbose ;; \
		*) echo "$(COLOR_YELLOW)⚠ Invalid choice$(COLOR_RESET)"; exit 1 ;; \
	esac

# ============================================================================
# Development
# ============================================================================

.PHONY: dev
dev: clean all ## Clean rebuild for development
	@echo "$(COLOR_GREEN)✓ Development build complete$(COLOR_RESET)"

.PHONY: lint
lint: ## Run linters
	@echo "$(COLOR_BLUE)Running linters...$(COLOR_RESET)"
	@golangci-lint run ./...
	@echo "$(COLOR_GREEN)✓ Linting complete$(COLOR_RESET)"

.PHONY: fmt
fmt: ## Format code
	@echo "$(COLOR_BLUE)Formatting code...$(COLOR_RESET)"
	@go fmt ./...
	@echo "$(COLOR_GREEN)✓ Formatting complete$(COLOR_RESET)"

.PHONY: deps
deps: ## Download dependencies
	@echo "$(COLOR_BLUE)Downloading dependencies...$(COLOR_RESET)"
	@go mod download
	@go mod tidy
	@echo "$(COLOR_GREEN)✓ Dependencies updated$(COLOR_RESET)"

# ============================================================================
# Verification
# ============================================================================

.PHONY: check-plugin-config
check-plugin-config: ## Verify plugin configuration file
	@echo "$(COLOR_BLUE)Checking plugin configuration...$(COLOR_RESET)"
	@if [ ! -f "$(RELEASE_CONFIG)" ]; then \
		echo "$(COLOR_YELLOW)⚠ $(RELEASE_CONFIG) not found$(COLOR_RESET)"; \
		exit 1; \
	fi
	@if [ ! -f "$(RELEASE_STATE)" ]; then \
		echo "$(COLOR_YELLOW)⚠ $(RELEASE_STATE) not found$(COLOR_RESET)"; \
		exit 1; \
	fi
	@echo "$(COLOR_CYAN)Release configuration file: $(RELEASE_CONFIG)$(COLOR_RESET)"
	@echo "$(COLOR_CYAN)Release state file: $(RELEASE_STATE)$(COLOR_RESET)"
	@echo "$(COLOR_CYAN)Configured plugins:$(COLOR_RESET)"
	@for plugin in $(PLUGINS); do \
		unit="plugin-$$plugin"; \
		version=$$(jq -r --arg unit "$$unit" '.units[$$unit].version // "unknown"' $(RELEASE_STATE) 2>/dev/null); \
		if [ -d "$(PLUGIN_DIR)/$$plugin" ]; then \
			printf "  $(COLOR_GREEN)✓$(COLOR_RESET) %-15s v%-10s $(COLOR_GREEN)(directory exists)$(COLOR_RESET)\n" "$$plugin" "$$version"; \
		else \
			printf "  $(COLOR_YELLOW)⚠$(COLOR_RESET) %-15s v%-10s $(COLOR_YELLOW)(directory missing)$(COLOR_RESET)\n" "$$plugin" "$$version"; \
		fi; \
	done
	@echo "$(COLOR_GREEN)✓ Configuration check complete$(COLOR_RESET)"

.PHONY: verify
verify: fmt lint test-all ## Run all verification steps (fmt, lint, test)
	@echo "$(COLOR_GREEN)✓ All verification steps passed$(COLOR_RESET)"

.PHONY: check-tools
check-tools: ## Verify required tools are installed
	@echo "$(COLOR_BLUE)Checking required tools...$(COLOR_RESET)"
	@command -v go >/dev/null 2>&1 || { echo "$(COLOR_YELLOW)⚠ go is not installed$(COLOR_RESET)"; exit 1; }
	@command -v jq >/dev/null 2>&1 || { echo "$(COLOR_YELLOW)⚠ jq is not installed$(COLOR_RESET)"; exit 1; }
	@command -v git >/dev/null 2>&1 || { echo "$(COLOR_YELLOW)⚠ git is not installed$(COLOR_RESET)"; exit 1; }
	@echo "$(COLOR_GREEN)✓ All required tools found$(COLOR_RESET)"
