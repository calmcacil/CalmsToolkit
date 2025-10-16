.PHONY: all build clean install test help

# Installation parameters
prefix?=/usr/local
INSTALL_DIR?=$(prefix)/bin

# Binary names and build directory
BUILD_DIR=bin
BINARY_STREAMSTOOL=media-streams
BINARY_CALENDAR=media-calendar
BINARY_REQUESTS=media-requests

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
	@echo "  make install       - Install to $(INSTALL_DIR) (Linux/macOS)"
	@echo "  make clean         - Remove build artifacts"
	@echo "  make test          - Run tests"
	@echo "  make tidy          - Tidy go.mod"

build:
	@echo "Building binaries..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -tags mediastreams -o $(BUILD_DIR)/$(BINARY_STREAMSTOOL) media-streams.go
	$(GOBUILD) $(LDFLAGS) -tags mediacalendar -o $(BUILD_DIR)/$(BINARY_CALENDAR) media-calendar.go
	$(GOBUILD) $(LDFLAGS) -tags mediarequests -o $(BUILD_DIR)/$(BINARY_REQUESTS) media-requests.go
	@echo "Build complete: $(BUILD_DIR)/*"

build-all:
	@echo "Building for all platforms..."
	@mkdir -p $(BUILD_DIR)

	@for GOOS in linux darwin windows freebsd; do \
		for GOARCH in amd64 arm64; do \
			for SRC in media-streams media-calendar media-requests; do \
				BIN=$${SRC}-$$GOOS-$$GOARCH; \
				EXT=$${GOOS} = "windows" && EXT=".exe" || EXT=""; \
				case $$SRC in \
					media-streams) TAG=mediastreams ;; \
					media-calendar) TAG=mediacalendar ;; \
					media-requests) TAG=mediarequests ;; \
				esac; \
				echo "Building $$SRC for $$GOOS/$$GOARCH..."; \
				GOOS=$$GOOS GOARCH=$$GOARCH $(GOBUILD) $(LDFLAGS) -tags $$TAG -o $(BUILD_DIR)/$$BIN$$EXT $$SRC.go || exit 1; \
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
	@echo "Make sure $(INSTALL_DIR) is in your PATH."
	@echo "Installation complete!"

clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	@echo "Clean complete!"

test:
	@echo "Running media-requests tests..."
	$(GOTEST) -tags mediarequests -v ./...
	@echo ""
	@echo "Running media-streams tests..."
	$(GOTEST) -tags mediastreams -v ./...
	@echo ""
	@echo "Running media-calendar tests..."
	$(GOTEST) -tags mediacalendar -v ./...
	@echo ""
	@echo "All tests complete!"

tidy:
	$(GOMOD) tidy
