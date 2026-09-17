<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/assets/liaison-brand-rays-dark.svg" />
  <img src="docs/assets/liaison-brand-rays-light.svg" width="270" height="88" alt="Liaison" />
</picture>

> **面向本地大模型与应用的 AI 原生零信任访问。**

支持私有化部署、安全 API 分享、浏览器工作台与上下文感知的 AI Agent。

[![CI](https://github.com/liaisonio/liaison/actions/workflows/go.yml/badge.svg)](https://github.com/liaisonio/liaison/actions/workflows/go.yml)
[![Release](https://img.shields.io/github/v/release/liaisonio/liaison?display_name=tag&sort=semver)](https://github.com/liaisonio/liaison/releases)
[![Downloads](https://img.shields.io/github/downloads/liaisonio/liaison/total)](https://github.com/liaisonio/liaison/releases)
[![License](https://img.shields.io/badge/License-AGPLv3-blue.svg?logo=gnu)](LICENSE)
[![Go](https://img.shields.io/github/go-mod/go-version/liaisonio/liaison?logo=go&logoColor=white)](go.mod)
[![React](https://img.shields.io/badge/React-20232A?logo=react&logoColor=61DAFB)](web/package.json)
[![TypeScript](https://img.shields.io/badge/TypeScript-3178C6?logo=typescript&logoColor=white)](web/package.json)
[![Vite](https://img.shields.io/badge/Vite-646CFF?logo=vite&logoColor=white)](web/vite.config.ts)
[![Tailwind CSS](https://img.shields.io/badge/Tailwind_CSS-06B6D4?logo=tailwindcss&logoColor=white)](web/tailwind.config.cjs)

[English](./README.md) | 简体中文 | [日本語](./README_ja.md) | [한국어](./README_ko.md) | [Español](./README_es.md) | [Français](./README_fr.md) | [Deutsch](./README_de.md)

[官网](https://liaison.cloud) · [产品文档](https://liaison.cloud/zh/docs/get-started/introduction) · [产品能力](#产品能力) · [安装](#安装) · [访问协议](#支持的访问协议) · [Agent 模型](#agent-模型提供方) · [产品展示](#产品展示) · [社区](#社区)

![Liaison 多应用流量总览](docs/assets/readme/overview-dark-v3.png)

## 产品能力

- ✨ **工作流中的 AI** — 让与当前连接关联的 Agent 读取终端输出、辅助编写命令和查询数据库。工具遵循用户权限，需要审批的操作会等待确认。
- 💬 **用对话查询资源** — 通过首页 Agent 查找和了解连接器、设备与应用，资源可见范围始终限定为当前登录用户。
- 🔌 **主动出站连接器**：私有网络无需开放入站端口。
- 🔐 **应用访问**：支持 TCP、HTTP、HTTPS、WebSocket 与 SSH，并可独立配置访问策略。
- 🖥️ **浏览器工作台**：使用 WebSSH、WebSFTP、WebRDP、WebVNC，以及 MySQL、MariaDB、PostgreSQL、SQL Server、Oracle、ClickHouse、MongoDB、Elasticsearch、OpenSearch、Redis 和 Memcached。
- 📁 **文件与对象浏览**：通过 WebSFTP 管理远程文件；WebS3 浏览 S3 兼容服务的 Bucket、目录前缀和对象元数据（当前为只读）。
- 🤖 **私有模型访问**：支持 OpenAI-compatible、Anthropic Messages 和 Ollama 上游，提供调用密钥、模型映射、用量记录与每密钥 Token 配额。
- 🔎 **应用发现**：扫描连接器所在设备，并在控制台中登记发现的服务。
- 👥 **身份与权限**：管理组织、用户与资源，权限由 Casbin 统一执行。
- 🛡️ **防火墙策略**：按来源 IP 与 CIDR 限制 TCP 和 HTTP 访问。
- 📋 **日志与审计**：集中记录管理操作与支持审计的应用会话。
- 📦 **私有化部署**：完整控制平面运行在自己的 Linux 服务器上。

## 安装

下载自带所有镜像的 Docker 离线包，解压后运行安装脚本（需要 Docker 20.10+ 及 Compose）：

```bash
wget https://github.com/liaisonio/liaison/releases/download/v1.13.0/liaison-1.13.0-linux-amd64.tar.gz
tar -xzf liaison-1.13.0-linux-amd64.tar.gz
cd liaison-1.13.0-linux-amd64
./install.sh
```

安装完成后访问 `https://<服务器地址>`，初始登录信息会由安装脚本输出。

## 支持的访问协议

通过 Liaison 访问已有的私有服务。

<table>
  <tr><th align="left">SSH / SFTP</th><td><img src="docs/assets/integrations/readme/ssh.svg" width="40" height="40" alt="SSH / WebSSH" title="SSH / WebSSH" />&nbsp;&nbsp; <img src="docs/assets/integrations/readme/sftp.svg" width="40" height="40" alt="WebSFTP" title="WebSFTP" /></td></tr>
  <tr><th align="left">Desktop</th><td><img src="docs/assets/integrations/windows.svg" width="40" height="40" alt="WebRDP" title="WebRDP" />&nbsp;&nbsp; <img src="docs/assets/integrations/vnc.png" width="40" height="40" alt="WebVNC" title="WebVNC" /></td></tr>
  <tr><th align="left">SQL databases</th><td><img src="docs/assets/integrations/mysql.svg" width="40" height="40" alt="WebMySQL" title="WebMySQL" />&nbsp;&nbsp; <img src="docs/assets/integrations/mariadb.svg" width="40" height="40" alt="WebMariaDB" title="WebMariaDB" />&nbsp;&nbsp; <img src="docs/assets/integrations/postgresql.svg" width="40" height="40" alt="WebPostgreSQL" title="WebPostgreSQL" />&nbsp;&nbsp; <img src="docs/assets/integrations/sqlserver.svg" width="40" height="40" alt="WebSQLServer" title="WebSQLServer" />&nbsp;&nbsp; <img src="docs/assets/integrations/oracle.svg" width="40" height="40" alt="WebOracle" title="WebOracle" />&nbsp;&nbsp; <img src="docs/assets/integrations/clickhouse.svg" width="40" height="40" alt="WebClickHouse" title="WebClickHouse" /></td></tr>
  <tr><th align="left">Data & search</th><td><img src="docs/assets/integrations/mongodb.svg" width="40" height="40" alt="WebMongoDB" title="WebMongoDB" />&nbsp;&nbsp; <img src="docs/assets/integrations/elasticsearch.svg" width="40" height="40" alt="WebElasticsearch" title="WebElasticsearch" />&nbsp;&nbsp; <img src="docs/assets/integrations/opensearch.svg" width="40" height="40" alt="WebOpenSearch" title="WebOpenSearch" /></td></tr>
  <tr><th align="left">Cache</th><td><img src="docs/assets/integrations/redis.svg" width="40" height="40" alt="WebRedis" title="WebRedis" />&nbsp;&nbsp; <img src="docs/assets/integrations/memcached.svg" width="40" height="40" alt="WebMemcached" title="WebMemcached" /></td></tr>
  <tr><th align="left">Storage</th><td><img src="docs/assets/integrations/readme/s3.svg" width="40" height="40" alt="WebS3 (S3-compatible)" title="WebS3 (S3-compatible)" /></td></tr>
  <tr><th align="left">LLM upstream protocols</th><td><picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/integrations/readme/openai-dark.svg" /><img src="docs/assets/integrations/readme/openai-light.svg" width="40" height="40" alt="OpenAI-compatible" title="OpenAI-compatible" /></picture>&nbsp;&nbsp; <img src="docs/assets/integrations/anthropic.svg" width="40" height="40" alt="Anthropic Messages" title="Anthropic Messages" />&nbsp;&nbsp; <picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/integrations/readme/ollama-dark.svg" /><img src="docs/assets/integrations/readme/ollama-light.svg" width="40" height="40" alt="Ollama" title="Ollama" /></picture></td></tr>
  <tr><th align="left">Web / TCP</th><td><img src="docs/assets/integrations/readme/web.svg" width="40" height="40" alt="HTTP / HTTPS / WebSocket" title="HTTP / HTTPS / WebSocket" />&nbsp;&nbsp; <img src="docs/assets/integrations/readme/tcp.svg" width="40" height="40" alt="TCP" title="TCP" /></td></tr>
</table>

LLM 上游支持 **OpenAI-compatible**、**Anthropic Messages** 和 **Ollama**。对外提供 OpenAI-compatible 接口；Anthropic 上游另支持原生 Messages 接口。图标表示可访问的已有服务，不代表 Liaison 捆绑部署这些产品；数据库和桌面图标指浏览器工作台，不代表原生数据库、RDP 或 VNC 服务端监听。

## Agent 模型提供方

为 Liaison 内置 Agent 配置模型，独立于上述服务访问能力。

<table>
  <tr>
    <th align="left">模型</th>
    <td><picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/integrations/readme/openai-dark.svg" /><img src="docs/assets/integrations/readme/openai-light.svg" width="40" height="40" alt="OpenAI" title="OpenAI" /></picture>&nbsp;&nbsp; <img src="docs/assets/integrations/anthropic.svg" width="40" height="40" alt="Anthropic / Claude" title="Anthropic / Claude" />&nbsp;&nbsp; <img src="docs/assets/integrations/gemini.svg" width="40" height="40" alt="Google Gemini" title="Google Gemini" />&nbsp;&nbsp; <img src="docs/assets/integrations/deepseek.svg" width="40" height="40" alt="DeepSeek" title="DeepSeek" />&nbsp;&nbsp; <picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/integrations/readme/zhipu-dark.svg" /><img src="docs/assets/integrations/readme/zhipu-light.svg" width="40" height="40" alt="Z.ai / GLM" title="Z.ai / GLM" /></picture>&nbsp;&nbsp; <picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/integrations/readme/kimi-dark.svg" /><img src="docs/assets/integrations/readme/kimi-light.svg" width="40" height="40" alt="Moonshot / Kimi" title="Moonshot / Kimi" /></picture>&nbsp;&nbsp; <img src="docs/assets/integrations/minimax.svg" width="40" height="40" alt="MiniMax" title="MiniMax" />&nbsp;&nbsp; <picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/integrations/readme/mimo-dark.svg" /><img src="docs/assets/integrations/readme/mimo-light.svg" width="40" height="40" alt="Xiaomi MiMo" title="Xiaomi MiMo" /></picture></td>
  </tr>
</table>

支持以上 8 家预置厂商，以及自定义 OpenAI 兼容服务。

## 产品展示

### Web SSH

在可审计的浏览器终端中工作，让关联当前会话的 Agent 读取输出并辅助编写命令。

![通过 Liaison 使用 Web SSH](docs/assets/readme/web-ssh-dark.png)

### 私有 Web 应用

通过 Liaison 访问媒体服务及其他内部 Web 应用。

![通过 Liaison 访问 Jellyfin](docs/pages/jellyfin-ss.png)

### 私有 AI 助手

无需暴露私有网络，也能安全访问内部 AI 工具。

![通过 Liaison 访问 OpenClaw](docs/pages/openclaw-ss.png)

### Web MySQL

查看表结构、执行 SQL，让会话 Agent 查询并解释结果，需要审批的操作先确认。

![通过 Liaison 使用 MySQL](docs/assets/readme/web-mysql-dark.png)

### Web MongoDB

无需本地客户端即可浏览集合，让会话 Agent 执行已批准的查询并解释文档。

![通过 Liaison 使用 MongoDB](docs/assets/readme/web-mongodb-dark.png)

### Web VNC

通过受管的浏览器会话访问私有 VNC 桌面。

![在 Liaison 中访问 VNC 桌面](docs/assets/readme/web-vnc-dark.png)

### Web RDP

使用同一套访问流程连接 RDP 桌面。

![在 Liaison 中访问 RDP 桌面](docs/assets/readme/web-rdp-dark-v2.png)

## 文档

- [Docker 部署](deploy/docker/README.md)
- [API 文档](docs/swagger/swagger.yaml)
- [版本发布](https://github.com/liaisonio/liaison/releases)
- [问题反馈](https://github.com/liaisonio/liaison/issues)
- [社区讨论](https://github.com/liaisonio/liaison/discussions)

## 社区

<div align="center">

<img src="docs/assets/readme/wechat-community.jpg" width="260" alt="Liaison 微信开发者社区二维码" />

使用微信扫码加入 Liaison 开发者社区。

</div>

## 贡献

欢迎提交问题、功能建议、文档改进与代码贡献。可以从 [Issues](https://github.com/liaisonio/liaison/issues) 或 [Pull Requests](https://github.com/liaisonio/liaison/pulls) 开始。

## 许可证

Liaison 使用 [GNU Affero 通用公共许可证 v3.0](LICENSE) 开源许可证。
