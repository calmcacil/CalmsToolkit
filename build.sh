#!/usr/bin/env bash

# Build script for plex-streams
# Compiles for multiple platforms

set -e

VERSION=${VERSION:-"1.0.0"}
BINARY_NAME="plex-streams"
BUILD_DIR="build"

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RESET='\033[0m'

echo -e "${BLUE}Building plex-streams v${VERSION}${RESET}"

# Create build directory
mkdir -p "$BUILD_DIR"

# Build for Linux (amd64)
echo -e "${GREEN}Building for Linux (amd64)...${RESET}"
GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o "$BUILD_DIR/${BINARY_NAME}-linux-amd64" plex-streams.go

# Build for Linux (arm64)
echo -e "${GREEN}Building for Linux (arm64)...${RESET}"
GOOS=linux GOARCH=arm64 go build -ldflags "-s -w" -o "$BUILD_DIR/${BINARY_NAME}-linux-arm64" plex-streams.go

# Build for macOS (amd64)
echo -e "${GREEN}Building for macOS (amd64)...${RESET}"
GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w" -o "$BUILD_DIR/${BINARY_NAME}-darwin-amd64" plex-streams.go

# Build for macOS (arm64)
echo -e "${GREEN}Building for macOS (arm64)...${RESET}"
GOOS=darwin GOARCH=arm64 go build -ldflags "-s -w" -o "$BUILD_DIR/${BINARY_NAME}-darwin-arm64" plex-streams.go

# Build for Windows (amd64)
echo -e "${GREEN}Building for Windows (amd64)...${RESET}"
GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o "$BUILD_DIR/${BINARY_NAME}-windows-amd64.exe" plex-streams.go

# Build for current platform
echo -e "${GREEN}Building for current platform...${RESET}"
go build -ldflags "-s -w" -o "$BUILD_DIR/${BINARY_NAME}" plex-streams.go

echo -e "${BLUE}Build complete! Binaries are in ${BUILD_DIR}/${RESET}"
ls -lh "$BUILD_DIR"
