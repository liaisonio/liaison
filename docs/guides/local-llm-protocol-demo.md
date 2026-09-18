# Local LLM protocol simulator

Runs beside an existing local vLLM/OpenAI Chat Completions service. Real text
inference is exposed through six protocol families. OpenAI/Ark Chat and Responses
use vLLM's own interfaces; other families translate text into native-shaped
responses. This is a development simulator, not a cloud vendor implementation.

```sh
node scripts/llm-protocol-demo.mjs
node --test scripts/llm-protocol-demo.test.mjs
# Requires the simulator and local model to be running; performs real inference:
node scripts/llm-protocol-demo-smoke.mjs
```

Defaults: upstream `http://127.0.0.1:18081/v1`, model `qwen3-0.6b`, simulator
`127.0.0.1:18082`. Override with `LLM_DEMO_UPSTREAM`, `LLM_DEMO_MODEL`,
`LLM_DEMO_PORT`. It binds only to loopback, with no upstream authentication.
Use a connector on the same host to reach it; do not expose it directly publicly.

| Liaison application protocol | Host / port | Base path | Model discovery |
| --- | --- | --- | --- |
| OpenAI | 127.0.0.1:18082 | /v1 | /v1/models |
| Ark | 127.0.0.1:18082 | /api/v3 | Manually enter the configured model |
| Qwen / DashScope | 127.0.0.1:18082 | /api/v1 | Manually enter the configured model |
| Anthropic | 127.0.0.1:18082 | /v1 | /v1/models |
| Gemini | 127.0.0.1:18082 | /v1beta | /v1beta/models |
| Ollama | 127.0.0.1:18082 | /api | /api/tags |

Use HTTP, leave the upstream key empty. The advertised model ID is the configured
local model; it is not renamed to a cloud model. Discovery lists that configured
model, not a vendor catalog or an upstream availability check.

```sh
curl http://127.0.0.1:18082/v1/messages \
  -H 'Content-Type: application/json' \
  -d '{"model":"qwen3-0.6b","max_tokens":64,"messages":[{"role":"user","content":"你好，请用一句话介绍你自己"}]}'
```

## Boundaries

- Text-only, system messages and multi-turn input; generation limit 1–1024 tokens (including Playground requests).
- OpenAI/Ark: POST chat/completions and responses under their base paths,
  JSON and real upstream SSE forwarding, with backpressure and disconnect cancellation.
  Requires a vLLM version that implements Responses; there is no silent fallback.
- Responses is stateless: forces store=false/background=false; rejects history
  references, tools and non-text inputs. No retrieval, cancellation-by-ID or delete endpoints.
- Qwen: POST services/aigc/text-generation/generation, with input.messages;
  parameters supports result_format (text/message), max_tokens, incremental_output
  and enable_thinking=false. X-DashScope-SSE: enable selects SSE.
- **Buffered streaming for Anthropic/Gemini/Ollama/Qwen**: vLLM completes inference before events are emitted.
  This tests parsing and termination, not token latency, backpressure or mid-stream cancellation.
- Token counts come from vLLM, not vendor tokenizers or billing. Translated
  responses fail if usage is missing; OpenAI/Ark preserve vLLM output, including
  incomplete responses and usage. No usage is invented. Disconnect cancels inference.
- Unsupported fields, tools, images, thinking, cached resources, model management,
  and unknown models are rejected. Not a full SDK compatibility suite.
- Local simulator has no authentication. Test Liaison authentication, quotas and
  user isolation at the Liaison entry, not by calling this port directly.
- Existing OpenAI service remains on its original port; no connector is changed.

Protocol references: [Anthropic streams](https://platform.claude.com/docs/en/build-with-claude/streaming),
[Gemini generation](https://ai.google.dev/api/generate-content),
[Ollama chat](https://docs.ollama.com/api/chat).

Additional references (OpenAI Docs used to constrain stateless Responses):
[OpenAI conversation state](https://developers.openai.com/api/docs/guides/conversation-state),
[DashScope generation](https://help.aliyun.com/zh/model-studio/qwen-api-via-dashscope),
[Ark SDK example](https://github.com/volcengine/volcengine-python-sdk/blob/master/volcenginesdkexamples/volcenginesdkarkruntime/completions.py).
