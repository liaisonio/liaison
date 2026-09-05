#!/bin/bash
# Uninstall the Liaison Docker bundle.
#   default       stop + remove containers and bundled images; KEEP data/certs/logs/.env
#   --purge       additionally delete data/, certs/, logs/, .env  (destroys the database)
set -eu

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
cd "$SCRIPT_DIR"

GREEN=$'\033[0;32m'
YELLOW=$'\033[1;33m'
RED=$'\033[0;31m'
BOLD=$'\033[1m'
NC=$'\033[0m'

PURGE=0
for arg in "$@"; do
    case "$arg" in
        --purge|-p) PURGE=1 ;;
        -h|--help)
            cat <<EOF
Usage: ./uninstall.sh [--purge]

  (no flag)    stop & remove containers and images; keep data/certs/logs/.env
  --purge      also delete data/, certs/, logs/, .env  — DESTROYS the database

Re-run ./install.sh afterwards to reinstall. Without --purge the existing user
accounts, proxies, and TLS cert are restored; with --purge the install is fresh.
EOF
            exit 0
            ;;
        *) printf "${RED}Unknown flag: %s${NC}\n" "$arg" >&2; exit 2 ;;
    esac
done

if ! command -v docker >/dev/null 2>&1; then
    printf "${RED}docker not found on PATH.${NC}\n" >&2
    exit 1
fi

if docker compose version >/dev/null 2>&1; then
    DC="docker compose"
elif command -v docker-compose >/dev/null 2>&1; then
    DC="docker-compose"
else
    printf "${RED}docker compose plugin not found.${NC}\n" >&2
    exit 1
fi

# Confirmation for destructive runs.
if [ "$PURGE" -eq 1 ]; then
    printf "${BOLD}${RED}This will DELETE data/, certs/, logs/, and .env — the SQLite DB is gone forever.${NC}\n"
    printf "Type ${BOLD}yes${NC} to continue: "
    read -r CONFIRM </dev/tty
    if [ "$CONFIRM" != "yes" ]; then
        printf "${YELLOW}Aborted.${NC}\n"
        exit 1
    fi
fi

printf "${GREEN}==> Stopping containers${NC}\n"
$DC down 2>&1 | sed 's/^/  /' || true

printf "${GREEN}==> Removing bundled images${NC}\n"
# Pick up the tag this bundle shipped with. Fall back to .env.example if .env
# was deleted or the user ran --purge previously.
if [ -f .env ]; then
    set -a; . ./.env; set +a
elif [ -f .env.example ]; then
    set -a; . ./.env.example; set +a
fi
: "${LIAISON_IMAGE_REGISTRY:=liaison}"
: "${LIAISON_IMAGE_TAG:=latest}"
for img in "$LIAISON_IMAGE_REGISTRY/liaison:$LIAISON_IMAGE_TAG" "$LIAISON_IMAGE_REGISTRY/frontier:$LIAISON_IMAGE_TAG" "guacamole/guacd:1.5.5"; do
    if docker image inspect "$img" >/dev/null 2>&1; then
        docker rmi "$img" 2>&1 | sed 's/^/  /' || true
    fi
done

if [ "$PURGE" -eq 1 ]; then
    printf "${GREEN}==> Purging data / certs / logs / .env${NC}\n"
    rm -rf data certs logs .env
    printf "  done\n"
else
    printf "${YELLOW}==> Keeping data/, certs/, logs/, .env — re-run ./install.sh to reinstall.${NC}\n"
fi

printf "${GREEN}✅ Uninstall complete.${NC}\n"
