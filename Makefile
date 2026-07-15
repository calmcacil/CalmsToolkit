.PHONY: all build build-all clean install test fmt tidy vet check setup

prefix ?= /usr/local
INSTALL_DIR ?= $(prefix)/bin
BUILD_DIR := bin
BINARY := calmstoolkit
GOCMD := go

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
BUILDINFO := -X github.com/calmcacil/CalmsToolkit/internal/buildinfo.Version=$(VERSION) \
             -X github.com/calmcacil/CalmsToolkit/internal/buildinfo.Commit=$(COMMIT) \
             -X github.com/calmcacil/CalmsToolkit/internal/buildinfo.Date=$(DATE)
LDFLAGS := -ldflags "-s -w $(BUILDINFO)"

all: clean build

build:
	@mkdir -p $(BUILD_DIR)
	$(GOCMD) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) ./cmd/calmstoolkit

build-all:
	@mkdir -p $(BUILD_DIR)
	@for arch in amd64 arm64; do \
		echo "Building Linux/$$arch..."; \
		GOOS=linux GOARCH=$$arch CGO_ENABLED=0 $(GOCMD) build $(LDFLAGS) \
			-o $(BUILD_DIR)/$(BINARY)-linux-$$arch ./cmd/calmstoolkit || exit 1; \
	done

install: build
	install -m 755 $(BUILD_DIR)/$(BINARY) $(INSTALL_DIR)/$(BINARY)

setup:
	$(GOCMD) run ./cmd/calmstoolkit config setup

test:
	$(GOCMD) test -race ./...

fmt:
	gofmt -w .

tidy:
	$(GOCMD) mod tidy

vet:
	$(GOCMD) vet ./...

check: fmt tidy vet test build-all

clean:
	$(GOCMD) clean
	rm -rf $(BUILD_DIR)
