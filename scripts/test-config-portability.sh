#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

bash -n deploy-liaison.sh
env -u MANAGER_HOST -u EDGE_HOST bash deploy-liaison.sh --help >/dev/null

expect_missing_host() {
    local expected="$1"
    shift
    local output
    if output=$(env -u MANAGER_HOST -u EDGE_HOST bash deploy-liaison.sh "$@" 2>&1); then
        echo "Expected deployment without a target to fail" >&2
        exit 1
    fi
    if [[ "$output" != *"$expected"* ]]; then
        echo "Expected missing-host validation before build or network access" >&2
        exit 1
    fi
}

expect_missing_host MANAGER_HOST --web
expect_missing_host MANAGER_HOST --liaison
expect_missing_host MANAGER_HOST --liaison-only
expect_missing_host EDGE_HOST --edge
expect_missing_host MANAGER_HOST

# Development configuration must require locally supplied credentials.
awk '
  /^[[:space:]]*(jwt_secret|access_key|secret_key):/ {
    count++
    value = $0
    sub(/^[^:]*:[[:space:]]*/, "", value)
    sub(/[[:space:]]*#.*/, "", value)
    if (value != "\"\"") invalid = 1
  }
  END { if (count != 3 || invalid) exit 1 }
' etc/liaison.yaml etc/liaison-edge.yaml

for path in etc/liaison.local.yaml etc/liaison-edge.local.yaml .env.local; do
    git check-ignore -q "$path"
done

echo "Configuration portability checks passed"
