#!/bin/sh
# Builds dist/StudioAlbum-x86_64-<version>.spk and dist/release.json (Linux/macOS/Git Bash). Requires Node.js and Go.
# Usage: ./build.sh <version> <build-number> [owner/repo]
set -e
VERSION="${1:-1.0.0}"
BUILD="${2:-1}"
REPO="${3:-${GITHUB_REPOSITORY:-}}"
ROOT="$(cd "$(dirname "$0")" && pwd)"
[ -d "$HOME/go-sdk/go/bin" ] && PATH="$HOME/go-sdk/go/bin:$PATH"
PKGV="$(printf '%s-%04d' "$VERSION" "$BUILD")"
SPK="$ROOT/dist/StudioAlbum-x86_64-$VERSION.spk"

echo "==> icons";    (cd "$ROOT/backend" && go run ./tools/mkicons ..)
echo "==> frontend"; (cd "$ROOT/frontend" && { [ -d node_modules ] || npm ci; } && npm run build)
echo "==> backend (linux/amd64) $PKGV repo=$REPO"
mkdir -p "$ROOT/dist"
(cd "$ROOT/backend" && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath \
  -ldflags "-s -w -X main.Version=$PKGV -X main.UpdateRepo=$REPO" -o "$ROOT/dist/studioalbum-linux-amd64" .)
echo "==> spk"
(cd "$ROOT/backend" && go run ./tools/mkspk -root .. -bin "$ROOT/dist/studioalbum-linux-amd64" -version "$PKGV" -out "$SPK")
MD5="$(md5sum "$SPK" | cut -d' ' -f1)"
SIZE="$(wc -c < "$SPK" | tr -d ' ')"
printf '{"version":"%s","md5":"%s","size":%s}\n' "$PKGV" "$MD5" "$SIZE" > "$ROOT/dist/release.json"
cat "$ROOT/dist/release.json"
