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
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ { printf "  $(GREEN)%-15s$(NC) %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

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

build-all: ## Build for all platforms
	@echo "$(BLUE)🔨 Building for all platforms...$(NC)"
	GOOS=linux GOARCH=amd64 go build -o build/$(BINARY_NAME)-linux-amd64 .
	GOOS=darwin GOARCH=amd64 go build -o build/$(BINARY_NAME)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build -o build/$(BINARY_NAME)-darwin-arm64 .
	GOOS=windows GOARCH=amd64 go build -o build/$(BINARY_NAME)-windows-amd64.exe .
	@echo "$(GREEN)✅ Multi-platform build complete!$(NC)"

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
	@echo "$(GREEN)✅ Clean complete!$(NC)"

# Git workflow helpers
git-hooks: install-pre-commit ## Alias for install-pre-commit

# Documentation
docs: ## Generate documentation
	@echo "$(BLUE)📚 Generating documentation...$(NC)"
	go doc -all . > docs.txt
	@echo "$(GREEN)✅ Documentation generated: docs.txt$(NC)"

# Release helpers
version: ## Show version info
	@echo "$(BLUE)📋 Version information:$(NC)"
	@echo "Go version: $(shell go version)"
	@echo "Git commit: $(shell git rev-parse --short HEAD 2>/dev/null || echo 'N/A')"
	@echo "Git branch: $(shell git branch --show-current 2>/dev/null || echo 'N/A')"
