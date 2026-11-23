# Makefile for Sambo - Share Management CLI for Linux and macOS

BINARY_NAME=sambo
VERSION := $(shell sed -n 's/^version = "\(.*\)"/\1/p' Cargo.toml | head -1)
BUILD_DIR=build
INSTALL_PATH=/usr/local/bin

# Linux binaries are linked statically against musl so one file runs on any
# distribution. They are built with `cross` (https://github.com/cross-rs/cross),
# which needs Docker or Podman; set CROSS=cargo if the targets and linkers are
# installed locally.
CROSS ?= cross
LINUX_TARGETS = amd64:x86_64-unknown-linux-musl arm64:aarch64-unknown-linux-musl arm:armv7-unknown-linux-musleabihf
# macOS binaries need a macOS host with both targets installed:
#   rustup target add x86_64-apple-darwin aarch64-apple-darwin
DARWIN_TARGETS = amd64:x86_64-apple-darwin arm64:aarch64-apple-darwin

.PHONY: all build build-linux build-darwin build-all release package-macos clean install uninstall test fmt lint help

all: build

# Build the binary
build:
	@echo "Building $(BINARY_NAME) $(VERSION)..."
	cargo build --release
	@mkdir -p $(BUILD_DIR)
	@cp target/release/$(BINARY_NAME) $(BUILD_DIR)/$(BINARY_NAME)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

build-linux:
	@mkdir -p $(BUILD_DIR)
	@set -e; for pair in $(LINUX_TARGETS); do \
		arch=$${pair%%:*}; target=$${pair#*:}; \
		echo "Building linux-$$arch ($$target)..."; \
		$(CROSS) build --release --target $$target; \
		cp target/$$target/release/$(BINARY_NAME) $(BUILD_DIR)/$(BINARY_NAME)-linux-$$arch; \
	done

build-darwin:
	@mkdir -p $(BUILD_DIR)
	@set -e; for pair in $(DARWIN_TARGETS); do \
		arch=$${pair%%:*}; target=$${pair#*:}; \
		echo "Building darwin-$$arch ($$target)..."; \
		cargo build --release --target $$target; \
		cp target/$$target/release/$(BINARY_NAME) $(BUILD_DIR)/$(BINARY_NAME)-darwin-$$arch; \
	done

# Build for multiple architectures
build-all: build-linux build-darwin
	@echo "All builds complete"

# Create macOS installer packages
package-macos: build-darwin
	@echo "Creating macOS installer packages..."
	@./scripts/build-pkg.sh $(VERSION)

# Create release packages with binaries and scripts
release: build-all
	@echo "Creating release packages..."
	@mkdir -p $(BUILD_DIR)/release
	@set -e; for platform in linux-amd64 linux-arm64 linux-arm darwin-amd64 darwin-arm64; do \
		name=$(BINARY_NAME)-$(VERSION)-$$platform; \
		mkdir -p $(BUILD_DIR)/release/$$name; \
		cp $(BUILD_DIR)/$(BINARY_NAME)-$$platform $(BUILD_DIR)/release/$$name/$(BINARY_NAME); \
		cp -r scripts README.md $(BUILD_DIR)/release/$$name/; \
		tar -czf $(BUILD_DIR)/release/$$name.tar.gz -C $(BUILD_DIR)/release $$name; \
		rm -rf $(BUILD_DIR)/release/$$name; \
	done
	@echo "Release packages created in $(BUILD_DIR)/release/"

# Install to system
install: build
	@echo "Installing $(BINARY_NAME) to $(INSTALL_PATH)..."
	@sudo install -m 755 $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "Installation complete. Run 'sudo $(BINARY_NAME)' to use."

# Uninstall from system
uninstall:
	@echo "Uninstalling $(BINARY_NAME)..."
	@sudo rm -f $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "Uninstall complete"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@cargo clean
	@echo "Clean complete"

# Run tests
test:
	cargo test

# Format code
fmt:
	cargo fmt

# Run linter
lint:
	cargo clippy --all-targets -- -D warnings

# Show help
help:
	@echo "Sambo - Share Management CLI for Linux and macOS"
	@echo ""
	@echo "Available targets:"
	@echo "  make build         - Build the binary (default)"
	@echo "  make build-linux   - Cross-compile static Linux binaries (needs cross)"
	@echo "  make build-darwin  - Build macOS binaries for Intel and Apple Silicon"
	@echo "  make build-all     - Both of the above"
	@echo "  make release       - Create release packages with scripts"
	@echo "  make package-macos - Create macOS installer packages (.pkg)"
	@echo "  make install       - Build and install to $(INSTALL_PATH)"
	@echo "  make uninstall     - Remove from system"
	@echo "  make clean         - Remove build artifacts"
	@echo "  make test          - Run tests"
	@echo "  make fmt           - Format code"
	@echo "  make lint          - Run clippy"
	@echo "  make help          - Show this help"
