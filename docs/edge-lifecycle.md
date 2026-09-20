# Edge 生命周期实现状态

## 当前能力：受控远程卸载（默认开启）

已实现预检、持久化任务、独立卸载执行器、短期结果回报凭据、审计、接入密钥撤销及控制台名称确认。新 Edge 默认允许控制台卸载，无需额外安装参数；仍须通过管理员权限和本机归属预检。新安装器使用独立实例，不迁移旧实例或提权。

新增 `GET /api/v1/edges/{id}/installation`，同时要求登录、`connectors.uninstall` 功能权限和目标连接器可见性。该权限默认只随管理员权限目录初始化，不给普通用户自动授权。

响应使用标准 `{code, message, data}` 包装；`data` 包含：

- `version`：当前为 1。
- `ownership_verified`：只读归属预检是否成功。
- `can_uninstall`：预检成功且本地启用远程卸载时为 true；每个 Edge 进程最多接受一次尝试。
- `instance_id`、`service`、`installation_kind`、`legacy`：仅在预检成功时提供。
- `reason`：安全的原因代码，不返回系统命令原始错误或认证密钥。

原因代码：`connector_offline`、`edge_upgrade_required`、`preflight_unavailable`、`installation_unverified`、`uninstall_executor_unavailable`。

Edge 提供可选 JSON RPC `edge_installation_status` 和 `edge_uninstall`。旧 Manager 不会调用它；旧 Edge 不支持时 Manager 返回不可用状态，不改变连接或尝试升级。

控制台入口：“连接器 → 更多操作 → 卸载”。先预检，再输入完整连接器名称确认。

- `POST /api/v1/edges/{id}/uninstall`：`confirm_name` 和预检返回的 `instance_id`，返回 HTTP 202 和任务。
- `GET /api/v1/edges/{id}/uninstall/{task_id}`：查询同一目标下的任务；保留 ID `active` 返回未结束任务，无任务时为 null。关闭弹框再打开可恢复进度。
- `POST /api/v1/edge-uninstall-results/{task_id}`：仅接受短期、单任务回报凭据，不使用用户或 Edge 长期凭据。

任务先持久化，再审计和发送。每个连接器最多一个未结束任务，结果不明时保留锁，不自动重试。状态为 `accepted`、`running`、`completed`、`failed`、`unknown`。回报有效期 5 分钟，不返回凭据及摘要，不以断线推断完成。

预检和指令还绑定每次 Edge 启动随机生成的 `runtime_id`，重启后的进程不接受旧进程的卸载命令。

Manager `server_url` 必须是根路径 HTTPS URL，主机必须与 Edge 的 Manager 连接主机一致。回报不使用环境代理、不跟随重定向，沿用 Edge 的 TLS 验证设置。

## 预检边界

- Linux：核对 root 运行的目标 systemd 服务，或新隔离布局的当前用户 `systemctl --user` 服务，以及 MainPID、FragmentPath、ExecStart；含 drop-in、自定义包装启动命令或权限不足时拒绝。
- macOS：核对当前用户的 LaunchAgent 标签、ProgramArguments 和 launchd 报告的 PID。新独立实例额外要求安装清单的版本、实例 ID、UID、平台和服务名一致，实例与程序目录为当前用户所有且权限 0700，清单权限 0600。
- 两者均核对受支持的布局、文件与父目录权限，拒绝符号链接、硬链接、共享程序引用及无法检查归属的情况。
- 旧标准布局可以做预检，不修改或迁移其目录。下载脚本和控制台生成的命令已接入显式新建模式；升级必须指定实例。代码尚未部署到下载服务。
- 容器、Windows、自定义目录及无法确认的安装返回不可用。不得在预检失败时调用旧卸载脚本兜底。
- Linux 用户级 systemd 已完成原生验证。新独立实例验证安装清单和 0700 专属目录，检查同用户进程和用户级服务引用；其他用户/系统级服务不在该用户安装域内。非 root 运行但由 root 安装的旧系统服务不支持远程卸载。

