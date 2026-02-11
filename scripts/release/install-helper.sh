#!/usr/bin/env bash
set -euo pipefail

APP_PATH="$(dirname "$0")/mcpvui.app"

echo "Preparing to install mcpvui..."

if [ -d "$APP_PATH" ]; then
  xattr -rd com.apple.quarantine "$APP_PATH" 2>/dev/null || true
fi

echo "Setup complete."
echo ""
echo "Drag mcpvui.app into Applications."
echo "Then right-click the app and choose \"Open\" for the first launch."
echo ""
echo "Future updates will install automatically."

open "$(dirname "$0")"
