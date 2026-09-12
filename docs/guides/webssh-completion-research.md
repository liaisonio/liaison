# WebSSH 智能补全：方案调研与上下文选型

更新时间：2026-09-11。状态：调研整理，包含已实现方案与待评审扩展，不代表全部能力已交付。

本文根据已有讨论、仓库原型记录和本次重新核对的官方资料整理。产品内部实现没有证据的部分不作推断。当前功能使用方式见 [AI-only WebSSH completion](../webssh-completion.md)。

## 1. 先说结论

Liaison 当前选择：**Shell Integration 提供边界，前端判断交互时机，后端调用 AI，前端展示并接受建议。**

现已增加第一版连接级 Shell 上下文，但不改成“后端猜测所有按键”的模式。如果需要可靠支持行中编辑、多行和历史搜索，再按 Shell 增加编辑器适配，而不是继续叠加终端正则。

必须分开三个问题：

1. **输入识别**：是否在 Shell 命令输入区，草稿与编辑位置是否可信？
2. **候选生成**：AI、历史记录、命令规格、目录等从哪里产生建议？
3. **展示与接受**：下拉框还是浅色 ghost text，按什么键填入？

“不要规则”指不使用静态规则生成候选，不等于去掉输入状态、安全校验、取消和版本检查。采用某项目的输入识别方法，也不意味着必须采用它的候选来源。

## 2. 几种路线对比

以下适用性和成本是针对 Liaison 的工程判断，不是上游项目的性能结论。

| 路线 | 输入状态从哪里来 | 优点 | 主要代价/边界 | Liaison 建议 |
|---|---|---|---|---|
| 客户端终端推断 | 屏幕缓冲区、提示符特征、按键和回显对齐 | 少改远端，集成入口轻 | 自定义提示符、延迟、历史搜索、TUI 容易使推断失效 | 借鉴 UI 与取消机制，不以正则作为自动上传的唯一依据 |
| VS Code 式 Shell Integration | Shell 主动报告阶段，客户端结合终端显示状态 | 明确命令阶段，逐步扩展目录/命令上下文 | 需要 Shell 配合；基础标记不等于完整编辑缓冲区 | 当前主路线 |
| Shell 编辑器深度集成 | Fish commandline、Zsh ZLE、Bash Readline 等 | 更接近真实草稿和逻辑光标 | 每种 Shell、编辑模式、插件都要适配 | 精确行中编辑的后续增强 |
| 独立补全运行时 | 安装运行时并与 Shell 集成 | 可复用完整候选、交互与配置体系 | 增加远端依赖、生命周期及兼容性成本 | 参考 inshellisense，不默认部署整套运行时 |
| 后端 PTY/终端模拟 | 后端解析字符流并重建屏幕 | 浏览器关闭后仍可保留部分输出状态 | 屏幕不是 Shell 编辑缓冲区；不知道浏览器焦点/选区/IME | 用于上下文采集，不单独承担精准补全 |
| 独立命令输入框 | 产品自己拥有的编辑器 | 草稿、光标与版本可控，跨协议一致 | 与真实终端输入割裂，仍需用户填入 | WebData 合适；WebSSH 仅作为可选退路 |

## 3. 开源实现能借鉴什么

### 3.1 VS Code：状态协议与终端侧输入模型

