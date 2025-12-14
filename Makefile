# Makefile for Sambo - Share Management CLI for Linux and macOS

BINARY_NAME=sambo
VERSION=1.5.0
BUILD_DIR=build
INSTALL_PATH=/usr/local/bin

# Build flags
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: all build clean install uninstall test test-cover test-race help

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
	@echo "Sambo - Linux Share Management CLI"
	@echo ""
	@echo "Available targets:"
	@echo "  make build       - Build the binary (default)"
	@echo "  make build-all   - Build for multiple architectures"
	@echo "  make install     - Build and install to $(INSTALL_PATH)"
	@echo "  make uninstall   - Remove from system"
	@echo "  make clean       - Remove build artifacts"
	@echo "  make test        - Run tests"
	@echo "  make test-cover  - Run tests with coverage report"
	@echo "  make test-race   - Run tests with race detector"
	@echo "  make fmt         - Format code"
	@echo "  make lint        - Run linter"
	@echo "  make help        - Show this help"