macOS 新独立实例的服务引用检查范围为 `~/Library/LaunchAgents` 和 `/Library/LaunchAgents`，覆盖当前用户 GUI 域的服务；实际进程的共享程序检查仍保留。系统级 `/Library/LaunchDaemons` 不作为用户实例的服务管理范围。管理员手工让系统服务复用用户私有实例文件不属于支持的安装方式。旧共享布局仍检查系统服务引用，未知归属不会自动放行。

Linux 用户级服务会检查用户配置/数据目录、用户 runtime 目录和系统用户单元模板目录。systemd 用户管理器可能禁止本用户读取其 `/proc/.../exe`：只在系统服务元数据证实其 PID、受 root 保护的 systemd 程序，以及 `init.scope`/父进程关系时排除管理器及其直接 helper；普通进程的读取错误不会被忽略。

## 显式安装与升级

新 Edge 二进制支持 `--edge-install-new`：标准输入依次提供 Manager 的 `host:port`、access key、secret key 三行。凭据不写入服务启动参数；配置权限为 0600。自动生成随机实例 ID，同一 Manager/接入密钥的重复安装会被拒绝。

远程卸载默认开启，新安装自动写入 `allow_remote_uninstall: true`。配置省略该字段时也默认开启；管理员可显式设置 `false` 关闭，升级保留已有配置。旧的 `--allow-remote-uninstall` 参数仅作为兼容入口，不再展示或要求用户添加。默认开启不绕过服务端权限、名称确认和本地归属检查。

- Linux root：程序位于 `/opt/liaison/edges/<id>/bin/liaison-edge`，配置位于 `/etc/liaison/edges/<id>/config.yaml`，使用独立系统服务。
- Linux 普通用户：程序和配置位于 `~/.local/share/liaison/edges/<id>/`，使用 `~/.config/systemd/user/liaison-edge-<id>.service`。必须已有可用的用户服务管理器，不自动启用 linger 或提权。
- macOS：位于 `~/Library/Application Support/liaison/edges/<id>/`，使用独立 LaunchAgent；必须以对应登录用户执行。

`--edge-upgrade-instance <id>` 只升级指定实例；`--edge-upgrade-instance legacy` 明确选择受支持的旧布局。保持配置、凭据、服务文件和路径不变。预检通过后备份原程序，停止目标服务、替换程序并检查进程稳定性；失败时尝试恢复旧程序。此检查不是与 Manager 连通性的保证。

升级与卸载共用实例级互斥锁。无法确认归属、配置变化、停止失败或存在未处理的恢复文件时拒绝继续；恢复失败可能需要人工处理，不自动清除遗留锁或备份。新安装启动失败时只清理本次创建的文件，停止失败则保留文件。日志和空目录可保留，不递归清理共享目录。

## 执行边界

独立 helper 由 systemd 临时服务或 launchd 任务运行，不随目标服务一起终止。任务文件仅当前用户可读；取得 Manager 的运行回执后，再次核对本地服务身份和文件摘要。停止精确服务，并确认原进程消失后，仅移除该服务文件、配置和程序。

日志、应用数据、父目录和业务记录保留。不运行旧卸载脚本，不按进程名批量停止，不递归删除目录，不提权。完成回报后原子撤销目标连接器的接入密钥并将其设为停止/离线；其他连接器密钥不变。

失败可能留下部分安装，需要人工检查；不会自动重启或重复执行。回报丢失时显示“结果未确认”。

## 验证

```sh
go test -race ./pkg/edge/...
go test -race ./pkg/liaison/manager/controlplane ./pkg/liaison/manager/frontierbound ./pkg/liaison/manager/web ./pkg/liaison/manager/iam ./pkg/liaison/repo/dao
cd web && npm run typecheck
```