官方协议用 `OSC 633` 的 A/B/C/D 标记提示符和执行阶段；E 可报告命令行，P 的 Cwd 属性可报告工作目录。它需要 Shell 脚本参与，并不是 PTY 原生事件。普通 SSH、嵌套 Shell 和复杂启动配置可能需要额外集成。[官方协议](https://code.visualstudio.com/docs/terminal/shell-integration#supported-escape-sequences)

我们的判断：借鉴阶段标记和终端侧模型；不要把采用 OSC 633 描述成复制了 VS Code 全部能力。尤其 E 的命令报告并不是“每次编辑后自动推送完整草稿与逻辑光标”的通用协议。

### 3.2 Netcatty：候选交互与回显一致性

本次检查本地检出提交 `0ea8fa494381d32dcc14363d83189a0da6f29881`：

- `terminalAutocompletePrompt.ts` 包含提示符特征判断和输入/回显协调逻辑。
- `completionEngine.ts` 使用 Fig 规格、路径、历史等来源，定义候选来源和排序信息。
- `useTerminalAutocomplete.ts` 包含 popup/ghost 的生命周期及输入可靠性处理。

这说明该补全路径不是单纯把终端文本送给大模型；但不能据此断言整个 Netcatty 不支持 AI。代码依据：[输入协调](https://github.com/binaricat/Netcatty/blob/0ea8fa494381d32dcc14363d83189a0da6f29881/components/terminal/autocomplete/terminalAutocompletePrompt.ts)、[候选引擎](https://github.com/binaricat/Netcatty/blob/0ea8fa494381d32dcc14363d83189a0da6f29881/components/terminal/autocomplete/completionEngine.ts)。

我们的判断：借鉴候选定位、过期结果丢弃和 ghost text 一致性；不把 Fig、历史匹配、目录枚举重新作为 Liaison 的静态候选兜底。

### 3.3 Fish / zsh-autosuggestions：在 Shell 内部做提示

Fish 有原生 autosuggestions；其 `commandline` 命令能读取/修改命令行缓冲区及光标位置。Zsh 的 zsh-autosuggestions 提供历史/补全策略以及接受、清除等 widgets。[Fish 交互文档](https://fishshell.com/docs/current/interactive.html#autosuggestions)、[Fish commandline](https://fishshell.com/docs/current/cmds/commandline.html)、[zsh-autosuggestions](https://github.com/zsh-users/zsh-autosuggestions)

我们的判断：这类能力适合作为“Shell 编辑状态适配器”的参考，而不是要求用户更换 Shell。Bash Readline 的适配需要单独验证，不能把 Zsh/Fish 的接口直接套到 Bash；也不能假定绑定一个快捷键就能覆盖所有输入变化。

### 3.4 inshellisense：独立运行时

inshellisense 是 terminal-native autocomplete runtime，提供 Shell 集成及候选接受/切换/关闭交互，需要安装和初始化。[项目 README](https://github.com/microsoft/inshellisense)

我们的判断：可参考运行时和 Shell 适配的组织方式；直接安装到每台 SSH 目标会增加依赖和运维面。它解决的完整规格补全需求，也不等同于我们现在的纯 AI 候选需求。

### 3.5 Termius 与其他产品参照

此前提到的 Termius 下拉提示属于交互参照。本轮没有可验证的核心实现依据，因此不将它归类为“使用 OSC 633”“完整接管 Readline”或“纯后端识别”。看到类似 UI 不能倒推出技术实现。

Ongrid 的模型配置、Agent 会话组织可用于另一层的参考，不能据此证明终端输入识别方案。

## 4. 当前实现：字符怎样流转

```text
用户输入 "git "
    │
    └─ 浏览器 ── WebSocket ── 后端 SSH ── PTY / Bash
                                                │
                       OSC 标记 + 输入回显 ◄─────┘
                                │
                       后端原样转发给浏览器
                                │
          前端确认：输入阶段、单行行尾、焦点、回显已跟上
                                │
              手动 Ctrl+Space / 自动模式停顿 800ms
                                │
              草稿 + 光标 + handle + editor + revision
                                │
                    后端鉴权、调用配置好的模型
                                │
                     返回后缀（例如 "status"）
                                │
              前端校验仍是原草稿 → 显示 → Tab 填入
```

示例中的候选仅说明流转，不保证模型每次返回相同文本。当前只展示下拉候选，不是已交付行内 ghost text。

OSC 是输出流里的控制序列，可跨网络帧拆分；解析器必须处理分片。A 表示提示符开始，B 表示提示符结束，C 表示执行前，D 携带可选退出码。当前集成脚本用 `PROMPT_COMMAND`/PS1/PS0 发出基础标记，不修改永久启动文件。

代码入口：

- [Shell 启动与注入](../../pkg/liaison/manager/web/webssh_shell.go)
- [Bash 标记脚本](../../pkg/liaison/manager/web/shell-integration/bash.sh)
- [前端状态、请求取消和接受](../../web/src/components/TerminalAssistant/terminalCompletion.ts)
- [WebSSH 请求与 UI](../../web/src/pages/WebSSH/index.tsx)
- [Assistance 会话](../../pkg/liaison/manager/agent/assistance/session.go)

## 5. 能不能完全放后端

**可以在后端生成候选、关联上下文，但仅靠 PTY 不能准确知道所有编辑和鼠标状态。**

| 信号 | 后端 PTY 是否直接拥有 | 判断边界 |
|---|---|---|
| 字符、方向键转义序列 | 有，前端发出时 | 收到左键不代表 Shell 光标必然左移一格 |
| 实际草稿、逻辑编辑光标 | 没有通用接口 | 需 Shell 编辑器报告，或有限场景推断 |
| 鼠标移动、文字选择、焦点 | 通常没有 | 普通终端选择是浏览器行为 |
| TUI 鼠标上报 | 启用上报时可能有 | 坐标属于应用交互，不是 Shell 草稿状态 |
| 屏幕光标和显示内容 | 可以用终端模拟器重建部分状态 | 显示位置不等于 Shell 内部编辑位置 |
| 执行阶段 | 集成有效时可获得 | 未集成、标记丢失或被伪造时不能信任 |

因此不能承诺“任意 PTY 下精准判断正在输入”。当前只处理明确的单行行尾输入，方向键等控制操作会暂时抑制提示；远端回显确认光标回到同一提示符的行尾后恢复，没有完整的行中替换能力。缩放和失焦会取消当前候选，但不再永久阻断本轮输入；恢复时仍校验提示符、单行边界和焦点。

Vim 常会进入 alternate screen，但并非所有 TUI 都会。不能仅用 alternate screen 判断是不是 Vim；Shell 执行阶段和交互状态必须共同约束。无法确认时停止自动提示。

## 6. Shell 上下文关联：第一版已实现

```text
Shell 阶段 / 目录 / 已提交命令 / 输出
                    │
           后端连接级上下文采集
                    │
       有界缓冲、分段、质量标记、脱敏
                    │
          ┌─────────┴─────────┐
          ▼                   ▼
    AI 补全请求            Sidepanel 工具
    草稿 + 少量上下文       按需读取命令及输出
```

当前 Bash 集成通过 OSC 633 P/E/C/D 记录目录、最近完成的命令、退出码、近似耗时和有限输出。命令文本取自 Shell 历史，来源标记为 `shell_history_best_effort`；历史忽略或不可用时标记 `unknown`，不复用上一条命令。尚未实现通用 Shell 类型发现或编辑缓冲区接口。

每个连接在内存中最多保留 8 条完成记录；共享快照仅选最近 10 分钟的最后 3 条，每条输出最多 2 KiB，总上下文不超过 16 KiB。快照带 `best_effort_untrusted` 质量标记，不新增数据库历史。常见密钥格式会做尽力脱敏，但不能保证任意敏感内容都被识别。

补全默认仍为“仅草稿”，可选择“共享目录与最近命令”或“同时共享最近输出”。切换会取消旧建议并关闭自动提示，重连恢复默认。Sidepanel 的 `terminal.read` 在有记录时另行返回结构化上下文；它沿用 Agent 工具授权，并不受补全下拉框控制。

注意这几个“并不精确”的地方：

- 命令阶段时间可作为近似耗时，不是进程级精密计时。
- 退出码采集必须验证已有 prompt hooks 不会覆盖 `$?`；现有基础脚本不能据此承诺所有用户配置都准确。
- 后台任务可能跨命令输出，按 C/D 分段不等于进程输出归属证明。
- 嵌套 SSH、Shell、tmux/screen 可能改变标记来源或传输，需独立质量状态和测试。

前端仍发送当前草稿与版本；后端依据已鉴权的连接读取上下文，而不是信任客户端提供的目录/历史。补全与 Sidepanel **共享可授权读取的连接上下文，不共享聊天历史**，Assistance Session 和 Agent Session 保持独立。

### 推荐分期

1. **已实现**：Bash 基础标记、前端单行行尾识别、纯 AI 后缀、取消/版本校验、用户接受后才填入。
2. **已实现第一版**：目录与命令报告、连接级有界上下文、质量标记、补全显式共享；Agent 可通过 `terminal.read` 获取结构化记录分析失败命令，尚无专用失败分析工作流。
3. **待适配**：Zsh/Fish/Bash 编辑器接口、多行、行中编辑、历史搜索与精确替换。
4. **可选 UI**：ghost text。它只是展示方式，不会自动提高输入识别准确度。

## 7. 安全与降级要求

- OSC 标记是协作信号，不是权限证明。nonce 可帮助识别伪造，但不能把不可信远端变成可信环境。
- 不采集逐键输入作为命令历史，不把密码提示下的输入发送模型；仅凭标记或正则也无法保证内容一定不敏感。
- 扩大到输出上下文前，必须明确用户授权范围，配置截断、保留期及脱敏。不能静默沿用“只上传草稿”的授权。
- 按用户和具体连接实例隔离；断开/重连使旧结果失效。外露的会话引用不是 bearer handle。
- 输出和命令都属于不可信数据，不可作为模型的高优先级指令或工具审批。
- 建议无结果、超时、错误时不使用静态候选兜底；不自动执行探测命令来补齐上下文。
- 当前仅接受无换行/控制字符的后缀，不自动回车；更深度的替换协议需额外处理版本一致性。
- 无 Shell 集成或输入状态不明确时，禁用终端内自动提示；如需通用退路，使用独立草稿编辑器。Ctrl+Space 也不能绕过状态与权限检查。

## 8. 后续验收清单

| 场景 | 应有行为 |
|---|---|
| 持续输入、删除、SSH 延迟回显 | 不展示或接受过期建议 |
| 左右移动、Home/End、历史搜索 | 未完成精确适配前抑制补全 |
| 多字节文字、宽字符、IME、多行 | 不截断字符、不写错位置；不支持时降级 |
| 鼠标选区、失焦、粘贴、缩放 | 取消或隐藏提示，不抢夺交互 |
| Vim/top、alternate screen、子 Shell | 不把应用输入误认成 Shell 草稿 |
| 后台输出、自定义 prompt hooks | 标记不完整/不可靠，不编造命令归属或退出码 |
| 断线、重连、权限撤销、跨用户访问 | 旧上下文与结果不可继续使用 |
| 模型返回换行、控制字符或危险建议 | 拒绝不合法插入；合法文本仍须用户审阅，不自动执行 |
| 开启输出上下文共享 | 用户看到准确范围，默认不上传全量历史/环境变量 |

2026-09-11 更新：第一版上下文采集和共享已部署测试环境，真实 Shell 回归已验证目录、非零退出码、历史忽略处理、Agent 读取和三档共享。上表仍包含尚未完成适配的场景，不代表全部能力已经交付。
