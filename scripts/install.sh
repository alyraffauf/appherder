#!/bin/bash
set -eu

REPO="alyraffauf/appherder"
DEST="$HOME/AppImages"
BIN="$HOME/.local/bin"

if [ -n "${GH_TOKEN:-}" ]; then
  AUTH="Authorization: Bearer $GH_TOKEN"
elif [ -n "${GITHUB_TOKEN:-}" ]; then
  AUTH="Authorization: Bearer $GITHUB_TOKEN"
else
  AUTH=""
fi

echo "Fetching latest release..."
if [ -n "$AUTH" ]; then
  URL=$(curl -sL -H "$AUTH" "https://api.github.com/repos/$REPO/releases/latest" |
    grep -o '"browser_download_url": "[^"]*AppImage"' |
    head -1 | cut -d'"' -f4)
else
  URL=$(curl -sL "https://api.github.com/repos/$REPO/releases/latest" |
    grep -o '"browser_download_url": "[^"]*AppImage"' |
    head -1 | cut -d'"' -f4)
fi

if [ -z "$URL" ]; then
  echo "Could not find AppImage asset in latest release" >&2
  exit 1
fi

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
APPIMAGE="$TMP/appherder.AppImage"

echo "Downloading..."
curl -sLo "$APPIMAGE" "$URL"
chmod +x "$APPIMAGE"

echo "Installing..."
mkdir -p "$DEST" "$BIN"
"$APPIMAGE" install "$APPIMAGE"

echo "Enabling background services..."
"$BIN/appherder" autosync
"$BIN/appherder" autoupgrade

echo
echo "appherder installed. Run 'appherder --help' to get started."
echo "If 'appherder' is not found, re-open your terminal or run:"
echo "  export PATH=\"$BIN:\$PATH\""
echo
echo "Coming from another AppImage manager? Run 'appherder migrate' to adopt"
echo "existing apps in $DEST."
