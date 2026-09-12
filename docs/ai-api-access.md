# AI API access

Liaison exposes an existing internal model service through its connector. Clients
receive an OpenAI-compatible API endpoint and a scoped Liaison key; they do not
receive the upstream credential or choose the upstream address.

This feature is separate from the model used by Liaison's built-in Agent. Saving
an Agent provider does **not** publish it as an application.

## Set up

1. Create an application with type **LLM API**, one connector, and the internal
   service's host and port. The service must already be running.
2. Open **Model API** on the application. Select the upstream protocol, API base
   path (usually `/v1`), HTTP or HTTPS, and optional upstream credential. Save and
   probe to inspect model metadata through the connector.
3. Create an **AI API** access for this application. No additional public TCP port
   is required. Open the access and map public aliases to internal model IDs.
4. Enable API calls, then create a named key with explicit model scope and expiry.
   Copy the secret immediately: it is never returned by subsequent reads.
5. Copy the access's Base URL into an OpenAI-compatible client. Use the Liaison
   virtual key, **not** your dashboard token or upstream key.

The models endpoint and inference enforce the same intersection: models explicitly
allowed by the key **and** still present in the access's current mappings.

Example using a client-provided environment variable:

```sh
curl "${LIAISON_AI_BASE_URL}/chat/completions" \
  -H "Authorization: Bearer ${LIAISON_AI_KEY}" \
  -H 'Content-Type: application/json' \
  -d '{"model":"chat","messages":[{"role":"user","content":"Hello"}],"stream":true}'
```

Base URL format: `https://<liaison-host>/api/v1/ai/accesses/<access-id>/v1`.
Supported client operations are `GET /models` and `POST /chat/completions`.
The console's **Try a request** makes a real inference request under the current
user's permissions and may incur costs at the internal upstream.

## Protocol compatibility

| External API | Internal API | Implemented behavior |
| --- | --- | --- |
| OpenAI Chat Completions | OpenAI-compatible Chat Completions | JSON and SSE; model alias rewriting; other request fields retained, including tools when supported by the upstream |
| OpenAI Chat Completions | Anthropic Messages | Text-only JSON and SSE conversion; leading system messages, user/assistant text, max_tokens, temperature, top_p and stop sequences |

For Anthropic conversion, the conversation must begin and end with a user turn
after any leading system messages. Default max_tokens is 1024; explicit values
must be 1–32768. Tools, images, audio, reasoning, response_format, assistant
prefill, and other unimplemented parameters are rejected rather than silently
dropped. The upstream can impose stricter limits or reject parameter combinations.

Responses, Realtime, embeddings, Gemini-native and Ollama-native endpoints are
not implemented. A service offering an OpenAI-compatible endpoint can use that
endpoint, but metadata compatibility does not certify every inference capability.

Probing issues a bounded models request using the selected protocol. It reports
compatible, authentication required, unknown or unreachable. It does not perform
inference, scan ports, guess the vendor from model names, or automatically prove
tool/multimodal support. Anthropic pagination is not followed: model IDs not in
the first page may be entered manually. Probe results are session-local UI data.

## Security and operational behavior

- Every call reloads the key's user and checks current IAM permission, resource
  visibility and enabled application/access state. Key lists and request records
  are user-isolated. Shared application configuration requires application access;
  changing it requires update permission.
- Keys have 256 bits of randomness, are hashed in storage, expire in 1–365 days,
  and are restricted to one user/access and explicit models. At most 100 active
  keys per user/access are allowed. Rotate by creating a replacement, updating the
  client and revoking the old key.
- Upstream credentials are authenticated-encrypted using a key derived from the
  existing credential key. Keep that key and database backups together. Changing
  the application target, connector, protocol, base path or TLS requires replacing
  or explicitly clearing the saved credential. Reads show only a saved indicator.
- Transport is connector-only and fixed-target. There is no direct-dial fallback,
  system HTTP proxy, redirect following, client-selected Host, arbitrary API path,
  credential forwarding from the caller, or automatic retry/failover.
- Upstream HTTPS verifies certificates and hostnames (TLS 1.2+). HTTP is explicit
  and unencrypted between the connector and the service; use HTTPS where required.
- A manager permits 8 concurrent inference requests, with a 5-minute request
  deadline, 30-second upstream response-header deadline and 10-second metadata
  probe deadline. Saturated requests receive 429. These are safety bounds, not
  distributed per-user billing quotas or rate limits.
- Request bodies are limited to 1 MiB, non-stream responses to 8 MiB and SSE
  lines/frames to 1 MiB. Streams flush incrementally. Disconnect/Stop cancels the
  upstream; permission/key/access/mapping changes are rechecked every 2 seconds
  during an active request. Incomplete streams do not receive a successful DONE.
- Records contain request ID, key ID (0 for console tests), model alias, status,
  timing and nullable upstream-reported token usage. No prompts, answers or raw
  credentials are recorded. Keep at most 500 records per user/access; the console
  displays the latest 50. Failed streams may have HTTP 200 already sent while their
  recorded final status is 502 or 499; check the stream error and completion flag.
- Deleting an application/access makes its old keys unusable. Its metadata is not
  cascade-purged by this first release; database retention/backups remain an
  administrator responsibility. Do not use this local bounded history as a
  compliance-grade immutable audit archive.

## Verification and rollout

### Durable Token usage

`llm_token_usage` stores one immutable measurement per inference request, bound
to user, access, key and public model alias. Key ID zero means the authenticated
Playground, not anonymous use. Request IDs deduplicate recording retries. The
ledger and request metadata are inserted in one transaction; the 500-request
retention policy does not delete usage. Revocation does not erase measurements.

