# 文档地图

## 阅读路径

```mermaid
flowchart LR
    A[仓库关系] --> B[架构总览]
    B --> C[快速开始]
    C --> D[模块级文档]
```

## 当前已生成文档

| 文档 | 主题 | 角色 |
| --- | --- | --- |
| [index.md](./index.md) | 总入口 | 所有人 |
| [architecture.md](./architecture.md) | 运行时拓扑与数据流 | 架构评审、维护者 |
| [repo-relationship.md](./repo-relationship.md) | `liaison` 与 `liaison-cloud` 关系 | 维护者、发布负责人 |
| [getting-started.md](./getting-started.md) | 接手与定位指南 | 新接手者 |

## 后续建议补充的模块页

| 优先级 | 模块 | 原因 |
| --- | --- | --- |
| P1 | `pkg/liaison/manager/*` | 控制面入口最集中 |
| P1 | `pkg/edge/*` | 连接器接入、扫描、上报核心逻辑 |
| P1 | `pkg/entry/*` | 实际访问流量入口 |
| P2 | `pkg/liaison/repo/*` | 数据模型与持久化边界 |
| P2 | `web/` + `website/` | 后台与官网双前端路径 |
| P3 | `deploy-liaison.sh` + `deploy/*` | 部署自动化与环境差异 |

## 目录关系图

```mermaid
flowchart TB
    subgraph Delivery[liaison]
        A1[README / docs / dist]
        A2[pkg]
        A3[web / website]
    end

    subgraph Cloud[liaison-cloud]
        B1[spec]
        B2[pkg]
        B3[deploy]
        B4[web / website]
    end

    A2 <--> B2
    A3 <--> B4
    A1 --> B1
    B3 --> B1
```

**Section sources**
- [README.md](file:///Users/zhaizenghui/austinzhai/liaison/README.md#L1)
- [README.md](file:///Users/zhaizenghui/austinzhai/liaison-cloud/README.md#L1)
- [spec/spec.md](file:///Users/zhaizenghui/austinzhai/liaison-cloud/spec/spec.md#L1)
