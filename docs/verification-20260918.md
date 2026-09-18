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
- PostgreSQL follow-up: Docker Hub, ECR and DaoCloud pulls hit EOF through
  Docker's network path. Host-side `crane` download from ECR followed by Docker
  import succeeded; PostgreSQL 16 Alpine ARM64 integration passed with `-race`.
  It covered table creation, insert/update/select, table/column/index metadata
  and invalid-query errors. The loopback-only temporary container used a 384 MB
  memory limit and tmpfs data; its test table, container and data were removed.
  Image ID: `sha256:ef738a34a8651d11b2bace81c55c7e2187f786b484add6c83070884340074368`.
  This tests the SQL workspace against a real database, not deployed connector
  or browser end-to-end behavior.
- Browser fixtures cover Chinese/English, light/dark and desktop/mobile where
  applicable. Screenshots inspected for zero usage and native request examples.
- Final shared run on `5dcee2f`: all 41 browser suites passed together after
  the asynchronous test waits described below were corrected.
- MariaDB follow-up: real MariaDB 11.4 ARM64 SQL workspace integration passed
  with `-race`, covering table creation, insert/update/select, table/column/index
  metadata and invalid-query errors. Host-side download from DaoCloud followed
  by Docker import avoided the Docker network EOF and slow ECR transfer.
  The loopback-only container was limited to 512 MB with tmpfs database storage;
  its test table, container and data were removed after the test. This is not a
  connector/browser end-to-end result.
- SMB follow-up: `TestSMB_RealShare` passed with `-race` against an isolated
  ARM64 Samba container, with password authentication, mandatory signing and a
  read-only share. Verified listing, UTF-8 preview, identical download content,
  size-limit rejection, traversal/UNC/drive/alternate-stream path rejection and
  wrong-password rejection. The test now retains these additional assertions.
  Image ID: `sha256:650b2875f18c44ecac3b4fbbd1ab9cc3c43629e00b3b50783da906d41fb0facf`.
  The container exposed only a loopback port, used a 256 MB memory limit and
  mounted only an isolated fixture. Container and runtime credentials were
  removed afterwards. Host system sharing and deployed access were unchanged.
  This does not establish Windows/NAS interoperability or connector/browser E2E.

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
- Real SQL integrations were invoked, but SQL Server and
  Doris/StarRocks/TiDB cases explicitly skipped because disposable DSNs were absent.
- Real DM8 service acceptance, vendor cloud SDK compatibility and live
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
