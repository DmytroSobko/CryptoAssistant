#!/usr/bin/env sh
set -eu

cleanup() {
  if [ -n "${backend_pid:-}" ]; then
    kill "$backend_pid" 2>/dev/null || true
    wait "$backend_pid" 2>/dev/null || true
  fi
}
trap cleanup EXIT INT TERM

(cd backend && go run ./cmd/server) &
backend_pid=$!

echo "Starting Crypto Strategy Assistant. API: http://localhost:8080"
cd frontend
npm run tauri:dev

