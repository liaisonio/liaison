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
