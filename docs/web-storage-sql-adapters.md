# WebSMB / WebDoris / WebStarRocks / WebTiDB

## Scope

- Storage: WebSMB, one configured SMB share per access connection.
- Database: WebDoris, WebStarRocks and WebTiDB, using the existing WebData SQL workspace.
- Existing connector transport, user-scoped credentials, access checks and audit remain in use. No new public database listener or permission-model redesign.

## SMB boundaries

SMB 2/3 with NTLM username/password and optional domain; message signing is required. SMB traffic uses the existing connector tunnel. SMB signing is integrity protection, not a claim that the connector-to-share hop is always encrypted.

The first release is read-only: directory tree, UTF-8 preview and download. No SMB1, guest login, local OS mounts, DFS referral connections, upload, rename or delete. Visible symlink/reparse components are rejected; the configured server/share remains the trust boundary, not a sandbox against a malicious server.

- One directory: at most 10,000 entries; exceeding this returns an explicit error, not a complete-looking partial list.
- Preview: 256 KiB; download: 64 MiB.
- UNC paths, traversal, alternate data streams and wildcard paths are rejected.
- Session ownership, current access availability and the existing file-read feature are checked at the API. Currently this reuses `webssh.files.read`; generic file-permission naming is deferred with the permission redesign.
- Sharing/reading file contents through Agent is not added in this release.

Dependency: `github.com/cloudsoda/go-smb2`, pinned in go.mod, BSD-2-Clause. No database-server binaries are bundled.

## SQL compatibility

The three engines have independent application/access types. Their MySQL-compatible wire transport reuses the existing driver and connector stream; this does not imply full MySQL SQL or administration compatibility. Doris and StarRocks use driver-escaped text parameters for metadata rather than assuming full prepared-statement support. Agent context explicitly distinguishes analytical engines.

Browser workspaces reuse database navigation, SQL execution, result display and connection-scoped Agent. Native ports default to 9030 (Doris/StarRocks) and 4000 (TiDB). SMB defaults to 445.

## Verification status

Implemented and checked: related Go race tests, credential isolation, SMB path boundaries and cancellation, frontend type checking, SQL type/quoting checks, mocked WebSMB UI checks for Chinese/English, light/dark, desktop/mobile, retry and unsaved-password flow. SMB dependency scan found no reachable known vulnerability.

Not yet accepted: real SMB server interoperability, real Doris/StarRocks/TiDB metadata and complete deployed connector E2E. The local Samba image download failed with registry EOF; available disk space was insufficient for a safe parallel OLAP test stack. These adapters must not be described as production-verified or deployed until these checks pass.

Reproducible checks:

```sh
go test -race ./pkg/liaison/manager/smbfiles ./pkg/liaison/manager/controlplane ./pkg/liaison/manager/web ./pkg/liaison/manager/agent/executor
node web/e2e/sql-protocols.cjs
```

Real-service tests use explicit disposable endpoints from environment variables, never repository credentials:

- `TestSMB_RealShare`: see `pkg/liaison/manager/smbfiles/integration_test.go` for `TEST_SMB_*` variables; provide a share containing `hello.txt` with “Liaison” in it.
- `TestMySQLFamily_RealMetadata`: `TEST_DORIS_DSN`, `TEST_STARROCKS_DSN`, `TEST_TIDB_DSN`; run with `-tags=integration`. These checks are read-only and do not replace full table/column/DDL compatibility acceptance.
- Browser fixture: `web/e2e/websmb-ui.cjs`, with `E2E_UI_URL` and `PLAYWRIGHT_MODULE` configured. Mock-backed UI checks are not real-service E2E.
