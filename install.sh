#!/bin/sh
# Copilot Proxy Installer
# Usage: curl -fsSL https://raw.githubusercontent.com/vanhiep99w/premium-saver/main/install.sh | sh

set -e

REPO="vanhiep99w/premium-saver"
BINARY_NAME="copilot-proxy"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info() { printf "${GREEN}[INFO]${NC} %s\n" "$1"; }
warn() { printf "${YELLOW}[WARN]${NC} %s\n" "$1"; }
error() { printf "${RED}[ERROR]${NC} %s\n" "$1"; exit 1; }

# Detect OS
detect_os() {
    case "$(uname -s)" in
        Linux*)  echo "linux" ;;
        Darwin*) echo "darwin" ;;
        MINGW*|MSYS*|CYGWIN*) echo "windows" ;;
        *) error "Unsupported operating system: $(uname -s)" ;;
    esac
}

# Detect architecture
detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64) echo "amd64" ;;
        aarch64|arm64) echo "arm64" ;;
        *) error "Unsupported architecture: $(uname -m)" ;;
    esac
}

# Get latest release tag
get_latest_version() {
    if command -v curl > /dev/null 2>&1; then
        curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/'
    elif command -v wget > /dev/null 2>&1; then
        wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/'
    else
        error "Neither curl nor wget found. Please install one of them."
    fi
}

# Download file
download() {
    local url="$1"
    local output="$2"
    if command -v curl > /dev/null 2>&1; then
        curl -fsSL "$url" -o "$output"
    elif command -v wget > /dev/null 2>&1; then
        wget -q "$url" -O "$output"
    fi
}

main() {
    OS=$(detect_os)
    ARCH=$(detect_arch)

    info "Detected: ${OS}/${ARCH}"

    # Get version
    VERSION="${1:-$(get_latest_version)}"
    if [ -z "$VERSION" ]; then
        error "Could not determine latest version. Specify a version: $0 v1.0.0"
    fi
    info "Version: ${VERSION}"

    # Build filename
    SUFFIX="${OS}-${ARCH}"
    if [ "$OS" = "windows" ]; then
        SUFFIX="${SUFFIX}.exe"
    fi
    FILENAME="${BINARY_NAME}-${SUFFIX}"

    # Download URL
    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${FILENAME}"
    info "Downloading: ${DOWNLOAD_URL}"

    # Create temp directory
    TMP_DIR=$(mktemp -d)
    trap 'rm -rf "$TMP_DIR"' EXIT

    # Download binary
    download "$DOWNLOAD_URL" "${TMP_DIR}/${BINARY_NAME}" || error "Download failed. Check that version ${VERSION} exists at https://github.com/${REPO}/releases"

    # Make executable
    chmod +x "${TMP_DIR}/${BINARY_NAME}"

    # Install
    if [ -w "$INSTALL_DIR" ]; then
        mv "${TMP_DIR}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
    else
        info "Need sudo to install to ${INSTALL_DIR}"
        sudo mv "${TMP_DIR}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
    fi

    info "Installed ${BINARY_NAME} to ${INSTALL_DIR}/${BINARY_NAME}"
    echo ""
    info "Get started:"
    echo "  1. ${BINARY_NAME} login     # Authenticate with GitHub Copilot"
    echo "  2. ${BINARY_NAME} serve     # Start the proxy server"
    echo ""
    info "Then configure your AI client:"
    echo "  Base URL: http://localhost:8787/v1"
    echo "  API Key:  any-value"
    echo ""

    # Verify
    if command -v "$BINARY_NAME" > /dev/null 2>&1; then
        info "Verified: $($BINARY_NAME help 2>&1 | head -1)"
    fi
}

main "$@"
