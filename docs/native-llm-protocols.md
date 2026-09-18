# ADR-008: Native LLM application protocols

- 状态：已接受（用户确认方案；发布仍需测试验收）
- 日期：2026-09-15
- 替代：不替代 ADR-007；扩展其协议边界。

## 背景

单一 LLM 应用类型无法清晰区分厂商原生接口。兼容 Chat Completions 不等于支持厂商原生请求、流式事件及 Token 用量。

## 决策

新增 OpenAI、Anthropic、Ark、Qwen（DashScope）、Gemini、Ollama 六种应用类型；LLM 保留为菜单分类及旧数据兼容类型。协议由配置决定，不能根据模型名猜测。

原生请求保持原生内容与事件形状；Liaison 只做允许的操作校验、模型别名映射、认证替换、用量计量与安全错误处理。所有网络访问仍经过已授权连接器。已有 OpenAI 转换入口保留，不能静默改变旧客户端。

Responses 首期无状态，不开放 response ID 检索、previous_response_id、conversation、后台任务或上游文件/缓存资源引用；这些功能需要额外的用户所有权登记。无原生模型列举接口时允许手动配置，不伪造“探测成功”。

## 备选方案

### A：仅更换厂商名称和 Logo

工作量小，但原生协议未适配会误导用户，不采用。

### B：所有厂商统一转换成 Chat Completions

客户端一致，但可能丢失思考、工具与原生参数；仅保留明确实现的兼容转换，不作为原生入口。

## 后果

应用分类清晰、原生 SDK 可接入；代价是需要分别验证事件终结、用量、取消与错误。缺失用量仍为未知，不能视为零，配额保持失败关闭。旧应用记录不迁移、不重写密钥指纹。

## 实现说明

复用现有 IAM、API key、模型范围、审计、配额与撤销链路。协议目录集中声明默认路径和能力；新应用类型与上游协议不一致时拒绝保存。原生接口遵循 api/ai_gateway.proto 的补充 HTTP 契约。

## 已实现范围

| 应用类型 | 原生调用 | 模型发现 |
| --- | --- | --- |
| OpenAI | 默认 Chat Completions；可开启无状态 Responses | models |
| Anthropic | Messages；保留已有 Chat Completions 转换 | models |
| Ark | /api/v3 Chat Completions、无状态 Responses | 手动填写部署/模型 ID |
| Qwen / DashScope | text-generation/generation、原生 SSE | 手动填写模型 ID |
| Gemini | generateContent、streamGenerateContent | 有界分页 models |
| Ollama | chat、NDJSON、授权范围 tags | tags |

- 新应用按具体协议创建；旧 llm 记录无需迁移。配置、访问列表、原生 curl 示例和在线体验已接入。
- Gemini/Qwen 在线体验仅适配文本消息，不声称提供完整 OpenAI 兼容 API。
- 复用用户隔离、模型别名、密钥撤销、Token 总配额与请求审计。缺失用量保留未知，不记作零；中断不计作成功完成。
- Gemini 模型探测最多 10 页、1000 个可生成模型，重复分页 Token 或超限不返回部分成功目录。
- Anthropic 模型探测按 `last_id` / `after_id` 自动翻页，最多 10 页、1000 个去重模型，共享 10 秒超时。缺失或重复游标、任一页失败或超限均不返回部分目录；不修改已保存的模型映射。
- Responses 支持文本输入和客户端函数工具；强制 store=false。不开放检索、conversation、previous_response_id、后台任务、上游文件和托管工具。
- Gemini 支持单候选、文本/内联数据/客户端函数；拒绝没有用户所有权登记的文件、缓存和 URL 工具。
- Ollama 不开放 pull/delete/加载卸载等模型生命周期与资源管理操作。
- DashScope 本轮仅文本生成 endpoint，不包括独立的多模态、音频和图像服务。

## OpenAI 能力配置

OpenAI 与 OpenAI-compatible 在应用、访问和导航中统一为 OpenAI。API 能力独立选择 Chat Completions 或 Chat Completions + Responses；新建默认前者，不根据模型名称推断 Responses 支持。

内部继续保留 `openai-compatible`（仅 Chat Completions）和 `openai`（含 Responses）配置值，兼容已保存配置与旧链接。已有配置不自动升级能力。同一目标仅切换能力时，已保存上游密钥重新绑定加密；目标地址、连接器、接口路径或 TLS 改变仍要求重新确认密钥。客户端密钥与调用地址不变。

## 验证记录

- aigateway / controlplane / web race 测试通过：包括七种应用类型、密钥隔离/撤销、流终止、异常用量及 Gemini 分页。
- 前端生产构建、SQL/Search 协议回归通过。
- 原生 LLM UI 浏览器 fixture 回归通过：七协议 × 中英文 × 深浅色，桌面/手机截图及页面溢出检查。
- 上述是自动化协议 fixture 与真实 DAO 测试，不等同于七家真实云账号的 SDK 联调。云服务凭据和付费调用未作为本轮测试前提。

## 协议参考

- [OpenAI Responses](https://developers.openai.com/api/reference/resources/responses/methods/create)
- [Ark SDK Responses](https://github.com/volcengine/volcengine-python-sdk/blob/master/volcenginesdkarkruntime/resources/responses/responses.py)
- [DashScope 文本生成](https://help.aliyun.com/zh/model-studio/qwen-api-via-dashscope)
- [Gemini GenerateContent](https://ai.google.dev/api/generate-content)
- [Ollama Chat](https://docs.ollama.com/api/chat)
