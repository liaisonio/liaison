<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/assets/liaison-brand-rays-dark.svg" />
  <img src="docs/assets/liaison-brand-rays-light.svg" width="270" height="88" alt="Liaison" />
</picture>

> **Selbst gehosteter Zugriff auf private Apps, LLMs und KI-Agenten.**

Browser-Arbeitsbereiche, API-Freigaben mit begrenztem Berechtigungsumfang und kontextbezogene KI-Unterstützung – über Konnektoren, die ausschließlich ausgehende Verbindungen aus dem privaten Netz aufbauen.

[![CI](https://github.com/liaisonio/liaison/actions/workflows/go.yml/badge.svg)](https://github.com/liaisonio/liaison/actions/workflows/go.yml)
[![Release](https://img.shields.io/github/v/release/liaisonio/liaison?display_name=tag&sort=semver)](https://github.com/liaisonio/liaison/releases)
[![Downloads](https://img.shields.io/github/downloads/liaisonio/liaison/total)](https://github.com/liaisonio/liaison/releases)
[![Go](https://img.shields.io/github/go-mod/go-version/liaisonio/liaison?logo=go&logoColor=white)](go.mod)
[![React](https://img.shields.io/badge/React-20232A?logo=react&logoColor=61DAFB)](web/package.json)
[![TypeScript](https://img.shields.io/badge/TypeScript-3178C6?logo=typescript&logoColor=white)](web/package.json)
[![Vite](https://img.shields.io/badge/Vite-646CFF?logo=vite&logoColor=white)](web/vite.config.ts)
[![Tailwind CSS](https://img.shields.io/badge/Tailwind_CSS-06B6D4?logo=tailwindcss&logoColor=white)](web/tailwind.config.cjs)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](#contributing)

[English](./README.md) | [简体中文](./README_zh.md) | [日本語](./README_ja.md) | [한국어](./README_ko.md) | [Español](./README_es.md) | [Français](./README_fr.md) | Deutsch

[Website](https://liaison.cloud) · [Dokumentation](https://liaison.cloud/docs/get-started/introduction) · [Funktionen](#features) · [Installation](#install) · [Zugriffsprotokolle](#supported-access-protocols) · [Agent-Modelle](#agent-model-providers) · [Produktrundgang](#product-tour)

![Liaison-Produktdemo mit Beispieldaten.](docs/assets/readme/liaison-product-demo-full-v2.gif)

<a id="features"></a>

## Funktionen

- 🤖 **Zugriff auf lokale LLMs** — Lokale Modelle über authentifizierte APIs und einen Playground im Browser teilen.
- 🔑 **Kontrollierte API-Freigaben** — Schlüssel auf Modelle beschränken, Token-Kontingente festlegen, Zugriff widerrufen und Nutzung verfolgen.
- 🤖 **Zugriff auf KI-Agenten** — Codex und Claude Code auf deinen Geräten im Browser nutzen – mit fortsetzbaren Sitzungen, Ausführungsfortschritt und Berechtigungsabfragen.
- 💻 **Browser-IDE** — Projekte mit code-server bearbeiten sowie Terminals und Erweiterungen nutzen.
- ✨ **KI im Arbeitsablauf** — Terminalausgaben prüfen, Befehle entwerfen und Datenbanken innerhalb der Benutzerberechtigungen abfragen; erforderliche Freigaben werden vorher eingeholt.
- 💬 **Ressourcen abfragen** — Konnektoren, Geräte und Anwendungen innerhalb deines Zugriffsbereichs finden.
- 🔌 **Nur ausgehende Verbindungen** — Private Geräte anbinden, ohne darauf eingehende Ports zu öffnen.
- 🔐 **Anwendungszugriff** — TCP-, HTTP-, HTTPS-, WebSocket- und SSH-Dienste mit individuellen Zugriffskontrollen nutzen.
- 🖥️ **Browser-Arbeitsbereiche** — Terminals, Remote-Desktops, Datenbanken und Caches im Browser verwenden.
- 📁 **Dateien und Objekte** — Dateien per SFTP verwalten und S3-kompatiblen Speicher durchsuchen (nur lesend).
- 🔎 **Anwendungserkennung** — Vorhandene Dienste auf Konnektor-Geräten erkennen und über die Konsole registrieren.
- 👥 **Identitäts- und Zugriffsverwaltung** — Benutzer und Ressourcen mit Casbin-basierter Autorisierung verwalten.
- 🛡️ **Firewall-Regeln** — TCP- und HTTP-Zugriffe anhand der Quell-IP oder eines CIDR-Bereichs einschränken.
- 📋 **Protokolle und Audit** — Verwaltungsaktionen und unterstützte Anwendungssitzungen prüfen.
- 📦 **Selbst gehosteter Betrieb** — Die vollständige Steuerungsebene auf dem eigenen Linux-Server betreiben.

<a id="install"></a>

## Installation

Lade das eigenständige Docker-Paket herunter, entpacke es und starte das Installationsprogramm (Docker 20.10 oder neuer mit Compose erforderlich):

```bash
wget https://github.com/liaisonio/liaison/releases/download/v1.15.4/liaison-1.15.4-linux-amd64.tar.gz
tar -xzf liaison-1.15.4-linux-amd64.tar.gz
cd liaison-1.15.4-linux-amd64
./install.sh
```

Öffne nach der Installation `https://<server-address>`. Das Installationsprogramm gibt die anfänglichen Anmeldedaten aus.

<a id="supported-access-protocols"></a>

## Unterstützte Zugriffsprotokolle

Greife mit Liaison auf deine vorhandenen privaten Dienste zu.

<table>
  <tr><th align="left">SSH / SFTP</th><td><img src="docs/assets/integrations/readme/ssh.svg" width="40" height="40" alt="SSH / WebSSH" title="SSH / WebSSH" />&nbsp;&nbsp; <img src="docs/assets/integrations/readme/sftp.svg" width="40" height="40" alt="WebSFTP" title="WebSFTP" /></td></tr>
  <tr><th align="left">Desktop</th><td><img src="docs/assets/integrations/windows.svg" width="40" height="40" alt="WebRDP" title="WebRDP" />&nbsp;&nbsp; <img src="docs/assets/integrations/vnc.png" width="40" height="40" alt="WebVNC" title="WebVNC" /></td></tr>
  <tr><th align="left">SQL-Datenbanken</th><td><img src="docs/assets/integrations/mysql.svg" width="40" height="40" alt="WebMySQL" title="WebMySQL" />&nbsp;&nbsp; <img src="docs/assets/integrations/mariadb.svg" width="40" height="40" alt="WebMariaDB" title="WebMariaDB" />&nbsp;&nbsp; <img src="docs/assets/integrations/postgresql.svg" width="40" height="40" alt="WebPostgreSQL" title="WebPostgreSQL" />&nbsp;&nbsp; <img src="docs/assets/integrations/sqlserver.svg" width="40" height="40" alt="WebSQLServer" title="WebSQLServer" />&nbsp;&nbsp; <img src="docs/assets/integrations/oracle.svg" width="40" height="40" alt="WebOracle" title="WebOracle" />&nbsp;&nbsp; <img src="docs/assets/integrations/clickhouse.svg" width="40" height="40" alt="WebClickHouse" title="WebClickHouse" /></td></tr>
  <tr><th align="left">Daten und Suche</th><td><img src="docs/assets/integrations/mongodb.svg" width="40" height="40" alt="WebMongoDB" title="WebMongoDB" />&nbsp;&nbsp; <img src="docs/assets/integrations/elasticsearch.svg" width="40" height="40" alt="WebElasticsearch" title="WebElasticsearch" />&nbsp;&nbsp; <img src="docs/assets/integrations/opensearch.svg" width="40" height="40" alt="WebOpenSearch" title="WebOpenSearch" /></td></tr>
  <tr><th align="left">Cache</th><td><img src="docs/assets/integrations/redis.svg" width="40" height="40" alt="WebRedis" title="WebRedis" />&nbsp;&nbsp; <img src="docs/assets/integrations/memcached.svg" width="40" height="40" alt="WebMemcached" title="WebMemcached" /></td></tr>
  <tr><th align="left">Speicher</th><td><img src="docs/assets/integrations/readme/s3.svg" width="40" height="40" alt="WebS3 (S3-compatible)" title="WebS3 (S3-compatible)" /></td></tr>
  <tr><th align="left">LLM-Upstream-Protokolle</th><td><picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/integrations/readme/openai-dark.svg" /><img src="docs/assets/integrations/readme/openai-light.svg" width="40" height="40" alt="OpenAI-compatible" title="OpenAI-compatible" /></picture>&nbsp;&nbsp; <img src="docs/assets/integrations/anthropic.svg" width="40" height="40" alt="Anthropic Messages" title="Anthropic Messages" />&nbsp;&nbsp; <picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/integrations/readme/ollama-dark.svg" /><img src="docs/assets/integrations/readme/ollama-light.svg" width="40" height="40" alt="Ollama" title="Ollama" /></picture></td></tr>
  <tr><th align="left">Web / TCP</th><td><img src="docs/assets/integrations/readme/web.svg" width="40" height="40" alt="HTTP / HTTPS / WebSocket" title="HTTP / HTTPS / WebSocket" />&nbsp;&nbsp; <img src="docs/assets/integrations/readme/tcp.svg" width="40" height="40" alt="TCP" title="TCP" /></td></tr>
</table>

LLM-Upstreams unterstützen **OpenAI-compatible**, **Anthropic Messages** und **Ollama**. Clients verwenden OpenAI-compatible-Endpunkte; für Anthropic-Upstreams stehen zusätzlich native Messages-Endpunkte bereit. Die Logos kennzeichnen vorhandene, zugängliche Dienste, nicht von Liaison mitgelieferte oder bereitgestellte Produkte. Datenbank- und Desktop-Logos stehen für Browser-Arbeitsbereiche, nicht für native Datenbank-, RDP- oder VNC-Server, die Verbindungen annehmen.

<a id="agent-model-providers"></a>

## Modellanbieter für den Agent

Das Modell des integrierten Liaison-Agent lässt sich unabhängig vom Dienstzugriff konfigurieren.

<table>
  <tr>
    <th align="left">Modelle</th>
    <td><picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/integrations/readme/openai-dark.svg" /><img src="docs/assets/integrations/readme/openai-light.svg" width="40" height="40" alt="OpenAI" title="OpenAI" /></picture>&nbsp;&nbsp; <img src="docs/assets/integrations/anthropic.svg" width="40" height="40" alt="Anthropic / Claude" title="Anthropic / Claude" />&nbsp;&nbsp; <img src="docs/assets/integrations/gemini.svg" width="40" height="40" alt="Google Gemini" title="Google Gemini" />&nbsp;&nbsp; <img src="docs/assets/integrations/deepseek.svg" width="40" height="40" alt="DeepSeek" title="DeepSeek" />&nbsp;&nbsp; <picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/integrations/readme/zhipu-dark.svg" /><img src="docs/assets/integrations/readme/zhipu-light.svg" width="40" height="40" alt="Z.ai / GLM" title="Z.ai / GLM" /></picture>&nbsp;&nbsp; <picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/integrations/readme/kimi-dark.svg" /><img src="docs/assets/integrations/readme/kimi-light.svg" width="40" height="40" alt="Moonshot / Kimi" title="Moonshot / Kimi" /></picture>&nbsp;&nbsp; <img src="docs/assets/integrations/minimax.svg" width="40" height="40" alt="MiniMax" title="MiniMax" />&nbsp;&nbsp; <picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/integrations/readme/mimo-dark.svg" /><img src="docs/assets/integrations/readme/mimo-light.svg" width="40" height="40" alt="Xiaomi MiMo" title="Xiaomi MiMo" /></picture></td>
  </tr>
</table>

Acht vorkonfigurierte Anbieter sowie benutzerdefinierte OpenAI-compatible-Dienste.

<a id="product-tour"></a>

## Produktrundgang

### Zugriff auf lokale LLMs

![Beispielkonfiguration für lokale LLM-Modelle, Protokolle und API-Schlüssel-Authentifizierung.](docs/assets/readme/local-llm-access-models-dark-v2.png)

### Web SSH

Arbeite in einem Browser-Terminal mit Audit-Funktion. Der Agent kann Sitzungsausgaben prüfen und beim Entwerfen von Befehlen helfen.

![Web SSH in Liaison](docs/assets/readme/web-ssh-dark.png)

### Private Webanwendungen

Greife über Liaison auf Medienserver und andere interne Webanwendungen zu.

![Jellyfin über Liaison](docs/pages/jellyfin-ss.png)

### Private KI-Assistenten

Nutze interne KI-Werkzeuge, ohne das private Netzwerk offenzulegen.

![OpenClaw über Liaison](docs/pages/openclaw-ss.png)

### Web MySQL

Durchsuche Schemas, führe SQL aus und bitte den Sitzungs-Agent um Abfragen und Erläuterungen der Ergebnisse – mit Freigabe, wo erforderlich.

![MySQL-Arbeitsbereich in Liaison](docs/assets/readme/web-mysql-dark.png)

### Web MongoDB

Durchsuche Collections und lass den Agent genehmigte Abfragen ausführen und Dokumente erklären, ohne lokalen Client.

![MongoDB-Arbeitsbereich in Liaison](docs/assets/readme/web-mongodb-dark.png)

### Web VNC

Öffne einen privaten VNC-Desktop in einer verwalteten Browser-Sitzung.

![VNC-Desktop in Liaison](docs/assets/readme/web-vnc-dark.png)

### Web RDP

Verbinde dich über denselben Zugriffsablauf mit einem RDP-Desktop.

![RDP-Desktop in Liaison](docs/assets/readme/web-rdp-dark-v2.png)

<a id="documentation"></a>

## Dokumentation

- [Docker-Bereitstellung](deploy/docker/README.md)
- [API-Referenz](docs/swagger/swagger.yaml)
- [Versionshinweise](https://github.com/liaisonio/liaison/releases)
- [Issues](https://github.com/liaisonio/liaison/issues)
- [Diskussionen](https://github.com/liaisonio/liaison/discussions)

<a id="contributing"></a>

## Mitwirken

Liaison wird kontinuierlich weiterentwickelt. Beiträge zur selbst gehosteten Edition in diesem Repository sind willkommen.

- KI-Agenten: Integrationen, Sitzungsverwaltung und Zusammenarbeit.
- Anwendungszugriff: SSH, Dateien, Datenbanken, IDEs, Remote-Desktops und Modell-APIs.
- Plattformunterstützung: Installation und Kompatibilität unter Linux, macOS und Windows.
- Benutzerfreundlichkeit: Verbesserungen der Oberfläche, Übersetzungen und Dokumentation.

Besprich größere Änderungen zuerst in einem Issue. Beschränke jeden PR auf ein Thema und beschreibe die durchgeführten Prüfungen. Füge keine Zugangsdaten, privaten Daten oder umgebungsspezifischen Deployment-Details hinzu. Die Mitarbeit ist vollständig auf GitHub möglich; die Teilnahme an einer Chatgruppe ist freiwillig.

Fehlermeldungen, Funktionsvorschläge, Verbesserungen der Dokumentation und Pull Requests sind willkommen. Starte unter [Issues](https://github.com/liaisonio/liaison/issues) oder erstelle einen [Pull Request](https://github.com/liaisonio/liaison/pulls).

<a id="license"></a>

## Lizenz

Liaison steht unter der [GNU Affero General Public License v3.0](LICENSE).
