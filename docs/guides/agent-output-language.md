# AI 输出语言

在「设置 → 模型配置 → AI 输出语言」选择简体中文或 English，点击保存配置。

- 全局配置沿用模型设置的管理权限和加密存储，不跟随浏览器语言。旧配置默认为中文；旧客户端不传此字段时保留当前值。
- 从下一次模型请求生效，适用于首页、访问层 Sidepanel、Shell Agent 分析及补全，无需重启。进行中的请求和历史消息不改写。
- 中文或英文约束的是模型生成的解释、标题、总结和建议。命令、SQL、代码、路径、标识符、JSON 字段与引用的原始证据保持原样，工具原始输出不会翻译。
- 所有业务推理经过共享模型管理器，在提供方调用前加入 system 语言约束，兼容 OpenAI-compatible 和 Anthropic。模型连接探测仍只要求返回 OK。
- 语言由提示词约束，并非字符级过滤。测试覆盖相反语言提问，但不能保证任意模型绝不偏离；不采用强制翻译以免破坏代码或证据。

验证：`go test -race ./pkg/liaison/manager/agent/modelsettings`；真实模型和设置页面回归：`node web/e2e/output-language.cjs`（使用 STAGING_URL、STAGING_EMAIL、STAGING_PASSWORD 环境变量）。回归短暂切换全局语言并在结束时恢复，同时归档测试会话。
