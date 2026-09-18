# Existing-feature closeout verification — 2026-09-18

Branch: `docs/ai-wordmark`, rebased onto main `dd423ac` (includes PR #91).
This is a local regression record, not complete remote acceptance.

## Verified

- `go build ./...`, `go vet ./...`, `go test -race ./...`.
- Explicit process E2E: `LIAISON_E2E=1 go test -race ./test/e2e -timeout 12m`.
- Frontend TypeScript and production build: `npm run build` in `web`.
- Deployment configuration portability and release-version shell tests.
- Three local LLM protocol-simulator unit tests; no live model inference in this run.
- Browser fixtures cover Chinese/English, light/dark and desktop/mobile where
  applicable. Screenshots inspected for zero usage and native request examples.
- Final shared run: 40 of 41 suites passed. `data-editor-handoff` hit the portal
  remount timing issue described below; its four language/theme cases passed
  after the test-only wait fix. The other 40 suites were not rerun after that fix.

## Fixes and test maintenance

- Preserve a manually customized upstream path when changing protocol; default
  paths still follow the selected protocol. Existing Ollama regression covers both.
- Update application/access creation fixtures to concrete provider types and
  current request-API labels; retain legacy read-path fixtures.
- Wait for the new usage response and chart points after a time-range change;
  continue checking exact range duration, confirmed zeros and unknown/error states.
- Wait for the exact Agent handoff-button count across session portal remounts.
  Reverified all four language/theme combinations, including stale draft rejection,
  explicit confirmation, byte/control limits and no automatic execution.
- Include the 15 newer fixtures in the shared runner (41 suites total).

## Still unverified in this run

- Authenticated remote flows need an existing test account or private token file.
  No password was reset or recovered from unrelated historical transcripts.
- Real SQL integrations were invoked, but MariaDB/PostgreSQL/SQL Server and
  Doris/StarRocks/TiDB cases explicitly skipped because disposable DSNs were absent.
- Real DM8 and SMB service acceptance, vendor cloud SDK compatibility and live
  model inference are not established by fixtures or skipped integration tests.

## Deployment follow-up

- Deployed the frontend built from `bbbfbb4`; the existing backend checksum
  matches the previously verified deployment. No database changes or restart.
- Preserved the previous frontend archive for rollback and old hashed assets
  for already-open tabs. Served index checksum matches the local build.
- Five protected endpoints reject unauthenticated requests with HTTP 401.
- Protected-page login redirects and static assets passed in Chinese/English,
  light/dark, desktop/mobile. No page errors or mobile document overflow;
  representative desktop-dark and mobile-light screenshots inspected.
- Authenticated remote acceptance remains pending. These checks do not verify
  saved connections, real model inference or logged-in Agent actions.

Temporary logs and screenshots remain outside version control. Do not treat
this record as authorization to merge before remaining acceptance is agreed.
