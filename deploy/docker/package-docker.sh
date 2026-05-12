#!/bin/bash
# Build a self-contained, offline-installable Docker bundle:
#   liaison-<VERSION>-docker-amd64.tar.gz
#       ├── images/
#       │   ├── liaison.tar    (docker save)
#       │   └── frontier.tar
#       ├── docker-compose.yaml   (release variant — no build: section)
#       ├── .env.example
#       ├── load.sh
#       └── README.md
#
# Prereqs (run from repo root):
#   make package                  # binaries + web + edge installers + frontier bin
#   make images                   # builds liaison/liaison:<tag> and liaison/frontier:<tag>
# This script is invoked by `make package-docker`.

set -eu

# macOS shipped tar preserves xattrs; prefer gtar if available.
if command -v gtar >/dev/null 2>&1; then
    TAR_CMD=gtar
else
    TAR_CMD=tar
fi
export COPYFILE_DISABLE=1

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$REPO_ROOT"

VERSION=$(cat VERSION 2>/dev/null | tr -d 'v' || echo unknown)
REGISTRY="${LIAISON_IMAGE_REGISTRY:-liaison}"
TAG="${LIAISON_IMAGE_TAG:-$VERSION}"

LIAISON_IMAGE="${REGISTRY}/liaison:${TAG}"
FRONTIER_IMAGE="${REGISTRY}/frontier:${TAG}"

PACK_DIR="liaison-${VERSION}-docker-amd64"
OUT_TAR="${PACK_DIR}.tar.gz"

echo -e "${GREEN}Packaging Docker bundle ${VERSION}...${NC}"
echo "  liaison image:  $LIAISON_IMAGE"
echo "  frontier image: $FRONTIER_IMAGE"

for img in "$LIAISON_IMAGE" "$FRONTIER_IMAGE"; do
    if ! docker image inspect "$img" >/dev/null 2>&1; then
        echo -e "${RED}❌ Image $img not found locally. Run 'make images' first.${NC}" >&2
        exit 1
    fi
done

rm -rf "$PACK_DIR" "$OUT_TAR"
mkdir -p "$PACK_DIR/images"

echo -e "${YELLOW}docker save $LIAISON_IMAGE...${NC}"
docker save "$LIAISON_IMAGE" -o "$PACK_DIR/images/liaison.tar"
echo -e "${YELLOW}docker save $FRONTIER_IMAGE...${NC}"
docker save "$FRONTIER_IMAGE" -o "$PACK_DIR/images/frontier.tar"

# Pin the image tag inside the shipped compose via .env, so the release works
# out of the box regardless of what LIAISON_IMAGE_TAG is set to in the user's shell.
cp deploy/docker/docker-compose.release.yaml "$PACK_DIR/docker-compose.yaml"
{
    cat deploy/docker/.env.example
    echo ""
    echo "# Pinned by package-docker.sh — matches the tags baked into images/*.tar"
    echo "LIAISON_IMAGE_REGISTRY=${REGISTRY}"
    echo "LIAISON_IMAGE_TAG=${TAG}"
} > "$PACK_DIR/.env.example.tmp"
# Replace the placeholder registry/tag lines (last occurrence wins with docker compose).
grep -v '^LIAISON_IMAGE_REGISTRY=' "$PACK_DIR/.env.example.tmp" | grep -v '^LIAISON_IMAGE_TAG=' > "$PACK_DIR/.env.example"
echo "LIAISON_IMAGE_REGISTRY=${REGISTRY}" >> "$PACK_DIR/.env.example"
echo "LIAISON_IMAGE_TAG=${TAG}" >> "$PACK_DIR/.env.example"
rm -f "$PACK_DIR/.env.example.tmp"

cp deploy/docker/load.sh "$PACK_DIR/load.sh"
cp deploy/docker/uninstall.sh "$PACK_DIR/uninstall.sh"
cp LICENSE "$PACK_DIR/LICENSE"
chmod +x "$PACK_DIR/load.sh" "$PACK_DIR/uninstall.sh"

cat > "$PACK_DIR/README.md" <<EOF
# Liaison ${VERSION} — Docker Bundle (offline)

Self-contained Docker deployment package. No registry / internet pulls required.

## Contents

\`\`\`
$(cd "$PACK_DIR" && find . -maxdepth 2 -type f | sort | sed 's|^\./||')
\`\`\`

Images shipped:
- \`${LIAISON_IMAGE}\`
- \`${FRONTIER_IMAGE}\`

## Install

\`\`\`bash
./load.sh
\`\`\`

That's it. The script will:

1. \`docker load\` both bundled images.
2. Auto-detect your public IP (fallback: prompt with 30s countdown default).
3. Write \`.env\`, pre-create bind-mount dirs with the right ownership.
4. \`docker compose up -d\`.
5. Tail the liaison log, capture the one-time admin password, and print it.

Open \`https://<LIAISON_PUBLIC_HOST>:<MANAGER_PORT>\` and log in with the printed password.

Re-running \`./load.sh\` on the same host is safe — if \`.env\` already exists it is reused, and \`docker compose up -d\` is a no-op on unchanged services.

## Persistence

First launch creates three host directories next to \`docker-compose.yaml\`, bind-mounted into the containers:

| dir | contents |
|:---|:---|
| \`data/\` | SQLite DB (\`liaison.db\`), \`.initialized\` marker |
| \`certs/\` | \`server.crt\` / \`server.key\` (shared between liaison & frontier) |
| \`logs/\` | liaison process logs |

## Upgrade

1. Download the new bundle, extract it to a fresh directory.
2. Copy over your \`data/\`, \`certs/\`, \`logs/\`, and \`.env\` from the old directory.
3. \`./load.sh\` — it will detect the existing \`.env\` and skip the prompt.

## Uninstall

\`\`\`bash
./uninstall.sh            # stop + remove containers and images, KEEP data/certs/logs/.env
./uninstall.sh --purge    # also delete data/certs/logs/.env — DESTROYS the database
\`\`\`

Re-running \`./load.sh\` after a plain \`uninstall\` reinstalls the stack with all existing users / proxies / certs intact. After \`--purge\` it's a fresh install.

See the repository's \`deploy/docker/README.md\` for the full reference (reverse proxy, custom certs, password reset, etc.).
EOF

echo -e "${YELLOW}Creating $OUT_TAR...${NC}"
$TAR_CMD -czf "$OUT_TAR" "$PACK_DIR"

SIZE=$(du -h "$OUT_TAR" | awk '{print $1}')
echo -e "${GREEN}✅ Built $OUT_TAR ($SIZE)${NC}"
echo
echo "Quick verify:"
echo "  tar -tzf $OUT_TAR | head"
