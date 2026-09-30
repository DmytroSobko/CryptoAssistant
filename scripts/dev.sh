#!/usr/bin/env sh
set -eu

runtime_dir="$(mktemp -d "${TMPDIR:-/tmp}/crypto-assistant-server.XXXXXX")"
backend_bin="$runtime_dir/server"

cleanup() {
  if [ -n "${backend_pid:-}" ]; then
    kill "$backend_pid" 2>/dev/null || true
    wait "$backend_pid" 2>/dev/null || true
  fi
  rm -rf "$runtime_dir"
}
trap cleanup EXIT INT TERM

(
  cd backend
  go build -o "$backend_bin" ./cmd/server
)
(
  cd backend
  exec "$backend_bin"
) &
backend_pid=$!

# Migrations run before the server begins listening. Fail fast instead of
# opening the desktop UI against a missing or locked backend.
sleep 1
if ! kill -0 "$backend_pid" 2>/dev/null; then
  echo "Backend server failed during startup." >&2
  wait "$backend_pid" 2>/dev/null || true
  exit 1
fi

echo "Starting Crypto Strategy Assistant. API: http://localhost:8080"
cd frontend
npm run tauri:dev
