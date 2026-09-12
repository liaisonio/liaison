# <img src="docs/diagrams/liaison-logo.svg" height="40" align="absmiddle" alt="Liaison logo" /> Liaison

> **AI-powered zero-trust access for private applications.**

Connect to private servers, databases, desktops, and web apps. Work with an AI Agent in your SSH and database sessions, or ask the home Agent about the connectors, devices, and applications you can access. Self-hosted, with outbound connectors and permission-controlled tools.

[![Go](https://github.com/liaisonio/liaison/actions/workflows/go.yml/badge.svg)](https://github.com/liaisonio/liaison/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/liaisonio/liaison)](https://goreportcard.com/report/github.com/liaisonio/liaison)
[![Release](https://img.shields.io/github/v/release/liaisonio/liaison?display_name=tag&sort=semver)](https://github.com/liaisonio/liaison/releases)
[![License](https://img.shields.io/badge/License-AGPLv3-blue.svg?logo=gnu)](LICENSE)

English | [简体中文](./README_zh.md) | [日本語](./README_ja.md) | [한국어](./README_ko.md) | [Español](./README_es.md) | [Français](./README_fr.md) | [Deutsch](./README_de.md)

[Website](https://liaison.cloud) · [Docs](https://liaison.cloud/docs/get-started/introduction) · [Features](#features) · [Install](#install) · [Access protocols](#supported-access-protocols) · [Agent models](#agent-model-providers) · [Product tour](#product-tour)

![Liaison overview with multi-application traffic](docs/assets/readme/overview-dark-v3.png)

## Features

- ✨ **AI in your workflow** — Inspect terminal output, draft commands, and query databases with an Agent tied to your connection. Tool access follows user permissions; operations requiring approval wait for your confirmation.
- 💬 **Ask about your resources** — Find and inspect your connectors, devices, and applications from the home Agent. Resource visibility stays scoped to the signed-in user.
- 🔌 **Outbound-only connectors** — connect private networks without opening inbound ports on them.
- 🔐 **Application access** — publish TCP, HTTP, HTTPS, WebSocket, and SSH services with per-access controls.
- 🖥️ **Browser workspaces** — use Web SSH, RDP, VNC, MySQL, MariaDB, PostgreSQL, SQL Server, Oracle, Redis, and MongoDB without local clients.
- 🔎 **Application discovery** — scan connector devices and register discovered services from the console.
- 👥 **Identity and access management** — organize users and resources, with Casbin-backed authorization.
- 🛡️ **Firewall policies** — restrict TCP and HTTP access by source IP and CIDR.
- 📋 **Logs and audit** — record management actions and supported application sessions in one place.
- 📦 **Self-hosted deployment** — run the complete control plane on your own Linux server.

## Install

Download the self-contained Docker bundle, extract it, and run the installer (Docker 20.10+ with Compose is required):

```bash
wget https://github.com/liaisonio/liaison/releases/download/v1.12.0/liaison-1.12.0-linux-amd64.tar.gz
tar -xzf liaison-1.12.0-linux-amd64.tar.gz
cd liaison-1.12.0-linux-amd64
./install.sh
```

Open `https://<server-address>` after installation. The installer prints the initial sign-in credentials.

## Supported access protocols

Reach existing private services through a Liaison connector. The logos below identify services and protocols you can access, not products bundled with or deployed by Liaison.

### Terminals and remote desktops

<p>
  <img src="docs/assets/integrations/ssh.svg" width="24" height="24" align="absmiddle" alt="SSH" /> SSH&emsp;
  <img src="docs/assets/integrations/windows.svg" width="24" height="24" align="absmiddle" alt="RDP" /> RDP&emsp;
  <img src="docs/assets/integrations/vnc.png" width="24" height="24" align="absmiddle" alt="VNC" /> VNC
</p>

Open SSH terminals and RDP / VNC desktops in your browser with WebSSH and WebDesktop.

### Databases and search services

<p>
  <img src="docs/assets/integrations/mysql.svg" width="24" height="24" align="absmiddle" alt="MySQL" /> MySQL&emsp;
  <img src="docs/assets/integrations/mariadb.svg" width="24" height="24" align="absmiddle" alt="MariaDB" /> MariaDB&emsp;
  <img src="docs/assets/integrations/postgresql.svg" width="24" height="24" align="absmiddle" alt="PostgreSQL" /> PostgreSQL&emsp;
  <img src="docs/assets/integrations/sqlserver.svg" width="24" height="24" align="absmiddle" alt="SQL Server" /> SQL Server&emsp;
  <img src="docs/assets/integrations/oracle.svg" width="24" height="24" align="absmiddle" alt="Oracle" /> Oracle
</p>

<p>
  <img src="docs/assets/integrations/mongodb.svg" width="24" height="24" align="absmiddle" alt="MongoDB" /> MongoDB&emsp;
  <img src="docs/assets/integrations/redis.svg" width="24" height="24" align="absmiddle" alt="Redis" /> Redis&emsp;
  <img src="docs/assets/integrations/clickhouse.svg" width="24" height="24" align="absmiddle" alt="ClickHouse" /> ClickHouse&emsp;
  <img src="docs/assets/integrations/elasticsearch.svg" width="24" height="24" align="absmiddle" alt="Elasticsearch" /> Elasticsearch&emsp;
  <img src="docs/assets/integrations/opensearch.svg" width="24" height="24" align="absmiddle" alt="OpenSearch" /> OpenSearch
</p>

Connect to existing databases and search services with WebData to browse schemas, run queries and inspect results in your browser.

### Web applications and TCP services

HTTP, HTTPS, WebSocket and TCP forwarding let you connect with a browser or native client without directly publishing the original backend address.

### LLM services

Reach private model services through a connector and expose an API-key-controlled endpoint. OpenAI-compatible and Anthropic Messages protocols are supported, with usage records and per-key Token quotas. Model capabilities depend on the upstream service.

## Agent model providers

These providers can power Liaison’s built-in Agent. This is separate from the service access protocols above.

<p>
  <img src="docs/assets/integrations/openai.svg" width="24" height="24" align="absmiddle" alt="OpenAI" /> OpenAI&emsp;
  <img src="docs/assets/integrations/anthropic.svg" width="24" height="24" align="absmiddle" alt="Claude" /> Claude&emsp;
  <img src="docs/assets/integrations/gemini.svg" width="24" height="24" align="absmiddle" alt="Gemini" /> Gemini&emsp;
  <img src="docs/assets/integrations/deepseek.svg" width="24" height="24" align="absmiddle" alt="DeepSeek" /> DeepSeek
</p>

<p>
  <img src="docs/assets/integrations/zhipu.svg" width="24" height="24" align="absmiddle" alt="GLM" /> GLM&emsp;
  <img src="docs/assets/integrations/kimi.svg" width="24" height="24" align="absmiddle" alt="Kimi" /> Kimi&emsp;
  <img src="docs/assets/integrations/minimax.svg" width="24" height="24" align="absmiddle" alt="MiniMax" /> MiniMax&emsp;
  <img src="docs/assets/integrations/mimo.svg" width="24" height="24" align="absmiddle" alt="MiMo" /> MiMo
</p>

Presets are available for OpenAI, Anthropic (Claude), Google (Gemini), DeepSeek, Z.ai (GLM), Moonshot (Kimi), MiniMax and Xiaomi (MiMo). Custom OpenAI-compatible services can also be configured.

## Product tour

### Web SSH

Work in an audited browser terminal with an Agent that can inspect session output and help draft commands.

![Using Web SSH through Liaison](docs/assets/readme/web-ssh-dark.png)

### Private web applications

Access media servers and other internal web applications through Liaison.

![Accessing Jellyfin through Liaison](docs/pages/jellyfin-ss.png)

### Private AI assistants

Keep internal AI tools reachable without exposing the private network.

![Accessing OpenClaw through Liaison](docs/pages/openclaw-ss.png)

### Web MySQL

Browse schemas, run SQL, and ask the session Agent to query and explain results, with approval where required.

![Using MySQL through Liaison](docs/assets/readme/web-mysql-dark.png)

### Web MongoDB

Explore collections and ask the session Agent to run approved queries and explain documents, without a local client.

![Using MongoDB through Liaison](docs/assets/readme/web-mongodb-dark.png)

### Web VNC

Open a private VNC desktop in a managed browser session.

![A VNC desktop opened in Liaison](docs/assets/readme/web-vnc-dark.png)

### Web RDP

Connect to an RDP desktop from the same access workflow.

![An RDP desktop opened in Liaison](docs/assets/readme/web-rdp-dark-v2.png)

## Documentation

- [Docker deployment](deploy/docker/README.md)
- [API reference](docs/swagger/swagger.yaml)
- [Release notes](https://github.com/liaisonio/liaison/releases)
- [Issues](https://github.com/liaisonio/liaison/issues)
- [Discussions](https://github.com/liaisonio/liaison/discussions)

## Contributing

Bug reports, feature proposals, documentation improvements, and pull requests are welcome. Start with [Issues](https://github.com/liaisonio/liaison/issues) or open a [Pull Request](https://github.com/liaisonio/liaison/pulls).

## License

Liaison is licensed under the [GNU Affero General Public License v3.0](LICENSE).
