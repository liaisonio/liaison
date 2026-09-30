# Existing Agent integration

This package discovers **installed** Agents and can launch an explicitly owned
Codex app-server over stdio. It does not install Codex, change its model/provider
settings, read authentication files, attach to desktop pipes, or stop externally
started processes. Codex reads its own local configuration and authentication.

## Local verification

Use the same non-system OS account that normally runs Codex:

```sh
liaison-edge --agent-discover
liaison-edge --agent-check --project /absolute/project/path
```

Both accept `--path /absolute/path/to/codex`. Discovery never executes candidates.
The check starts a temporary app-server, initializes the protocol, checks only
authentication readiness, and stops the owned process. It does **not** send an
inference request. It does not require Edge tunnel configuration.

Discovery checks explicit path, account PATH, standard locations, the macOS app
bundle, npm prefix, Volta, and bounded nvm/fnm directories. Results identify
candidates, not verified software identities. Empty/relative PATH entries and
duplicate symlink targets are skipped. No login shell or whole-disk scan runs.

## Boundaries

- Agent layouts/protocol adapters are separate from platform discovery/process
  mechanics. `runtime.Adapter` and `runtime.Session` define the launch contract.
- Registry handles are scoped by owner, access and project, with bounded live and
  in-flight launches. Manager checks feature permission, connector update permission,
  and exact connector ownership on every request, including polling.
- Codex threads start read-only, and only threads created in that owned
  session can receive messages. Saved accesses support explicit workspace-write
  selection and one-shot command approvals; unsupported requests fail closed.
  This is not a filesystem isolation
  guarantee between projects or Liaison users sharing an OS account.
- The transport bounds frame size, pending requests and event queue; overflow
  closes the session instead of silently dropping events. Consumers must drain
  events and handle/reject server requests. Tasks are never replayed automatically.
- macOS/Linux launch uses a separate process group. Root/system-user launch is
  rejected. Windows discovery is implemented, but Windows launch deliberately
  fails closed until Job Object supervision and native tests are implemented.
- Environment forwarding is allowlisted; Edge-specific secrets are excluded.
  Custom providers requiring additional environment variable names are not yet
  supported. Codex app-server protocol compatibility is tested against 0.153.4;
  older/newer versions are not guaranteed.
- Access → Agent provides discovery, project selection, persistent chat,
  interruption and owned-instance shutdown through the existing Edge tunnel.
  Native Codex threads are persisted by the existing installation. The Edge keeps
  an owner/access/session-scoped binding in its instance directory and can resume
  it after an owned app-server or Edge restart. Live instances have a 30-minute
  idle lifetime; instances older than 24 hours are reclaimed only while idle.
  Saved-access turns have no fixed wall-clock timeout. Server history has no TTL.
  Completed rounds are encrypted and paginated on Manager. Display limits may
  truncate very large rounds, but never cancel native execution.
- Apps, configured MCP servers and plugins are disabled with per-thread
  overrides because a filesystem sandbox cannot constrain their external effects.
  The runtime tool inventory must then be empty or explicitly disabled; otherwise
  inference is rejected. Model/auth settings are not overridden, and local settings
  are never rewritten. Configuration and inventory are never exposed to the browser.
- Resume is explicit and applies only to Liaison-created bindings. Liaison never
  lists or imports unrelated native Codex threads. Missing local records, changed
  project/install identity and legacy ephemeral sessions fail closed while server
  history remains readable. Arbitrary file-root permission grants and Windows
  process supervision are not provided. Deployment must be verified separately.

## Verification

```sh
go test -race ./pkg/edge/agent/...
LIAISON_TEST_AGENT_DISCOVERY=1 go test -v ./pkg/edge/agent/adapters/codex -run TestCurrentHostDiscovery
LIAISON_TEST_CODEX_HANDSHAKE=1 go test -v ./pkg/edge/agent/adapters/codex -run TestLocalHandshake
```

