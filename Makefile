# Makefile for Sambo - Share Management CLI for Linux and macOS

BINARY_NAME=sambo
VERSION=1.6.1
BUILD_DIR=build
INSTALL_PATH=/usr/local/bin

# Build flags
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: all build build-all release package-macos clean install uninstall test test-cover test-race fmt lint help

all: build

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) .
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Build for multiple architectures
build-all:
	@echo "Building for multiple architectures..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 .
	GOOS=linux GOARCH=arm go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm .
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 .
	@echo "All builds complete"

# Create macOS installer packages
package-macos: build-all
	@echo "Creating macOS installer packages..."
	@./scripts/build-pkg.sh $(VERSION)


# Create release packages with binaries and scripts
release: build-all
	@echo "Creating release packages..."
	@mkdir -p $(BUILD_DIR)/release
	@# Linux AMD64
	@mkdir -p $(BUILD_DIR)/release/$(BINARY_NAME)-$(VERSION)-linux-amd64
	@cp $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(BUILD_DIR)/release/$(BINARY_NAME)-$(VERSION)-linux-amd64/$(BINARY_NAME)
	@cp -r scripts $(BUILD_DIR)/release/$(BINARY_NAME)-$(VERSION)-linux-amd64/
	@cp README.md $(BUILD_DIR)/release/$(BINARY_NAME)-$(VERSION)-linux-amd64/
	@tar -czf $(BUILD_DIR)/release/$(BINARY_NAME)-$(VERSION)-linux-amd64.tar.gz -C $(BUILD_DIR)/release $(BINARY_NAME)-$(VERSION)-linux-amd64
	@# Linux ARM64
	@mkdir -p $(BUILD_DIR)/release/$(BINARY_NAME)-$(VERSION)-linux-arm64
	@cp $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(BUILD_DIR)/release/$(BINARY_NAME)-$(VERSION)-linux-arm64/$(BINARY_NAME)
	@cp -r scripts $(BUILD_DIR)/release/$(BINARY_NAME)-$(VERSION)-linux-arm64/
	@cp README.md $(BUILD_DIR)/release/$(BINARY_NAME)-$(VERSION)-linux-arm64/
	@tar -czf $(BUILD_DIR)/release/$(BINARY_NAME)-$(VERSION)-linux-arm64.tar.gz -C $(BUILD_DIR)/release $(BINARY_NAME)-$(VERSION)-linux-arm64
	@# Linux ARM
	@mkdir -p $(BUILD_DIR)/release/$(BINARY_NAME)-$(VERSION)-linux-arm
	@cp $(BUILD_DIR)/$(BINARY_NAME)-linux-arm $(BUILD_DIR)/release/$(BINARY_NAME)-$(VERSION)-linux-arm/$(BINARY_NAME)
	@cp -r scripts $(BUILD_DIR)/release/$(BINARY_NAME)-$(VERSION)-linux-arm/
	@cp README.md $(BUILD_DIR)/release/$(BINARY_NAME)-$(VERSION)-linux-arm/
	@tar -czf $(BUILD_DIR)/release/$(BINARY_NAME)-$(VERSION)-linux-arm.tar.gz -C $(BUILD_DIR)/release $(BINARY_NAME)-$(VERSION)-linux-arm
	@# Darwin AMD64
	@mkdir -p $(BUILD_DIR)/release/$(BINARY_NAME)-$(VERSION)-darwin-amd64
	@cp $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(BUILD_DIR)/release/$(BINARY_NAME)-$(VERSION)-darwin-amd64/$(BINARY_NAME)
	@cp -r scripts $(BUILD_DIR)/release/$(BINARY_NAME)-$(VERSION)-darwin-amd64/
	@cp README.md $(BUILD_DIR)/release/$(BINARY_NAME)-$(VERSION)-darwin-amd64/
	@tar -czf $(BUILD_DIR)/release/$(BINARY_NAME)-$(VERSION)-darwin-amd64.tar.gz -C $(BUILD_DIR)/release $(BINARY_NAME)-$(VERSION)-darwin-amd64
	@# Darwin ARM64
	@mkdir -p $(BUILD_DIR)/release/$(BINARY_NAME)-$(VERSION)-darwin-arm64
	@cp $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(BUILD_DIR)/release/$(BINARY_NAME)-$(VERSION)-darwin-arm64/$(BINARY_NAME)
	@cp -r scripts $(BUILD_DIR)/release/$(BINARY_NAME)-$(VERSION)-darwin-arm64/
	@cp README.md $(BUILD_DIR)/release/$(BINARY_NAME)-$(VERSION)-darwin-arm64/
	@tar -czf $(BUILD_DIR)/release/$(BINARY_NAME)-$(VERSION)-darwin-arm64.tar.gz -C $(BUILD_DIR)/release $(BINARY_NAME)-$(VERSION)-darwin-arm64
	@# Cleanup temp dirs
	@rm -rf $(BUILD_DIR)/release/$(BINARY_NAME)-$(VERSION)-*-*/
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
	@go clean
	@echo "Clean complete"

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Run tests with coverage
test-cover:
	@echo "Running tests with coverage..."
	@mkdir -p $(BUILD_DIR)
	go test -v -coverprofile=$(BUILD_DIR)/coverage.out ./...
	go tool cover -func=$(BUILD_DIR)/coverage.out
	@echo "Coverage report saved to $(BUILD_DIR)/coverage.out"
	@echo "To view HTML report: go tool cover -html=$(BUILD_DIR)/coverage.out"

# Run tests with race detector
test-race:
	@echo "Running tests with race detector..."
	go test -v -race ./...

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Run linter
lint:
	@echo "Running linter..."
	golangci-lint run

# Show help
help:
	@echo "Sambo - Share Management CLI for Linux and macOS"
	@echo ""
	@echo "Available targets:"
	@echo "  make build       - Build the binary (default)"
	@echo "  make build-all   - Build for multiple architectures"
	@echo "  make release     - Create release packages with scripts"
	@echo "  make package-macos - Create macOS installer packages (.pkg)"
	@echo "  make install     - Build and install to $(INSTALL_PATH)"
	@echo "  make uninstall   - Remove from system"
	@echo "  make clean       - Remove build artifacts"
	@echo "  make test        - Run tests"
	@echo "  make test-cover  - Run tests with coverage report"
	@echo "  make test-race   - Run tests with race detector"
	@echo "  make fmt         - Format code"
	@echo "  make lint        - Run linter"
	@echo "  make help        - Show this help"
