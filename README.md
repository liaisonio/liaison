# <img src="docs/diagrams/liaison-logo.svg" height="40" align="absmiddle" alt="Liaison logo" /> Liaison

> **Zero-trust access for private applications.**

Connect private web apps, servers, remote desktops, and databases through outbound connectors, without exposing the private network.

[![Go](https://github.com/liaisonio/liaison/actions/workflows/go.yml/badge.svg)](https://github.com/liaisonio/liaison/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/liaisonio/liaison)](https://goreportcard.com/report/github.com/liaisonio/liaison)
[![Release](https://img.shields.io/github/v/release/liaisonio/liaison?display_name=tag&sort=semver)](https://github.com/liaisonio/liaison/releases)
[![License](https://img.shields.io/badge/License-AGPLv3-blue.svg?logo=gnu)](LICENSE)

English | [简体中文](./README_zh.md) | [日本語](./README_ja.md) | [한국어](./README_ko.md) | [Español](./README_es.md) | [Français](./README_fr.md) | [Deutsch](./README_de.md)

[Features](#features) · [Install](#install) · [Integrations](#integrations) · [Product tour](#product-tour) · [Documentation](#documentation)

![Liaison overview with multi-application traffic](docs/assets/readme/overview-dark-v3.png)

## Features

- 🔌 **Outbound-only connectors** — connect private networks without opening inbound ports on them.
- 🔐 **Application access** — publish TCP, HTTP, HTTPS, WebSocket, and SSH services with per-access controls.
- 🖥️ **Browser workspaces** — use Web SSH, RDP, VNC, MySQL, PostgreSQL, Redis, and MongoDB without local clients.
- 🔎 **Application discovery** — scan connector devices and register discovered services from the console.
- 👥 **Identity and access management** — organize users and resources, with Casbin-backed authorization.
- 🛡️ **Firewall policies** — restrict TCP and HTTP access by source IP and CIDR.
- 📋 **Logs and audit** — record management actions and supported application sessions in one place.
- 📦 **Self-hosted deployment** — run the complete control plane on your own Linux server.

## Install

Download the Linux server archive, extract it, and run the installer:

```bash
wget https://github.com/liaisonio/liaison/releases/download/v1.8.0/liaison-1.8.0-linux-amd64.tar.gz
tar -xzf liaison-1.8.0-linux-amd64.tar.gz
cd liaison-1.8.0-linux-amd64
sudo ./install.sh
```

Open `https://<server-address>` after installation. The installer prints the initial sign-in credentials.

## Integrations

Native protocols and browser workspaces supported by Liaison.

<p align="center">
  <img src="docs/assets/integrations/ssh.svg" height="58" alt="SSH" title="SSH" />&nbsp;&nbsp;&nbsp;&nbsp;
  <img src="https://api.iconify.design/logos:microsoft-windows-icon.svg" height="50" alt="RDP" title="RDP" />&nbsp;&nbsp;&nbsp;&nbsp;
  <img src="https://upload.wikimedia.org/wikipedia/en/5/51/Virtual_Network_Computing_%28logo%29.svg" height="40" alt="VNC" title="VNC" />&nbsp;&nbsp;&nbsp;&nbsp;
  <img src="https://api.iconify.design/logos:mysql-icon.svg" height="50" alt="MySQL" title="MySQL" />&nbsp;&nbsp;&nbsp;&nbsp;
  <img src="https://api.iconify.design/logos:postgresql.svg" height="50" alt="PostgreSQL" title="PostgreSQL" />&nbsp;&nbsp;&nbsp;&nbsp;
  <img src="https://api.iconify.design/logos:redis.svg" height="50" alt="Redis" title="Redis" />&nbsp;&nbsp;&nbsp;&nbsp;
  <img src="https://api.iconify.design/logos:mongodb-icon.svg" height="54" alt="MongoDB" title="MongoDB" />
</p>

## Product tour

### Private web applications

Access media servers and other internal web applications through Liaison.

![Accessing Jellyfin through Liaison](docs/pages/jellyfin-ss.png)

### Private AI assistants

Keep internal AI tools reachable without exposing the private network.

![Accessing OpenClaw through Liaison](docs/pages/openclaw-ss.png)

### Web MySQL

Inspect schemas and run audited SQL queries directly in the browser.

![Using MySQL through Liaison](docs/assets/readme/web-mysql-dark.png)

### Web MongoDB

Explore document databases and execute commands without a local client.

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
