# AI-only WebSSH completion

For the Chinese comparison of approaches and proposed shell-context extensions,
see [WebSSH completion research](guides/webssh-completion-research.md).

The 2026-09-11 revision replaces the experimental static command/option catalog
and remote directory snapshots with model-generated suggestions only. There is
no rule-based fallback. Shell integration supplies input boundaries, not candidates.

## Use

Enable `LIAISON_WEBSSH_SHELL_INTEGRATION=true` on the manager and reconnect to
Bash >= 4.4. Users also need the existing `ai.access.use` permission and an enabled
Agent model. The permission and live handle ownership are checked by the existing
Assistance service, not only by the frontend.

- Type a command in the terminal, then press Ctrl+Space or click Suggest.
- Optionally enable automatic AI for this connection. After 800 ms of idle,
  confirmed remote echo is submitted. It starts disabled on each connection.
- Tab or clicking inserts the returned suffix. Enter is never sent automatically.
- Escape dismisses; typing, leaving the terminal, execution, paste, composition,
  resize and disconnection cancel pending suggestions.

By default only the current command draft is sent to the configured model.
The connection-local context selector can explicitly include the current directory
and recent completed commands, or additionally their bounded output. Changing
scope cancels pending suggestions and turns automatic suggestions off. Reconnecting
resets sharing to draft only. Directory listings and Agent chat history are never
added by this feature. Do not put secrets in shared drafts, commands or output;
best-effort redaction is not a guarantee that all sensitive text is removed.
No candidate is shown when the model is unsure, fails, or times out (15 seconds).
Model output is untrusted: it can be incorrect and must be reviewed before Enter.
Without directory context, paths proposed by the model are not verified to exist.

## Implementation

OSC 633 A/B/C/D marks prompt and execution boundaries. xterm's normal buffer
provides the echoed, single-line input; the cursor must be at the end and the
terminal focused. This protocol is cooperative, not an authorization boundary.
Remote programs can forge OSC markers. Do not treat them as proof that input is
non-sensitive or as approval to execute a tool.

The popup calls `/api/v1/assistance/suggestions` through a separate editor ID tied
to the active access handle, reusing the existing model provider and permissions.
Revisions, AbortController, input equality checks and handle checks discard stale
results. Newlines/control characters and oversized results are rejected. No tools
are invoked and no terminal command is executed by the completion request.

The Bash startup uses a temporary rc file and leaves remote dotfiles unchanged.
The directory enumeration extension has been removed. Zsh/Fish, multiline input,
middle-of-line replacement are not supported yet.

## Connection-scoped Shell context

The backend parses fragmented OSC 633 P/E/C/D reports from the bound WebSSH output
stream. It records cwd, best-effort submitted command text, exit status, approximate
elapsed time and bounded output. Bash command text comes from shell history, not
keystroke reconstruction: ignored or unavailable history is marked `unknown`.
Background output, forged markers and custom shell configuration can make this
association inaccurate; the snapshot is labelled `best_effort_untrusted`.

Storage is in connection memory: at most eight completed records, with snapshots
limited to the latest three records within ten minutes. Output per record is capped
at 2 KiB and serialized context at 16 KiB. No new database history is created.
Completed output is filtered for terminal controls and common secret patterns.

The browser sends `context_mode` (`none`, `commands`, or `output`), not history or
cwd. The backend resolves the active handle under the authenticated owner before
reading context; the existing Assistance permission checks still apply.
Sidepanel `terminal.read` also returns structured `shell_context` when available,
alongside its existing terminal-output result. It is an independently authorized
Agent tool: the completion selector controls completion sharing, not that tool.
The two session types do not share conversation history.

## Verification and rollback

```sh
node web/e2e/terminal-completion.cjs
go test -race ./pkg/liaison/manager/agent/assistance ./pkg/liaison/manager/web -run 'Test(ShellLiteral|WebSSH|Suggest|ModelGenerator|Assistance)' -count=1
npm --prefix web run build
```

The Vite-only `/e2e/terminal-completion.html` harness uses a deterministic mock
provider to exercise opt-in, cancellation, stale responses, empty/error responses,
control-character rejection, suffix insertion, echo and alternate-screen gating.
It is not a substitute for a real-model staging test or part of the production build.

Disable automatic AI to stop background requests. Disable the manager feature
flag, recreate the manager and reconnect to restore the original SSH shell request.
No database migration or remote dotfile rollback is required.

## Staging verification (2026-09-11)

Real-model WebSSH tests passed: no request before opt-in, Ctrl+Space request,
exact equality between the model response and popup, suffix-only Tab, automatic
AI, disabling clears the suggestion, no directory enumeration and no browser
errors. No generated commands were executed. The manual sample returned in
approximately 0.8 seconds; this is a single observation, not a latency guarantee.

The connection-context revision additionally passed the real-shell
`web/e2e/ssh-context-ui.js` scenario: cwd, nonzero exit status, reported history,
ignored history without stale-command reuse, structured `terminal.read`, all three
sharing scopes, and server-authored context. Generated commands were not executed.
Backend race tests cover fragmented/oversized markers, bounds, expiry, common
secret patterns and cross-user isolation.
