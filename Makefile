WAILS_BIN := $(shell go env GOPATH)/bin/wails
APP_DIR := cmd/loomgui
FRONTEND_DIR := $(APP_DIR)/frontend
LINUX_BUILDER_FILE := Dockerfile.linux
BUILD_DIR := $(APP_DIR)/build/bin
DIST_DIR := dist
APP_NAME := loom
ENV_PATH := /usr/local/go/bin:/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
CACHE_ENV := -e HOME=/tmp -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -e XDG_CACHE_HOME=/tmp/cache -e npm_config_cache=/tmp/npm-cache

NM_VOL_AMD64 := loom_nm_linux_amd64
NM_VOL_ARM64 := loom_nm_linux_arm64

ifeq ($(shell command -v wails),)
	WAILS := $(WAILS_BIN)
else
	WAILS := wails
endif

DOCKER_IMAGE := loom-linux-builder
DOCKER_RUN := docker run --rm -v $(PWD):/app -w /app

.PHONY: all
all: deps build-macos-universal build-windows build-linux-all

.PHONY: deps
deps: deps-go deps-frontend deps-tools

.PHONY: deps-go
deps-go:
	go mod tidy
	go install github.com/wailsapp/wails/v2/cmd/wails@latest

.PHONY: deps-frontend
deps-frontend:
	cd $(FRONTEND_DIR) && npm install

.PHONY: deps-tools
deps-tools:
	which rg || brew install ripgrep

.PHONY: dev
dev:
	cd $(APP_DIR) && $(WAILS) dev

.PHONY: debug
debug:
	cd $(APP_DIR) && $(WAILS) dev -debug

.PHONY: clean
clean:
	cd $(APP_DIR) && $(WAILS) build -clean
	rm -rf $(BUILD_DIR) $(DIST_DIR)

.PHONY: docker-image
docker-image:
	docker build -t $(DOCKER_IMAGE) -f Dockerfile.linux .

.PHONY: build
build:
	cd $(APP_DIR) && $(WAILS) build
	@mkdir -p $(DIST_DIR)
	@GOOS=$$(go env GOOS); GOARCH=$$(go env GOARCH); \
	if [ "$$GOOS" = "darwin" ]; then \
	  rm -rf "$(DIST_DIR)/$(APP_NAME)-darwin-$$GOARCH.app"; \
	  mv "$(BUILD_DIR)/Loom.app" "$(DIST_DIR)/$(APP_NAME)-darwin-$$GOARCH.app"; \
	elif [ "$$GOOS" = "windows" ]; then \
	  mv "$(BUILD_DIR)/Loom.exe" "$(DIST_DIR)/$(APP_NAME)-windows-$$GOARCH.exe"; \
	else \
	  mv "$(BUILD_DIR)/Loom" "$(DIST_DIR)/$(APP_NAME)-linux-$$GOARCH"; \
	fi

.PHONY: build-macos-amd64
build-macos-amd64:
	cd $(APP_DIR) && $(WAILS) build -platform=darwin/amd64 -clean
	@mkdir -p $(DIST_DIR)
	rm -rf "$(DIST_DIR)/$(APP_NAME)-darwin-amd64.app"
	mv "$(BUILD_DIR)/Loom.app" "$(DIST_DIR)/$(APP_NAME)-darwin-amd64.app"

.PHONY: build-macos-arm64
build-macos-arm64:
	cd $(APP_DIR) && $(WAILS) build -platform=darwin/arm64 -clean
	@mkdir -p $(DIST_DIR)
	rm -rf "$(DIST_DIR)/$(APP_NAME)-darwin-arm64.app"
	mv "$(BUILD_DIR)/Loom.app" "$(DIST_DIR)/$(APP_NAME)-darwin-arm64.app"

.PHONY: build-macos-universal
build-macos-universal:
	cd $(APP_DIR) && $(WAILS) build -platform=darwin/universal -clean
	@mkdir -p $(DIST_DIR)
	rm -rf "$(DIST_DIR)/$(APP_NAME)-darwin-universal.app"
	mv "$(BUILD_DIR)/Loom.app" "$(DIST_DIR)/$(APP_NAME)-darwin-universal.app"

