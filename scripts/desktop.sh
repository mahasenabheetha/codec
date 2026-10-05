#!/usr/bin/env bash
# Build the desktop app for Windows (Git Bash on Windows).
#   scripts/desktop.sh            portable exe and installer in dist/desktop/
#   scripts/desktop.sh exe        portable exe only
# VERSION=v3.0.0 sets the version (default: git describe). The installer
# needs NSIS 3: makensis on PATH, or MAKENSIS=/path/to/makensis.exe.
set -euo pipefail
cd "$(dirname "$0")/.."

wails=github.com/wailsapp/wails/v3/cmd/wails3@$(go list -m -f '{{.Version}}' github.com/wailsapp/wails/v3)
out=dist/desktop
version="${VERSION:-$(git describe --tags --always --dirty)}"
# Windows wants four numbers (3.0.0.0); anything else counts as 0.0.0.
if [[ "$version" =~ ^v?([0-9]+)\.([0-9]+)\.([0-9]+) ]]; then
  numeric="${BASH_REMATCH[1]}.${BASH_REMATCH[2]}.${BASH_REMATCH[3]}"
else
  numeric="0.0.0"
fi
mkdir -p "$out"

# 1. The UI, embedded in the binary (reinstall packages only when the
#    lockfile changed, as scripts/dev.sh does).
if [[ frontend/package-lock.json -nt frontend/node_modules/.package-lock.json ]]; then
  npm --prefix frontend ci
fi
npm --prefix frontend run build

# 2. Windows resources: icon, version details, manifest. Go links any
#    .syso file in the package; it is generated, not committed.
info=$(mktemp)
trap 'rm -f "$info"' EXIT
cat > "$info" <<EOF
{
  "fixed": { "file_version": "$numeric.0", "product_version": "$numeric.0" },
  "info": {
    "0409": {
      "ProductName": "codec",
      "ProductVersion": "$version",
      "FileDescription": "codec",
      "CompanyName": "Mahasen Abheetha",
      "LegalCopyright": "MIT License"
    }
  }
}
EOF
go run "$wails" generate syso -arch amd64 \
  -icon desktop/windows/icon.ico -manifest desktop/windows/manifest.xml \
  -info "$info" -out cmd/codec-desktop/rsrc_windows_amd64.syso

# 3. The exe: a window app (no console), stamped like the release CLI.
pkg=github.com/mahasenabheetha/codec/v2/internal/version
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath \
  -ldflags "-s -w -H windowsgui -X $pkg.Version=$version -X $pkg.Commit=$(git rev-parse HEAD) -X $pkg.Date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -o "$out/codec-desktop.exe" ./cmd/codec-desktop
echo "built $out/codec-desktop.exe ($version)"
[[ "${1:-}" == exe ]] && exit 0

# 4. The per-user installer.
makensis="${MAKENSIS:-makensis}"
"$makensis" -V2 -DVERSION="${version#v}" -DVERSION4="$numeric.0" \
  -DEXE="$(cygpath -w "$PWD/$out/codec-desktop.exe" 2>/dev/null || echo "$PWD/$out/codec-desktop.exe")" \
  -DOUTFILE="$(cygpath -w "$PWD/$out/codec-setup.exe" 2>/dev/null || echo "$PWD/$out/codec-setup.exe")" \
  desktop/windows/installer.nsi
echo "built $out/codec-setup.exe"
