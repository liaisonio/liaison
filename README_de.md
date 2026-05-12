# <img src="docs/diagrams/liaison-logo.svg" height="40" align="absmiddle" alt="" /> Liaison

> **Konnektor-basierter Zugriff auf Geräte und Apps hinter NAT**

[![Go](https://github.com/liaisonio/liaison/actions/workflows/go.yml/badge.svg)](https://github.com/liaisonio/liaison/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/liaisonio/liaison)](https://goreportcard.com/report/github.com/liaisonio/liaison)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Tech](https://img.shields.io/badge/Tech-Go%20%7C%20TypeScript%20%7C%20React-blue)](#)
[![Version](https://img.shields.io/badge/Version-v1.5.0-green)](#)

[简体中文](./README.md) | [English](./README_en.md) | [日本語](./README_ja.md) | [한국어](./README_ko.md) | [Español](./README_es.md) | [Français](./README_fr.md) | Deutsch

![Dashboard](docs/pages/home_en.png)

| Jellyfin (Heimfilme überall streamen) | OpenClaw (Heim-KI überall nutzen) |
|:---:|:---:|
| ![Jellyfin](docs/pages/jellyfin-ss.png) | ![OpenClaw](docs/pages/openclaw-ss.png) |

[Schnellstart](#schnellstart) • [Einführung](#einführung) • [Dokumentation](#dokumentation) • [Mitwirken](#mitwirken)

---

## Einführung

Liaison ist eine Enterprise-taugliche Lösung für Anwendungszugriff, die jederzeit ein- und ausgeschaltet werden kann, ohne Ports im LAN oder Heimnetz zu öffnen. Sie bietet ein vollständiges Feature-Set: automatische App-Erkennung auf verbundenen Geräten, Echtzeit-Traffic-Metriken und sichere TLS-verschlüsselte Kommunikation.

Dieses Projekt adressiert:

- **Zugriff auf private Netze** — Geräte und Dienste hinter NAT mit minimaler Konfiguration aus dem öffentlichen Internet erreichen
- **Multi-Device-Management** — Geräte an verschiedenen Standorten mit Linux/macOS/Windows-Support verwalten
- **Sichere Konnektivität** — TLS-verschlüsseltes Transport, ohne LAN- oder Heimnetz-Ports freizugeben
- **Firewall je Entry** — Source-IP-CIDR-Whitelist auf jedem TCP- oder HTTP-Entry, wird beim Verbindungsaufbau durchgesetzt
- **Traffic-Monitoring** — Echtzeit-Gerätestatus und Traffic-Metriken für Betrieb und Kapazitätsplanung
- **Anwendungs-Proxy** — TCP, HTTP/HTTPS, WebSocket und weitere Protokolle
- **API-Automatisierung** — Personal Access Tokens (PAT) für CLI/Skripte, Browser-basierter Login-Flow unter `/cli-auth`

Anwendungsfälle:

<div align="center">

| **💼 Remote Work & Dev** | **🧑‍💻 Personal Studio** | **🏠 Heimnetz / NAS** | **🌐 Multi-DC / Multi-Region** | **⚡ Edge & Ops** |
|:---:|:---:|:---:|:---:|:---:|
| Büro- und Heim-Geräte verbinden — Remote-Entwicklung und -Debugging | Workstations und private Umgebungen sicher verbinden, Geräte zentral verwalten | Heim-NAS und Smart-Home-Dienste aus dem Internet erreichen | Einheitliche Konnektivität zwischen Servern und Anwendungen über Regionen und DCs | Edge-Anwendungen verbinden, per Remote Health- und Traffic-Checks überwachen |

</div>

---

## Schnellstart

Wähle eine der beiden Server-Deployment-Optionen, danach installiere einen Konnektor.

### Server installieren — Option 1: Binary + systemd

**1. Herunterladen**

```bash
wget https://github.com/liaisonio/liaison/releases/download/v1.5.0/liaison-1.5.0-linux-amd64.tar.gz
tar -xzf liaison-1.5.0-linux-amd64.tar.gz
cd liaison-1.5.0-linux-amd64
```

**2. Installationsskript ausführen**

```bash
sudo ./install.sh
```

Öffentliche IP oder Domain werden abgefragt. Ohne Eingabe innerhalb von 30 Sekunden wird die erkannte öffentliche IP verwendet.

**3. Web-Konsole öffnen**

Öffne `https://deine-öffentliche-ip`, um die Web-Konsole zu erreichen.

> **Hinweis:** Die Standard-Admin-Zugangsdaten erscheinen in der Ausgabe von install.sh oder in der Konfiguration.

### Server installieren — Option 2: Docker Compose

Erfordert Docker 20.10+ und das `docker compose`-Plugin. Das Bundle liefert `liaison` (Web-Konsole + API) und `frontier` (Konnektor-Gateway) als zwei Container; die Images sind vorgebaut — kein Registry oder Source-Checkout nötig.

```bash
wget https://github.com/liaisonio/liaison/releases/download/v1.5.0/liaison-1.5.0-docker-amd64.tar.gz
tar -xzf liaison-1.5.0-docker-amd64.tar.gz
cd liaison-1.5.0-docker-amd64
./load.sh
```

`load.sh` erkennt automatisch die öffentliche IP (mit 30-Sekunden-Bestätigungszähler), lädt die Images, startet den Stack und gibt das einmalige Admin-Passwort aus, sobald liaison bereit ist. Speichere das Passwort und melde dich unter `https://<öffentliche-ip>` an.

Daten (`data/` SQLite), TLS-Zertifikate (`certs/`) und Logs (`logs/`) werden neben der `docker-compose.yaml` als Bind-Mount persistiert. Siehe [`deploy/docker/README.md`](deploy/docker/README.md) für Source-Builds, Upgrade, Reset, Reverse-Proxy und eigene Zertifikate.

### Konnektor installieren

Zwei Installationswege, wähle den passenden für das Zielgerät.

#### Option A — Liaison Desktop (GUI, macOS / Windows)

Menüleisten- / Tray-App, die den Konnektor umschließt und Single-Click-Anmeldung, Statusanzeige, Pause / Fortsetzen sowie Ein-Klick-Zugriff auf das Dashboard bietet. Ideal für Laptops und Workstations.

<div align="center">

| macOS | Windows |
|:---:|:---:|
| <img src="docs/images/desktop-client/popup-macos.png" alt="Liaison Desktop on macOS" width="360" /> | <img src="docs/images/desktop-client/popup-windows.png" alt="Liaison Desktop on Windows" width="360" /> |

</div>

- **Ein-Klick-Anmeldung** — browservermittelter OAuth-Flow, PAT wird im OS-Schlüsselbund gespeichert (Keychain auf macOS, Anmeldeinformationsverwaltung auf Windows)
- **Multi-Deployment** — Standard ist `liaison.cloud`; das Zahnrad-Symbol unten links erlaubt den Wechsel zu jedem privaten Deployment ohne Neuinstallation
- **Heartbeat-bewusster Status** — Verbindung herstellen → Online-Übergänge spiegeln den tatsächlichen Tunnelzustand wider, nicht nur Prozess-Lebendigkeit
- **Pause überlebt das Beenden** — Absicht auf Disk persistiert, eine pausierte Session bleibt nach Neustart pausiert

**Download (Rolling Pre-Release, neuester Build aus `feat/desktop-client`):**

| Plattform | Datei |
|:---|:---|
| macOS (Apple Silicon + Intel, universal) | [`Liaison-Desktop-dev-feat-desktop-client-macOS-universal.dmg`](https://github.com/liaisonio/liaison/releases/download/desktop-latest/Liaison-Desktop-dev-feat-desktop-client-macOS-universal.dmg) |
| Windows (.msi-Installer) | [`Liaison-Desktop-dev-feat-desktop-client-Windows-x64.msi`](https://github.com/liaisonio/liaison/releases/download/desktop-latest/Liaison-Desktop-dev-feat-desktop-client-Windows-x64.msi) |
| Windows (.exe NSIS, mit Keychain-Bereinigung beim Deinstallieren) | [`Liaison-Desktop-dev-feat-desktop-client-Windows-x64-setup.exe`](https://github.com/liaisonio/liaison/releases/download/desktop-latest/Liaison-Desktop-dev-feat-desktop-client-Windows-x64-setup.exe) |

> Die v0.1-Installer sind unsigniert. macOS Gatekeeper und Windows SmartScreen warnen beim ersten Start — Rechtsklick → Öffnen auf macOS, oder „Weitere Informationen" → „Trotzdem ausführen" auf Windows. WebView2 Runtime ist auf Windows erforderlich; Win10 1803+ und Win11 enthalten es.

#### Option B — CLI-Installationsbefehl (Linux / headless)

**Erstelle einen neuen Konnektor** in der Web-Konsole, kopiere den Installationsbefehl für deine Plattform aus der UI und führe ihn auf dem Zielgerät aus. Der Konnektor erscheint automatisch in der Konsole.

---

## Systemanforderungen

| Komponente | Anforderungen |
|:---|:---|
| **Server** | Linux (Ubuntu 20.04+ oder CentOS 7+ empfohlen) |
| **Konnektor** | Linux / macOS / Windows (x86_64 und ARM64) |
| **Browser** | Chrome 90+, Firefox 88+, Safari 14+, Edge 90+ |

---

## Architektur

<img src="./docs/diagrams/liaison.png" width="80%">

Liaison nutzt eine zentralisierte Architektur, bei der Frontier alle Konnektoren verwaltet.

**Komponenten**

- **Liaison** — Web-UI und API, plus Applikations-Entry-Points
- **Frontier** — Konnektor-Gateway, verwaltet Verbindungen und Traffic-Routing
- **Edge** — Konnektor-Client auf den Zielgeräten

---

## Feature-Showcase

| Feature | Screenshot |
|:---:|:---:|
| Geräteverwaltung | ![Device](docs/pages/device_en.png) |
| Anwendungsverwaltung | ![Application](docs/pages/application_en.png) |
| Proxy-Konfiguration | ![Proxy](docs/pages/proxy_en.png) |
| Konnektor-Verwaltung | ![Edge](docs/pages/edge_en.png) |

---

## Dokumentation

- [Business Flow](./docs/biz_sequence.md)
- [API](./docs/swagger/)

---

## Mitwirken

Beiträge sind willkommen.

- [Bug melden](https://github.com/liaisonio/liaison/issues/new?template=bug_report.md)
- [Feature vorschlagen](https://github.com/liaisonio/liaison/issues/new?template=feature_request.md)
- [PR eröffnen](https://github.com/liaisonio/liaison/pulls)
- [Dokumentation verbessern](https://github.com/liaisonio/liaison/issues/new?template=documentation.md)

1. Repository forken
2. Branch erstellen (`git checkout -b feature/AmazingFeature`)
3. Committen (`git commit -m 'Add some AmazingFeature'`)
4. Pushen (`git push origin feature/AmazingFeature`)
5. Pull Request öffnen

---

## Lizenz

[Apache License 2.0](LICENSE).

---

<div align="center">

**Wenn das Projekt dir hilft, vergib bitte einen ⭐ Star!**

Made with ❤️ by [Liaison Contributors](https://github.com/liaisonio/liaison/graphs/contributors)

[GitHub](https://github.com/liaisonio/liaison) • [Issues](https://github.com/liaisonio/liaison/issues) • [Discussions](https://github.com/liaisonio/liaison/discussions)

</div>
