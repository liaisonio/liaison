
# <img src="docs/diagrams/liaison-logo.svg" height="40" align="absmiddle" alt="" /> Liaison

> **网络马上通达，轻松连接分布在不同位置的设备与应用**

[![Go](https://github.com/liaisonio/liaison/actions/workflows/go.yml/badge.svg)](https://github.com/liaisonio/liaison/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/liaisonio/liaison)](https://goreportcard.com/report/github.com/liaisonio/liaison)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![技术栈](https://img.shields.io/badge/Tech-Go%20%7C%20TypeScript%20%7C%20React-blue)](#技术栈一览)
[![版本](https://img.shields.io/badge/Version-v1.8.0-green)](#)

简体中文 | [English](./README_en.md) | [日本語](./README_ja.md) | [한국어](./README_ko.md) | [Español](./README_es.md) | [Français](./README_fr.md) | [Deutsch](./README_de.md)

![Dashboard](docs/pages/home.png)

| Jellyfin（随时随地看家庭影片） | OpenClaw（随时随地用家庭 AI 助手） |
|:---:|:---:|
| ![Jellyfin](docs/pages/jellyfin-ss.png) | ![OpenClaw](docs/pages/openclaw-ss.png) |

[快速开始](#-快速开始) • [简介](#-项目简介) • [贡献](#-贡献)


---

## 📖 项目简介

Liaison 是一个生产级应用访问解决方案，不暴露任何内网端口，随时开启关闭。它提供了完整的产品功能，支持自动发现设备应用，实时流量统计，以及安全的 TLS 加密传输。

本项目主要解决以下问题：

- **内网穿透难题**：无需复杂配置，即可从公网访问内网设备和服务
- **多设备管理**：统一管理分布在不同位置的设备，支持 Linux/macOS/Windows 全平台
- **安全连接**：TLS 加密保障连接安全，不暴露内网，随时开启关闭
- **按入口防火墙**：每条 TCP / HTTP 访问都可配置来源 IP CIDR 白名单，在连接 accept 阶段直接过滤
- **流量监控**：实时监控设备状态、流量统计，为运维和容量评估提供数据依据
- **应用代理**：支持 TCP、HTTP/HTTPS、WebSocket 等多种协议的应用代理
- **WebDesktop**：通过 guacd 在浏览器中访问 RDP / VNC 远程桌面
- **API 自动化**：内置个人访问令牌（PAT）用于 CLI / 脚本调用，`/cli-auth` 页面提供浏览器辅助的 CLI 登录流程

适用场景：

<div align="center">

| **💼 远程办公与开发** | **🧑‍💻 个人工作室** | **🏠 家庭网络 / NAS** | **🌐 多机房 / 多地域部署** | **⚡ 边缘计算与运维管理** |
|:---:|:---:|:---:|:---:|:---:|
| 连接办公室和家中设备，随时远程开发与调试 | 安全连接工作站与私有环境，统一访问创作设备 | 从公网访问家庭 NAS 与智能家居服务 | 连接分布在不同机房和地域的服务器与应用 | 连接并监控边缘设备应用，远程巡检状态与流量 |

</div>

---

## 🚀 快速开始

服务端二选一,之后再安装连接器。

### 📦 安装服务端 — 方式一：二进制 + systemd

**1. 下载安装包并运行安装脚本**

```bash
# 下载最新版本
wget https://github.com/liaisonio/liaison/releases/download/v1.8.0/liaison-1.8.0-linux-amd64.tar.gz

# 解压
tar -xzf liaison-1.8.0-linux-amd64.tar.gz
cd liaison-1.8.0-linux-amd64
sudo ./install.sh
```

安装过程中会提示输入公网地址或域名，30 秒内未输入将自动使用检测到的公网 IP。

**2. 访问 Web 控制台**

安装完成后，访问 `https://你的公网IP` 即可进入 Web 控制台。

> 💡 **提示**: 默认管理员账号密码请查看安装脚本输出或配置文件

### 🐳 安装服务端 — 方式二：Docker Compose

需要 Docker 20.10+ 及 `docker compose` 插件。安装包内含已构建好的镜像,无需拉仓库或拉镜像。

```bash
wget https://github.com/liaisonio/liaison/releases/download/v1.8.0/liaison-1.8.0-docker-amd64.tar.gz
tar -xzf liaison-1.8.0-docker-amd64.tar.gz
cd liaison-1.8.0-docker-amd64
./load.sh
```

`load.sh` 会自动探测公网 IP(30 秒倒计时提示用户确认),加载镜像,启动容器,等 liaison 就绪后打印一次性管理员密码。记下密码,访问 `https://<公网IP>` 登录即可。

数据 (`data/` SQLite)、TLS 证书 (`certs/`)、日志 (`logs/`) 以 bind mount 方式挂在 `docker-compose.yaml` 同目录下持久化。源码构建、升级 / 重置 / 反向代理 / 自定义证书等高阶用法见 [`deploy/docker/README.md`](deploy/docker/README.md)。

### 🔌 安装连接器

两种安装方式，按目标设备类型选择。

#### 方式 A — Liaison Desktop（图形界面，macOS / Windows）

菜单栏 / 系统托盘应用，封装连接器本身，提供一键登录、状态指示、暂停 / 恢复以及一键打开 Dashboard。适合笔记本和工作站。

<div align="center">

| macOS | Windows |
|:---:|:---:|
| <img src="docs/images/desktop-client/popup-macos-zh.png" alt="Liaison Desktop 在 macOS" width="360" /> | <img src="docs/images/desktop-client/popup-windows-zh.png" alt="Liaison Desktop 在 Windows" width="360" /> |

</div>

- **一键登录** — 浏览器辅助 OAuth 流程，PAT 存入系统钥匙串（macOS Keychain，Windows 凭据管理器）
- **多部署支持** — 默认连 `liaison.cloud`；左下角齿轮图标可随时切换到任意私有化部署，无需重装
- **基于心跳的状态判断** — 连接中 → 已连接的切换反映真实隧道状态，不只看进程是否存活
- **暂停状态跨重启保留** — 用户意图持久化到磁盘，暂停后退出再启动还是暂停

**下载（rolling 预发布，`feat/desktop-client` 最新构建）：**

| 平台 | 文件 |
|:---|:---|
| macOS（Apple Silicon + Intel 通用） | [`Liaison_0.1.0_universal.dmg`](https://github.com/liaisonio/liaison/releases/download/desktop-latest/Liaison_0.1.0_universal.dmg) |
| Windows（.msi 安装器） | [`Liaison_0.1.0_x64_en-US.msi`](https://github.com/liaisonio/liaison/releases/download/desktop-latest/Liaison_0.1.0_x64_en-US.msi) |
| Windows（.exe NSIS，卸载时清理 keychain） | [`Liaison_0.1.0_x64-setup.exe`](https://github.com/liaisonio/liaison/releases/download/desktop-latest/Liaison_0.1.0_x64-setup.exe) |

> v0.1 的安装包未签名。macOS Gatekeeper 与 Windows SmartScreen 首次启动会提示——macOS 上右键点击 → 打开，Windows 上选「更多信息」→「仍要运行」。Windows 需要 WebView2 Runtime；Win10 1803+ 和 Win11 已自带。

> WebDesktop 说明：RDP / VNC 的网页登录依赖 `guacd`。Docker 部署会默认启动 guacd；`make package` / Release 会从 `guacamole/guacd:1.5.5` 准备内置 `bin/guacd` 和运行时，二进制 / systemd 安装脚本会优先使用安装包内置版本，缺失时再尝试系统 guacd / 包管理器 / Docker sidecar，并默认只监听 `127.0.0.1:4822`。如果你自行部署 guacd，请在 `manager.guacd_addr` 中配置地址。

#### 方式 B — 命令行安装（Linux / 无图形环境）

在 Web 控制台**新建连接器**，在页面上拷贝对应平台的安装命令，在目标设备上执行即可完成安装。安装后连接器会自动出现在控制台中。

---

## 📋 系统要求

| 组件 | 要求 |
|:---|:---|
| **服务端** | Linux 系统（推荐 Ubuntu 20.04+ 或 CentOS 7+） |
| **连接器** | Linux / macOS / Windows（支持 x86_64 和 ARM64 架构） |
| **浏览器** | Chrome 90+, Firefox 88+, Safari 14+, Edge 90+ |

---

## 🏗️ 架构说明

<div align="center">

<img src="./docs/diagrams/liaison.png" width="80%">

**中心化架构，通过Liaison服务统一管理所有连接器**

</div>

### 核心组件

- **Liaison** - 管理中心，提供 Web 界面和 API，访问入口
- **Frontier** - 连接器网关，处理所有连接器的连接和通信
- **Edge** - 连接器客户端，部署在目标设备上

---

## 🤝 贡献

我们欢迎所有形式的贡献！

- 🐛 [报告 Bug](https://github.com/liaisonio/liaison/issues/new?template=bug_report.md)
- 💡 [提出建议](https://github.com/liaisonio/liaison/issues/new?template=feature_request.md)
- 📝 [提交 PR](https://github.com/liaisonio/liaison/pulls)
- 📖 [改进文档](https://github.com/liaisonio/liaison/issues/new?template=documentation.md)

### 贡献指南

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启 Pull Request

---

## 📄 许可证

本项目采用 [Apache License 2.0](LICENSE) 许可证。

---

<div align="center">

**如果这个项目对你有帮助，请给一个 ⭐ Star！**

Made with ❤️ by [Liaison Contributors](https://github.com/liaisonio/liaison/graphs/contributors)

| 飞书群 | 微信群 |
|:---:|:---:|
| <img src="./web/public/feishu.png" width="220"> | <img src="./web/public/wechat-group.png" width="220"> |


[GitHub](https://github.com/liaisonio/liaison) • [Issues](https://github.com/liaisonio/liaison/issues) • [Discussions](https://github.com/liaisonio/liaison/discussions)

</div>
