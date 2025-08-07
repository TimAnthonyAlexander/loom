# Loom v2 Makefile

# Variables
WAILS_BIN := $(shell go env GOPATH)/bin/wails
APP_DIR := cmd/loomgui
FRONTEND_DIR := $(APP_DIR)/frontend
BUILD_DIR := $(APP_DIR)/build/bin

# Check if wails is available in PATH, if not use local path
ifeq ($(shell command -v wails),)
	WAILS := $(WAILS_BIN)
else
	WAILS := wails
endif

# Default target
.PHONY: all
all: deps dev

# Install dependencies
.PHONY: deps
deps: deps-go deps-frontend deps-tools

# Install Go dependencies
.PHONY: deps-go
deps-go:
	@echo "Installing Go dependencies..."
	go mod tidy
	go install github.com/wailsapp/wails/v2/cmd/wails@latest

# Install frontend dependencies
.PHONY: deps-frontend
deps-frontend:
	@echo "Installing frontend dependencies..."
	cd $(FRONTEND_DIR) && npm install

# Install required tools
.PHONY: deps-tools
deps-tools:
	@echo "Installing required tools..."
	which ripgrep || brew install ripgrep

# Run in development mode
.PHONY: dev
dev:
	@echo "Starting development server..."
	cd $(APP_DIR) && $(WAILS) dev

# Run in debug mode with Chrome DevTools
.PHONY: debug
debug:
	@echo "Starting development server with debug mode..."
	cd $(APP_DIR) && $(WAILS) dev -debug

# Build for current platform
.PHONY: build
build:
	@echo "Building for current platform..."
	cd $(APP_DIR) && $(WAILS) build

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	cd $(APP_DIR) && $(WAILS) build -clean
	rm -rf $(BUILD_DIR)

# Build for macOS (universal binary)
.PHONY: build-macos
build-macos:
	@echo "Building for macOS (universal)..."
	cd $(APP_DIR) && $(WAILS) build -platform=darwin/universal -clean

# Build for Windows
.PHONY: build-windows
build-windows:
	@echo "Building for Windows (amd64)..."
	cd $(APP_DIR) && $(WAILS) build -platform=windows/amd64 -clean

# Build for Linux
.PHONY: build-linux
build-linux:
	@echo "Building for Linux (amd64)..."
	cd $(APP_DIR) && $(WAILS) build -platform=linux/amd64 -clean

# Build for all platforms
.PHONY: build-all
build-all: build-macos build-windows build-linux
	@echo "Built for all platforms"

# Frontend-specific targets
.PHONY: frontend-dev
frontend-dev:
	@echo "Starting frontend dev server..."
	cd $(FRONTEND_DIR) && npm run dev

.PHONY: frontend-build
frontend-build:
	@echo "Building frontend..."
	cd $(FRONTEND_DIR) && npm run build

# Help
.PHONY: help
help:
	@echo "Loom v2 Makefile"
	@echo ""
	@echo "Available targets:"
	@echo "  deps            - Install all dependencies"
	@echo "  deps-go         - Install Go dependencies"
	@echo "  deps-frontend   - Install frontend dependencies"
	@echo "  deps-tools      - Install required tools (ripgrep)"
	@echo "  dev             - Run in development mode"
	@echo "  debug           - Run in debug mode with Chrome DevTools"
	@echo "  build           - Build for current platform"
	@echo "  build-macos     - Build for macOS (universal)"
	@echo "  build-windows   - Build for Windows (amd64)"
	@echo "  build-linux     - Build for Linux (amd64)"
	@echo "  build-all       - Build for all platforms"
	@echo "  clean           - Clean build artifacts"
	@echo "  frontend-dev    - Run frontend dev server only"
	@echo "  frontend-build  - Build frontend only"
	@echo "  help            - Show this help"