.PHONY: build-windows
build-windows:
	cd $(APP_DIR) && $(WAILS) build -platform=windows/amd64 -clean
	@mkdir -p $(DIST_DIR)
	mv "$(BUILD_DIR)/Loom.exe" "$(DIST_DIR)/$(APP_NAME)-windows-amd64.exe"

UID := $(shell id -u)
GID := $(shell id -g)

.PHONY: linux-image-amd64
linux-image-amd64:
	docker buildx build --platform linux/amd64 --load \
		-t loom-linux-builder:amd64 -f $(LINUX_BUILDER_FILE) .

.PHONY: linux-image-arm64
linux-image-arm64:
	docker buildx build --platform linux/arm64 --load \
		-t loom-linux-builder:arm64 -f $(LINUX_BUILDER_FILE) .

# Install Linux-native node_modules (as root) into per-arch volume
.PHONY: prepare-linux-amd64
prepare-linux-amd64: linux-image-amd64
	docker run --rm --platform=linux/amd64 \
		-v $(PWD):/app -w /app \
		-v $(NM_VOL_AMD64):/app/$(FRONTEND_DIR)/node_modules \
		-e PATH=$(ENV_PATH) $(CACHE_ENV) \
		--user 0:0 \
		loom-linux-builder:amd64 \
		bash -c 'set -e; cd $(FRONTEND_DIR); if [ -f package-lock.json ]; then npm ci; else npm install; fi'

.PHONY: prepare-linux-arm64
prepare-linux-arm64: linux-image-arm64
	docker run --rm --platform=linux/arm64 \
		-v $(PWD):/app -w /app \
		-v $(NM_VOL_ARM64):/app/$(FRONTEND_DIR)/node_modules \
		-e PATH=$(ENV_PATH) $(CACHE_ENV) \
		--user 0:0 \
		loom-linux-builder:arm64 \
		bash -c 'set -e; cd $(FRONTEND_DIR); if [ -f package-lock.json ]; then npm ci; else npm install; fi'

# Build binaries as your UID (read-only use of node_modules volume)
.PHONY: build-linux-amd64
build-linux-amd64: prepare-linux-amd64
	docker run --rm --platform=linux/amd64 \
		-v $(PWD):/app -w /app \
		-v $(NM_VOL_AMD64):/app/$(FRONTEND_DIR)/node_modules \
		-u $(UID):$(GID) \
		-e PATH=$(ENV_PATH) $(CACHE_ENV) \
		loom-linux-builder:amd64 \
		bash -c 'set -e; cd $(APP_DIR); wails build -platform=linux/amd64 -clean'
	mkdir -p $(DIST_DIR)
	mv "$(BUILD_DIR)/Loom" "$(DIST_DIR)/$(APP_NAME)-linux-amd64"

.PHONY: build-linux-arm64
build-linux-arm64: prepare-linux-arm64
	docker run --rm --platform=linux/arm64 \
		-v $(PWD):/app -w /app \
		-v $(NM_VOL_ARM64):/app/$(FRONTEND_DIR)/node_modules \
		-u $(UID):$(GID) \
		-e PATH=$(ENV_PATH) $(CACHE_ENV) \
		loom-linux-builder:arm64 \
		bash -c 'set -e; cd $(APP_DIR); wails build -platform=linux/arm64 -clean'
	mkdir -p $(DIST_DIR)
	mv "$(BUILD_DIR)/Loom" "$(DIST_DIR)/$(APP_NAME)-linux-arm64"

.PHONY: build-linux-all
build-linux-all: build-linux-amd64 build-linux-arm64

.PHONY: help
help:
	@echo "targets:"
	@echo "  dev debug build clean"
	@echo "  build-macos-amd64 build-macos-arm64 build-macos-universal build-macos-all"
	@echo "  build-windows"
	@echo "  build-linux-amd64 build-linux-arm64 build-linux-all"
	@echo "  build-all"
