# ClickHouse workspace

ClickHouse is available as an application type and as Web ClickHouse access.
Connections use the native TCP protocol (normally port 9000, or the configured
secure native port with TLS), not the HTTP endpoint on port 8123.

## Capabilities

- Saved per-user credentials, database selection and optional TLS.
- Connector-scoped connections: no direct manager-to-database network access.
- Database/table/view tree, columns, CREATE TABLE definition and table preview.
- SQL editor, bounded result sets, filtering, export and EXPLAIN.
- Attached Agent schema/query tools and separate command assistance, with the
  existing user isolation, approval and audit pipeline.

## Boundaries

ClickHouse sorting/primary keys do not guarantee unique rows. Inline row editing
and deletion are therefore disabled. Use explicit SQL for mutations and inspect
`system.mutations` for asynchronous completion. A successful submission is not a
claim that every background mutation has completed.

Custom connection parameters and target redirection are rejected. TLS verifies
the application hostname unless the user explicitly selects `skip-verify`.
Query execution has a 30-second default limit and bounded results; this workspace
is not intended as a bulk data loader. Database permissions remain enforced by
the connected ClickHouse account.

## Verification

Run backend tests with `go test -race ./pkg/liaison/manager/...` and frontend
checks with `node web/e2e/sql-protocols.cjs`.

The opt-in real-database test uses `TEST_CLICKHOUSE_ADDRESS` and
`TEST_CLICKHOUSE_PASSWORD`, with a dedicated `liaison_test` database/user. It
creates and cleans up a test table and exercises SQL, Unicode, NULL, metadata,
DDL, EXPLAIN and errors. Do not point it at production.

### First acceptance

Validated against ClickHouse 25.8: real native-driver integration and the full
connector path, aggregate SQL, column metadata, DDL and EXPLAIN. Browser checks
covered Agent approval/execution/structured results, header alignment, resizing,
focus retention, desktop/tablet/mobile bounds and draft preservation. Full Go
race regression, manager vet, frontend protocol checks and production build
passed. TLS against a real ClickHouse TLS endpoint and large-scale ingestion
are not included in this acceptance.
