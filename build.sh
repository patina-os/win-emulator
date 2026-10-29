#!/usr/bin/env bash
# SPDX-License-Identifier: GPL-3.0-or-later
# Build TryPatina.exe: patina's small icon from patina's branding/export/, the Windows
# resources (icon, manifest, version), then the launcher, cross-compiled from any OS.
set -euo pipefail

version="${1:-dev}"
# The patina tag whose branding/export/ the icon comes from; the mark is never copied
# into this repository (patina's CLAUDE.md, "Never").
branding_ref="${PATINA_BRANDING_REF:-v0.1.0-alpha.3}"
winres_version="v0.3.3"
here="$(cd "$(dirname "$(readlink -f "$0")")" && pwd)"
cd "$here"

mkdir -p assets
for size in 32 64 256; do
  f="assets/icon-small-$size.png"
  [[ -s "$f" ]] && continue
  echo "build: fetching $f from patina $branding_ref"
  curl -fsSL --retry 4 -o "$f" \
    "https://raw.githubusercontent.com/patina-os/patina/$branding_ref/branding/export/icon-small-$size.png"
done

# The file version wants four numbers: v0.2.1-alpha.1 becomes 0.2.1.0.
numeric="$(printf '%s' "${version#v}" | sed -E 's/^([0-9]+)\.([0-9]+)\.([0-9]+).*/\1.\2.\3.0/')"
[[ "$numeric" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.0$ ]] || numeric="0.0.0.0"
go run "github.com/tc-hib/go-winres@$winres_version" simply \
  --arch amd64 --icon assets/icon-small-256.png --manifest gui \
  --product-name "Try patina" --file-description "Try patina for Windows" \
  --product-version "$version" --file-version "$numeric" \
  --copyright "GPL-3.0-or-later, the patina contributors" --original-filename TryPatina.exe

mkdir -p dist
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath \
  -ldflags "-H windowsgui -s -w -X main.version=$version" -o dist/TryPatina.exe .
(cd dist && sha256sum TryPatina.exe > TryPatina.exe.sha256)
echo "build: dist/TryPatina.exe ($version)"
