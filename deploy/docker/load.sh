#!/bin/bash
# Liaison Docker bundle installer.
#   1. Load bundled images into the local Docker daemon.
#   2. Interactively (or by countdown-default) configure LIAISON_PUBLIC_HOST.
#   3. Ensure bind-mount dirs are writable by the container user (uid 1000).
#   4. docker compose up -d.
#   5. Tail logs until the first-run admin password is printed, then show it.
set -eu

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
cd "$SCRIPT_DIR"

GREEN=$'\033[0;32m'
YELLOW=$'\033[1;33m'
RED=$'\033[0;31m'
CYAN=$'\033[0;36m'
BOLD=$'\033[1m'
NC=$'\033[0m'

log()  { printf "${GREEN}%s${NC}\n" "$*"; }
warn() { printf "${YELLOW}%s${NC}\n" "$*"; }
err()  { printf "${RED}%s${NC}\n" "$*" >&2; }

canonical_url() {
    case "${MANAGER_PORT:-443}" in
        443) printf "https://%s" "${LIAISON_PUBLIC_HOST:-localhost}" ;;
        80)  printf "http://%s" "${LIAISON_PUBLIC_HOST:-localhost}" ;;
        *)   printf "https://%s:%s" "${LIAISON_PUBLIC_HOST:-localhost}" "${MANAGER_PORT:-443}" ;;
    esac
}

# ---------------------------------------------------------------------------
# 1. Environment checks
# ---------------------------------------------------------------------------
if ! command -v docker >/dev/null 2>&1; then
    err "docker not found on PATH. Install Docker 20.10+ first."
    exit 1
fi

if docker compose version >/dev/null 2>&1; then
    DC="docker compose"
elif command -v docker-compose >/dev/null 2>&1; then
    DC="docker-compose"
else
    err "docker compose plugin not found."
    err "Install the compose v2 plugin: https://docs.docker.com/compose/install/"
    exit 1
fi

# ---------------------------------------------------------------------------
# 2. Load images
# ---------------------------------------------------------------------------
if [ ! -d "$SCRIPT_DIR/images" ]; then
    err "images/ directory missing. Run this script from the extracted bundle root."
    exit 1
fi

