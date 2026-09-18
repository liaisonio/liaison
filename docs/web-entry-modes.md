# Web entry modes

Status: implemented, pending deployment validation. Approved scope: private deployments may use
the console origin for path entries; this is routing, not browser-origin isolation.

## Decision

- New HTTP website accesses default to `path`, under `/access/{id}/web/`.
- Everything after that prefix belongs to the upstream website, including paths
  named `sessions`, `keys` or `api`. Query strings and encoded paths are preserved.
  Other `/access/{id}/...` routes remain console routes, not proxy routes.
- Legacy `/_liaison/a/{id}/` URLs remain supported without redirects, using their
  existing prefix-scoped cookies. New launch URLs use `/access/{id}/web/`;
  the new prefix requires launching from Liaison again. No database migration is needed.
- Existing accesses (empty entry mode) retain `port`; no automatic conversion.
- `domain` uses `a-{id}.<web_domain>` on the manager HTTPS listener. It is
  available only when that listener has a currently valid certificate covering
  the configured wildcard domain. DNS must point the wildcard to the manager.
- Shared entries use connector streams, not local per-access listeners.
- Browser entry requires a one-use, short-lived launch ticket issued under the
  user's existing access permission. The resulting session is access-scoped;
  permissions and resource availability are rechecked for each request.
- Strip the reserved path before forwarding. Rewrite same-upstream redirects
  and cookie paths; preserve streaming and WebSocket upgrades. Never forward
  console credentials. Do not rewrite arbitrary JavaScript or HTML.

## Compatibility

Sites must support a configurable base path (including API/assets/WebSocket URLs)
for path mode. Root-absolute URLs in scripts cannot be made universally safe by
response rewriting. Use port/domain for such sites. Path mode shares browser
storage and origin with the console and must only host trusted applications.
Upstream cookies in path mode are namespaced, so console/root cookies are never
forwarded. Sites reading cookie names directly in JavaScript, or requiring
`__Host-` cookies, must use a domain/port entry. Application Basic/Bearer headers
are preserved, but Liaison JWTs and PATs are stripped. The upstream transport retains existing
HTTP application behavior (plain HTTP over the connector stream).

## Installation

For Docker installations, set `LIAISON_WEB_DOMAIN=apps.example.com` in `.env`
and supply `certs/web.crt` and `certs/web.key` before running the installer. The
certificate must contain `*.apps.example.com`, match the key, and be valid now.
Set wildcard DNS to the gateway; entries use `a-<id>.apps.example.com` and the
manager's public port. The manager loads this certificate alongside the console
certificate. Leave the setting empty to omit domain routing and its UI option.

For an existing manually managed configuration, add `manager.web_domain` and
the certificate/key pair to `manager.listen.tls.certs`, then restart. Existing
configuration files are not overwritten by the installer.

The launch API is `POST /api/v1/web-entries/{id}/launch` with the regular console
bearer credential. The response contains `data.url`; it includes a 30-second,
single-use ticket, never the bearer credential. Opening it sets an HttpOnly,
access-scoped one-hour session and redirects to the clean entry URL. Do not log
or share launch URLs. Gateway restart expires these in-memory sessions.
`GET /api/v1/web-entries/capabilities` returns `data.domain` without exposing
certificate paths or secrets.

## Migration and rollback

Add an `http_entry_mode` column with empty default through the existing GORM
schema migration. Empty means legacy port, never path. New API field is optional;
creation without a mode defaults to path only for HTTP, while an explicit port
or `expose_public_port: true` request retains legacy port semantics. Updates
without the field preserve the mode.
Back up SQLite before deployment. Old binaries do not understand shared entries:
before rollback, stop or convert path/domain entries to ports, then restore the
old binary. Do not drop the additive column or rewrite existing access IDs.

## Acceptance

Cover path/query preservation, encoded paths, redirect and cookie scoping,
credential stripping, large/streaming responses, WebSocket upgrades, expired and
reused tickets, revoked access, offline connectors, unknown hosts, certificate
gating, and legacy port behavior. UI acceptance covers create/edit in both
languages/themes and desktop/mobile widths. Deployment is separate from code
verification and must not touch unrelated cloud connectors.
