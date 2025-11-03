#!/bin/bash
# Install GoLinkfinderEVO (golinkfinder binary)
# This tool is required for Stage 3 (Crawl) in AethonX

set -e

REPO="https://github.com/lcalzada-xor/GoLinkfinderEVO.git"
BINARY_NAME="golinkfinder"
INSTALL_DIR="${HOME}/go/bin"

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Installing GoLinkfinderEVO"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "❌ Error: Go is not installed. Please install Go 1.21+ first."
    exit 1
fi

echo "✓ Go version: $(go version | awk '{print $3}')"
echo ""

# Check if already installed
if command -v golinkfinder &> /dev/null; then
    echo "⚠ golinkfinder is already installed at: $(which golinkfinder)"
    read -p "Do you want to reinstall? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Installation cancelled."
        exit 0
    fi
fi

# Create temp directory
TMP_DIR=$(mktemp -d)
echo "📦 Cloning repository to ${TMP_DIR}..."
cd "${TMP_DIR}"

# Clone repo
if ! git clone "${REPO}" golinkfinderevo; then
    echo "❌ Failed to clone repository"
    rm -rf "${TMP_DIR}"
    exit 1
fi

cd golinkfinderevo
echo ""

# Build binary
echo "🔨 Building binary..."
if ! go build -o "${BINARY_NAME}"; then
    echo "❌ Build failed"
    cd /
    rm -rf "${TMP_DIR}"
    exit 1
fi

# Ensure install directory exists
mkdir -p "${INSTALL_DIR}"

# Install binary
echo "📥 Installing to ${INSTALL_DIR}..."
if ! mv "${BINARY_NAME}" "${INSTALL_DIR}/"; then
    echo "❌ Failed to move binary to ${INSTALL_DIR}"
    cd /
    rm -rf "${TMP_DIR}"
    exit 1
fi

# Make executable
chmod +x "${INSTALL_DIR}/${BINARY_NAME}"

# Cleanup
cd /
rm -rf "${TMP_DIR}"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✅ Installation complete!"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Binary installed at: ${INSTALL_DIR}/${BINARY_NAME}"
echo ""

# Check if install dir is in PATH
if [[ ":$PATH:" == *":${INSTALL_DIR}:"* ]]; then
    echo "✓ ${INSTALL_DIR} is already in your PATH"
else
    echo "⚠ ${INSTALL_DIR} is not in your PATH"
    echo ""
    echo "Add it to your shell configuration file:"
    echo "  export PATH=\"\$PATH:${INSTALL_DIR}\""
    echo ""
fi

# Verify installation
if command -v golinkfinder &> /dev/null; then
    echo "✓ Verification: golinkfinder is now available"
    echo ""
    echo "Test it with:"
    echo "  golinkfinder -h"
else
    echo "⚠ golinkfinder command not found. You may need to restart your shell."
fi

echo ""
