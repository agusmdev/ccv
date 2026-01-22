#!/usr/bin/env bash

# CCV Installation Script
# Installs the latest version of ccv (Claude Code Viewer)

set -e

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
REPO="agusmdev/ccv"
BINARY_NAME="ccv"
INSTALL_DIR="/usr/local/bin"
FALLBACK_DIR="$HOME/.local/bin"

# Helper functions
info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

success() {
    echo -e "${GREEN}✓${NC} $1"
}

warn() {
    echo -e "${YELLOW}⚠${NC} $1"
}

error() {
    echo -e "${RED}✗${NC} $1"
    exit 1
}

# Detect OS
detect_os() {
    case "$(uname -s)" in
        Darwin*)
            echo "darwin"
            ;;
        Linux*)
            echo "linux"
            ;;
        *)
            error "Unsupported operating system: $(uname -s)"
            ;;
    esac
}

# Detect architecture
detect_arch() {
    local arch="$(uname -m)"
    case "$arch" in
        x86_64|amd64)
            echo "amd64"
            ;;
        aarch64|arm64)
            echo "arm64"
            ;;
        *)
            error "Unsupported architecture: $arch"
            ;;
    esac
}

# Check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Get latest release version from GitHub
get_latest_version() {
    if command_exists curl; then
        curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | \
            grep '"tag_name":' | \
            sed -E 's/.*"([^"]+)".*/\1/'
    elif command_exists wget; then
        wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" | \
            grep '"tag_name":' | \
            sed -E 's/.*"([^"]+)".*/\1/'
    else
        error "Neither curl nor wget found. Please install one of them."
    fi
}

# Download file
download_file() {
    local url="$1"
    local output="$2"

    info "Downloading from $url"

    if command_exists curl; then
        curl -fsSL -o "$output" "$url"
    elif command_exists wget; then
        wget -q -O "$output" "$url"
    else
        error "Neither curl nor wget found. Please install one of them."
    fi
}

# Verify checksum
verify_checksum() {
    local file="$1"
    local checksum_url="$2"
    local temp_checksum="/tmp/ccv_checksums.txt"

    info "Verifying checksum..."
    download_file "$checksum_url" "$temp_checksum"

    cd "$(dirname "$file")"
    local filename="$(basename "$file")"
    local expected_line
    expected_line=$(grep "$filename" "$temp_checksum" | head -1)

    if [ -z "$expected_line" ]; then
        warn "No checksum found for $filename, skipping verification"
        rm -f "$temp_checksum"
        return 0
    fi

    # Use shasum on macOS (BSD sha256sum doesn't support -c flag)
    # Use sha256sum on Linux
    if command_exists shasum; then
        if echo "$expected_line" | shasum -a 256 -c --status 2>/dev/null; then
            success "Checksum verified"
            rm -f "$temp_checksum"
            return 0
        fi
    elif command_exists sha256sum; then
        if echo "$expected_line" | sha256sum -c --status 2>/dev/null; then
            success "Checksum verified"
            rm -f "$temp_checksum"
            return 0
        fi
    else
        warn "No checksum utility found, skipping verification"
        rm -f "$temp_checksum"
        return 0
    fi

    rm -f "$temp_checksum"
    error "Checksum verification failed"
}

# Determine install directory
get_install_dir() {
    if [ -w "$INSTALL_DIR" ]; then
        echo "$INSTALL_DIR"
    elif [ "$(id -u)" = "0" ]; then
        # Running as root
        echo "$INSTALL_DIR"
    else
        # Try with sudo
        if sudo -n true 2>/dev/null; then
            echo "$INSTALL_DIR"
        else
            warn "No write access to $INSTALL_DIR and sudo not available"
            warn "Installing to $FALLBACK_DIR instead"
            mkdir -p "$FALLBACK_DIR"
            echo "$FALLBACK_DIR"
        fi
    fi
}

# Main installation
main() {
    echo ""
    info "CCV (Claude Code Viewer) Installation"
    echo ""

    # Detect system
    OS=$(detect_os)
    ARCH=$(detect_arch)
    info "Detected: $OS/$ARCH"

    # Get latest version
    info "Fetching latest release..."
    VERSION=$(get_latest_version)

    if [ -z "$VERSION" ]; then
        error "Failed to fetch latest version"
    fi

    success "Latest version: $VERSION"

    # Construct download URLs
    BINARY_FILE="${BINARY_NAME}_${OS}_${ARCH}"
    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY_FILE}"
    CHECKSUM_URL="https://github.com/${REPO}/releases/download/${VERSION}/checksums.txt"

    # Create temp directory
    TEMP_DIR=$(mktemp -d)
    trap "rm -rf $TEMP_DIR" EXIT

    TEMP_BINARY="$TEMP_DIR/$BINARY_FILE"

    # Download binary
    download_file "$DOWNLOAD_URL" "$TEMP_BINARY"
    success "Downloaded binary"

    # Verify checksum
    verify_checksum "$TEMP_BINARY" "$CHECKSUM_URL" || true

    # Make executable
    chmod +x "$TEMP_BINARY"

    # Determine install location
    FINAL_INSTALL_DIR=$(get_install_dir)
    FINAL_BINARY="$FINAL_INSTALL_DIR/$BINARY_NAME"

    # Install binary
    info "Installing to $FINAL_BINARY"

    if [ -w "$FINAL_INSTALL_DIR" ]; then
        mv "$TEMP_BINARY" "$FINAL_BINARY"
    else
        sudo mv "$TEMP_BINARY" "$FINAL_BINARY"
    fi

    success "Installed successfully"

    # Check if in PATH
    if ! command_exists "$BINARY_NAME"; then
        echo ""
        warn "$FINAL_INSTALL_DIR is not in your PATH"
        echo ""
        echo "Add it to your PATH by adding this line to your shell profile:"
        echo "  export PATH=\"$FINAL_INSTALL_DIR:\$PATH\""
        echo ""
    fi

    # Verify installation
    echo ""
    info "Verifying installation..."
    if "$FINAL_BINARY" --version >/dev/null 2>&1; then
        success "Installation verified"
        echo ""
        echo -e "${GREEN}CCV installed successfully!${NC}"
        echo ""
        echo "Get started with:"
        echo "  ccv \"Explain this codebase\""
        echo ""
        echo "For help:"
        echo "  ccv --help"
        echo ""
    else
        error "Installation verification failed"
    fi
}

# Run main
main
