#!/usr/bin/env bash
# Build a macOS .app bundle with icon from theme/icons/asm2hex.png
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN_SRC="${1:-$ROOT/build/asm2hex}"
APP_NAME="ASM to HEX Converter"
APP_DIR="${2:-$ROOT/build/${APP_NAME}.app}"
PNG="$ROOT/theme/icons/asm2hex.png"
ICONSET="$ROOT/build/AppIcon.iconset"
ICNS="$ROOT/build/AppIcon.icns"

if [[ ! -f "$BIN_SRC" ]]; then
  echo "binary not found: $BIN_SRC" >&2
  exit 1
fi
if [[ ! -f "$PNG" ]]; then
  echo "icon not found: $PNG" >&2
  exit 1
fi

rm -rf "$APP_DIR" "$ICONSET" "$ICNS"
mkdir -p "$ICONSET"

# Generate iconset sizes (sips is built-in on macOS)
sips -z 16 16     "$PNG" --out "$ICONSET/icon_16x16.png" >/dev/null
sips -z 32 32     "$PNG" --out "$ICONSET/icon_16x16@2x.png" >/dev/null
sips -z 32 32     "$PNG" --out "$ICONSET/icon_32x32.png" >/dev/null
sips -z 64 64     "$PNG" --out "$ICONSET/icon_32x32@2x.png" >/dev/null
sips -z 128 128   "$PNG" --out "$ICONSET/icon_128x128.png" >/dev/null
sips -z 256 256   "$PNG" --out "$ICONSET/icon_128x128@2x.png" >/dev/null
sips -z 256 256   "$PNG" --out "$ICONSET/icon_256x256.png" >/dev/null
sips -z 512 512   "$PNG" --out "$ICONSET/icon_256x256@2x.png" >/dev/null
sips -z 512 512   "$PNG" --out "$ICONSET/icon_512x512.png" >/dev/null
sips -z 1024 1024 "$PNG" --out "$ICONSET/icon_512x512@2x.png" >/dev/null

iconutil -c icns "$ICONSET" -o "$ICNS"

mkdir -p "$APP_DIR/Contents/MacOS" "$APP_DIR/Contents/Resources"
cp "$BIN_SRC" "$APP_DIR/Contents/MacOS/${APP_NAME}"
chmod +x "$APP_DIR/Contents/MacOS/${APP_NAME}"
cp "$ICNS" "$APP_DIR/Contents/Resources/AppIcon.icns"
cp "$ROOT/packaging/macos/Info.plist" "$APP_DIR/Contents/Info.plist"

echo "created $APP_DIR"
