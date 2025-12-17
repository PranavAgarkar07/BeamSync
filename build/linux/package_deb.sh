#!/bin/bash
set -e

# Configuration
APP_NAME="beamsync"
VERSION="0.1.0"
ARCH="amd64"
PKG_NAME="${APP_NAME}_${VERSION}_${ARCH}"
BUILD_DIR="build/linux/deb_pkg"
BINARY_PATH="../../desktop/build/bin/desktop" # Path to wails build output relative to this script
FIREWALL_SCRIPT="firewall_setup.sh"

echo "📦 Packaging $APP_NAME v$VERSION..."

# 1. Clean previous build
rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR"

# 2. Build the application (using Wails)
echo "🔨 Building Wails application..."
cd ../../desktop
wails build -platform linux/amd64
cd ../build/linux

# 3. Create Directory Structure
echo "📂 Creating package structure..."
mkdir -p "$BUILD_DIR/$PKG_NAME/DEBIAN"
mkdir -p "$BUILD_DIR/$PKG_NAME/usr/bin"
mkdir -p "$BUILD_DIR/$PKG_NAME/usr/share/applications"
mkdir -p "$BUILD_DIR/$PKG_NAME/usr/share/$APP_NAME"
mkdir -p "$BUILD_DIR/$PKG_NAME/usr/share/$APP_NAME/sounds"

# 4. Copy Files
echo "qc Copying files..."

# Binary
if [ -f "$BINARY_PATH" ]; then
    cp "$BINARY_PATH" "$BUILD_DIR/$PKG_NAME/usr/bin/$APP_NAME"
    chmod +x "$BUILD_DIR/$PKG_NAME/usr/bin/$APP_NAME"
else
    echo "❌ Binary not found at $BINARY_PATH"
    exit 1
fi

# Firewall Script
cp "$FIREWALL_SCRIPT" "$BUILD_DIR/$PKG_NAME/usr/share/$APP_NAME/$FIREWALL_SCRIPT"
chmod +x "$BUILD_DIR/$PKG_NAME/usr/share/$APP_NAME/$FIREWALL_SCRIPT"

# Sounds
# Assuming sounds are in desktop/build/bin/sounds or similar after build
# Wails puts them in build/bin usually alongside binary
SOUNDS_SRC="../../desktop/build/bin/sounds"
if [ -d "$SOUNDS_SRC" ]; then
    cp -r "$SOUNDS_SRC/"* "$BUILD_DIR/$PKG_NAME/usr/share/$APP_NAME/sounds/"
else
    echo "⚠️ Sounds directory not found at $SOUNDS_SRC"
fi

# Desktop File
cp "beamsync.desktop" "$BUILD_DIR/$PKG_NAME/usr/share/applications/"

# Control File
cp "control" "$BUILD_DIR/$PKG_NAME/DEBIAN/control"

# 5. Build .deb manually (ar + tar) since dpkg-deb is missing
echo "💿 Building .deb manually..."

cd "$BUILD_DIR/$PKG_NAME"

# Create debian-binary
echo "2.0" > debian-binary

# Create control.tar.gz
# DEBIAN contents should be at root of this tar
cd DEBIAN
tar -czf ../control.tar.gz .
cd ..

# Create data.tar.gz
# usr/ contents should be at root of this tar
tar -czf data.tar.gz usr

# Assemble .deb using ar
# Order is critical: debian-binary, control.tar.gz, data.tar.gz
ar rCS "../$PKG_NAME.deb" debian-binary control.tar.gz data.tar.gz

cd ..
echo "✅ Package created: $BUILD_DIR/$PKG_NAME.deb"

