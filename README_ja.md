<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/assets/liaison-brand-rays-dark.svg" />
  <img src="docs/assets/liaison-brand-rays-light.svg" width="270" height="88" alt="Liaison" />
</picture>

> **プライベートなアプリ、LLM、AI Agent にアクセスするためのセルフホスト基盤。**

ブラウザワークスペース、権限を限定した API 共有、作業のコンテキストに応じた AI 支援を提供。コネクターからの発信接続でプライベートサービスにつながります。

[![CI](https://github.com/liaisonio/liaison/actions/workflows/go.yml/badge.svg)](https://github.com/liaisonio/liaison/actions/workflows/go.yml)
[![Release](https://img.shields.io/github/v/release/liaisonio/liaison?display_name=tag&sort=semver)](https://github.com/liaisonio/liaison/releases)
[![Downloads](https://img.shields.io/github/downloads/liaisonio/liaison/total)](https://github.com/liaisonio/liaison/releases)
[![License](https://img.shields.io/badge/License-AGPLv3-blue.svg?logo=gnu)](LICENSE)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](#contributing)
[![Go](https://img.shields.io/github/go-mod/go-version/liaisonio/liaison?logo=go&logoColor=white)](go.mod)
[![React](https://img.shields.io/badge/React-20232A?logo=react&logoColor=61DAFB)](web/package.json)
[![TypeScript](https://img.shields.io/badge/TypeScript-3178C6?logo=typescript&logoColor=white)](web/package.json)
[![Vite](https://img.shields.io/badge/Vite-646CFF?logo=vite&logoColor=white)](web/vite.config.ts)
[![Tailwind CSS](https://img.shields.io/badge/Tailwind_CSS-06B6D4?logo=tailwindcss&logoColor=white)](web/tailwind.config.cjs)

[English](./README.md) | [简体中文](./README_zh.md) | 日本語 | [한국어](./README_ko.md) | [Español](./README_es.md) | [Français](./README_fr.md) | [Deutsch](./README_de.md)

[公式サイト](https://liaison.cloud) · [ドキュメント](https://liaison.cloud/docs/get-started/introduction) · [機能](#features) · [インストール](#install) · [アクセスプロトコル](#supported-access-protocols) · [Agent モデル](#agent-model-providers) · [製品紹介](#product-tour)

![Liaison の製品デモ。サンプルデータを使用。](docs/assets/readme/liaison-product-demo-full-v2.gif)

<a id="features"></a>

## 機能

- 🤖 **ローカル LLM へのアクセス** — 認証付き API とブラウザの Playground でローカルモデルを共有。
- 🔑 **API 共有の制御** — キーごとのモデル制限、Token 上限の設定、アクセスの取り消し、使用量の確認。
- 🤖 **AI Agent へのアクセス** — デバイス上の Codex と Claude Code をブラウザで利用。セッション再開、実行状況、権限確認に対応。
- 💻 **ブラウザ IDE** — code-server でプロジェクトを編集し、ターミナルや拡張機能を利用。
- ✨ **作業を支える AI** — ユーザー権限の範囲内で端末出力を確認し、コマンド作成やデータベース照会を支援。必要な操作には承認を要求。
- 💬 **リソースを対話で確認** — アクセス権のあるコネクター、デバイス、アプリを検索。
- 🔌 **発信接続のみのコネクター** — デバイス側で受信ポートを開けずにプライベートデバイスへ接続。
- 🔐 **アプリケーションへのアクセス** — アクセスごとの制御で TCP、HTTP、HTTPS、WebSocket、SSH サービスを利用。
- 🖥️ **ブラウザワークスペース** — ターミナル、リモートデスクトップ、データベース、キャッシュをブラウザで操作。
- 📁 **ファイルとオブジェクト** — SFTP でファイルを管理し、S3 互換ストレージを閲覧（読み取り専用）。
- 🔎 **アプリケーションの検出** — コネクターのデバイスにある既存サービスを検出し、コンソールから登録。
- 👥 **ID とアクセス管理** — Casbin による認可でユーザーとリソースを管理。
- 🛡️ **ファイアウォールポリシー** — 送信元 IP または CIDR で TCP と HTTP のアクセスを制限。
- 📋 **ログと監査** — 管理操作と、対応するアプリケーションセッションを確認。
- 📦 **セルフホスト** — 完全なコントロールプレーンを自分の Linux サーバーで実行。

<a id="install"></a>

## インストール

自己完結型の Docker バンドルをダウンロード・展開し、インストーラーを実行します（Docker 20.10 以降と Compose が必要です）：

```bash
wget https://github.com/liaisonio/liaison/releases/download/v1.15.4/liaison-1.15.4-linux-amd64.tar.gz
tar -xzf liaison-1.15.4-linux-amd64.tar.gz
cd liaison-1.15.4-linux-amd64
./install.sh
```

インストール後に `https://<server-address>` を開きます。初期ログイン情報はインストーラーが出力します。

<a id="supported-access-protocols"></a>

## 対応アクセスプロトコル

Liaison を通じて既存のプライベートサービスにアクセスします。

<table>
  <tr><th align="left">SSH / SFTP</th><td><img src="docs/assets/integrations/readme/ssh.svg" width="40" height="40" alt="SSH / WebSSH" title="SSH / WebSSH" />&nbsp;&nbsp; <img src="docs/assets/integrations/readme/sftp.svg" width="40" height="40" alt="WebSFTP" title="WebSFTP" /></td></tr>
  <tr><th align="left">デスクトップ</th><td><img src="docs/assets/integrations/windows.svg" width="40" height="40" alt="WebRDP" title="WebRDP" />&nbsp;&nbsp; <img src="docs/assets/integrations/vnc.png" width="40" height="40" alt="WebVNC" title="WebVNC" /></td></tr>
  <tr><th align="left">SQL データベース</th><td><img src="docs/assets/integrations/mysql.svg" width="40" height="40" alt="WebMySQL" title="WebMySQL" />&nbsp;&nbsp; <img src="docs/assets/integrations/mariadb.svg" width="40" height="40" alt="WebMariaDB" title="WebMariaDB" />&nbsp;&nbsp; <img src="docs/assets/integrations/postgresql.svg" width="40" height="40" alt="WebPostgreSQL" title="WebPostgreSQL" />&nbsp;&nbsp; <img src="docs/assets/integrations/sqlserver.svg" width="40" height="40" alt="WebSQLServer" title="WebSQLServer" />&nbsp;&nbsp; <img src="docs/assets/integrations/oracle.svg" width="40" height="40" alt="WebOracle" title="WebOracle" />&nbsp;&nbsp; <img src="docs/assets/integrations/clickhouse.svg" width="40" height="40" alt="WebClickHouse" title="WebClickHouse" /></td></tr>
  <tr><th align="left">データと検索</th><td><img src="docs/assets/integrations/mongodb.svg" width="40" height="40" alt="WebMongoDB" title="WebMongoDB" />&nbsp;&nbsp; <img src="docs/assets/integrations/elasticsearch.svg" width="40" height="40" alt="WebElasticsearch" title="WebElasticsearch" />&nbsp;&nbsp; <img src="docs/assets/integrations/opensearch.svg" width="40" height="40" alt="WebOpenSearch" title="WebOpenSearch" /></td></tr>
  <tr><th align="left">キャッシュ</th><td><img src="docs/assets/integrations/redis.svg" width="40" height="40" alt="WebRedis" title="WebRedis" />&nbsp;&nbsp; <img src="docs/assets/integrations/memcached.svg" width="40" height="40" alt="WebMemcached" title="WebMemcached" /></td></tr>
  <tr><th align="left">ストレージ</th><td><img src="docs/assets/integrations/readme/s3.svg" width="40" height="40" alt="WebS3 (S3-compatible)" title="WebS3 (S3-compatible)" /></td></tr>
  <tr><th align="left">LLM 上流プロトコル</th><td><picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/integrations/readme/openai-dark.svg" /><img src="docs/assets/integrations/readme/openai-light.svg" width="40" height="40" alt="OpenAI-compatible" title="OpenAI-compatible" /></picture>&nbsp;&nbsp; <img src="docs/assets/integrations/anthropic.svg" width="40" height="40" alt="Anthropic Messages" title="Anthropic Messages" />&nbsp;&nbsp; <picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/integrations/readme/ollama-dark.svg" /><img src="docs/assets/integrations/readme/ollama-light.svg" width="40" height="40" alt="Ollama" title="Ollama" /></picture></td></tr>
  <tr><th align="left">Web / TCP</th><td><img src="docs/assets/integrations/readme/web.svg" width="40" height="40" alt="HTTP / HTTPS / WebSocket" title="HTTP / HTTPS / WebSocket" />&nbsp;&nbsp; <img src="docs/assets/integrations/readme/tcp.svg" width="40" height="40" alt="TCP" title="TCP" /></td></tr>
</table>

LLM の上流プロトコルは **OpenAI-compatible**、**Anthropic Messages**、**Ollama** に対応。クライアントは OpenAI-compatible エンドポイントを使用でき、Anthropic の上流にはネイティブの Messages エンドポイントも用意されています。ロゴはアクセス可能な既存サービスを示し、Liaison が同梱・導入する製品を意味しません。データベースとデスクトップのロゴはブラウザワークスペースを示し、ネイティブのデータベース・RDP・VNC サーバーの待受機能を意味しません。

<a id="agent-model-providers"></a>

## Agent のモデルプロバイダー

Liaison 内蔵 Agent のモデルは、サービスへのアクセス設定とは独立して構成できます。

<table>
  <tr>
    <th align="left">モデル</th>
    <td><picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/integrations/readme/openai-dark.svg" /><img src="docs/assets/integrations/readme/openai-light.svg" width="40" height="40" alt="OpenAI" title="OpenAI" /></picture>&nbsp;&nbsp; <img src="docs/assets/integrations/anthropic.svg" width="40" height="40" alt="Anthropic / Claude" title="Anthropic / Claude" />&nbsp;&nbsp; <img src="docs/assets/integrations/gemini.svg" width="40" height="40" alt="Google Gemini" title="Google Gemini" />&nbsp;&nbsp; <img src="docs/assets/integrations/deepseek.svg" width="40" height="40" alt="DeepSeek" title="DeepSeek" />&nbsp;&nbsp; <picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/integrations/readme/zhipu-dark.svg" /><img src="docs/assets/integrations/readme/zhipu-light.svg" width="40" height="40" alt="Z.ai / GLM" title="Z.ai / GLM" /></picture>&nbsp;&nbsp; <picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/integrations/readme/kimi-dark.svg" /><img src="docs/assets/integrations/readme/kimi-light.svg" width="40" height="40" alt="Moonshot / Kimi" title="Moonshot / Kimi" /></picture>&nbsp;&nbsp; <img src="docs/assets/integrations/minimax.svg" width="40" height="40" alt="MiniMax" title="MiniMax" />&nbsp;&nbsp; <picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/integrations/readme/mimo-dark.svg" /><img src="docs/assets/integrations/readme/mimo-light.svg" width="40" height="40" alt="Xiaomi MiMo" title="Xiaomi MiMo" /></picture></td>
  </tr>
</table>

8 種類のプロバイダープリセットと、カスタムの OpenAI-compatible サービスに対応。

<a id="product-tour"></a>

## 製品紹介

### ローカル LLM へのアクセス

![ローカル LLM のモデルとプロトコル、API キー認証の設定例。](docs/assets/readme/local-llm-access-models-dark-v2.png)

### Web SSH

監査に対応したブラウザターミナルで作業。Agent がセッション出力を確認し、コマンド作成を支援します。

![Liaison の Web SSH](docs/assets/readme/web-ssh-dark.png)

### プライベート Web アプリ

メディアサーバーや社内 Web アプリに Liaison 経由でアクセスします。

![Liaison 経由の Jellyfin](docs/pages/jellyfin-ss.png)

### プライベート AI アシスタント

プライベートネットワークを公開せず、内部の AI ツールを利用できます。

![Liaison 経由の OpenClaw](docs/pages/openclaw-ss.png)

### Web MySQL

スキーマの参照や SQL の実行に加え、セッション Agent に照会と結果の説明を依頼できます。必要な操作には承認を要求します。

![Liaison の MySQL ワークスペース](docs/assets/readme/web-mysql-dark.png)

### Web MongoDB

ローカルクライアントなしでコレクションを参照し、Agent に承認済みクエリの実行やドキュメントの説明を依頼できます。

![Liaison の MongoDB ワークスペース](docs/assets/readme/web-mongodb-dark.png)

### Web VNC

管理されたブラウザセッションでプライベート VNC デスクトップを開きます。

![Liaison の VNC デスクトップ](docs/assets/readme/web-vnc-dark.png)

### Web RDP

同じアクセス手順で RDP デスクトップに接続します。

![Liaison の RDP デスクトップ](docs/assets/readme/web-rdp-dark-v2.png)

<a id="documentation"></a>

## ドキュメント

- [Docker デプロイ](deploy/docker/README.md)
- [API リファレンス](docs/swagger/swagger.yaml)
- [リリースノート](https://github.com/liaisonio/liaison/releases)
- [Issue](https://github.com/liaisonio/liaison/issues)
- [ディスカッション](https://github.com/liaisonio/liaison/discussions)

<a id="contributing"></a>

## コントリビュート

Liaison は継続的に進化しています。このリポジトリのセルフホスト版の開発にぜひご参加ください。

- AI Agent：ツール連携、セッション管理、協調作業。
- アプリケーションアクセス：SSH、ファイル、データベース、IDE、リモートデスクトップ、モデル API。
- クロスプラットフォーム対応：Linux、macOS、Windows のインストールと互換性。
- 使いやすさ：UI 改善、翻訳、ドキュメント。

大きな変更は、まず Issue で範囲を相談してください。PR は一つのテーマに絞り、検証方法を記載してください。認証情報、個人データ、特定環境のデプロイ情報は含めないでください。チャットグループへの参加は任意で、GitHub だけで貢献できます。

バグ報告、機能提案、ドキュメント改善、Pull Request を歓迎します。[Issues](https://github.com/liaisonio/liaison/issues) または [Pull Request](https://github.com/liaisonio/liaison/pulls) からご参加ください。

<a id="license"></a>

## ライセンス

Liaison は [GNU Affero General Public License v3.0](LICENSE) で公開されています。
