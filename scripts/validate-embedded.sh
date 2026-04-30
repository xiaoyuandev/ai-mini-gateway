#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_DIR="$ROOT_DIR/bin"
BIN_PATH="$BIN_DIR/ai-mini-gateway"
DATA_DIR="$ROOT_DIR/.tmp/embedded-data"
HOST="127.0.0.1"
PORT="3457"
BASE_URL="http://${HOST}:${PORT}"

mkdir -p "$BIN_DIR" "$DATA_DIR"

cleanup() {
  if [[ -n "${GATEWAY_PID:-}" ]] && kill -0 "$GATEWAY_PID" >/dev/null 2>&1; then
    kill "$GATEWAY_PID" >/dev/null 2>&1 || true
    wait "$GATEWAY_PID" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

go build -o "$BIN_PATH" ./cmd/gateway

"$BIN_PATH" --host "$HOST" --port "$PORT" --data-dir "$DATA_DIR" --models-cache-ttl 15s &
GATEWAY_PID=$!

for _ in {1..20}; do
  if curl -fsS "$BASE_URL/health" >/dev/null 2>&1; then
    break
  fi
  sleep 0.5
done

curl -fsS "$BASE_URL/health"
curl -fsS "$BASE_URL/capabilities"
