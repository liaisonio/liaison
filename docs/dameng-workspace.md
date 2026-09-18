# WebDameng

状态：标准构建已内置 Go 驱动；连接器拨号及权限隔离测试通过，真实 DM8 数据库联调待完成。

## 使用与构建

无需在浏览器、连接器或服务器上另外安装驱动。标准 Go 构建会将
`gitee.com/chunanyong/dm v1.8.23` 编译进 Liaison 后端，依赖校验和记录于
`go.sum`。该模块由第三方维护，README 声明同步达梦官方驱动；当前版本对应
DM 驱动 8.1.4.200。不将第三方维护仓库表述为达梦官方发布渠道。

从 Database → WebDameng 创建访问，填写连接器可达的地址（默认端口
5236）、用户名、密码及可选 Schema。支持保存或不保存密码。
网络可达并不替代数据库认证、Liaison 权限检查或数据库版本兼容性。

已认证活跃用户可通过 `/api/v1/webdata/capabilities` 查询构建能力，标准
构建返回 `dameng:true`；前端按此显示入口，后端仍独立校验资源访问权限。

`scripts/build-dameng.go` 仅保留作特殊发行版替换本地驱动的工具，普通构建
不需要执行。替换驱动必须提供 `RegisterDialContext`，旧驱动会被拒绝。

## 连接与能力边界

- 驱动只注册一个固定拨号钩子，每条会话使用随机 `.invalid` 虚拟地址，
  映射到已授权连接器目标。真实地址不交给驱动自行选择。
- 关闭会话撤销映射并取消正在拨号的请求；未知或已撤销的路由直接拒绝。
- 支持 SQL 查询、表/视图/列浏览、结果限额、现有审计与 Agent 授权链路。
  使用独立达梦元数据 SQL，不借用 Oracle 网络协议。
- 当前不承诺数据库末段 TLS、集群切换、任意 DSN 参数、索引/DDL 提取、
  大字段特殊类型完整支持或自动逐行编辑。连接器加密不等于数据库 TLS。
- 驱动错误经安全转换，不向页面或 Agent 暴露 DSN、凭证或原始驱动错误。

## 验证

```sh
go test -race ./pkg/dameng ./pkg/liaison/manager/web ./pkg/liaison/manager/controlplane -run 'Dameng|SQLProtocols_TargetAndCredentialIsolation' -count=1
```

已验证：原生驱动调用连接器拨号钩子、特殊字符 DSN 编码、32 路并发隔离、
撤销取消、失败清理、跨用户凭证隔离、能力接口认证、参数拒绝及语句审计。
UI fixture 覆盖启用/禁用、中英文、明暗主题和桌面/手机。

尚未验证：真实 DM8 登录与 SQL、中文/NULL/日期/数值/LOB、Schema 元数据、
真实连接器端到端链路、查询取消及数据库审计。不能仅凭构建和拨号测试
宣称完整兼容或无缝使用。

回滚使用部署前的二进制和前端备份；不会主动删除已有应用或历史记录。

## 依赖来源记录

- 模块：<https://gitee.com/chunanyong/dm>
- 官方接口文档：<https://eco.dameng.com/document/dm/zh-cn/pm/go-rogramming-guide.html>
- 依赖保留上游版权信息，未修改或复制驱动源码到本仓库。下载的模块未包含
  独立 LICENSE 文件；这里记录来源与技术集成，不作再分发许可已获确认的声明。
