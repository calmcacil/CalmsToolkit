.PHONY: all build build-all clean clean-all install test help setup setup-install fmt tidy

# Installation parameters
prefix?=/usr/local
INSTALL_DIR?=$(prefix)/bin

# Binary names and build directory
BUILD_DIR=bin
BINARY_STREAMSTOOL=media-streams
BINARY_CALENDAR=media-calendar
BINARY_REQUESTS=media-requests
BINARY_ARRFEED=arr-feed
BINARY_AIRTIME=media-airtime
BINARY_SETUP=calmstoolkit-setup

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Build flags with version injection
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
BUILDINFO=-X github.com/calmcacil/CalmsToolkit/internal/buildinfo.Version=$(VERSION) \
          -X github.com/calmcacil/CalmsToolkit/internal/buildinfo.Commit=$(COMMIT) \
          -X github.com/calmcacil/CalmsToolkit/internal/buildinfo.Date=$(DATE)
LDFLAGS=-ldflags "-s -w $(BUILDINFO)"

all: clean build

help:
	@echo "Available targets:"
	@echo "  make build         - Build for current platform"
	@echo "  make build-all     - Build for all platforms"
	@echo "  make install       - Install to $(INSTALL_DIR) (Linux/macOS)"
	@echo "  make setup         - Interactive config wizard"
	@echo "  make setup-install - Install setup binary to $(BUILD_DIR)"
	@echo "  make clean         - Remove build artifacts"
	@echo "  make clean-all     - Remove build artifacts and stray repo-root binaries"
	@echo "  make test          - Run tests"
	@echo "  make fmt           - Format Go source files"
	@echo "  make tidy          - Tidy go.mod"

fmt:
	@gofmt -w .
	@echo "Formatted all Go source files"

build:
	@echo "Building binaries..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_STREAMSTOOL) ./cmd/media-streams/
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_CALENDAR) ./cmd/media-calendar/
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_REQUESTS) ./cmd/media-requests/
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_ARRFEED) ./cmd/arr-feed/
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_AIRTIME) ./cmd/media-airtime/
	@echo "Build complete: $(BUILD_DIR)/*"

build-all:
	@echo "Building for all platforms..."
	@mkdir -p $(BUILD_DIR)

	@for GOOS in linux darwin windows freebsd; do \
		for GOARCH in amd64 arm64; do \
			for SRC in media-streams media-calendar media-requests arr-feed media-airtime; do \
				EXT=""; \
				if [ "$$GOOS" = "windows" ]; then EXT=".exe"; fi; \
				BIN=$${SRC}-$$GOOS-$$GOARCH$$EXT; \
				echo "Building $$SRC for $$GOOS/$$GOARCH..."; \
				GOOS=$$GOOS GOARCH=$$GOARCH $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$$BIN ./cmd/$$SRC/ || exit 1; \
			done; \
		done; \
	done

	@echo "All builds complete!"
	@ls -lh $(BUILD_DIR)

install: build
	@echo "Installing binaries to $(INSTALL_DIR)..."
	@install -m 755 $(BUILD_DIR)/$(BINARY_STREAMSTOOL) $(INSTALL_DIR)/$(BINARY_STREAMSTOOL)
	@install -m 755 $(BUILD_DIR)/$(BINARY_CALENDAR) $(INSTALL_DIR)/$(BINARY_CALENDAR)
	@install -m 755 $(BUILD_DIR)/$(BINARY_REQUESTS) $(INSTALL_DIR)/$(BINARY_REQUESTS)
	@install -m 755 $(BUILD_DIR)/$(BINARY_ARRFEED) $(INSTALL_DIR)/$(BINARY_ARRFEED)
	@install -m 755 $(BUILD_DIR)/$(BINARY_AIRTIME) $(INSTALL_DIR)/$(BINARY_AIRTIME)
	@echo "Make sure $(INSTALL_DIR) is in your PATH."
	@echo "Installation complete!"

clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	@echo "Clean complete!"

clean-all: clean
	@echo "Removing stray binaries from repo root..."
	@find . -maxdepth 1 -type f \( -name 'media-*' -o -name 'arr-*' \) -not -name '*.go' -not -name '*.md' -exec rm -v {} \;
	@echo "Clean-all complete!"

setup:
	@echo "=== CalmsToolkit Configuration Setup ==="
	@go run ./cmd/calmstoolkit-setup/

setup-install:
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_SETUP) ./cmd/calmstoolkit-setup/
	@echo "Installed: $(BUILD_DIR)/$(BINARY_SETUP)"

test:
	@echo "Running all tests..."
	$(GOTEST) -v ./...
	@echo ""
	@echo "All tests complete!"

tidy:
	$(GOMOD) tidy
