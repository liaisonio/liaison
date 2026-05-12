#!/usr/bin/env bash

set -euo pipefail

usage() {
    echo "Usage: $0 <version>"
    echo "Example: $0 1.6.0"
    echo "         $0 v1.6.0"
}

if [ "${1:-}" = "" ]; then
    usage >&2
    exit 1
fi

RAW_VERSION="$1"
VERSION="${RAW_VERSION#v}"
TAG="v${VERSION}"

if ! [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "Error: invalid version '${RAW_VERSION}'. Expect 1.6.0 or v1.6.0" >&2
    exit 1
fi

cd "$(dirname "$0")/.."

echo "$VERSION" > VERSION

README_FILES=(
    README.md
    README_en.md
    README_ja.md
    README_ko.md
    README_es.md
    README_fr.md
    README_de.md
)

for file in "${README_FILES[@]}"; do
    [ -f "$file" ] || continue

    VERSION="$VERSION" TAG="$TAG" perl -0pi -e '
        my $version = $ENV{"VERSION"};
        my $tag = $ENV{"TAG"};

        s{Version-v[0-9]+\.[0-9]+\.[0-9]+}{Version-$tag}g;
        s{releases/download/v[0-9]+\.[0-9]+\.[0-9]+}{releases/download/$tag}g;
        s{liaison-[0-9]+\.[0-9]+\.[0-9]+-(linux|docker)-amd64\.tar\.gz}{liaison-$version-$1-amd64.tar.gz}g;
        s{cd liaison-[0-9]+\.[0-9]+\.[0-9]+-(linux|docker)-amd64}{cd liaison-$version-$1-amd64}g;

        s{Liaison_0\.1\.0_universal\.dmg}{Liaison-Desktop-dev-feat-desktop-client-macOS-universal.dmg}g;
        s{Liaison_0\.1\.0_x64_en-US\.msi}{Liaison-Desktop-dev-feat-desktop-client-Windows-x64.msi}g;
        s{Liaison_0\.1\.0_x64-setup\.exe}{Liaison-Desktop-dev-feat-desktop-client-Windows-x64-setup.exe}g;
    ' "$file"
done

echo "Updated release version to ${TAG}"
