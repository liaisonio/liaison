# Liaison Web

Liaison 私有化管理控制台。前端框架参考 Ongrid，采用 Vite、React 18、React Router、Zustand、Tailwind CSS，并保留 Ant Design / Pro Components 作为业务组件层。

## 开发

```bash
pnpm install
LIAISON_API_TARGET=http://127.0.0.1:8080 pnpm dev
```

开发服务默认运行在 `http://127.0.0.1:8000`，`/api` 由 Vite 代理到 `LIAISON_API_TARGET`。

## 门禁

```bash
pnpm typecheck
pnpm lint
pnpm build
```

生产构建输出到 `dist/`。生产环境中的 API、WebSocket 与静态页面保持同源，由 Liaison Manager 统一提供。

## 结构

- `src/api/`：统一请求客户端
- `src/store/`：会话、主题与界面状态
- `src/components/layout/`：应用框架与导航
- `src/styles/`：设计 Token、主题和全局组件约束
- `src/pages/`：现有业务页面及远程访问功能
