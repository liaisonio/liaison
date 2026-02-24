# <img src="website/assets/favicon.svg" alt="" width="48" style="vertical-align: middle;" /> Liaison

[中文](./README.md) | English

[![Go](https://github.com/singchia/liaison/actions/workflows/go.yml/badge.svg)](https://github.com/singchia/liaison/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/singchia/liaison)](https://goreportcard.com/report/github.com/singchia/liaison)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Tech](https://img.shields.io/badge/Tech-Go%20%7C%20TypeScript%20%7C%20React-blue)](#)
[![Version](https://img.shields.io/badge/Version-v1.3.1-green)](#)

> **Network connectivity made simple — Easily connect devices and applications across different locations**

![Dashboard](docs/pages/home_en.png)

[Quick Start](#quick-start) • [Introduction](#introduction) • [Documentation](#documentation) • [Contributing](#contributing)

---

## Introduction

Liaison is an enterprise-grade application access solution that does not expose any internal ports and can be enabled or disabled at any time. It provides full product features: auto-discovery of device applications, real-time traffic statistics, and secure TLS-encrypted transport.

This project addresses:

- **Intranet access** — Access internal devices and services from the public internet without complex setup
- **Multi-device management** — Manage devices across locations with Linux/macOS/Windows support
- **Secure connectivity** — TLS encryption, no exposure of the internal network, enable or disable at any time
- **Traffic monitoring** — Real-time device status and traffic metrics for operations and capacity planning
- **Application proxy** — TCP, HTTP/HTTPS, WebSocket and other protocols

Use cases:

<div align="center">

| **💼 Remote Work & Dev** | **🧑‍💻 Personal Studio** | **🏠 Home Network / NAS** | **🌐 Multi-datacenter / Multi-region** | **⚡ Edge & Ops** |
|:---:|:---:|:---:|:---:|:---:|
| Connect office and home devices for remote development and debugging | Securely connect workstations and private environments with unified device management | Access home NAS and smart-home services from the public internet | Unified connectivity for servers and applications across regions and datacenters | Connect and monitor edge applications with remote health and traffic checks |

</div>

---

## Quick Start

### Install Server

**1. Download**

```bash
wget https://github.com/singchia/liaison/releases/download/v1.3.1/liaison-v1.3.1-linux-amd64.tar.gz
tar -xzf liaison-v1.3.1-linux-amd64.tar.gz
cd liaison-v1.3.1-linux-amd64
```

**2. Run install script**

```bash
sudo ./install.sh
```

You will be prompted for a public IP or domain; if none is entered within 30 seconds, the detected public IP is used.

**3. Open Web console**

Visit `https://your-public-ip` to access the Web console.

> **Tip:** Default admin credentials are shown in the install script output or config.

### Install Connector

**Create a new connector** in the Web console, copy the install command for your platform from the page, and run it on the target device. The connector will appear in the console automatically.

---

## System Requirements

| Component | Requirements |
|:---|:---|
| **Server** | Linux (Ubuntu 20.04+ or CentOS 7+ recommended) |
| **Connector** | Linux / macOS / Windows (x86_64 and ARM64) |
| **Browser** | Chrome 90+, Firefox 88+, Safari 14+, Edge 90+ |

---

## Architecture

<img src="./docs/diagrams/liaison.png" width="80%">

Liaison uses a centralized architecture with Frontier managing all connectors.

**Components**

- **Liaison** — Web UI and API, Entries for applications
- **Frontier** — Connector gateway for connections and traffic
- **Edge** — Connector client on target devices

---

## Feature Showcase

| Feature | Screenshot |
|:---:|:---:|
| Device Management | ![Device](docs/pages/device_en.png) |
| Application Management | ![Application](docs/pages/application_en.png) |
| Proxy Configuration | ![Proxy](docs/pages/proxy_en.png) |
| Edge Management | ![Edge](docs/pages/edge.png) |

---

## Documentation

- [Business flow](./docs/biz_sequence.md)
- [API](./docs/swagger/)

---

## Contributing

Contributions are welcome.

- [Report a bug](https://github.com/singchia/liaison/issues/new?template=bug_report.md)
- [Suggest a feature](https://github.com/singchia/liaison/issues/new?template=feature_request.md)
- [Open a PR](https://github.com/singchia/liaison/pulls)
- [Improve docs](https://github.com/singchia/liaison/issues/new?template=documentation.md)

1. Fork the repo  
2. Create a branch (`git checkout -b feature/AmazingFeature`)  
3. Commit (`git commit -m 'Add some AmazingFeature'`)  
4. Push (`git push origin feature/AmazingFeature`)  
5. Open a Pull Request  

---

## License

[Apache License 2.0](LICENSE).

---

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=singchia/liaison&type=Date)](https://star-history.com/#singchia/liaison&Date)

---

<div align="center">

**If this project helps you, please give it a ⭐ Star!**

Made with ❤️ by [Liaison Contributors](https://github.com/singchia/liaison/graphs/contributors)

[GitHub](https://github.com/singchia/liaison) • [Issues](https://github.com/singchia/liaison/issues) • [Discussions](https://github.com/singchia/liaison/discussions)

</div>
