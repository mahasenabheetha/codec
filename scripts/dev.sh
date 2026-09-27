#!/usr/bin/env bash
# Build and run codec locally (Git Bash on Windows, or any bash on macOS/Linux).
#   scripts/dev.sh build        build frontend + binary (./codec-dev)
#   scripts/dev.sh run [port]   build, then serve on port (default 8766)
#   scripts/dev.sh ui [port]    build, serve, and start the hot-reload UI dev server
set -euo pipefail
cd "$(dirname "$0")/.."

bin=./codec-dev
[[ "$(go env GOOS)" == windows ]] && bin=./codec-dev.exe
port="${2:-8766}"

build() {
  # Reinstall frontend deps only when the lockfile changed since the last install.
  if [[ frontend/package-lock.json -nt frontend/node_modules/.package-lock.json ]]; then
    npm --prefix frontend ci
  fi
  npm --prefix frontend run build
  go build -o "$bin" ./cmd/codec
  echo "built $bin — $("$bin" version)"
}

case "${1:-run}" in
  build)
    build
    ;;
  run)
    build
    echo "UI: http://127.0.0.1:$port/"
    exec "$bin" serve --port "$port"
    ;;
  ui)
    build
    "$bin" serve --port "$port" &
    server=$!
    trap 'kill "$server" 2>/dev/null' EXIT
    echo "hot-reload UI: http://localhost:5173/"
    CODEC_API="http://127.0.0.1:$port" npm --prefix frontend run dev
    ;;
  *)
    sed -n '2,5p' "$0"
    exit 1
    ;;
esac
