#!/usr/bin/env bash
# Build every CDF artifact from source into dist/out/:
#   cdf                         terminal viewer/editor (Bun single-file binary, Linux x64)
#   cdf-gui                     GUI viewer/editor (Go + Fyne, Linux x64)
#   cdf_<VERSION>_amd64.deb     Debian/Ubuntu package with both programs, .cdf MIME type and icon
#   cdf-viewer-<VERSION>-setup.exe   Windows installer (GUI, Start Menu, .cdf file association)
#   SHA256SUMS
#
# Needs: bun, go >= 1.22 with a C compiler (Fyne uses cgo), the Fyne build packages
# (libgl1-mesa-dev xorg-dev), dpkg-deb, and for Windows x86_64-w64-mingw32-gcc + makensis.
# usage: dist/build.sh            (VERSION=1.2.3 dist/build.sh to override)
set -euo pipefail
VERSION=${VERSION:-1.0.1}
ROOT=$(cd "$(dirname "$0")/.." && pwd)
B=$ROOT/dist/build
OUT=$ROOT/dist/out
rm -rf "$OUT"; mkdir -p "$OUT"

echo "== cdf (terminal) $VERSION"
bun build --compile "$ROOT/Tools/cdf.ts" --outfile "$OUT/cdf"

echo "== cdf-gui (Linux)"
(cd "$ROOT/GUI" && CGO_ENABLED=1 go build -trimpath -buildvcs=false -ldflags "-s -w" -o "$OUT/cdf-gui" .)

echo "== cdf_${VERSION}_amd64.deb"
STAGE=$(mktemp -d); trap 'rm -rf "$STAGE"' EXIT
cp -a "$B/deb/." "$STAGE/"
install -Dm755 "$OUT/cdf" "$STAGE/usr/bin/cdf"
install -Dm755 "$OUT/cdf-gui" "$STAGE/usr/bin/cdf-gui"
sed -i "s/^Version: .*/Version: $VERSION/" "$STAGE/DEBIAN/control"
SIZE=$(du -sk --exclude=DEBIAN "$STAGE" | cut -f1)
sed -i "s/^Installed-Size: .*/Installed-Size: $SIZE/" "$STAGE/DEBIAN/control"
find "$STAGE" -type d -exec chmod 755 {} +
find "$STAGE" -type f -exec chmod 644 {} +
chmod 755 "$STAGE/usr/bin/cdf" "$STAGE/usr/bin/cdf-gui" "$STAGE/DEBIAN/postinst" "$STAGE/DEBIAN/postrm"
dpkg-deb --root-owner-group --build "$STAGE" "$OUT/cdf_${VERSION}_amd64.deb" >/dev/null

echo "== cdf-gui.exe + Windows installer"
(cd "$ROOT/GUI" && CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc \
  go build -trimpath -buildvcs=false -ldflags "-s -w -H windowsgui" -o "$B/cdf-gui.exe" .)
(cd "$B" && makensis -V2 -DVERSION="$VERSION" cdf-installer.nsi)
mv "$B/cdf-viewer-${VERSION}-setup.exe" "$OUT/"

(cd "$OUT" && sha256sum "cdf_${VERSION}_amd64.deb" "cdf-viewer-${VERSION}-setup.exe" cdf cdf-gui > SHA256SUMS)
ls -l "$OUT"
