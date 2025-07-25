# GoFast Server Makefile

# Get version from git tag or use default
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Build flags
LDFLAGS = -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildTime=$(BUILD_TIME)"

# Go settings
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

# Binary names
BINARY_NAME = gofast-server
BINARY_PATH = bin/$(BINARY_NAME)

# Directories
SRC_DIR = .
BUILD_DIR = bin

.PHONY: all build build-all clean test run fmt vet deps help install

# Default target
all: clean fmt vet test build

# Build for current platform
build:
	@echo "Building GoFast Server v$(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(BINARY_PATH) $(SRC_DIR)
	@echo "✅ Build complete: $(BINARY_PATH)"

# Build for all major platforms
build-all: clean
	@echo "Building for all platforms..."
	@mkdir -p $(BUILD_DIR)
	@echo "Building for Linux (amd64)..."
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(SRC_DIR)
	@echo "Building for Linux (arm64)..."
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(SRC_DIR)
	@echo "Building for Windows (amd64)..."
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(SRC_DIR)
	@echo "Building for macOS (amd64)..."
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(SRC_DIR)
	@echo "Building for macOS (arm64)..."
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(SRC_DIR)
	@echo "✅ All builds complete!"
	@ls -la $(BUILD_DIR)/

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@echo "✅ Clean complete"

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...
	@echo "✅ Tests complete"

# Run with race detection
test-race:
	@echo "Running tests with race detection..."
	go test -race -v ./...
	@echo "✅ Race tests complete"

# Benchmark tests
benchmark:
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...
	@echo "✅ Benchmarks complete"

# Run the server in development mode
run:
	@echo "Starting GoFast server (development mode)..."
	go run $(SRC_DIR) --log-level debug

# Run the server with custom config
run-config:
	@echo "Starting GoFast server with config..."
	go run $(SRC_DIR) --config ./configs/gofast.yaml

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	@echo "✅ Format complete"

# Vet code
vet:
	@echo "Vetting code..."
	go vet ./...
	@echo "✅ Vet complete"

# Lint code (requires golangci-lint)
lint:
	@echo "Linting code..."
	golangci-lint run
	@echo "✅ Lint complete"

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy
	@echo "✅ Dependencies updated"

# Install binary to $GOPATH/bin
install:
	@echo "Installing GoFast server..."
	go install $(LDFLAGS) $(SRC_DIR)
	@echo "✅ Installation complete"

# Create release archives
release: build-all
	@echo "Creating release archives..."
	@mkdir -p $(BUILD_DIR)/releases
	cd $(BUILD_DIR) && tar -czf releases/$(BINARY_NAME)-$(VERSION)-linux-amd64.tar.gz $(BINARY_NAME)-linux-amd64
	cd $(BUILD_DIR) && tar -czf releases/$(BINARY_NAME)-$(VERSION)-linux-arm64.tar.gz $(BINARY_NAME)-linux-arm64
	cd $(BUILD_DIR) && tar -czf releases/$(BINARY_NAME)-$(VERSION)-darwin-amd64.tar.gz $(BINARY_NAME)-darwin-amd64
	cd $(BUILD_DIR) && tar -czf releases/$(BINARY_NAME)-$(VERSION)-darwin-arm64.tar.gz $(BINARY_NAME)-darwin-arm64
	cd $(BUILD_DIR) && zip -q releases/$(BINARY_NAME)-$(VERSION)-windows-amd64.zip $(BINARY_NAME)-windows-amd64.exe
	@echo "✅ Release archives created in $(BUILD_DIR)/releases/"
	@ls -la $(BUILD_DIR)/releases/

# Show version information
version:
	@echo "GoFast Server Build Information:"
	@echo "Version: $(VERSION)"
	@echo "Commit: $(COMMIT)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Go Version: $(shell go version)"
	@echo "Platform: $(GOOS)/$(GOARCH)"

# Show help
help:
	@echo "GoFast Server Build Commands:"
	@echo ""
	@echo "Development:"
	@echo "  make build      - Build binary for current platform"
	@echo "  make run        - Run server in development mode"
	@echo "  make run-config - Run server with config file"
	@echo "  make test       - Run tests"
	@echo "  make test-race  - Run tests with race detection"
	@echo "  make benchmark  - Run benchmark tests"
	@echo ""
	@echo "Production:"
	@echo "  make build-all  - Build for all platforms"
	@echo "  make release    - Create release archives"
	@echo "  make install    - Install to GOPATH/bin"
	@echo ""
	@echo "Code Quality:"
	@echo "  make fmt   - Format code"
	@echo "  make vet   - Vet code"
	@echo "  make lint  - Lint code (requires golangci-lint)"
	@echo "  make deps  - Update dependencies"
	@echo ""
	@echo "Utilities:"
	@echo "  make clean   - Clean build artifacts"
	@echo "  make version - Show version information"
	@echo "  make help    - Show this help"