# <img src="docs/diagrams/liaison-logo.svg" height="40" align="absmiddle" alt="Liaison logo" /> Liaison

> **Zero-trust access for private applications.**

Connect private web apps, servers, remote desktops, and databases through outbound connectors, without exposing the private network.

[![Go](https://github.com/liaisonio/liaison/actions/workflows/go.yml/badge.svg)](https://github.com/liaisonio/liaison/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/liaisonio/liaison)](https://goreportcard.com/report/github.com/liaisonio/liaison)
[![Release](https://img.shields.io/github/v/release/liaisonio/liaison?display_name=tag&sort=semver)](https://github.com/liaisonio/liaison/releases)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

English | [简体中文](./README_zh.md) | [日本語](./README_ja.md) | [한국어](./README_ko.md) | [Español](./README_es.md) | [Français](./README_fr.md) | [Deutsch](./README_de.md)

[Features](#features) · [Install](#install) · [Product tour](#product-tour) · [Documentation](#documentation) · [Community](#community)

![Liaison overview with multi-application traffic](docs/assets/readme/overview-dark.png)

## Features

- **Outbound-only connectors** — connect private networks without opening inbound ports on them.
- **Application access** — publish TCP, HTTP, HTTPS, WebSocket, and SSH services with per-access controls.
- **Browser workspaces** — use Web SSH, RDP, VNC, MySQL, PostgreSQL, Redis, and MongoDB without local clients.
- **Application discovery** — scan connector devices and register discovered services from the console.
- **Identity and access management** — organize users and resources, with Casbin-backed authorization.
- **Firewall policies** — restrict TCP and HTTP access by source IP and CIDR.
- **Logs and audit** — record management actions and supported application sessions in one place.
- **Self-hosted deployment** — run the complete control plane on your own Linux server.

## Install

Choose a server package from the [latest release](https://github.com/liaisonio/liaison/releases/latest), then install a connector from the Web console.

### Binary and systemd

```bash
wget https://github.com/liaisonio/liaison/releases/download/v1.8.0/liaison-1.8.0-linux-amd64.tar.gz
tar -xzf liaison-1.8.0-linux-amd64.tar.gz
cd liaison-1.8.0-linux-amd64
sudo ./install.sh
```

Open `https://<server-address>` after installation. The installer prints the initial sign-in credentials.

### Docker Compose

```bash
wget https://github.com/liaisonio/liaison/releases/download/v1.8.0/liaison-1.8.0-docker-amd64.tar.gz
tar -xzf liaison-1.8.0-docker-amd64.tar.gz
cd liaison-1.8.0-docker-amd64
./load.sh
```

The bundle contains the required images. Runtime data, certificates, and logs are stored beside the Compose file. See the [Docker deployment guide](deploy/docker/README.md) for configuration, upgrades, reverse proxies, and custom certificates.

### Connector

In the Web console, open **Connectors**, create one, copy the generated command, and run it on the target Linux, macOS, or Windows device.

The connector initiates the connection to Liaison. Register or discover applications reachable from that device, then create access for them—no public address or inbound firewall rule is required on the private network.

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

![A VNC desktop opened in Liaison](docs/assets/readme/web-vnc.png)

### Web RDP

Connect to an RDP desktop from the same access workflow.

![An RDP desktop opened in Liaison](docs/assets/readme/web-rdp.png)

## Documentation

- [Docker deployment](deploy/docker/README.md)
- [API reference](docs/swagger/swagger.yaml)
- [Release notes](https://github.com/liaisonio/liaison/releases)
- [Issues](https://github.com/liaisonio/liaison/issues)
- [Discussions](https://github.com/liaisonio/liaison/discussions)

## Community

<div align="center">

<img src="docs/assets/readme/wechat-community.jpg" width="260" alt="Liaison WeChat community QR code" />

Scan with WeChat to join the Liaison developer community.

</div>

## Contributing

Bug reports, feature proposals, documentation improvements, and pull requests are welcome. Start with [Issues](https://github.com/liaisonio/liaison/issues) or open a [Pull Request](https://github.com/liaisonio/liaison/pulls).

## License

Liaison is licensed under the [Apache License 2.0](LICENSE).
