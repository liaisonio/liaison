#!/bin/bash
# Liaison container entrypoint.
# First-run bootstrap:
#   1. Render /opt/liaison/conf/liaison.yaml from template using env vars
#   2. Generate a self-signed TLS cert if one is not present
#   3. Create the initial admin user with a random password and print it once
# Subsequent runs are idempotent: existing certs / .initialized marker are respected.

set -eu

CONF_DIR=/opt/liaison/conf
DATA_DIR=/opt/liaison/data
CERTS_DIR=/opt/liaison/certs
LOG_DIR=/opt/liaison/logs
BIN_DIR=/opt/liaison/bin
INIT_MARKER="${DATA_DIR}/.initialized"

mkdir -p "$DATA_DIR" "$CERTS_DIR" "$LOG_DIR"

# ---- Render config --------------------------------------------------------
: "${FRONTIER_PORT:=30012}"
: "${FRONTIER_CONTROLPLANE_PORT:=30010}"
: "${LIAISON_PUBLIC_HOST:=localhost}"
: "${MANAGER_PORT:=443}"
: "${LIAISON_ADMIN_EMAIL:=default@liaison.com}"
: "${GUACD_ADDR:=127.0.0.1:4822}"
: "${GUACD_BRIDGE_ADDR:=127.0.0.1:0}"
: "${GUACD_BRIDGE_HOST:=127.0.0.1}"
: "${AGENT_ENABLED:=false}"
: "${AGENT_BASE_URL:=https://api.openai.com/v1}"
: "${AGENT_MODEL:=gpt-4.1-mini}"
: "${AGENT_REQUEST_TIMEOUT:=2m}"
: "${AGENT_MAX_MODEL_STEPS:=12}"
: "${AGENT_APPROVAL_EXPIRY:=15m}"

# server_url: omit :PORT for the well-known TLS / HTTP defaults so the URL
# baked into the web console / install commands is canonical.
if [ -z "${SERVER_URL:-}" ]; then
    case "$MANAGER_PORT" in
        443) SERVER_URL="https://${LIAISON_PUBLIC_HOST}" ;;
        80)  SERVER_URL="http://${LIAISON_PUBLIC_HOST}" ;;
        *)   SERVER_URL="https://${LIAISON_PUBLIC_HOST}:${MANAGER_PORT}" ;;
    esac
fi

if [ ! -f "$CONF_DIR/liaison.yaml" ]; then
    if [ -z "${JWT_SECRET:-}" ]; then
        JWT_SECRET=$(openssl rand -base64 32 | tr -d "=+/" | cut -c1-32)
    fi
    export FRONTIER_PORT FRONTIER_CONTROLPLANE_PORT MANAGER_PORT SERVER_URL JWT_SECRET GUACD_ADDR GUACD_BRIDGE_ADDR GUACD_BRIDGE_HOST
    export AGENT_ENABLED AGENT_BASE_URL AGENT_MODEL AGENT_REQUEST_TIMEOUT AGENT_MAX_MODEL_STEPS AGENT_APPROVAL_EXPIRY
    # shellcheck disable=SC2016
    envsubst '${FRONTIER_PORT} ${FRONTIER_CONTROLPLANE_PORT} ${MANAGER_PORT} ${SERVER_URL} ${JWT_SECRET} ${GUACD_ADDR} ${GUACD_BRIDGE_ADDR} ${GUACD_BRIDGE_HOST} ${AGENT_ENABLED} ${AGENT_BASE_URL} ${AGENT_MODEL} ${AGENT_REQUEST_TIMEOUT} ${AGENT_MAX_MODEL_STEPS} ${AGENT_APPROVAL_EXPIRY}' \
        < "$CONF_DIR/liaison.yaml.template" > "$CONF_DIR/liaison.yaml"
    echo "[entrypoint] rendered $CONF_DIR/liaison.yaml (public_host=$LIAISON_PUBLIC_HOST manager_port=$MANAGER_PORT frontier_port=$FRONTIER_PORT controlplane_port=$FRONTIER_CONTROLPLANE_PORT)"
fi

# ---- Generate TLS cert ----------------------------------------------------
if [ ! -f "$CERTS_DIR/server.crt" ] || [ ! -f "$CERTS_DIR/server.key" ]; then
    echo "[entrypoint] generating self-signed TLS cert for CN=$LIAISON_PUBLIC_HOST"
    openssl req -x509 -newkey rsa:4096 \
        -keyout "$CERTS_DIR/server.key" \
        -out "$CERTS_DIR/server.crt" \
        -days 3650 -nodes \
        -subj "/C=CN/ST=Beijing/L=Beijing/O=Liaison/OU=IT/CN=${LIAISON_PUBLIC_HOST}" \
        2>/dev/null
    chmod 600 "$CERTS_DIR/server.key"
    chmod 644 "$CERTS_DIR/server.crt"
fi

# ---- Seed admin user ------------------------------------------------------
# Race note: liaison boots AutoMigrate on startup, so we can't seed before
# liaison has run once. We start liaison in the background, wait for the DB
# schema, seed, then bring liaison to the foreground.
if [ ! -f "$INIT_MARKER" ]; then
    INITIAL_PASSWORD=$(openssl rand -base64 18 | tr -d "=+/" | cut -c1-16)

    echo "[entrypoint] first-run: starting liaison to initialise schema"
    "$@" &
    LIAISON_PID=$!

    # Wait for liaison.db to exist AND to have the users table created.
    for _ in $(seq 1 60); do
        if [ -f "$DATA_DIR/liaison.db" ] \
            && sqlite3 "$DATA_DIR/liaison.db" "SELECT name FROM sqlite_master WHERE type='table' AND name='users';" 2>/dev/null | grep -q users; then
            break
        fi
        sleep 1
    done

    if ! sqlite3 "$DATA_DIR/liaison.db" "SELECT name FROM sqlite_master WHERE type='table' AND name='users';" 2>/dev/null | grep -q users; then
        echo "[entrypoint] ERROR: users table never appeared; liaison failed to initialise" >&2
        kill "$LIAISON_PID" 2>/dev/null || true
        wait "$LIAISON_PID" 2>/dev/null || true
        exit 1
    fi

    "$BIN_DIR/password-generator" \
        -password "$INITIAL_PASSWORD" \
        -email "$LIAISON_ADMIN_EMAIL" \
        -create >/dev/null

    touch "$INIT_MARKER"

    cat <<EOF

============================================================
  Liaison first-run credentials (shown ONCE, save them now)
  Email:    $LIAISON_ADMIN_EMAIL
  Password: $INITIAL_PASSWORD
  URL:      $SERVER_URL
============================================================

EOF

    # The bootstrap process loaded IAM/Casbin state before the first user
    # existed. Restart it once so the new administrator membership and role
    # binding are loaded into the in-memory authorizer before serving traffic.
    kill "$LIAISON_PID" 2>/dev/null || true
    wait "$LIAISON_PID" 2>/dev/null || true
    exec "$@"
fi

exec "$@"