The handshake test starts an owned app-server and creates a native thread without
model usage. Adding `LIAISON_TEST_CODEX_TURN=1` performs one minimal read-only
inference using the existing Codex configuration and may incur model usage. It
also verifies streamed text, turn completion, owned process shutdown and native
restart/resume. Codex does not persist an empty thread before its first turn.

Adding `LIAISON_TEST_CODEX_INPUT=1` with the turn flag verifies an actual native
question, answer and continuation in plan mode. Experimental mode is enabled
only for this isolated integration test, not for normal product sessions.

Native macOS discovery, handshake/authentication and a streamed turn have passed.
Cross-compilation is not native Linux/Windows runtime validation.

## Claude Code integration

`adapters/claude` adds installed-CLI discovery and a separate bidirectional
`stream-json` transport. It does not pretend to be Codex app-server JSON-RPC.
Manager and the web application support Claude Code applications and accesses.
The Edge selects the provider by the registered installation; Codex retains its
separate app-server transport. No model or credential configuration is copied.

- Discovery covers native default paths, PATH, npm, Volta and bounded nvm/fnm
  layouts on macOS/Linux/Windows. The current launch probe accepts native binaries
  only; npm interpreters and Windows execution are not enabled.
- `StartProbe` starts an owned child with tools disabled, safe mode, an empty
  strict MCP configuration and no session persistence. It reads the user's native
  model/auth configuration through Claude Code, without copying credentials.
- `StartPersistent` is a separate, still tools-disabled driver entry point for
  the trusted session layer. It creates an explicit Edge-generated UUID or resumes
  an exact UUID from an authorized binding, never `--continue`, a name or a path.
  Native response IDs must match. Missing/empty/symlink checkpoint files fail
  closed; new sessions cannot overwrite an existing checkpoint. Close stops the
  owned child only and preserves native history. This is not an authorization API:
  the caller must first validate owner/access/project/installation ownership.
- Transport supports initialization, user messages, partial events, interruption,
  one-shot permission decisions and permission cancellation. Unknown controls,
  malformed frames and queue overflow fail closed; uncertain sends are not retried.
- `AskUserQuestion` is handled separately from approval: local question IDs map
  back to original question text, with bounded single/multiple/free-text answers.
  Missing/duplicate answers and cancelled/replayed requests are rejected. A plain
  allow response cannot accidentally approve an unanswered question.
- An internal provider-neutral update mapper handles streamed text, complete
  message deduplication, tool starts/results and turn completion. Tool input
  generation finishing does not mean execution finished. Native configuration and
  raw failure payloads are excluded; text and tracked event identities are bounded.
  The runtime adapter feeds this mapper into the shared
  session bridge, with explicit native-thread and turn identities. The legacy
  Codex RPC capability is separate and continues to use its existing handler.
- Native events remain an internal SDK boundary, not a browser payload. The
  provider-neutral mapper excludes configuration/errors. Interactive sessions
  expose Read, Glob, Grep, Bash and AskUserQuestion; structured reads follow native
  default permission checks without a blanket Bash ask override, and retain
  safe mode plus an empty strict MCP inventory.
  This is not an OS sandbox: users must inspect commands before approving.
- Shared typed interactions now map inspectable Bash requests to existing
  approval cards and single-choice/free-text questions to existing input cards.
  Native request IDs remain on Edge; decisions are turn-bound and one-shot.
  Cancellation removes pending cards. Unknown tools/fields, background commands,
  sandbox bypass, more than three questions and multi-select are explicitly
  denied pending complete presentation/capability support. This is not a grant
  of general command execution or permission-mode switching.
- New/resumed Claude runtime sessions use persistent native checkpoints and the
  shared bridge's existing owner/access/project/installation bindings. Claude
  has no authentication preflight capability here: initialization is not proof
  of valid provider credentials; authentication failure surfaces as turn failure.
- Limits: 2 MiB per frame, 64 queued events, 32 outstanding outgoing controls and
  32 pending approvals. Callers must drain events and close abandoned clients.

Opt-in native test (uses configured provider; small billable requests):

