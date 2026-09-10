# Oracle workspace

Implementation branch: `feature/oracle-webdata`.

## Scope

- Oracle application and Web Oracle access, default port 1521.
- SQL username/password and Service Name; optional current schema.
- Per-connection connector tunnel; listener redirects to other targets rejected.
- TCP or explicitly selected TCPS, with TLS 1.2 minimum and certificate verification.
- Schema/table/view navigation, columns, indexes and primary-key metadata.
- Quoted identifiers, FETCH FIRST previews and filters, SQL execution and row editing.
- Existing Agent schema/query tools, approval policy and user/session isolation.

Requires Oracle 12c or newer for pagination and identity-column metadata.
SQL statements are submitted individually. SQL*Plus commands, script slash
delimiters, SID-only connections, RAC redirects, wallets, external authentication,
full DDL reconstruction and large-object editing are not covered by this first version.
Date/time editing and LOB rendering require live compatibility acceptance.

## Verification status

Go tests, targeted race tests, vet, frontend build and pure SQL dialect checks
passed. Driver tests verify injected tunnel use, target redirect rejection,
credential encoding and rejection of custom connection overrides.

Live API acceptance passed on 2026-09-10 against the official
`container-registry.oracle.com/database/free:23.9.0.0-lite` image through the
authorized connector: saved login, metadata, columns/indexes, Unicode values,
FETCH FIRST pagination, aggregate queries and UPDATE followed by restoration.
The demo uses a dedicated tablespace and non-administrator SQL account.
Oracle NUMBER metadata flags are returned as strings; primary-key and generated
column handling has regression coverage for that representation.

The SQL Server demo was stopped with user approval to provide memory; its data
and saved connection are retained. Oracle has a 2 GB container limit and its port
is bound only to the internal host address. Browser/Agent E2E remains pending;
API acceptance does not establish complete UI, model or TCPS compatibility.
