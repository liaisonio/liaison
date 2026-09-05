#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
RESOLVER="$ROOT_DIR/scripts/resolve-release-version.sh"

assert_output() {
    local tag="$1"
    local expected="$2"
    local actual

    actual="$($RESOLVER "$tag")"
    if [ "$actual" != "$expected" ]; then
        echo "Unexpected output for $tag" >&2
        diff -u <(printf '%s\n' "$expected") <(printf '%s\n' "$actual") >&2 || true
        exit 1
    fi
}

assert_rejected() {
    local tag="$1"
    if "$RESOLVER" "$tag" >/dev/null 2>&1; then
        echo "Expected invalid tag to be rejected: $tag" >&2
        exit 1
    fi
}

assert_output "v1.9.0" $'tag=v1.9.0\nversion=1.9.0\nprerelease=false'
assert_output "v1.9.0-rc.1" $'tag=v1.9.0-rc.1\nversion=1.9.0-rc.1\nprerelease=true'

assert_rejected "1.9.0"
assert_rejected "v1.9"
assert_rejected "v1.9.0-rc"
assert_rejected "v1.9.0-beta.1"
assert_rejected "v1.9.0-rc.0-extra"
assert_rejected "v01.9.0"

echo "release version tests passed"
