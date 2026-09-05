#!/usr/bin/env bash

set -euo pipefail

TAG="${1:-}"
OUTPUT_FILE="${2:-}"

if ! [[ "$TAG" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-rc\.(0|[1-9][0-9]*))?$ ]]; then
    echo "Error: release tag must match vMAJOR.MINOR.PATCH or vMAJOR.MINOR.PATCH-rc.NUMBER, got '${TAG}'" >&2
    exit 2
fi

VERSION="${TAG#v}"
PRERELEASE=false
if [[ "$VERSION" == *-rc.* ]]; then
    PRERELEASE=true
fi

emit() {
    printf 'tag=%s\n' "$TAG"
    printf 'version=%s\n' "$VERSION"
    printf 'prerelease=%s\n' "$PRERELEASE"
}

if [ -n "$OUTPUT_FILE" ]; then
    emit >> "$OUTPUT_FILE"
else
    emit
fi
