# Edge Agent durable history

Status: implementation of the user-approved server-side history proposal.

- Manager owns durable conversation history in the existing database; Edge owns execution.
- History has no TTL. Idle execution shutdown and Edge memory eviction never delete server history.
- Store bounded encrypted snapshots (messages, safe activity metadata, model, project, native thread ID), scoped by owner, access, connector and session ID. Do not persist credentials or raw RPC events.
- A server lifecycle worker samples active snapshots independently of browsers every two seconds, saving changed revisions. Final snapshots and successful user operations are saved synchronously. A sudden Edge failure may lose events since the last successful sync; a cloud snapshot is not a native execution checkpoint.
- Offline history is read-only. When the original connector is online, explicit resume asks that Edge instance to reopen only its owner/access/session-scoped native binding. Never silently create a new conversation when opening archived history.
- Native Codex state remains on the original device under its existing `CODEX_HOME`; Manager stores normalized encrypted display history, not a restorable native checkpoint. Missing device state or legacy ephemeral threads keep readable server history and fail resume closed.
- Durable deletion tombstones prevent delayed replies from resurrecting deleted history; clear snapshot and title payloads. Access deletion removes its associated payloads transactionally.
- Paginate history. Recheck owner, feature and connector permissions on reads, mutations and background sync. No admin bypass.
- Additive schema only. Back up database and encryption secret before rollout. Roll back binaries without dropping the history table. Down migration is destructive and must not run during normal rollback.
- Reuse credential encryption secret (JWT secret fallback), with a dedicated domain-separated key and scope-bound authenticated encryption. Losing or rotating the secret without migrating ciphertext makes history unreadable.

## Verification and rollout

### Decision: bounded display windows, durable round pages

Before a new turn, Manager archives the completed display window in a separate encrypted row, then sends an internal revision-checked reset acknowledgement to Edge. The browser cannot supply that acknowledgement. If archiving fails, the new turn is not sent and the previous window remains intact. Concurrent sends cannot reset a newer revision. Existing snapshots are window zero; old Edges continue the legacy protocol until upgraded.

The current round remains bounded (recent messages and activities, explicitly marked truncation) but never stops native execution because of display limits. Previous rounds are loaded one at a time using an exclusive window cursor. Pages have the same owner/access/connector/session scope, encryption, deletion semantics and no TTL as the session. A single extremely long round is not a lossless transcript: its display may be truncated; the native Codex history remains on the device. Rollback retains the additive page table.

### Structured interaction and execution details

- Native `item/tool/requestUserInput` requests render single-choice options or free text (password inputs for secret questions). Liaison does not infer questions from assistant Markdown. Support at most three questions and eight options per question; unsupported requests fail closed.
- Answers use owner/access-scoped, opaque, single-use IDs and the original native request ID. Turn completion, shutdown and `serverRequest/resolved` invalidate pending requests. Never archive live request IDs or replay them on resume.
- Display history keeps questions and answers, but masks secret answers. This does not redact secrets subsequently echoed by the model or a command; execution output must be treated as sensitive owner-only history.
- Command output streams into expandable details; authoritative completed items replace partial output. Show command, working directory, exit code, duration and per-file unified diffs as plain text, without executing commands or resolving file paths. Raw reasoning is not displayed.
- Details are bounded to 16 KiB per text field and 96 KiB per display window, with explicit truncation; truncation must never terminate a task. Recent steps can reclaim older completed output. Discovery previews retain metadata only. Older histories without these fields remain readable.

### Execution permissions and lifetime

- Saved-access turns have no three-minute wall-clock deadline. The 30-minute idle lease and 24-hour process age reclaim only idle runtimes; neither deletes history. Temporary discovery previews retain their short lease.
- Sessions start and resume read-only. A user may select workspace-write while idle; the next native turn permits writes under that session's canonical working directory, keeps network access disabled, and excludes ambient temporary directories. Selection is session-local and resets to read-only on native resume. It never edits Codex configuration.
- Native command approval requests are shown to the owner with their exact command, working directory and reason. Only accept-once and decline are supported. No execution-policy amendments, persistent grants, generic JSON-RPC forwarding, or automatic approval.
- Approval IDs are opaque, live, turn-bound and single-use. Owner/access authorization runs on every action. Completed/closed turns clear pending requests; encrypted history does not store them. Unknown, oversized, network-only, stdin, file-root and permission-profile requests fail closed until their scopes have dedicated UI support.

- Run race tests for control plane, DAO, Edge bridge and protocol; cover stale writes, deletion tombstones, owner/access/connector isolation and non-renewing background snapshots.
- Validate SQLite migration and reopening the database; test pagination beyond 50 histories.
- Exercise Chinese/English, light/dark, desktop/mobile, including entering history from an offline connector's access list.
- Back up existing Edge snapshots before rollout. Deploy Manager first and read each existing session through it to persist history before upgrading Edge.
- Verify a minimal model response is saved after closing the browser, then verify Manager restart and archived history after Edge restart. Delete only synthetic regression accesses.
- Rollback keeps the additive history table and stable encryption secret. Old binaries may not display durable history, but must not drop the table.