```sh
LIAISON_TEST_CLAUDE_PROBE=1 go test -race -v ./pkg/edge/agent/adapters/claude -run '^TestNativeProbe$'
LIAISON_TEST_CLAUDE_INTERACTIONS=1 go test -race -v ./pkg/edge/agent/adapters/claude -run '^TestNativeInteractions$'
LIAISON_TEST_CLAUDE_RESUME=1 go test -race -v ./pkg/edge/agent/adapters/claude -run '^TestNativeResume$'
```

Compatibility was probed with Claude Code 2.1.211 on macOS and an existing
Anthropic-compatible DeepSeek configuration. The test uses an empty temporary
directory, disables tools/persistence and never prints credentials or raw frames.
It verifies initialization, two streamed turns retaining context/session identity,
idle interrupt acknowledgement and owned-process cleanup. The separate interaction
probe enables only Bash or AskUserQuestion, forces Bash to ask, and approves only
one exact harmless printf command in an empty temporary directory. Native
allow/deny and question/answer/result roundtrips through the shared interaction
adapter have passed. Native bridge coverage verifies discovery, start, one-shot
approval, shutdown and exact native-session recovery. Browser E2E fixtures verify
application and access creation, provider labels, chat, approval and questions in
Chinese/English, light/dark themes and desktop/mobile layouts. Unit tests cover
cancellation, validation and concurrent one-shot answers. The native resume probe
creates a dedicated transcript, stops/restarts the child, verifies retained random
context and exact session identity, refuses a missing checkpoint, then removes
only its own test transcript. It also reopens that checkpoint via the shared
runtime adapter and verifies normalized thread/turn identities and retained
context. Bridge tests cover non-RPC event delivery, owner isolation and stale
event rejection. The opt-in active-turn interruption test interrupts after the
first streamed text, waits for a terminal result (not just the control ACK),
then verifies a second turn in the same native session. Multi-select questions
remain unsupported. Browser fixtures cover reload during a running turn,
temporary connection failure/recovery without replaying Send, and one-shot Stop.

Persistent launch currently validates the default `~/.claude/projects` layout
with a canonical working directory whose encoded name is at most 200 characters.
Custom config stores and the CLI's long-directory hashing are not enabled until
their checkpoint lookup is tested. These restrictions fail explicitly, rather
than guessing a different session or silently starting fresh.

The web model picker and slash menu use the native initialization catalog.
Model selection is applied through native `set_model` before the next message,
without modifying device defaults. Slash commands are dispatched only by a
catalog-bound ID; paths and arbitrary control payloads are not accepted.
Only commands loaded by the isolated native process are listed: project settings,
hooks and additional tools are not enabled just to populate the menu.
Image/file message attachments, multi-select questions and permanent permissions
remain unsupported. The configured native model is displayed after initialization. Existing authorized
project-file preview and download remain available. Unsupported native interactions
fail explicitly rather than granting extra authority. Deployment and native
Linux/Windows validation remain separate release checks.

```sh
LIAISON_TEST_CLAUDE_BRIDGE=1 go test -race -v ./pkg/edge/agent/bridge -run '^TestClaudeNativeBridgeLifecycle$'
LIAISON_TEST_CLAUDE_INTERRUPT=1 go test -race -v ./pkg/edge/agent/adapters/claude -run '^TestNativeActiveInterrupt$'
# Requires the local web dev server and Playwright; synthetic API fixtures only.
node web/e2e/agent-claude.cjs
```

Neither these tests nor this implementation deploys or restarts a production Edge.

## Timing diagnostics

Session details distinguish browser Send/request/reply observations, Manager
send-handler processing (including authorization, history and connector RPC),
and Edge-local dispatch/first-text/terminal-result durations. Edge values use
one process's monotonic clock and are optional for old connectors or resumed
instances with no new turn. Completed values may be retained with encrypted
history. Missing is not zero. Dispatch is native request acceptance, not proof
that inference started. Completion includes tools and approval waits. None of
these measurements is pure model latency; browser-minus-server is not pure
network latency either. Raw provider frames, prompts and credentials are not
included in diagnostics.
