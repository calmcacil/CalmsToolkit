.PHONY: all build clean install test help

# Binary name
BUILD_DIR=build
BINARY_PLEX=plex-streams
BINARY_JELLYFIN=jellyfin-streams
BINARY_MEDIA=media-streams
BINARY_CALENDAR=media-calendar

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Build flags
LDFLAGS=-ldflags "-s -w"

all: clean build

help:
	@echo "Available targets:"
	@echo "  make build         - Build for current platform"
	@echo "  make build-all     - Build for all platforms"
	@echo "  make install       - Install to /usr/local/bin (Linux/macOS)"
	@echo "  make clean         - Remove build artifacts"
	@echo "  make test          - Run tests"
	@echo "  make tidy          - Tidy go.mod"

build:
	@echo "Building binaries..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_PLEX) plex-streams.go
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_JELLYFIN) jellyfin-streams.go
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_MEDIA) media-streams.go
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_CALENDAR) media-calendar.go
	@echo "Build complete: $(BUILD_DIR)/*"

build-all:
	@echo "Building for all platforms..."
	@mkdir -p $(BUILD_DIR)

	@for GOOS in linux darwin windows freebsd; do \
		for GOARCH in amd64 arm64; do \
			for SRC in plex-streams jellyfin-streams media-streams media-calendar; do \
				BIN=$${SRC}-$$GOOS-$$GOARCH; \
				EXT=$${GOOS} = "windows" && EXT=".exe" || EXT=""; \
				echo "Building $$SRC for $$GOOS/$$GOARCH..."; \
				GOOS=$$GOOS GOARCH=$$GOARCH $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$$BIN$$EXT $$SRC.go || exit 1; \
			done; \
		done; \
	done

	@echo "All builds complete!"
	@ls -lh $(BUILD_DIR)

install: build
	@echo "Installing binaries to /usr/local/bin..."
	@install -m 755 $(BUILD_DIR)/$(BINARY_PLEX) ~/.local/bin/$(BINARY_PLEX)
	@install -m 755 $(BUILD_DIR)/$(BINARY_JELLYFIN) ~/.local/bin/$(BINARY_JELLYFIN)
	@install -m 755 $(BUILD_DIR)/$(BINARY_MEDIA) ~/.local/bin/$(BINARY_MEDIA)
	@install -m 755 $(BUILD_DIR)/$(BINARY_CALENDAR) ~/.local/bin/$(BINARY_CALENDAR)
	@echo "Installation complete!"

clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	@echo "Clean complete!"

test:
	$(GOTEST) -v ./...

tidy:
	$(GOMOD) tidy

# Windows-specific targets
build-windows:
	@echo "Building for Windows..."
	@if not exist $(BUILD_DIR) mkdir $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME).exe plex-streams.go
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME).exe"
