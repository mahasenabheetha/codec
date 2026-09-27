#!/usr/bin/env bash
# Time codec on large inputs: a 10k-file repo, a 5 MB file and a chart
# rendering 500 documents. Budgets: design/phases/14-release.md.
#   scripts/perf.sh [dir]   fixtures go in dir (default: a temp folder)
# Uses a throwaway settings folder; schemas download on first use.
set -euo pipefail
cd "$(dirname "$0")/.."

dir="${1:-$(mktemp -d)}"
[[ -d "$dir/repo10k" ]] || go run ./scripts/perfgen "$dir"
now() { perl -MTime::HiRes=time -e 'printf "%d", time * 1000'; } # ms; macOS date has no %N
path() { if command -v cygpath >/dev/null; then cygpath -m "$1"; else echo "$1"; fi; }

bin=./codec-dev
[[ "$(go env GOOS)" == windows ]] && bin=./codec-dev.exe
go build -o "$bin" ./cmd/codec
port=8799
CODEC_CONFIG_DIR="$(mktemp -d)" "$bin" serve --port $port >/dev/null &
server=$!
trap 'kill "$server" 2>/dev/null' EXIT
base=http://127.0.0.1:$port
for _ in $(seq 50); do curl -sf "$base/" >/dev/null && break; sleep 0.1; done
token=$(curl -s "$base/" | grep -o 'name="codec-token" content="[^"]*"' | sed 's/.*content="//;s/"//')

# call METHOD PATH [BODY]: prints status, time and size; the body lands in $out.
out=$(mktemp)
call() {
  curl -s -o "$out" -w '%{http_code} %{time_total}s %{size_download}B' -X "$1" \
    -H "X-Codec-Token: $token" -H 'Content-Type: application/json' ${3:+-d "$3"} "$base$2"
}
open() { call POST /api/v2/workspace/open "{\"path\":\"$(path "$1")\"}" >/dev/null; }

echo "== 10k-file repo"
# The server says "tree" once every file has its type; polling the tree
# instead would slow that down.
events=$(mktemp)
curl -sN "$base/api/v2/events?token=$token" >"$events" &
listener=$!
trap 'kill "$server" "$listener" 2>/dev/null' EXIT
sleep 0.3
start=$(now)
open "$dir/repo10k"
echo "open        $(( $(now) - start )) ms"
echo "tree        $(call GET /api/v2/files/tree)"
until grep -q '^event: tree' "$events"; do sleep 0.05; done
echo "types       $(( $(now) - start )) ms after opening"
kill "$listener"
echo "lint        $(call POST /api/v2/lint/workspace '{}')"
echo "resources   $(call POST /api/v2/k8s/analyze '{"kind":"folder","path":"."}')"

echo "== 5 MB file"
open "$dir/big"
echo "read        $(call GET '/api/v2/files/content?path=big.yaml')"
echo "analyze     $(call POST /api/v2/yaml/analyze '{"path":"big.yaml"}')"
echo "analyze     $(call POST /api/v2/yaml/analyze '{"path":"big.yaml"}') (again)"

echo "== 500-document chart"
open "$dir/chart500"
for i in 1 2 3; do echo "render $i    $(call POST /api/v2/helm/render '{"chart":"many"}')"; done
