#!/usr/bin/env bash
set -euo pipefail

APP_PATH="${1:?app path required}"
OUT_DMG="${2:?output dmg path required}"
VOLUME_NAME="${3:-mcpvui}"

if [ ! -d "$APP_PATH" ]; then
  echo "App bundle not found: $APP_PATH" >&2
  exit 1
fi

STAGING_DIR="$(mktemp -d)"
cleanup() {
  rm -rf "$STAGING_DIR"
}
trap cleanup EXIT

cp -R "$APP_PATH" "$STAGING_DIR/"
if [ -f "$(dirname "$APP_PATH")/install-helper.sh" ]; then
  cp "$(dirname "$APP_PATH")/install-helper.sh" "$STAGING_DIR/"
  chmod +x "$STAGING_DIR/install-helper.sh"
fi
ln -s /Applications "$STAGING_DIR/Applications"

mkdir -p "$(dirname "$OUT_DMG")"
hdiutil create -volname "$VOLUME_NAME" -srcfolder "$STAGING_DIR" -ov -format UDZO "$OUT_DMG"
