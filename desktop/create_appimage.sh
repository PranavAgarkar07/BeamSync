#!/bin/bash
set -e

# Configuration
APP_NAME="BeamSync"
BUILD_DIR="build/bin"
APP_DIR="AppDir"
ICON_PATH="build/appicon/icon.png"

echo "🚀 Starting AppImage Creation for $APP_NAME..."

# 1. Download appimagetool if not present
if [ ! -f "appimagetool-x86_64.AppImage" ]; then
    echo "⬇️ Downloading appimagetool..."
    wget -q https://github.com/AppImage/appimagetool/releases/download/continuous/appimagetool-x86_64.AppImage
    chmod +x appimagetool-x86_64.AppImage
fi

# 2. Build the Application
echo "🔨 Building BeamSync..."
wails build -platform linux/amd64

# 3. Create AppDir Structure
echo "📂 Preparing AppDir..."
rm -rf $APP_DIR
mkdir -p $APP_DIR/usr/bin
mkdir -p $APP_DIR/usr/share/icons/hicolor/256x256/apps

# 4. Copy Files
cp "$BUILD_DIR/$APP_NAME" "$APP_DIR/usr/bin/$APP_NAME"
cp "$ICON_PATH" "$APP_DIR/$APP_NAME.png"
cp "$ICON_PATH" "$APP_DIR/.DirIcon"

# 5. Create Desktop File
echo "📝 Creating desktop entry..."
cat > "$APP_DIR/$APP_NAME.desktop" <<EOF
[Desktop Entry]
Name=$APP_NAME
Exec=$APP_NAME
Icon=$APP_NAME
Type=Application
Categories=Utility;Network;
Comment=Fast Local File Transfer
Terminal=false
StartupWMClass=$APP_NAME
EOF

# 6. Create AppRun
echo "🏃 Creating AppRun..."
# Simple symlink approach usually works for self-contained binaries
ln -s usr/bin/$APP_NAME $APP_DIR/AppRun

# 7. Generate AppImage
echo "📦 Packaging AppImage..."
# ARCH=x86_64 ./appimagetool-x86_64.AppImage $APP_DIR
# Using --no-appstream because we might not have appstream metadata which can cause fail
ARCH=x86_64 ./appimagetool-x86_64.AppImage --no-appstream $APP_DIR

echo "✅ AppImage created successfully!"
ls -lh *.AppImage
