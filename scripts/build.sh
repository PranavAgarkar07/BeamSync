#!/usr/bin/env bash
set -euo pipefail

# BeamSync — build helper
# Builds the Wails desktop app with the version from desktop/VERSION.
# Usage: ./scripts/build.sh [--webkit2_41]

APP="BeamSync"
TAGS=""
BUILD_DIR="desktop/build/bin"

if [[ "${1:-}" == "--webkit2_41" ]]; then
  TAGS="-tags webkit2_41"
fi

VERSION=$(tr -d '[:space:]' < desktop/VERSION)
echo ":: Building $APP v$VERSION ..."

cd desktop
wails build $TAGS -o "$APP"

echo ":: Built: $(realpath build/bin/$APP)"
echo ":: Version: v$VERSION"
ls -lh "build/bin/$APP"
