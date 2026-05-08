#!/usr/bin/env bash
set -euo pipefail

# BeamSync — direct binary install
# Usage: curl -fsSL https://raw.githubusercontent.com/PranavAgarkar07/BeamSync/main/scripts/install.sh | bash
# Pass a version as arg to override: install.sh 2.3.0

APP=beamsync
REPO="PranavAgarkar07/BeamSync"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [[ -f "$SCRIPT_DIR/../desktop/VERSION ]]; then
  DEFAULT_VERSION=$(tr -d '[:space:]' < "$SCRIPT_DIR/../desktop/VERSION")
else
  DEFAULT_VERSION="2.4.0"
fi
VERSION=${1:-$DEFAULT_VERSION}
BINDIR=${BINDIR:-/usr/local/bin}
DATADIR=${DATADIR:-/usr/local/share}

echo ":: Downloading BeamSync v$VERSION..."

URL="https://github.com/$REPO/releases/download/v$VERSION/BeamSync"
if ! curl -fsSL "$URL" -o /tmp/BeamSync; then
  echo ":: Binary not found at $URL"
  echo ":: Download from GitHub Releases: https://github.com/$REPO/releases"
  exit 1
fi

chmod +x /tmp/BeamSync

# Check webkit2gtk-4.1 runtime dependency
if ! ldconfig -p | grep -q libwebkit2gtk-4.1; then
  echo ":: WARNING: libwebkit2gtk-4.1 not found."
  echo "   Install it:"
  echo "     Arch:  sudo pacman -S webkit2gtk-4.1 gtk3"
  echo "     Fedora:sudo dnf install webkit2gtk4.1 gtk3"
  echo "     Debian:sudo apt install libwebkit2gtk-4.1-dev gtk3"
fi

echo ":: Installing to $BINDIR/$APP"
sudo install -Dm755 /tmp/BeamSync "$BINDIR/$APP"

echo ":: Installing desktop entry"
sudo install -Dm644 /dev/stdin "$DATADIR/applications/$APP.desktop" <<EOF
[Desktop Entry]
Name=BeamSync
Comment=Secure local peer-to-peer file transfer
Exec=$BINDIR/$APP
Icon=$APP
Terminal=false
Type=Application
Categories=Network;FileTransfer;
StartupNotify=true
EOF

echo ":: Done! Run: beamsync"
