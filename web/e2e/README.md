# Agent E2E

## Workspace regression runner

Start the frontend development server, then run from the repository root:

```sh
E2E_UI_URL=http://127.0.0.1:8000 node web/e2e/run-fixtures.cjs
```

Set `PLAYWRIGHT_MODULE` when Playwright is installed outside this repository.
The runner executes 41 deterministic browser/protocol suites and prints the
temporary directory containing per-suite logs and a JSON report. Keep frontend
sources unchanged during the run: Vite hot reload can invalidate an active test.
This is not a runner for all live-service integrations.

The additional coverage includes native LLM application/access types, usage
ranges and zero/unknown/error states, temporary copy feedback, Agent handoff,
Web entry modes, SMB, Dameng, offline devices and dashboard traffic.

### Branch reconciliation verification (2026-09-18)

`feature/storage-agent-workspace` retains `fb9ffb6` and merges main at
`bc32244`; the PR does not replay branding commits or import the source
worktree's uncommitted follow-up features.

- `go build ./...`, `go vet ./...` and `go test -race ./...` passed.
- `LIAISON_E2E=1 go test -race ./test/e2e -timeout 12m` passed with the
  process-level suite explicitly enabled.
- Initial browser runner: 25 of 26 suites passed. Draft review hit a viewport
  transition race when Enter targeted the textarea before the mobile portal
  returned to the desktop dock. After adding a dock-ready wait, all 16 draft
  review protocol/language/theme combinations passed on rerun. The other
  25 suites were not rerun after this test-only change.
- Inspected representative WebData, SSH handoff and S3 Agent screenshots,
  including desktop/mobile and dark/light variants.

These results are local regressions, not a fresh deployment or a rerun of
real-provider/model and remote storage acceptance.

`agent-session-isolation.cjs` checks draft isolation when switching conversations
on the same connection, while preserving drafts on initial session binding and
closing/reopening the same panel. Other fixtures cover selected rows/columns,
secret-field masking, selected terminal output, manual sending, reviewed code
handoff, stale input, disconnected workspaces and Chinese/English theme layouts.

For the opt-in `data-context-live.cjs`, `data-result-share-live.cjs`,
`ssh-handoff-live.cjs` and `home-toolbar-live.cjs`, set `E2E_BASE_URL` and
`E2E_TOKEN_FILE` to a private JSON file containing a test user's `token`. Do not
commit that file. Use only disposable demo accesses. These tests archive their
Agent sessions and close temporary connections; they do not own or revoke the
caller-supplied login token. The runner must revoke that token after acceptance.

Pure SQL protocol routing, quoting and TLS-option checks:
`node web/e2e/sql-protocols.cjs` (run from the repository root).

Native SQL workspace integration tests:
`go test -tags=integration -race ./pkg/liaison/manager/web -run TestSQLWorkspace -v`.
Set `TEST_MYSQL_DSN` / `TEST_MARIADB_DSN` (Go MySQL DSNs),
`TEST_POSTGRESQL_DSN` (pgx DSN) or `TEST_SQLSERVER_DSN` (SQL Server DSN)
to disposable databases. These tests create and clean a uniquely named table;
they cover query results, mutations, metadata, indexes and error handling.
Missing DSNs are reported as skipped, not evidence of database compatibility.

Only run these suites against an isolated test deployment. Never use production
credentials or approve an unverified tool call.

## Real database and model tests

`data-agent.cjs` covers MySQL, MariaDB, PostgreSQL, MongoDB and Redis: metadata, approvals,
rejection, errors, temporary object mutations, draft completion and disconnects.
It reads the login password from standard input; do not put credentials in source.

Set `E2E_BASE_URL` and optionally `E2E_EMAIL`, then invoke the script with protocol
names (or omit names to run all five). Provide the password via a secure runner.
The deployment needs existing applications named `Agent Demo <protocol>` and
saved connections named `Agent demo <protocol>`, pointing at disposable databases.
Only uniquely named objects created by a test are modified and removed.

## Browser suites

The `.js` files export async function expressions for an authenticated Playwright
page. Load them in a browser runner and invoke the function with that page.

- `data-agent-ui.js`: real model send/approval/results, panel sizing, focus and
  responsive layouts. Start from a `/webdata/` page.
- `data-assistance-ui.js`: real database connections with stubbed completion
  responses to reproduce stale-revision races deterministically.
- `management-mentions-ui.js`: deterministic mention-picker acceptance using the
  fixtures in `fixtures/`.

Browser suites clean their sessions and temporary connections in `finally`.
Screenshots are written to the OS temporary directory, not committed to the repo.
