# <img src="docs/diagrams/liaison-logo.svg" height="40" align="absmiddle" alt="Liaison Logo" /> Liaison

> **面向私有应用的零信任访问。**

通过主动出站的连接器访问私有 Web 应用、服务器、远程桌面与数据库，无需暴露私有网络。

[![Go](https://github.com/liaisonio/liaison/actions/workflows/go.yml/badge.svg)](https://github.com/liaisonio/liaison/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/liaisonio/liaison)](https://goreportcard.com/report/github.com/liaisonio/liaison)
[![Release](https://img.shields.io/github/v/release/liaisonio/liaison?display_name=tag&sort=semver)](https://github.com/liaisonio/liaison/releases)
[![License](https://img.shields.io/badge/License-AGPLv3-blue.svg?logo=gnu)](LICENSE)

[English](./README.md) | 简体中文 | [日本語](./README_ja.md) | [한국어](./README_ko.md) | [Español](./README_es.md) | [Français](./README_fr.md) | [Deutsch](./README_de.md)

[产品能力](#产品能力) · [安装](#安装) · [集成](#集成) · [产品展示](#产品展示) · [文档](#文档) · [社区](#社区)

![Liaison 多应用流量总览](docs/assets/readme/overview-dark-v2.png)

## 产品能力

- 🔌 **主动出站连接器**：私有网络无需开放入站端口。
- 🔐 **应用访问**：支持 TCP、HTTP、HTTPS、WebSocket 与 SSH，并可独立配置访问策略。
- 🖥️ **浏览器工作台**：无需本地客户端即可使用 Web SSH、RDP、VNC、MySQL、PostgreSQL、Redis 和 MongoDB。
- 🔎 **应用发现**：扫描连接器所在设备，并在控制台中登记发现的服务。
- 👥 **身份与权限**：管理组织、用户与资源，权限由 Casbin 统一执行。
- 🛡️ **防火墙策略**：按来源 IP 与 CIDR 限制 TCP 和 HTTP 访问。
- 📋 **日志与审计**：集中记录管理操作与支持审计的应用会话。
- 📦 **私有化部署**：完整控制平面运行在自己的 Linux 服务器上。

## 安装

下载 Linux 服务端压缩包，解压后运行安装脚本：

```bash
wget https://github.com/liaisonio/liaison/releases/download/v1.8.0/liaison-1.8.0-linux-amd64.tar.gz
tar -xzf liaison-1.8.0-linux-amd64.tar.gz
cd liaison-1.8.0-linux-amd64
sudo ./install.sh
```

安装完成后访问 `https://<服务器地址>`，初始登录信息会由安装脚本输出。

## 集成

<div align="center">

| | | | | | | | | |
|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|
| <img src="https://api.iconify.design/mdi:lan-connect.svg?color=%2364748b" width="38" alt="TCP" /><br><sub>TCP</sub> | <img src="https://api.iconify.design/mdi:web.svg?color=%2364748b" width="38" alt="HTTP" /><br><sub>HTTP</sub> | <img src="https://api.iconify.design/mdi:console.svg?color=%2364748b" width="38" alt="SSH" /><br><sub>SSH</sub> | <img src="https://api.iconify.design/mdi:desktop-classic.svg?color=%2364748b" width="38" alt="RDP" /><br><sub>RDP</sub> | <img src="https://api.iconify.design/mdi:monitor-share.svg?color=%2364748b" width="38" alt="VNC" /><br><sub>VNC</sub> | <img src="https://api.iconify.design/logos:mysql-icon.svg" width="38" alt="MySQL" /><br><sub>MySQL</sub> | <img src="https://api.iconify.design/logos:postgresql.svg" width="38" alt="PostgreSQL" /><br><sub>PostgreSQL</sub> | <img src="https://api.iconify.design/logos:redis.svg" width="38" alt="Redis" /><br><sub>Redis</sub> | <img src="https://api.iconify.design/logos:mongodb-icon.svg" width="38" alt="MongoDB" /><br><sub>MongoDB</sub> |

</div>

## 产品展示

### 私有 Web 应用

通过 Liaison 访问媒体服务及其他内部 Web 应用。

![通过 Liaison 访问 Jellyfin](docs/pages/jellyfin-ss.png)

### 私有 AI 助手

无需暴露私有网络，也能安全访问内部 AI 工具。

![通过 Liaison 访问 OpenClaw](docs/pages/openclaw-ss.png)

### Web MySQL

直接在浏览器中查看表结构并执行可审计的 SQL 查询。

![通过 Liaison 使用 MySQL](docs/assets/readme/web-mysql-dark.png)

### Web MongoDB

无需本地客户端即可浏览文档数据库并执行命令。

![通过 Liaison 使用 MongoDB](docs/assets/readme/web-mongodb-dark.png)

### Web VNC

通过受管的浏览器会话访问私有 VNC 桌面。

![在 Liaison 中访问 VNC 桌面](docs/assets/readme/web-vnc-dark.png)

### Web RDP

使用同一套访问流程连接 RDP 桌面。

![在 Liaison 中访问 RDP 桌面](docs/assets/readme/web-rdp-dark.png)

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
