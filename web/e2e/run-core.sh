#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
e2e_port=${E2E_PORT:-5198}
export E2E_UI_URL=${E2E_UI_URL:-http://127.0.0.1:$e2e_port}
if [[ "${E2E_EXTERNAL_SERVER:-0}" != 1 ]]; then
  ./node_modules/.bin/vite --host 127.0.0.1 --port "$e2e_port" --strictPort &
  vite_pid=$!
  trap 'kill "$vite_pid" 2>/dev/null || true' EXIT
  for attempt in {1..40}; do
    if ! kill -0 "$vite_pid" 2>/dev/null; then
      wait "$vite_pid"
      exit 1
    fi
    if curl -fsS "$E2E_UI_URL/e2e/session-model.html" >/dev/null; then break; fi
    sleep 0.5
  done
  curl -fsS "$E2E_UI_URL/e2e/session-model.html" >/dev/null
fi
for test in models-ui session-model terminal-completion shell-agent-ui product-polish; do
  node "e2e/$test.cjs"
done
