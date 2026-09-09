# <img src="docs/diagrams/liaison-logo.svg" height="40" align="absmiddle" alt="Liaison Logo" /> Liaison

> **AI 驱动的私有应用零信任访问。**

连接私有服务器、数据库、远程桌面与 Web 应用。在 SSH 和数据库会话中使用 AI Agent，或通过首页 Agent 查询自己可见的连接器、设备与应用。支持私有化部署，通过主动出站连接器接入，工具调用受用户权限约束。

[![Go](https://github.com/liaisonio/liaison/actions/workflows/go.yml/badge.svg)](https://github.com/liaisonio/liaison/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/liaisonio/liaison)](https://goreportcard.com/report/github.com/liaisonio/liaison)
[![Release](https://img.shields.io/github/v/release/liaisonio/liaison?display_name=tag&sort=semver)](https://github.com/liaisonio/liaison/releases)
[![License](https://img.shields.io/badge/License-AGPLv3-blue.svg?logo=gnu)](LICENSE)

[English](./README.md) | 简体中文 | [日本語](./README_ja.md) | [한국어](./README_ko.md) | [Español](./README_es.md) | [Français](./README_fr.md) | [Deutsch](./README_de.md)

[官网](https://liaison.cloud) · [产品文档](https://liaison.cloud/zh/docs/get-started/introduction) · [产品能力](#产品能力) · [安装](#安装) · [集成](#集成) · [产品展示](#产品展示) · [社区](#社区)

![Liaison 多应用流量总览](docs/assets/readme/overview-dark-v3.png)

## 产品能力

- ✨ **工作流中的 AI** — 让与当前连接关联的 Agent 读取终端输出、辅助编写命令和查询数据库。工具遵循用户权限，需要审批的操作会等待确认。
- 💬 **用对话查询资源** — 通过首页 Agent 查找和了解连接器、设备与应用，资源可见范围始终限定为当前登录用户。
- 🔌 **主动出站连接器**：私有网络无需开放入站端口。
- 🔐 **应用访问**：支持 TCP、HTTP、HTTPS、WebSocket 与 SSH，并可独立配置访问策略。
- 🖥️ **浏览器工作台**：无需本地客户端即可使用 Web SSH、RDP、VNC、MySQL、MariaDB、PostgreSQL、Redis 和 MongoDB。
- 🔎 **应用发现**：扫描连接器所在设备，并在控制台中登记发现的服务。
- 👥 **身份与权限**：管理组织、用户与资源，权限由 Casbin 统一执行。
- 🛡️ **防火墙策略**：按来源 IP 与 CIDR 限制 TCP 和 HTTP 访问。
- 📋 **日志与审计**：集中记录管理操作与支持审计的应用会话。
- 📦 **私有化部署**：完整控制平面运行在自己的 Linux 服务器上。

## 安装

下载自带所有镜像的 Docker 离线包，解压后运行安装脚本（需要 Docker 20.10+ 及 Compose）：

```bash
wget https://github.com/liaisonio/liaison/releases/download/v1.10.0/liaison-1.10.0-linux-amd64.tar.gz
tar -xzf liaison-1.10.0-linux-amd64.tar.gz
cd liaison-1.10.0-linux-amd64
./install.sh
```

安装完成后访问 `https://<服务器地址>`，初始登录信息会由安装脚本输出。

## 集成

Liaison 支持以下原生协议与浏览器工作台。

<p align="center">
  <img src="docs/assets/integrations/ssh.svg" height="52" alt="SSH" title="SSH" />&nbsp;&nbsp;&nbsp;&nbsp;
  <img src="docs/assets/integrations/windows.svg" height="52" alt="RDP" title="RDP" />&nbsp;&nbsp;&nbsp;&nbsp;
  <img src="docs/assets/integrations/vnc.png" height="52" alt="VNC" title="VNC" />&nbsp;&nbsp;&nbsp;&nbsp;
  <img src="docs/assets/integrations/mysql.svg" height="52" alt="MySQL" title="MySQL" />&nbsp;&nbsp;&nbsp;&nbsp;
  <img src="docs/assets/integrations/postgresql.svg" height="52" alt="PostgreSQL" title="PostgreSQL" />&nbsp;&nbsp;&nbsp;&nbsp;
  <img src="docs/assets/integrations/redis.svg" height="52" alt="Redis" title="Redis" />&nbsp;&nbsp;&nbsp;&nbsp;
  <img src="docs/assets/integrations/mongodb.svg" height="52" alt="MongoDB" title="MongoDB" />
</p>

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