log "==> Loading Docker images"
for tar in "$SCRIPT_DIR"/images/*.tar; do
    [ -f "$tar" ] || continue
    printf "  %s\n" "$(basename "$tar")"
    docker load -i "$tar" | sed 's/^/    /'
done

# ---------------------------------------------------------------------------
# 3. Detect public host + prompt (with countdown default)
# ---------------------------------------------------------------------------
detect_public_ip() {
    local ip=""
    for url in https://ifconfig.me https://ipinfo.io/ip https://api.ipify.org; do
        ip=$(curl -fsS --max-time 5 "$url" 2>/dev/null | tr -d '[:space:]' || true)
        if [[ "$ip" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
            echo "$ip"; return
        fi
    done
    echo ""
}

if [ -f .env ]; then
    warn "==> .env already exists, keeping it (delete it if you want to re-prompt)"
else
    log "==> Detecting public IP..."
    DETECTED_IP=$(detect_public_ip)
    if [ -n "$DETECTED_IP" ]; then
        printf "  detected: ${CYAN}%s${NC}\n" "$DETECTED_IP"
    else
        warn "  could not auto-detect; you'll be asked to enter one."
        DETECTED_IP=""
    fi

    DEFAULT_HOST="${DETECTED_IP:-localhost}"
    printf "\n${BOLD}Enter public IP or domain${NC} [${CYAN}%s${NC}] (auto-accept in 30s): " "$DEFAULT_HOST"
    if [ -r /dev/tty ] && read -r -t 30 INPUT_HOST </dev/tty; then
        PUBLIC_HOST="${INPUT_HOST:-$DEFAULT_HOST}"
    else
        echo
        PUBLIC_HOST="$DEFAULT_HOST"
        warn "  no input — using $PUBLIC_HOST"
    fi

    cp .env.example .env
    INSTALL_MANAGER_PORT="${MANAGER_PORT:-443}"
    # Portable in-place sed (BSD + GNU).
    sed \
        -e "s|^LIAISON_PUBLIC_HOST=.*|LIAISON_PUBLIC_HOST=${PUBLIC_HOST}|" \
        -e "s|^MANAGER_PORT=.*|MANAGER_PORT=${INSTALL_MANAGER_PORT}|" \
        .env > .env.tmp && mv .env.tmp .env
    log "==> Wrote .env (LIAISON_PUBLIC_HOST=${PUBLIC_HOST}, MANAGER_PORT=${INSTALL_MANAGER_PORT})"
fi

# Keep JWT sessions valid across container recreation. Older bundles did not
# persist this value, so also backfill existing .env files during upgrades.
if ! grep -q '^JWT_SECRET=.' .env; then
    JWT_SECRET=$(openssl rand -base64 48 | tr -d '=+/[:space:]' | cut -c1-48)
    sed -e '/^JWT_SECRET=/d' .env > .env.tmp
    printf 'JWT_SECRET=%s\n' "$JWT_SECRET" >> .env.tmp
    mv .env.tmp .env
    chmod 600 .env
    log "==> Generated and persisted JWT signing secret"
fi

# Load the final values for post-start messages.
# shellcheck disable=SC1091
set -a; . ./.env; set +a
: "${LIAISON_PUBLIC_HOST:=localhost}"
: "${MANAGER_PORT:=443}"
ACCESS_URL=$(canonical_url)

# ---------------------------------------------------------------------------
# 4. Pre-create bind-mount dirs with correct ownership (uid 1000 inside)
# ---------------------------------------------------------------------------
mkdir -p data certs logs
# Best-effort: needs root on Linux. Silently ignore if we lack capability
# (Docker Desktop on macOS/Windows auto-maps UIDs anyway).
chown 1000:1000 data certs logs 2>/dev/null || true

FRESH_INSTALL=0
if [ ! -f data/.initialized ]; then
    FRESH_INSTALL=1
fi

# ---------------------------------------------------------------------------
# 5. Launch
# ---------------------------------------------------------------------------
log "==> Starting services"
$DC up -d

# ---------------------------------------------------------------------------
# 6. Show connection info
#    Fresh install: wait up to 60s for the entrypoint to print the new password.
#    Re-run on existing data/.initialized: no new password is generated, so
#    just tell the operator to use their existing credentials.
# ---------------------------------------------------------------------------
echo
if [ "$FRESH_INSTALL" -eq 0 ]; then
    cat <<EOF
${BOLD}${GREEN}============================================================${NC}
  Liaison is up (reusing existing data).
  URL:      ${CYAN}${ACCESS_URL}${NC}
  Login:    use your existing admin credentials.
${BOLD}${GREEN}============================================================${NC}

Forgot the password? Reset it:
  $DC exec liaison /opt/liaison/bin/password-generator \\
      -password NEW_PASSWORD -email default@liaison.com

Operate:
  $DC ps
  $DC logs -f liaison
  $DC down          # stop (keeps data/ certs/ logs/)
EOF
    exit 0
fi

log "==> Waiting for first-run admin password..."
PASSWORD=""
for _ in $(seq 1 60); do
    PASSWORD=$($DC logs liaison 2>&1 | grep -oE "Password:[[:space:]]+[A-Za-z0-9]+" | head -1 | awk '{print $2}')
    if [ -n "$PASSWORD" ]; then break; fi
    sleep 1
done

echo
if [ -n "$PASSWORD" ]; then
    cat <<EOF
${BOLD}${GREEN}============================================================${NC}
  Liaison is up.
  URL:      ${CYAN}${ACCESS_URL}${NC}
  Email:    ${LIAISON_ADMIN_EMAIL:-default@liaison.com}
  Password: ${CYAN}${PASSWORD}${NC}
${BOLD}${GREEN}============================================================${NC}

Save the password — it is only printed once.

Operate:
  $DC ps
  $DC logs -f liaison
  $DC down          # stop (keeps data/ certs/ logs/)
EOF
else
    warn "Could not capture the first-run password within 60s."
    warn "Containers may still be starting. Inspect with:"
    warn "  $DC logs liaison"
    warn "  $DC logs liaison | grep -A5 'first-run credentials'"
fi
