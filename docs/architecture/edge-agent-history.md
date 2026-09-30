# Edge Agent durable history

Status: implementation of the user-approved server-side history proposal.

## Conversation search and notifications

`sessions.history_search` accepts at most 100 Unicode characters and matches
case-insensitive substrings of the conversation title (including renames) or
project path, within the authenticated owner/access/connector scope. It does not
search message bodies. Results use the existing 50-row pagination and a matched
`history_total`; `history_search_available` advertises support. Older servers
must not be presented as having searched all history.

Metadata remains encrypted. The first implementation decrypts scoped batches
and keeps only the requested result page in memory; cancellation returns an
error rather than incomplete results. Search cost is linear in scoped history,
so the UI debounces explicit searches and does not repeat them on its five-second
status refresh. Large installations should benchmark this path before enabling
frequent searches, and consider a separate encrypted metadata index later.

While a search is open, the client refreshes the complete matching page roughly
every 30 seconds (plus request time), including sessions outside the first
unfiltered page, renames and deletions. Five-second updates still refresh matching
live rows and attention signals. Explicit Refresh performs a full search
immediately. Search results do not display the project group's Show more control.

## Drafts and browser timing observations

Unsent composer text is kept in `sessionStorage`, scoped by authenticated account,
access and conversation. Switching conversations and refreshing the same tab
restores it. Successful sends, conversation deletion and logout clear the relevant
drafts; failures do not automatically resend or clear them. Storage failures leave
the in-memory draft usable and display a warning. Only text (up to 16,000 characters)
is saved, not attachments, selected skills, approval answers or permissions. This
is tab-local plaintext browser storage, not encrypted cloud history or cross-device
draft synchronization. Closing the tab normally ends its lifetime; browser session
restoration may preserve tab storage. Scope keys never contain authentication tokens.

Session details show observations for the last send in the current mounted
workspace: send request round trip, first new assistant text received, and observed
turn end. Measurements use one browser's monotonic clock, exclude old assistant
text, and flag background-tab observations. A failed or ambiguous send is labelled
unconfirmed rather than given a fabricated completion time. These intervals start
at Send and overlap; they are not additive. They include transport and processing,
and turn duration may also include tools and approval waits. Network, Manager/Edge,
App Server and model-only latencies are not independently instrumented yet. No
message bodies or timing telemetry are sent to an external service.

Live summaries expose `attention` (`approval`, `input`, `failed`) and scoped
`attention_sessions` independent of the selected search page. These are display
signals only, never authorization capabilities or archived approval requests.
`reply_token` is a digest of assistant output and its round, unaffected by
renaming or tool-only updates. The client keeps read markers per access in this
browser tab, baselines previously unseen sessions, and clears unread only after
the corresponding snapshot is loaded in a visible tab. Read state is not synced
between browsers. No retention, database schema or native execution policy changes.

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

The current round remains bounded (recent messages and activities, explicitly marked truncation) but never stops native execution because of display limits. The UI shows a continuous transcript: initially up to 19 archived rounds plus the live round, then prepends up to 20 earlier rounds on demand without replacing visible content. Completed execution details remain collapsed by default. Only the live round is refreshed during streaming.

The `transcript` action accepts an optional `history_limit` (1–20), with the existing exclusive `history_before` cursor. Batch replies contain `history_pages` in descending window order and an exclusive `history_before` cursor for the next request (absent at the start of history). A streamed composite-index query caps encrypted payloads below 2 MiB per request; oversized batches return fewer complete rounds, never skip or split a round. Authorization and deletion are rechecked after reading. Omitting the limit preserves the legacy single-round response. This changes no schema, retention, or native context. Pages have the same owner/access/connector/session scope, encryption, deletion semantics and no TTL as the session. A single extremely long round is not a lossless transcript: its display may be truncated; the native Codex history remains on the device. Rollback retains the additive page table.

### Structured interaction and execution details

- Project documented App Server `turn/plan/updated` events into bounded plan steps with live completion state. Proposed `plan` items use the safe Markdown renderer. Show only safe names/queries for search and tool calls, not tool arguments, raw results, raw reasoning, or remote images. See the [official event protocol](https://developers.openai.com/codex/app-server).
- Keep conversation width constrained. Long commands, paths and tool names must not enlarge the transcript; wide code and diffs scroll only inside their own containers.

- Native `item/tool/requestUserInput` requests render single-choice options or free text (password inputs for secret questions). Liaison does not infer questions from assistant Markdown. Support at most three questions and eight options per question; unsupported requests fail closed.
- Answers use owner/access-scoped, opaque, single-use IDs and the original native request ID. Turn completion, shutdown and `serverRequest/resolved` invalidate pending requests. Never archive live request IDs or replay them on resume.
- Display history keeps questions and answers, but masks secret answers. This does not redact secrets subsequently echoed by the model or a command; execution output must be treated as sensitive owner-only history.
- Command output streams into expandable details; authoritative completed items replace partial output. Show command, working directory, exit code, duration and per-file unified diffs as plain text, without executing commands or resolving file paths. Raw reasoning is not displayed.
- Details are bounded to 16 KiB per text field and 96 KiB per display window, with explicit truncation; truncation must never terminate a task. Recent steps can reclaim older completed output. Discovery previews retain metadata only. Older histories without these fields remain readable.

### Execution permissions and lifetime

- Saved-access turns have no three-minute wall-clock deadline. The 30-minute idle lease and 24-hour process age reclaim only idle runtimes; neither deletes history. Temporary discovery previews retain their short lease.
- Sessions start and resume read-only. A user may select workspace-write while idle; the next native turn permits writes under that session's canonical working directory, keeps network access disabled, and excludes ambient temporary directories. Selection is session-local and resets to read-only on native resume. It never edits Codex configuration.
- Native command approval requests are shown to the owner with their exact command, working directory and reason. Only accept-once and decline are supported. No execution-policy amendments, persistent grants, generic JSON-RPC forwarding, or automatic approval.
- Native file-change requests can be approved once or declined when their current item contains complete, non-truncated text diffs and all source/destination paths remain inside the canonical project. Missing details, unsupported change kinds, missing parent directories, symlink escapes, and `grantRoot` requests fail closed. Pending diffs are live-only, like command approvals.
- Question cards have an explicit Stop turn action; cancellation interrupts the native turn rather than fabricating an answer. Activity and command details open during execution and collapse on completion unless the user explicitly changed their expansion state. Prior rounds do not reopen when a new round runs.
- Approval IDs are opaque, live, turn-bound and single-use. Owner/access authorization runs on every action. Completed/closed turns clear pending requests; encrypted history does not store them. Unknown, oversized, network-only, stdin, file-root and permission-profile requests fail closed until their scopes have dedicated UI support.

- Run race tests for control plane, DAO, Edge bridge and protocol; cover stale writes, deletion tombstones, owner/access/connector isolation and non-renewing background snapshots.
- Validate SQLite migration and reopening the database; test pagination beyond 50 histories.
- Exercise Chinese/English, light/dark, desktop/mobile, including entering history from an offline connector's access list.
- Back up existing Edge snapshots before rollout. Deploy Manager first and read each existing session through it to persist history before upgrading Edge.
- Verify a minimal model response is saved after closing the browser, then verify Manager restart and archived history after Edge restart. Delete only synthetic regression accesses.
- Rollback keeps the additive history table and stable encryption secret. Old binaries may not display durable history, but must not drop the table.