`GET /api/v1/ai/accesses/{id}/usage` requires a dashboard login and current access
use permission. It returns the caller's rolling 30-day summary and latest 100
measurements in that window. Even administrators see only their own usage here.
Input and output totals sum reported values independently; null means unreported,
not zero. `unknown_requests` counts requests missing either count. Check `complete`
for interrupted responses; reported partial usage is not a final billed amount.
All timestamps are normalized to UTC.

Accounting starts after this schema/code rollout; old request logs are not
backfilled. This is operational metering, not guaranteed billing: a process crash
or database write failure can leave a gap, and upstream reporting can be partial.
Write failures are logged with a request ID. This is not monetary billing. Store no prompts,
answers or credentials in this table. Export before any explicit retention cleanup.

### Consumer workspace

The access console separates Overview, Playground, API keys, Request records and
Access configuration. Upstream protocol and credential settings stay on the
application page. `/api/v1/ai/accesses/{id}/workspace` returns only public aliases,
access name, enablement and the current actor's management capability. Configuration
GETs require update permission; all routes still enforce resource visibility.
Keys and request records remain scoped to the current user.

Playground sends real inference, supports in-page multi-turn history, Markdown,
Stop and a new-conversation action. History is not persisted. Safe error codes and
request IDs help distinguish upstream authentication, model/endpoint and rate-limit
failures without exposing provider response bodies. Existing SDK URLs are unchanged.

This is the workspace foundation, not a billing or multi-upstream routing release.
Routing/failover and aggregate usage dashboards remain later work.

### API key Token limits

Create a key with `token_limit` or update its lifetime limit with
`PUT /api/v1/ai/accesses/{id}/keys/{key_id}/quota`, body `{"token_limit":100000}`.
Explicit null removes the limit; zero blocks inference. Limits are integers from
0 to 1e12. Editing requires access update permission AND ownership of the key.
Existing keys remain unlimited. Used tokens are not reset when the limit changes,
the month changes, the model alias changes or request logs are pruned.

The key list displays reported lifetime input+output, limit, remaining and state.
Each new inference checks settled usage before forwarding: HTTP 429 with
`TOKEN_QUOTA_EXHAUSTED` means used >= limit. `TOKEN_USAGE_UNCONFIRMED` means a
previous request has incomplete or missing usage; limited keys pause rather than
treating unknown consumption as zero. Increasing a limit does not clear unknown
usage. An authorized owner can remove the limit or revoke and replace the key;
neither action deletes its usage history. Unlimited keys show unknown usage too.
Limited OpenAI-compatible streams request `stream_options.include_usage=true`,
even if the caller omits or disables it. The upstream must support usage reporting.

This is an admission threshold, not hard billing. Already admitted requests,
including concurrent requests, can exceed the remaining amount and are allowed
to finish. No estimated reservation or per-token cutoff is claimed. Missing usage
from a crash or a failed database write remains a metering limitation. Model-list
requests do not consume Token quota. Playground uses the dashboard identity and
does not spend or bypass authentication on an API key; it is not covered by this
per-key policy. No global user/organization budget is implemented.

Quota acceptance: 33 protocol/access E2E checks passed, including exhausted-key
concurrent rejection, JSON/SSE settlement, missing usage and quota adjustment.
`web/e2e/ai-key-quota.cjs` additionally verifies creation, invalid limits, editing,
zero-limit state and removing limits in Chinese/English, light/dark and mobile.
Both scripts use staging-only environment inputs and clean up their resources.

The workspace acceptance passed 29 protocol/UI checks plus real-model JSON and SSE
calls through a connector. Build the manager with CGO enabled: SQLite is required.

Backend tests:

```sh
go test -race -timeout 120s ./pkg/liaison/...
cd web && npm run build
```

`web/e2e/ai-gateway.cjs` creates isolated applications/accesses and a temporary
user, checks both protocol paths through a real connector, then deletes its
resources. Supply E2E_BASE_URL, E2E_EMAIL, E2E_PASSWORD, E2E_AI_EDGE_ID,
E2E_AI_HOST, E2E_AI_PORT and optionally PLAYWRIGHT_MODULE. The explicit protocol
fixture (`python3 web/e2e/ai-gateway-fixture.py`, loopback port 19380) must run on
the connector host and expose `/v1` and `/anthropic`. This validates
transport/protocol behavior, not real-model quality or provider compatibility.

The September 12 staging acceptance passed 27 checks covering scoped discovery, JSON/SSE alias
rewriting, conversion rejection, credential read protection, access disabling,
key revocation during a live stream, cross-user denial, metadata-only records,
Chinese/English UI, mobile layout, console Stop, oversized requests, unsupported
endpoints and concurrent-request saturation. Supplementary tests exercise invalid paths,
redirect denial, cancellation, key expiry, changed targets, request/response
bounds, and SQLite migration up/down plus AutoMigrate compatibility.

Before rollout, make an online-consistent SQLite backup and preserve the current
container image/Compose configuration. The schema change is additive. Roll back
the image to disable the new routes; do not run the destructive down migration
on live data without a separate export and explicit approval. If replacing a
binary in the non-root container, restore `cap_net_bind_service` before starting
it on a privileged port such as 443.

## Protocol references

- [OpenAI Chat Completions](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create)
- [OpenAI models](https://developers.openai.com/api/reference/resources/models/methods/list)
- [Anthropic Messages](https://platform.claude.com/docs/en/api/messages/create)
- [Anthropic streaming](https://platform.claude.com/docs/en/build-with-claude/streaming)
