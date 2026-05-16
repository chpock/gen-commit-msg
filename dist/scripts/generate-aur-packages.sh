#!/bin/bash
# Script to generate AUR package files from templates
# Downloads source tarball and computes SHA256 automatically
# Usage: ./generate-aur-packages.sh VERSION

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
TEMPLATES_DIR="$PROJECT_ROOT/dist/aur"
OUTPUT_DIR="$PROJECT_ROOT/target/aur"
GITHUB_REPO="chpock/gen-commit-msg"
STABLE_PKG="gen-commit-msg"
GIT_PKG="gen-commit-msg-git"

if [ $# -lt 1 ]; then
    echo "Usage: $0 VERSION"
    echo "Example: $0 1.2.3"
    exit 1
fi

VERSION="$1"

# Download tarball and compute SHA256
echo "Downloading source tarball for version $VERSION..."
SOURCE_URL="https://github.com/${GITHUB_REPO}/archive/refs/tags/v${VERSION}.tar.gz"
TEMP_TARBALL="/tmp/${STABLE_PKG}-${VERSION}.tar.gz"

curl -fsSL "$SOURCE_URL" -o "$TEMP_TARBALL"
SHA256=$(sha256sum "$TEMP_TARBALL" | cut -d' ' -f1)

echo "Source SHA256: $SHA256"
echo ""
echo "Generating AUR packages..."
echo "Templates: $TEMPLATES_DIR"
echo "Output: $OUTPUT_DIR"

# Clean and create output directory
rm -rf "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR/$STABLE_PKG"
mkdir -p "$OUTPUT_DIR/$GIT_PKG"

# Copy PKGBUILD templates
cp "$TEMPLATES_DIR/$STABLE_PKG/PKGBUILD" "$OUTPUT_DIR/$STABLE_PKG/"
cp "$TEMPLATES_DIR/$GIT_PKG/PKGBUILD" "$OUTPUT_DIR/$GIT_PKG/"

# Update stable package (gen-commit-msg)
echo "Updating $STABLE_PKG (stable)..."
sed -i "s/VERSION_PLACEHOLDER/$VERSION/g" "$OUTPUT_DIR/$STABLE_PKG/PKGBUILD"
sed -i "s/SHA256_PLACEHOLDER/$SHA256/g" "$OUTPUT_DIR/$STABLE_PKG/PKGBUILD"

# Git package doesn't need version replacement (pkgver() handles it)
echo "Copying $GIT_PKG (no version replacement needed)..."

echo ""
echo "AUR package files generated successfully in: $OUTPUT_DIR"
echo ""
echo "Contents of $OUTPUT_DIR/$STABLE_PKG/:"
ls -la "$OUTPUT_DIR/$STABLE_PKG/"
echo ""
echo "Contents of $OUTPUT_DIR/$GIT_PKG/:"
ls -la "$OUTPUT_DIR/$GIT_PKG/"

# Cleanup
rm -f "$TEMP_TARBALL"
