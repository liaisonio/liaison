# Existing-feature closeout verification — 2026-09-18

Branch: `docs/ai-wordmark`, rebased onto main `dd423ac` (includes PR #91).
This records local regression and scoped remote acceptance, not certification
of every supported upstream service.

## Verified

- `go build ./...`, `go vet ./...`, `go test -race ./...`.
- Explicit process E2E: `LIAISON_E2E=1 go test -race ./test/e2e -timeout 12m`.
- Frontend TypeScript and production build: `npm run build` in `web`.
- Deployment configuration portability and release-version shell tests.
- Three local LLM protocol-simulator unit tests; no live model inference in this run.
- Follow-up: real MySQL 8 workspace integration passed with `-race` against
  a disposable local container, covering mutations, result preview, table/column/
  index metadata and invalid-query errors. Test tables and the container were
  removed afterwards. This is not a connector or browser end-to-end test.
- PostgreSQL image download failed at the registry (EOF); no PostgreSQL service
  was started and no compatibility result is claimed.
- Browser fixtures cover Chinese/English, light/dark and desktop/mobile where
  applicable. Screenshots inspected for zero usage and native request examples.
- Final shared run on `5dcee2f`: all 41 browser suites passed together after
  the asynchronous test waits described below were corrected.

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
- Follow-up: wait for the requested usage range and rendered keyboard selection
  in `token-trend-feedback`, avoiding immediate reads during asynchronous updates.
  All four language/theme cases passed after this test-only change.

## Still unverified in this run

- The supplied account has no available SSH/SQL examples; authenticated remote
  acceptance below does not establish those connector-to-workspace flows.
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
- These initial anonymous checks were followed by authenticated acceptance below.

## Authenticated remote acceptance

- Verified all seven available LLM examples: workspace metadata, request records,
  five usage ranges, and short Playground inference completed successfully.
- Temporary scoped API keys exercised each exposed client protocol, including
  streaming completion, model-scope rejection, and rejection after revocation.
  All seven temporary keys were revoked; no plaintext key was recorded.
- Overview, statistics, request records, Playground and home-toolbar layout
  passed Chinese/English, light/dark, desktop/mobile checks. Representative
  screenshots were inspected; no page errors or document overflow were found.
- A read-only home Agent session discovered accessible LLMs and queried usage;
  it finished without pending approval and was archived afterwards.
- Existing examples may use a protocol simulator backed by local inference.
  These results do not certify native vendor cloud services or their SDKs.

## JWT logout security follow-up

- Fixed the discovered stateless logout gap in `8278204`: persist SHA-256 token
  fingerprints until expiry and check revocation on authenticated requests.
  New logins receive distinct random JWT IDs; unrelated sessions remain valid.
- Full Go build, vet, race tests and explicit process E2E passed again. Tests
  cover legacy JWTs, database reopen persistence and storage failure rejection.
- CI run `35302634677` passed both backend and frontend jobs on `8278204`.
- Deployed the backend after backing up its binary and database. This follow-up
  adds the revocation table and restarts Manager; the earlier frontend-only
  deployment description above applies only to that earlier step.
- Live two-session verification passed: after logout, the first JWT receives
  HTTP 401 from profile, access-list and WebData routes while the second login
  still receives HTTP 200. Both temporary sessions were revoked afterwards;
  no token files were written.
- A subsequent SSH health check timed out, so no new container-health claim is
  made from that check; the authenticated HTTPS acceptance above completed.
- See `session-revocation.md` for legacy-token and established-session limits.

Temporary logs and screenshots remain outside version control. Do not treat
this record as authorization to merge before remaining acceptance is agreed.