单元测试使用临时文件和假的原生命令执行器，不停止真实服务。覆盖：Linux/macOS 旧布局与新实例布局、进程和配置不匹配、Cloud 服务名、越界实例 ID、权限不足、符号链接、不安全文件权限、旧 Edge RPC 不可用、跨用户资源拒绝、匿名拒绝和错误脱敏。

另有任务去重、单任务凭据、终态重放、密钥撤销隔离、SQL 迁移和停止失败禁止删除的测试。Linux 原生测试在 `pkg/edge/lifecycle/native_linux_test.go`，需 `integration` build tag、root 和 `LIAISON_LIFECYCLE_NATIVE_TEST=1` 显式启用。仅在允许创建临时系统服务的测试主机执行。

Linux 双实例原生测试已通过：卸载一个随机隔离实例后，另一个的进程和文件保持不变。测试实例已清理，现有私有化及 Cloud Edge PID 未变化。

新增 Linux 原生安装/升级测试也已通过：两个独立实例共存、指定实例升级、故意使用立即退出的程序触发回滚、配置和服务文件保持原样。原生测试发现并修复了 systemd `WorkingDirectory` 引号问题。

macOS 双 LaunchAgent 原生安装、升级、故意启动失败后的回滚、独立 helper 自卸载及结果回报已通过，不需要 sudo；卸载一个实例后，另一个实例 PID 和文件不变。原先因无关系统 daemon 不可读而阻断的问题，已在新独立实例上按用户服务域修正；不放宽旧布局。

Linux 用户级原生回归在临时、禁止登录的测试账号下通过：两个独立 `systemctl --user` 实例安装、升级、失败回滚、独立 `systemd-run --user` helper 自卸载和结果回报，第二实例保持原 PID。测试账号及用户服务在结束后清理，不修改已有 Edge，不自动启用 linger。普通用户常驻运行仍需可用的用户服务会话或管理员自行配置 linger。

macOS 原生测试已开启 `-race`，并覆盖清单篡改、实例目录权限放宽、同用户服务共享引用的拒绝。`bootout` 返回不等于进程退出，升级和卸载会限时等待原 PID 消失；回滚替换及失败安装清理也会等待该程序停止，超时则保留文件并报错。

安装脚本自动化测试覆盖新建/升级分发、凭据通过 stdin 传递、省略模式拒绝、二进制失败不回退到旧安装逻辑。升级模式不要求重新提供接入凭据。

开发环境 `web/e2e/connector-uninstall.html` 使用模拟接口，不连接 Edge。已检查中英文、深浅主题、桌面/手机尺寸、名称确认和完成状态，并对照同视口的删除弹框。

## 部署验收与使用边界

安装脚本已接入，Linux 系统级、Linux 用户级及 macOS 新独立实例的安装、升级回滚、卸载原生回归通过。测试环境已同步部署 Manager、前端、下载脚本及五个平台的 Edge 包；不得单独发布新脚本搭配旧 Edge。

真实验收使用新建 macOS 连接器，从部署环境下载安装，再通过控制台确认并卸载。已核对任务完成、本机程序/配置/LaunchAgent 移除、接入密钥撤销和离线状态；现有 Edge 进程未被替换或停止。联调发现并修复 Kratos/Gorilla 路径参数与标准库 PathValue 的差异，新增真实 Kratos 路由测试，覆盖安装预检、创建/查询卸载任务及结果回调。

远程卸载默认允许，不需要修改安装命令。旧 Edge 不会自动迁移或升级，需使用明确的升级目标；显式关闭卸载的配置继续生效，无法验证归属的安装只支持本机处理。上述部署是测试环境验收，不等同于已发布正式版本或验证全部操作系统发行版。

默认开启版本已通过无额外卸载参数的 macOS 实际安装与自卸载回归。错误名称仍被拒绝；相关接口统一使用控制层错误映射，名称确认失败返回 400，资源权限拒绝返回 403，而不是误报 500。
