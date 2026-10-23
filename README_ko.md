<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/assets/liaison-brand-rays-dark.svg" />
  <img src="docs/assets/liaison-brand-rays-light.svg" width="270" height="88" alt="Liaison" />
</picture>

> **비공개 앱, LLM, AI 에이전트에 접근하는 셀프 호스팅 플랫폼.**

브라우저 작업 공간, 범위를 제한한 API 공유, 작업 맥락을 이해하는 AI 지원을 제공합니다. 커넥터의 아웃바운드 연결로 비공개 서비스에 접근합니다.

[![CI](https://github.com/liaisonio/liaison/actions/workflows/go.yml/badge.svg)](https://github.com/liaisonio/liaison/actions/workflows/go.yml)
[![Release](https://img.shields.io/github/v/release/liaisonio/liaison?display_name=tag&sort=semver)](https://github.com/liaisonio/liaison/releases)
[![Downloads](https://img.shields.io/github/downloads/liaisonio/liaison/total)](https://github.com/liaisonio/liaison/releases)
[![Go](https://img.shields.io/github/go-mod/go-version/liaisonio/liaison?logo=go&logoColor=white)](go.mod)
[![React](https://img.shields.io/badge/React-20232A?logo=react&logoColor=61DAFB)](web/package.json)
[![TypeScript](https://img.shields.io/badge/TypeScript-3178C6?logo=typescript&logoColor=white)](web/package.json)
[![Vite](https://img.shields.io/badge/Vite-646CFF?logo=vite&logoColor=white)](web/vite.config.ts)
[![Tailwind CSS](https://img.shields.io/badge/Tailwind_CSS-06B6D4?logo=tailwindcss&logoColor=white)](web/tailwind.config.cjs)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](#contributing)

[English](./README.md) | [简体中文](./README_zh.md) | [日本語](./README_ja.md) | 한국어 | [Español](./README_es.md) | [Français](./README_fr.md) | [Deutsch](./README_de.md)

[웹사이트](https://liaison.cloud) · [문서](https://liaison.cloud/docs/get-started/introduction) · [기능](#features) · [설치](#install) · [접근 프로토콜](#supported-access-protocols) · [Agent 모델](#agent-model-providers) · [제품 둘러보기](#product-tour)

![샘플 데이터를 사용하는 Liaison 제품 데모.](docs/assets/readme/liaison-product-demo-full-v2.gif)

<a id="features"></a>

## 주요 기능

- 🤖 **로컬 LLM 접근** — 인증된 API와 브라우저 Playground로 로컬 모델을 공유합니다.
- 🔑 **API 공유 제어** — 키별 모델 범위와 Token 한도를 설정하고, 접근을 취소하며 사용량을 확인합니다.
- 🤖 **AI 에이전트 접근** — 브라우저에서 기기의 Codex와 Claude Code를 사용합니다. 세션 재개, 실행 진행 상황, 권한 확인을 지원합니다.
- 💻 **브라우저 IDE** — code-server로 프로젝트를 편집하고 터미널과 확장 기능을 사용합니다.
- ✨ **작업 속 AI 지원** — 사용자 권한 내에서 터미널 출력 확인, 명령 작성, 데이터베이스 조회를 지원하며 필요한 작업은 승인을 요청합니다.
- 💬 **리소스 검색** — 접근 권한이 있는 커넥터, 기기, 애플리케이션을 찾습니다.
- 🔌 **아웃바운드 전용 커넥터** — 기기의 인바운드 포트를 열지 않고 비공개 기기에 연결합니다.
- 🔐 **애플리케이션 접근** — 접근 항목별 제어로 TCP, HTTP, HTTPS, WebSocket, SSH 서비스를 사용합니다.
- 🖥️ **브라우저 작업 공간** — 브라우저에서 터미널, 원격 데스크톱, 데이터베이스, 캐시를 사용합니다.
- 📁 **파일과 객체** — SFTP로 파일을 관리하고 S3 호환 스토리지를 탐색합니다(읽기 전용).
- 🔎 **애플리케이션 탐지** — 커넥터 기기의 기존 서비스를 탐지하고 콘솔에서 등록합니다.
- 👥 **ID 및 접근 관리** — Casbin 기반 인가로 사용자와 리소스를 관리합니다.
- 🛡️ **방화벽 정책** — 출발지 IP 또는 CIDR로 TCP와 HTTP 접근을 제한합니다.
- 📋 **로그 및 감사** — 관리 작업과 지원되는 애플리케이션 세션을 검토합니다.
- 📦 **셀프 호스팅** — 자체 Linux 서버에서 전체 컨트롤 플레인을 실행합니다.

<a id="install"></a>

## 설치

자체 완결형 Docker 번들을 다운로드하고 압축을 푼 뒤 설치 프로그램을 실행합니다(Docker 20.10 이상과 Compose 필요):

```bash
wget https://github.com/liaisonio/liaison/releases/download/v1.15.4/liaison-1.15.4-linux-amd64.tar.gz
tar -xzf liaison-1.15.4-linux-amd64.tar.gz
cd liaison-1.15.4-linux-amd64
./install.sh
```

설치 후 `https://<server-address>`를 엽니다. 초기 로그인 정보는 설치 프로그램이 출력합니다.

<a id="supported-access-protocols"></a>

## 지원 접근 프로토콜

Liaison으로 기존 비공개 서비스에 접근합니다.

<table>
  <tr><th align="left">SSH / SFTP</th><td><img src="docs/assets/integrations/readme/ssh.svg" width="40" height="40" alt="SSH / WebSSH" title="SSH / WebSSH" />&nbsp;&nbsp; <img src="docs/assets/integrations/readme/sftp.svg" width="40" height="40" alt="WebSFTP" title="WebSFTP" /></td></tr>
  <tr><th align="left">데스크톱</th><td><img src="docs/assets/integrations/windows.svg" width="40" height="40" alt="WebRDP" title="WebRDP" />&nbsp;&nbsp; <img src="docs/assets/integrations/vnc.png" width="40" height="40" alt="WebVNC" title="WebVNC" /></td></tr>
  <tr><th align="left">SQL 데이터베이스</th><td><img src="docs/assets/integrations/mysql.svg" width="40" height="40" alt="WebMySQL" title="WebMySQL" />&nbsp;&nbsp; <img src="docs/assets/integrations/mariadb.svg" width="40" height="40" alt="WebMariaDB" title="WebMariaDB" />&nbsp;&nbsp; <img src="docs/assets/integrations/postgresql.svg" width="40" height="40" alt="WebPostgreSQL" title="WebPostgreSQL" />&nbsp;&nbsp; <img src="docs/assets/integrations/sqlserver.svg" width="40" height="40" alt="WebSQLServer" title="WebSQLServer" />&nbsp;&nbsp; <img src="docs/assets/integrations/oracle.svg" width="40" height="40" alt="WebOracle" title="WebOracle" />&nbsp;&nbsp; <img src="docs/assets/integrations/clickhouse.svg" width="40" height="40" alt="WebClickHouse" title="WebClickHouse" /></td></tr>
  <tr><th align="left">데이터 및 검색</th><td><img src="docs/assets/integrations/mongodb.svg" width="40" height="40" alt="WebMongoDB" title="WebMongoDB" />&nbsp;&nbsp; <img src="docs/assets/integrations/elasticsearch.svg" width="40" height="40" alt="WebElasticsearch" title="WebElasticsearch" />&nbsp;&nbsp; <img src="docs/assets/integrations/opensearch.svg" width="40" height="40" alt="WebOpenSearch" title="WebOpenSearch" /></td></tr>
  <tr><th align="left">캐시</th><td><img src="docs/assets/integrations/redis.svg" width="40" height="40" alt="WebRedis" title="WebRedis" />&nbsp;&nbsp; <img src="docs/assets/integrations/memcached.svg" width="40" height="40" alt="WebMemcached" title="WebMemcached" /></td></tr>
  <tr><th align="left">스토리지</th><td><img src="docs/assets/integrations/readme/s3.svg" width="40" height="40" alt="WebS3 (S3-compatible)" title="WebS3 (S3-compatible)" /></td></tr>
  <tr><th align="left">LLM 업스트림 프로토콜</th><td><picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/integrations/readme/openai-dark.svg" /><img src="docs/assets/integrations/readme/openai-light.svg" width="40" height="40" alt="OpenAI-compatible" title="OpenAI-compatible" /></picture>&nbsp;&nbsp; <img src="docs/assets/integrations/anthropic.svg" width="40" height="40" alt="Anthropic Messages" title="Anthropic Messages" />&nbsp;&nbsp; <picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/integrations/readme/ollama-dark.svg" /><img src="docs/assets/integrations/readme/ollama-light.svg" width="40" height="40" alt="Ollama" title="Ollama" /></picture></td></tr>
  <tr><th align="left">Web / TCP</th><td><img src="docs/assets/integrations/readme/web.svg" width="40" height="40" alt="HTTP / HTTPS / WebSocket" title="HTTP / HTTPS / WebSocket" />&nbsp;&nbsp; <img src="docs/assets/integrations/readme/tcp.svg" width="40" height="40" alt="TCP" title="TCP" /></td></tr>
</table>

LLM 업스트림은 **OpenAI-compatible**, **Anthropic Messages**, **Ollama**를 지원합니다. 클라이언트는 OpenAI-compatible 엔드포인트를 사용하며, Anthropic 업스트림에는 네이티브 Messages 엔드포인트도 제공됩니다. 로고는 접근 가능한 기존 서비스를 나타내며 Liaison에 포함되거나 Liaison이 배포하는 제품을 뜻하지 않습니다. 데이터베이스와 데스크톱 로고는 브라우저 작업 공간을 뜻하며 네이티브 데이터베이스/RDP/VNC 서버 리스너를 뜻하지 않습니다.

<a id="agent-model-providers"></a>

## Agent 모델 제공업체

Liaison 내장 Agent의 모델을 서비스 접근 설정과 별도로 구성합니다.

<table>
  <tr>
    <th align="left">모델</th>
    <td><picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/integrations/readme/openai-dark.svg" /><img src="docs/assets/integrations/readme/openai-light.svg" width="40" height="40" alt="OpenAI" title="OpenAI" /></picture>&nbsp;&nbsp; <img src="docs/assets/integrations/anthropic.svg" width="40" height="40" alt="Anthropic / Claude" title="Anthropic / Claude" />&nbsp;&nbsp; <img src="docs/assets/integrations/gemini.svg" width="40" height="40" alt="Google Gemini" title="Google Gemini" />&nbsp;&nbsp; <img src="docs/assets/integrations/deepseek.svg" width="40" height="40" alt="DeepSeek" title="DeepSeek" />&nbsp;&nbsp; <picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/integrations/readme/zhipu-dark.svg" /><img src="docs/assets/integrations/readme/zhipu-light.svg" width="40" height="40" alt="Z.ai / GLM" title="Z.ai / GLM" /></picture>&nbsp;&nbsp; <picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/integrations/readme/kimi-dark.svg" /><img src="docs/assets/integrations/readme/kimi-light.svg" width="40" height="40" alt="Moonshot / Kimi" title="Moonshot / Kimi" /></picture>&nbsp;&nbsp; <img src="docs/assets/integrations/minimax.svg" width="40" height="40" alt="MiniMax" title="MiniMax" />&nbsp;&nbsp; <picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/integrations/readme/mimo-dark.svg" /><img src="docs/assets/integrations/readme/mimo-light.svg" width="40" height="40" alt="Xiaomi MiMo" title="Xiaomi MiMo" /></picture></td>
  </tr>
</table>

8개 제공업체 프리셋과 사용자 지정 OpenAI-compatible 서비스를 지원합니다.

<a id="product-tour"></a>

## 제품 둘러보기

### 로컬 LLM 접근

![로컬 LLM 모델, 프로토콜, API 키 인증 설정 예시.](docs/assets/readme/local-llm-access-models-dark-v2.png)

### Web SSH

감사 기능이 있는 브라우저 터미널에서 작업합니다. Agent가 세션 출력을 확인하고 명령 작성을 도와줍니다.

![Liaison Web SSH](docs/assets/readme/web-ssh-dark.png)

### 비공개 웹 애플리케이션

Liaison을 통해 미디어 서버와 내부 웹 애플리케이션에 접근합니다.

![Liaison을 통한 Jellyfin](docs/pages/jellyfin-ss.png)

### 비공개 AI 어시스턴트

비공개 네트워크를 노출하지 않고 내부 AI 도구를 사용합니다.

![Liaison을 통한 OpenClaw](docs/pages/openclaw-ss.png)

### Web MySQL

스키마를 탐색하고 SQL을 실행하며, 세션 Agent에 조회와 결과 설명을 요청합니다. 필요한 작업은 승인을 받습니다.

![Liaison MySQL 작업 공간](docs/assets/readme/web-mysql-dark.png)

### Web MongoDB

로컬 클라이언트 없이 컬렉션을 탐색하고, Agent에 승인된 쿼리 실행과 문서 설명을 요청합니다.

![Liaison MongoDB 작업 공간](docs/assets/readme/web-mongodb-dark.png)

### Web VNC

관리되는 브라우저 세션에서 비공개 VNC 데스크톱을 엽니다.

![Liaison VNC 데스크톱](docs/assets/readme/web-vnc-dark.png)

### Web RDP

동일한 접근 절차로 RDP 데스크톱에 연결합니다.

![Liaison RDP 데스크톱](docs/assets/readme/web-rdp-dark-v2.png)

<a id="documentation"></a>

## 문서

- [Docker 배포](deploy/docker/README.md)
- [API 참조](docs/swagger/swagger.yaml)
- [릴리스 노트](https://github.com/liaisonio/liaison/releases)
- [이슈](https://github.com/liaisonio/liaison/issues)
- [토론](https://github.com/liaisonio/liaison/discussions)

<a id="contributing"></a>

## 기여

Liaison은 계속 발전하고 있습니다. 이 저장소의 셀프 호스팅 버전 개발에 함께해 주세요.

- AI Agent: 도구 통합, 세션 관리, 협업.
- 애플리케이션 접근: SSH, 파일, 데이터베이스, IDE, 원격 데스크톱, 모델 API.
- 크로스 플랫폼 지원: Linux, macOS, Windows 설치 및 호환성.
- 사용자 경험: UI 개선, 번역, 문서.

큰 변경은 먼저 Issue에서 범위를 논의해 주세요. PR은 하나의 주제에 집중하고 검증 방법을 설명해 주세요. 자격 증명, 개인 데이터, 특정 환경의 배포 정보는 포함하지 마세요. 채팅방 가입 없이 GitHub에서 바로 참여할 수 있습니다.

버그 제보, 기능 제안, 문서 개선, Pull Request를 환영합니다. [Issues](https://github.com/liaisonio/liaison/issues) 또는 [Pull Request](https://github.com/liaisonio/liaison/pulls)로 참여해 주세요.

<a id="license"></a>

## 라이선스

Liaison은 [GNU Affero General Public License v3.0](LICENSE)에 따라 배포됩니다.
