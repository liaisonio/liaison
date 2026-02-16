# Liaison 官网（静态站点）

该目录提供一个可直接部署到公网的产品官网静态页，面向“产品介绍 / 优势 / 架构 / 快速开始 / 文档入口”场景。

## 本地预览

```bash
cd website
python3 -m http.server 5173
```

然后访问 `http://localhost:5173/`。

**注意**：本地预览时，由于使用了 `<base href="/products/">` 标签，建议使用 nginx 配置进行本地测试，或者临时注释掉 base 标签。

## 生产部署

把 `website/` 目录作为静态站点发布即可（任意对象存储 + CDN、Nginx、Caddy、GitHub Pages 等均可）。

**注意**：静态网站已配置为部署在 `/products` 路径下，访问地址为 `https://example.com/products/`。

### Nginx 示例

```nginx
server {
  listen 443 ssl http2;
  server_name example.com;

  # SSL 证书配置
  ssl_certificate /path/to/your/cert.pem;
  ssl_certificate_key /path/to/your/key.pem;
  ssl_protocols TLSv1.2 TLSv1.3;
  ssl_ciphers HIGH:!aNULL:!MD5;

  # 静态网站根路径
  root /var/www/liaison-website;
  index index.html;

  # 静态网站（放在 /products 路径下）
  location /products/ {
    alias /var/www/liaison-website/;
    try_files $uri $uri/ /products/index.html;
    index index.html;
  }
  
  # 处理 /products 重定向到 /products/
  location = /products {
    return 301 /products/;
  }

  # 代理 liaison
  location / {
    proxy_pass https://127.0.0.1:443;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Forwarded-Host $host;
    proxy_set_header X-Forwarded-Port $server_port;
    
    # WebSocket 支持
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    
    # 超时设置
    proxy_connect_timeout 60s;
    proxy_send_timeout 60s;
    proxy_read_timeout 60s;
  }

  # HTTP 重定向到 HTTPS（可选）
  # server {
  #   listen 80;
  #   server_name example.com;
  #   return 301 https://$server_name$request_uri;
  # }
}
```

## 处理静态文件路径问题

当通过 `/liaison` 路径代理 liaison 服务时，静态文件内部的链接（如 HTML、CSS、JS 中的绝对路径）可能不是以 `/liaison` 开头的，这会导致链接无法正常工作。

### 方案1：使用 nginx sub_filter（已在上方配置中启用）

在 nginx 配置中使用 `sub_filter` 自动重写响应中的绝对路径。这是**推荐方案**，因为：
- 无需修改前端代码
- 透明处理，对后端无影响
- 自动处理所有类型的资源链接

配置已在上面添加，会自动将响应中的：
- `="/` → `="/liaison/`
- `url(/` → `url(/liaison/`
- `href="/` → `href="/liaison/`
- `src="/` → `src="/liaison/`

### 方案2：修改前端构建配置（适用于 UmiJS/React 应用）

如果 liaison 前端使用 UmiJS 构建，可以在 `web/.umirc.ts` 中添加 `base` 配置：

```typescript
export default defineConfig({
  base: '/liaison',  // 添加这行
  // ... 其他配置
});
```

这样构建后的所有资源路径和路由都会自动带上 `/liaison` 前缀。

**注意**：如果使用方案2，需要重新构建前端并部署。

## 关联文档与仓库

官网中的"文档/Swagger/API/安装指南"等链接默认指向 GitHub 仓库页面，适合将 `website/` 作为独立静态站点单独部署。若你希望跳转到自建文档站/控制台域名，可按需调整 `index.html` 中的链接目标。
