# SQL Server workspace

Initial implementation; live SQL Server 2022 acceptance completed on 2026-09-10.

The deployed workspace was verified through the authorized connector tunnel:
schema/table metadata, column and primary-key details, Unicode values, TOP
previews, aggregate queries, UPDATE and transaction rollback. A dedicated demo
database and SQL login were used, with an internal-only database listener.
This is API-level live acceptance, not a complete browser or Agent E2E run.

- Application type: `sqlserver`, default TCP port 1433.
- Access type: `Web SQL Server` (browser workspace), or generic TCP passthrough.
- SQL username/password authentication; database defaults to `master`.
- Encryption required by default; `skip-verify` is an explicit compatibility option.
- Connections use the authorized connector tunnel, including private DNS targets.
- Browse schemas, tables/views, columns and indexes in the selected database.
- Query/editor, bounded TOP previews, filters and editable rows use T-SQL quoting.
- Agent schema/query and draft assistance retain existing user/session isolation,
  approvals and audit behavior. Query execution requires Agent approval.

Not included: Windows/Entra integrated authentication, SQL Browser named-instance
discovery, full CREATE TABLE DDL reconstruction, GO batch splitting or multiple
result-set UI. Use an explicit host and port, and submit individual SQL statements.

Verification:

```sh
node web/e2e/sql-protocols.cjs
go test -race ./pkg/liaison/manager/web ./pkg/liaison/manager/controlplane ./pkg/liaison/manager/agent/...
go test -tags=integration -race ./pkg/liaison/manager/web -run TestSQLWorkspace -v
```

For the integration test, supply `TEST_SQLSERVER_DSN` using a disposable SQL Server
database and an account whose default schema is `dbo`. The test creates and cleans
its own uniquely named table. Missing DSNs are reported as skipped, not passed
database acceptance. Never store credentials in the repository.
