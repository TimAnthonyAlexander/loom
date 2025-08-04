.PHONY: help build test lint fmt vet tidy clean install-tools setup dev-setup check coverage benchmark security pre-commit

# Go parameters
BINARY_NAME=loom
BINARY_PATH=./$(BINARY_NAME)
GO_FILES=$(shell find . -name "*.go" -type f -not -path "./vendor/*")
PACKAGES=$(shell go list ./...)

# Colors for pretty output
RED=\033[0;31m
GREEN=\033[0;32m
YELLOW=\033[1;33m
BLUE=\033[0;34m
NC=\033[0m # No Color

help: ## Show this help message
	@echo "$(BLUE)Available commands:$(NC)"
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ { printf "  $(GREEN)%-18s$(NC) %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
	@echo ""
	@echo "$(YELLOW)GUI Development Options:$(NC)"
	@echo "  $(GREEN)make dev-gui$(NC)       - Full output (good for debugging)"
	@echo "  $(GREEN)make dev-gui-quiet$(NC) - Clean output (recommended)"
	@echo "  $(GREEN)make dev-gui-silent$(NC)- Background mode (minimal output)"
	@echo ""
	@echo "$(YELLOW)GUI Build Options:$(NC)"
	@echo "  $(GREEN)make build-gui$(NC)              - Build for current platform"
	@echo "  $(GREEN)make build-gui-linux$(NC)        - Build for Linux (requires Docker)"
	@echo "  $(GREEN)make build-gui-all-platforms$(NC) - Build for ALL platforms including Linux"
	@echo ""
	@echo "$(YELLOW)Distribution Build Options:$(NC)"
	@echo "  $(GREEN)make dist$(NC)                   - Build TUI & GUI for all platforms"
	@echo "  $(GREEN)make dist-macos-arm$(NC)         - Build TUI & GUI for macOS ARM only"
	@echo "  $(GREEN)make dist-with-linux-gui$(NC)    - Build all platforms including Linux GUI"

# Development workflow
dev-setup: install-tools setup ## Complete development environment setup
	@echo "$(GREEN)✅ Development environment ready!$(NC)"

setup: tidy fmt lint test ## Run setup tasks (tidy, format, lint, test)
	@echo "$(GREEN)✅ Setup complete!$(NC)"

check: lint vet test ## Run all checks (lint, vet, test)
	@echo "$(GREEN)✅ All checks passed!$(NC)"

# Build commands
build: ## Build the binary
	@echo "$(BLUE)🔨 Building $(BINARY_NAME)...$(NC)"
	go build -o $(BINARY_PATH) .
	@echo "$(GREEN)✅ Build complete: $(BINARY_PATH)$(NC)"

build-embedded: download-ripgrep build ## Build the binary with embedded ripgrep
	@echo "$(GREEN)✅ Build complete with embedded ripgrep: $(BINARY_PATH)$(NC)"

# GUI build commands
build-gui: ## Build the GUI application
	@echo "$(BLUE)🔨 Building GUI application...$(NC)"
	cd gui && export PATH=$$PATH:$(shell go env GOPATH)/bin && wails build
	@echo "$(GREEN)✅ GUI build complete: gui/build/$(NC)"

build-gui-linux: ## Build GUI for Linux using Docker
	@echo "$(BLUE)🔨 Building GUI for Linux using Docker...$(NC)"
	@echo "$(YELLOW)⚠️  This requires Docker to be installed and running$(NC)"
	cd gui && ./build-linux.sh
	@echo "$(GREEN)✅ Linux GUI build complete: gui/build/bin/linux/$(NC)"

build-gui-all-platforms: build-gui-linux ## Build GUI for ALL platforms including Linux
	@echo "$(BLUE)🔨 Building GUI for all platforms (including Linux)...$(NC)"
	cd gui && export PATH=$$PATH:$(shell go env GOPATH)/bin && wails build --platform=darwin/amd64,darwin/arm64,windows/amd64
	@echo "$(GREEN)✅ All-platform GUI build complete!$(NC)"
	@echo "$(BLUE)📦 Available GUI builds:$(NC)"
	@echo "  Linux: gui/build/bin/linux/"
	@echo "  Other platforms: gui/build/"

dev-gui: ## Start GUI development server
	@echo "$(BLUE)🚀 Starting GUI development server...$(NC)"
	@echo "$(YELLOW)⚠️  Note: Verbose output is normal for Wails development mode$(NC)"
	@echo "$(YELLOW)⚠️  'Private APIs' warning is expected on macOS and safe to ignore$(NC)"
	@echo ""
	cd gui && export PATH=$$PATH:$(shell go env GOPATH)/bin && wails dev

dev-gui-quiet: ## Start GUI development server with minimal output
	@echo "$(BLUE)🚀 Starting GUI development server (quiet mode)...$(NC)"
	@echo "$(GREEN)✨ Filtering verbose output... Use 'make dev-gui' for full details$(NC)"
	@echo ""
	cd gui && export PATH=$$PATH:$(shell go env GOPATH)/bin && ./scripts/dev-quiet.sh

dev-gui-silent: ## Start GUI development server with almost no output
	@echo "$(BLUE)🚀 Starting GUI (silent mode)... $(GREEN)http://localhost:34115$(NC)"
	@cd gui && export PATH=$$PATH:$(shell go env GOPATH)/bin && nohup wails dev > /dev/null 2>&1 &

build-frontend: ## Build only the frontend
	@echo "$(BLUE)🔨 Building frontend...$(NC)"
	cd gui/frontend && npm install && npm run build
	@echo "$(GREEN)✅ Frontend build complete!$(NC)"

build-all: download-ripgrep build-frontend ## Build for all platforms with embedded ripgrep and GUI
	@echo "$(BLUE)🔨 Building for all platforms with embedded ripgrep...$(NC)"
	mkdir -p build
	GOOS=linux GOARCH=amd64 go build -o build/$(BINARY_NAME)-linux-amd64 .
	GOOS=darwin GOARCH=amd64 go build -o build/$(BINARY_NAME)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build -o build/$(BINARY_NAME)-darwin-arm64 .
	GOOS=windows GOARCH=amd64 go build -o build/$(BINARY_NAME)-windows-amd64.exe .
	@echo "$(GREEN)✅ Multi-platform build complete with embedded ripgrep!$(NC)"
	@echo "$(BLUE)🔨 Building GUI for non-Linux platforms...$(NC)"
	cd gui && export PATH=$$PATH:$(shell go env GOPATH)/bin && wails build --platform=darwin/amd64,darwin/arm64,windows/amd64
	@echo "$(GREEN)✅ Non-Linux GUI build complete!$(NC)"
	@echo "$(YELLOW)⚠️  Linux GUI build requires Docker. Run 'make build-gui-linux' separately.$(NC)"

# Deployment targets
deploy-gui: build-gui ## Build GUI for current platform and show deployment info
	@echo "$(GREEN)🎉 GUI Application Built Successfully!$(NC)"
	@echo "$(BLUE)📦 Executable location: gui/build/gui$(NC)"
	@echo ""
	@echo "$(YELLOW)🚀 Ready to Deploy:$(NC)"
	@echo "  • The executable is self-contained (includes web UI)"
	@echo "  • No separate web server needed"
	@echo "  • Double-click to run or use from command line"
	@echo ""
	@ls -la gui/build/

deploy-gui-all: build-gui-all-platforms ## Build GUI for all platforms with distribution info
	@echo "$(GREEN)🎉 Multi-Platform GUI Applications Built!$(NC)"
	@echo "$(BLUE)📦 GUI Applications:$(NC)"
	@echo "$(YELLOW)macOS/Windows builds:$(NC)"
	@ls -la gui/build/ | grep -v "^total" || true
	@echo "$(YELLOW)Linux builds:$(NC)"
	@ls -la gui/build/bin/linux/ | grep -v "^total" || true
	@echo ""
	@echo "$(YELLOW)🚀 Distribution Ready:$(NC)"
	@echo "  • Each executable is self-contained"
	@echo "  • No additional files or servers needed"
	@echo "  • Ready for distribution to end users"

package-gui: build-gui ## Build and create distribution package
	@echo "$(BLUE)📦 Creating distribution package...$(NC)"
	mkdir -p dist
	cp gui/build/gui dist/loom-gui
	@echo "$(GREEN)✅ Package ready: dist/loom-gui$(NC)"
	@echo "$(YELLOW)💡 To distribute: Zip the dist/ folder or copy the executable$(NC)"

package-gui-installer: ## Build GUI with installer (Windows/macOS)
	@echo "$(BLUE)📦 Building GUI with installer...$(NC)"
	cd gui && export PATH=$$PATH:$(shell go env GOPATH)/bin && wails build -nsis
	@echo "$(GREEN)✅ Installer build complete!$(NC)"

RIPGREP_VERSION=14.1.0
RIPGREP_DIR=bin

download-ripgrep: ## Download ripgrep binaries for Linux, macOS, and Windows
	@echo "$(BLUE)⬇️  Downloading ripgrep v$(RIPGREP_VERSION)...$(NC)"
	# Linux
	curl -Ls -o /tmp/rg-linux.tar.gz https://github.com/BurntSushi/ripgrep/releases/download/$(RIPGREP_VERSION)/ripgrep-$(RIPGREP_VERSION)-x86_64-unknown-linux-musl.tar.gz
	mkdir -p $(RIPGREP_DIR)
	tar -xzf /tmp/rg-linux.tar.gz --strip-components=1 -C $(RIPGREP_DIR) 'ripgrep-$(RIPGREP_VERSION)-x86_64-unknown-linux-musl/rg'
	mv $(RIPGREP_DIR)/rg $(RIPGREP_DIR)/rg-linux
	chmod +x $(RIPGREP_DIR)/rg-linux

	# macOS
	curl -Ls -o /tmp/rg-macos.tar.gz https://github.com/BurntSushi/ripgrep/releases/download/$(RIPGREP_VERSION)/ripgrep-$(RIPGREP_VERSION)-x86_64-apple-darwin.tar.gz
	tar -xzf /tmp/rg-macos.tar.gz --strip-components=1 -C $(RIPGREP_DIR) 'ripgrep-$(RIPGREP_VERSION)-x86_64-apple-darwin/rg'
	mv $(RIPGREP_DIR)/rg $(RIPGREP_DIR)/rg-macos
	chmod +x $(RIPGREP_DIR)/rg-macos

	# Windows
	curl -Ls -o /tmp/rg-windows.zip https://github.com/BurntSushi/ripgrep/releases/download/$(RIPGREP_VERSION)/ripgrep-$(RIPGREP_VERSION)-x86_64-pc-windows-msvc.zip
	unzip -j -o /tmp/rg-windows.zip 'ripgrep-$(RIPGREP_VERSION)-x86_64-pc-windows-msvc/rg.exe' -d $(RIPGREP_DIR)
	mv $(RIPGREP_DIR)/rg.exe $(RIPGREP_DIR)/rg-windows.exe

	@echo "$(GREEN)✅ ripgrep binaries downloaded to $(RIPGREP_DIR)/$(NC)"

# Testing
test: ## Run tests
	@echo "$(BLUE)🧪 Running tests...$(NC)"
	go test -v ./...

test-short: ## Run tests (short mode)
	@echo "$(BLUE)🧪 Running tests (short)...$(NC)"
	go test -short ./...

test-race: ## Run tests with race detection
	@echo "$(BLUE)🧪 Running tests with race detection...$(NC)"
	go test -race ./...

coverage: ## Run tests with coverage
	@echo "$(BLUE)📊 Running tests with coverage...$(NC)"
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)✅ Coverage report: coverage.html$(NC)"

benchmark: ## Run benchmarks
	@echo "$(BLUE)⚡ Running benchmarks...$(NC)"
	go test -bench=. -benchmem ./...

# Code quality
fmt: ## Format code
	@echo "$(BLUE)🎨 Formatting code...$(NC)"
	gofmt -s -w $(GO_FILES)
	@if command -v goimports >/dev/null 2>&1; then \
		goimports -w $(GO_FILES); \
	elif [ -f ~/go/bin/goimports ]; then \
		~/go/bin/goimports -w $(GO_FILES); \
	else \
		echo "$(YELLOW)⚠️ goimports not found, skipping import formatting$(NC)"; \
	fi
	@echo "$(GREEN)✅ Code formatted!$(NC)"

lint: ## Run linter
	@echo "$(BLUE)🔍 Running linter...$(NC)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
		echo "$(GREEN)✅ Linting complete!$(NC)"; \
	else \
		echo "$(RED)❌ golangci-lint not installed. Run 'make install-tools'$(NC)"; \
		exit 1; \
	fi

vet: ## Run go vet
	@echo "$(BLUE)🔍 Running go vet...$(NC)"
	go vet ./...
	@echo "$(GREEN)✅ Vet complete!$(NC)"

tidy: ## Tidy go modules
	@echo "$(BLUE)🧹 Tidying modules...$(NC)"
	go mod tidy
	go mod verify
	@echo "$(GREEN)✅ Modules tidied!$(NC)"

security: ## Run security checks
	@echo "$(BLUE)🔒 Running security checks...$(NC)"
	@if command -v govulncheck >/dev/null 2>&1; then \
		govulncheck ./...; \
		echo "$(GREEN)✅ Security check complete!$(NC)"; \
	else \
		echo "$(YELLOW)⚠️ govulncheck not installed. Installing...$(NC)"; \
		go install golang.org/x/vuln/cmd/govulncheck@latest; \
		govulncheck ./...; \
	fi

# Pre-commit
pre-commit: ## Run pre-commit hooks
	@echo "$(BLUE)🚀 Running pre-commit hooks...$(NC)"
	@if command -v pre-commit >/dev/null 2>&1; then \
		pre-commit run --all-files; \
		echo "$(GREEN)✅ Pre-commit hooks complete!$(NC)"; \
	else \
		echo "$(RED)❌ pre-commit not installed. Run 'make install-tools'$(NC)"; \
		exit 1; \
	fi

install-pre-commit: ## Install pre-commit hooks
	@echo "$(BLUE)📦 Installing pre-commit hooks...$(NC)"
	@if command -v pre-commit >/dev/null 2>&1; then \
		pre-commit install; \
		pre-commit install --hook-type pre-push; \
		echo "$(GREEN)✅ Pre-commit hooks installed!$(NC)"; \
	else \
		echo "$(RED)❌ pre-commit not installed. Please install it first:$(NC)"; \
		echo "$(YELLOW)  macOS: brew install pre-commit$(NC)"; \
		echo "$(YELLOW)  pip: pip install pre-commit$(NC)"; \
		exit 1; \
	fi

# Tool installation
install-tools: ## Install development tools
	@echo "$(BLUE)📦 Installing Go tools...$(NC)"
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest
	@echo "$(GREEN)✅ Go tools installed!$(NC)"
	@echo "$(YELLOW)📝 Don't forget to install pre-commit:$(NC)"
	@echo "$(YELLOW)  macOS: brew install pre-commit$(NC)"
	@echo "$(YELLOW)  pip: pip install pre-commit$(NC)"
	@echo "$(YELLOW)  Then run: make install-pre-commit$(NC)"

# Cleanup
clean: ## Clean build artifacts
	@echo "$(BLUE)🧹 Cleaning...$(NC)"
	go clean
	rm -f $(BINARY_PATH)
	rm -rf build/
	rm -f coverage.out coverage.html
	cd gui && rm -rf build/ dist/
	cd gui/frontend && rm -rf dist/ node_modules/
	@echo "$(GREEN)✅ Clean complete!$(NC)"

# Git workflow helpers
git-hooks: install-pre-commit ## Alias for install-pre-commit

# Documentation
docs: ## Generate documentation
	@echo "$(BLUE)📚 Generating documentation...$(NC)"
	go doc -all . > docs.txt
	@echo "$(GREEN)✅ Documentation generated: docs.txt$(NC)"

dist: download-ripgrep ## Build both TUI and GUI executables for all platforms in dist/
	@echo "$(BLUE)🔨 Building TUI and GUI executables for all platforms...$(NC)"
	rm -rf dist/
	mkdir -p dist/
	
	@echo "$(BLUE)Building TUI executables...$(NC)"
	GOOS=darwin GOARCH=amd64 go build -o dist/loom-tui-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build -o dist/loom-tui-darwin-arm64 .
	GOOS=linux GOARCH=amd64 go build -o dist/loom-tui-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build -o dist/loom-tui-linux-arm64 .
	GOOS=windows GOARCH=amd64 go build -o dist/loom-tui-windows-amd64.exe .
	GOOS=windows GOARCH=arm64 go build -o dist/loom-tui-windows-arm64.exe .
	
	@echo "$(BLUE)Building GUI executables...$(NC)"
	cd gui && export PATH=$$PATH:$(shell go env GOPATH)/bin && wails build --platform=darwin/amd64,darwin/arm64,windows/amd64,windows/arm64
	
	@echo "$(BLUE)Copying GUI executables to dist/...$(NC)"
	# macOS .app bundles
	if [ -d "gui/build/bin/gui-amd64.app" ]; then cp -r gui/build/bin/gui-amd64.app dist/loom-gui-darwin-amd64.app; fi
	if [ -d "gui/build/bin/gui-arm64.app" ]; then cp -r gui/build/bin/gui-arm64.app dist/loom-gui-darwin-arm64.app; fi
	# Windows executables  
	if [ -f "gui/build/bin/gui-amd64.exe" ]; then cp gui/build/bin/gui-amd64.exe dist/loom-gui-windows-amd64.exe; fi
	if [ -f "gui/build/bin/gui-arm64.exe" ]; then cp gui/build/bin/gui-arm64.exe dist/loom-gui-windows-arm64.exe; fi
	
	@echo "$(BLUE)Creating macOS DMG files...$(NC)"
	# Create DMG for Intel macOS
	if [ -d "dist/loom-gui-darwin-amd64.app" ]; then \
		hdiutil create -volname "Loom GUI (Intel)" -srcfolder "dist/loom-gui-darwin-amd64.app" -ov -format UDZO "dist/loom-gui-darwin-amd64.dmg"; \
		echo "$(GREEN)✅ Created dist/loom-gui-darwin-amd64.dmg$(NC)"; \
	fi
	# Create DMG for Apple Silicon macOS
	if [ -d "dist/loom-gui-darwin-arm64.app" ]; then \
		hdiutil create -volname "Loom GUI (Apple Silicon)" -srcfolder "dist/loom-gui-darwin-arm64.app" -ov -format UDZO "dist/loom-gui-darwin-arm64.dmg"; \
		echo "$(GREEN)✅ Created dist/loom-gui-darwin-arm64.dmg$(NC)"; \
	fi
	
	@echo "$(GREEN)✅ All executables built and placed in dist/!$(NC)"
	@echo "$(YELLOW)💡 Note: Linux GUI requires Docker. Run 'make dist-with-linux-gui' to include it.$(NC)"
	@echo "$(BLUE)📦 Contents:$(NC)"
	@ls -la dist/

dist-with-linux-gui: dist ## Build all executables including Linux GUI (requires Docker)
	@echo "$(BLUE)🔨 Building Linux GUI (requires Docker)...$(NC)"
	@if command -v docker >/dev/null 2>&1; then \
		cd gui && ./build-linux.sh; \
		if [ -f "gui/build/bin/linux/gui" ]; then cp gui/build/bin/linux/gui dist/loom-gui-linux-amd64; fi; \
		echo "$(GREEN)✅ Linux GUI added to dist/!$(NC)"; \
	else \
		echo "$(RED)❌ Docker not found. Linux GUI build requires Docker.$(NC)"; \
	fi
	@echo "$(BLUE)📦 Final contents:$(NC)"
	@ls -la dist/

dist-macos-arm: download-ripgrep ## Build both TUI and GUI executables for macOS ARM only
	@echo "$(BLUE)🔨 Building TUI and GUI executables for macOS ARM (Apple Silicon)...$(NC)"
	rm -rf dist/
	mkdir -p dist/
	
	@echo "$(BLUE)Building TUI executable for macOS ARM...$(NC)"
	GOOS=darwin GOARCH=arm64 go build -o dist/loom-tui-darwin-arm64 .
	
	@echo "$(BLUE)Building GUI executable for macOS ARM...$(NC)"
	cd gui && export PATH=$$PATH:$(shell go env GOPATH)/bin && wails build --platform=darwin/arm64
	
	@echo "$(BLUE)Copying GUI executable to dist/...$(NC)"
	# macOS .app bundle for ARM
	if [ -d "gui/build/bin/gui-arm64.app" ]; then cp -r gui/build/bin/gui-arm64.app dist/loom-gui-darwin-arm64.app; fi
	
	@echo "$(BLUE)Creating macOS ARM DMG file...$(NC)"
	# Create DMG for Apple Silicon macOS
	if [ -d "dist/loom-gui-darwin-arm64.app" ]; then \
		hdiutil create -volname "Loom GUI (Apple Silicon)" -srcfolder "dist/loom-gui-darwin-arm64.app" -ov -format UDZO "dist/loom-gui-darwin-arm64.dmg"; \
		echo "$(GREEN)✅ Created dist/loom-gui-darwin-arm64.dmg$(NC)"; \
	fi
	
	@echo "$(GREEN)✅ macOS ARM executables built and placed in dist/!$(NC)"
	@echo "$(BLUE)📦 Contents:$(NC)"
	@ls -la dist/

# Release helpers
version: ## Show version info
	@echo "$(BLUE)📋 Version information:$(NC)"
	@echo "Go version: $(shell go version)"
	@echo "Git commit: $(shell git rev-parse --short HEAD 2>/dev/null || echo 'N/A')"
	@echo "Git branch: $(shell git branch --show-current 2>/dev/null || echo 'N/A')"
