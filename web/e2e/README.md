# Agent E2E

Only run these suites against an isolated test deployment. Never use production
credentials or approve an unverified tool call.

## Real database and model tests

`data-agent.cjs` covers MySQL, PostgreSQL, MongoDB and Redis: metadata, approvals,
rejection, errors, temporary object mutations, draft completion and disconnects.
It reads the login password from standard input; do not put credentials in source.

Set `E2E_BASE_URL` and optionally `E2E_EMAIL`, then invoke the script with protocol
names (or omit names to run all four). Provide the password via a secure runner.
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
