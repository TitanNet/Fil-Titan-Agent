# Titan Agent Makefile
# Multi-platform build support: Linux, Windows, macOS, Android

# Version information
VERSION ?= 0.1.0
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Build parameters
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)"
CGO_ENABLED ?= 1

# Output directories
BIN_DIR := bin
BUILD_DIR := build

# Target platforms
PLATFORMS := linux/amd64 linux/arm64 linux/arm windows/amd64 windows/arm64 darwin/amd64 darwin/arm64 android/arm64 android/arm

# Create output directories
$(BIN_DIR):
	mkdir -p $(BIN_DIR)

$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

# Clean
.PHONY: clean
clean:
	rm -rf $(BIN_DIR) $(BUILD_DIR)
	go clean -cache

# Testing
.PHONY: test
test:
	go test -v ./...

# Code formatting
.PHONY: fmt
fmt:
	go fmt ./...
	gofmt -s -w .

# Build Controller
.PHONY: controller
controller: $(BIN_DIR)
	go build $(LDFLAGS) -o $(BIN_DIR)/controller cmd/controller/main.go

# Build Agent
.PHONY: agent
agent: $(BIN_DIR)
	go build $(LDFLAGS) -o $(BIN_DIR)/agent cmd/agent/main.go

# Build Server
.PHONY: server
server: $(BIN_DIR)
	go build $(LDFLAGS) -o $(BIN_DIR)/server cmd/server/main.go

# Cross-compilation function
define build-cross
	@echo "Building for $(1)/$(2)..."
	@mkdir -p $(BIN_DIR)/$(1)-$(2)
	GOOS=$(1) GOARCH=$(2) CGO_ENABLED=$(CGO_ENABLED) go build $(LDFLAGS) \
		-o $(BIN_DIR)/$(1)-$(2)/controller cmd/controller/main.go
	GOOS=$(1) GOARCH=$(2) CGO_ENABLED=$(CGO_ENABLED) go build $(LDFLAGS) \
		-o $(BIN_DIR)/$(1)-$(2)/agent cmd/agent/main.go
	GOOS=$(1) GOARCH=$(2) CGO_ENABLED=$(CGO_ENABLED) go build $(LDFLAGS) \
		-o $(BIN_DIR)/$(1)-$(2)/server cmd/server/main.go
endef

# Build all platforms
.PHONY: build-all
build-all: $(BIN_DIR)
	@echo "Building for all platforms..."
	$(foreach platform,$(PLATFORMS),\
		$(call build-cross,$(word 1,$(subst /, ,$(platform))),$(word 2,$(subst /, ,$(platform)))))

# Linux builds
.PHONY: build-linux
build-linux: $(BIN_DIR)
	$(call build-cross,linux,amd64)
	$(call build-cross,linux,arm64)
	$(call build-cross,linux,arm)

# Windows builds
.PHONY: build-windows
build-windows: $(BIN_DIR)
	$(call build-cross,windows,amd64)
	$(call build-cross,windows,arm64)

# macOS builds
.PHONY: build-darwin
build-darwin: $(BIN_DIR)
	$(call build-cross,darwin,amd64)
	$(call build-cross,darwin,arm64)

# Android builds
.PHONY: build-android
build-android: $(BIN_DIR)
	@echo "Building for Android..."
	@mkdir -p $(BIN_DIR)/android-arm
	
	# Set Android NDK environment
	ANDROID_NDK_HOME ?= /opt/android-ndk-r27-beta1
	ANDROID_CC ?= $(ANDROID_NDK_HOME)/toolchains/llvm/prebuilt/linux-x86_64/bin/armv7a-linux-androideabi34-clang
	
	# Build for Android ARM
	CGO_ENABLED=1 GOOS=android GOARCH=arm GOARM=7 CC=$(ANDROID_CC) \
	go build -trimpath $(LDFLAGS) -o $(BIN_DIR)/android-arm/controller cmd/controller/main.go
	
	CGO_ENABLED=1 GOOS=android GOARCH=arm GOARM=7 CC=$(ANDROID_CC) \
	go build -trimpath $(LDFLAGS) -o $(BIN_DIR)/android-arm/agent cmd/agent/main.go
	
	CGO_ENABLED=1 GOOS=android GOARCH=arm GOARM=7 CC=$(ANDROID_CC) \
	go build -trimpath $(LDFLAGS) -o $(BIN_DIR)/android-arm/server cmd/server/main.go


# Help information
.PHONY: help
help:
	@echo "Titan Agent Makefile"
	@echo ""
	@echo "Targets:"
	@echo "  controller       - Build controller only"
	@echo "  agent           - Build agent only"
	@echo "  server          - Build server only"
	@echo "  build-all       - Build for all platforms"
	@echo "  build-linux     - Build for Linux platforms"
	@echo "  build-windows   - Build for Windows platforms"
	@echo "  build-darwin    - Build for macOS platforms"
	@echo "  build-android   - Build for Android ARM platform"
	@echo "  clean           - Clean build artifacts"
	@echo "  test            - Run tests"
	@echo "  fmt             - Format code"
	@echo "  help            - Show this help"
	@echo ""
	@echo "Environment variables:"
	@echo "  VERSION         - Set version (default: 0.1.0)"
	@echo "  CGO_ENABLED     - Enable CGO (default: 1)"
	@echo "  ANDROID_NDK_HOME - Android NDK path (default: /opt/android-ndk-r27-beta1)"
	@echo ""
	@echo "Examples:"
	@echo "  make controller                             # Build controller"
	@echo "  make build-android                         # Build for Android"
	@echo "  make build-all                              # Build all platforms"

# Default target
.DEFAULT_GOAL := help 