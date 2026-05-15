# <img src="docs/diagrams/liaison-logo.svg" height="40" align="absmiddle" alt="" /> Liaison

> **NAT の内側にあるデバイスとアプリケーションへ、コネクター経由でアクセス**

[![Go](https://github.com/liaisonio/liaison/actions/workflows/go.yml/badge.svg)](https://github.com/liaisonio/liaison/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/liaisonio/liaison)](https://goreportcard.com/report/github.com/liaisonio/liaison)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Tech](https://img.shields.io/badge/Tech-Go%20%7C%20TypeScript%20%7C%20React-blue)](#)
[![Version](https://img.shields.io/badge/Version-v1.8.0-green)](#)

[简体中文](./README.md) | [English](./README_en.md) | 日本語 | [한국어](./README_ko.md) | [Español](./README_es.md) | [Français](./README_fr.md) | [Deutsch](./README_de.md)

![Dashboard](docs/pages/home_en.png)

| Jellyfin(どこからでも家の映画をストリーミング) | OpenClaw(どこからでも家の AI を利用) |
|:---:|:---:|
| ![Jellyfin](docs/pages/jellyfin-ss.png) | ![OpenClaw](docs/pages/openclaw-ss.png) |

[クイックスタート](#クイックスタート) • [概要](#概要) • [ドキュメント](#ドキュメント) • [コントリビュート](#コントリビュート)

---

## 概要

Liaison はエンタープライズ向けのアプリケーション接続ソリューションで、いつでも有効化・無効化でき、LAN や家庭ネットワークのポートを公開する必要がありません。接続済みデバイスのアプリを自動検出し、リアルタイムのトラフィックメトリクスを取得し、TLS 暗号化された安全な通信を提供します。

本プロジェクトが解決する課題:

- **プライベートネットワークへのアクセス** — NAT 配下のデバイスやサービスに、最小限の設定でインターネット経由から到達
- **マルチデバイス管理** — Linux/macOS/Windows をまたいで複数拠点のデバイスを一元管理
- **セキュアな接続** — LAN や家庭ネットワークのポートを晒さずに TLS 暗号化で通信
- **エントリー単位のファイアウォール** — TCP / HTTP 各エントリーに送信元 IP CIDR の許可リストを設定でき、接続受け入れ段階で遮断
- **トラフィックモニタリング** — 運用・キャパシティプランニング向けのリアルタイムなデバイス状態とトラフィック統計
- **アプリケーションプロキシ** — TCP、HTTP/HTTPS、WebSocket などのプロトコル対応
- **API の自動化** — CLI / スクリプト用の Personal Access Token (PAT)、`/cli-auth` でブラウザ経由のサインインフロー

ユースケース:

<div align="center">

| **💼 リモートワーク & 開発** | **🧑‍💻 個人スタジオ** | **🏠 ホームネット / NAS** | **🌐 マルチリージョン** | **⚡ エッジ & 運用** |
|:---:|:---:|:---:|:---:|:---:|
| オフィスと自宅のデバイスを接続し、リモート開発・デバッグ | ワークステーションと私的な環境をセキュアに接続し、機材を統合管理 | 家庭の NAS やスマートホームをインターネットから利用 | 複数リージョン・DC をまたぐサーバーとアプリケーションの一元接続 | エッジアプリのリモート健全性確認とトラフィック監視 |

</div>

---

## クイックスタート

2 つのサーバー導入方式からいずれかを選び、その後にコネクターを導入します。

### サーバー導入 — オプション 1: バイナリ + systemd

**1. ダウンロード**

```bash
wget https://github.com/liaisonio/liaison/releases/download/v1.8.0/liaison-1.8.0-linux-amd64.tar.gz
tar -xzf liaison-1.8.0-linux-amd64.tar.gz
cd liaison-1.8.0-linux-amd64
```

**2. インストールスクリプトを実行**

```bash
sudo ./install.sh
```

公開 IP またはドメインの入力が求められます。30 秒以内に入力しない場合は検出された公開 IP が自動で使用されます。

**3. Web コンソールを開く**

`https://公開IP` にアクセスして Web コンソールへ。

> **ヒント:** 初期管理者の認証情報は install.sh の出力、あるいは設定ファイルに記載されます。

### サーバー導入 — オプション 2: Docker Compose

Docker 20.10+ と `docker compose` プラグインが必要です。`liaison`(Web コンソール + API)と `frontier`(コネクターゲートウェイ)の 2 コンテナを提供し、イメージは同梱済みのため、レジストリからの pull やソースの取得は不要です。

```bash
wget https://github.com/liaisonio/liaison/releases/download/v1.8.0/liaison-1.8.0-docker-amd64.tar.gz
tar -xzf liaison-1.8.0-docker-amd64.tar.gz
cd liaison-1.8.0-docker-amd64
./load.sh
```

`load.sh` は公開 IP を自動検出(30 秒カウントダウンの確認付き)し、イメージをロードしてスタックを起動、liaison の準備ができ次第、一度限りの管理者パスワードを表示します。パスワードを控えた上で `https://<公開IP>` からログインしてください。

データ(`data/` の SQLite)、TLS 証明書(`certs/`)、ログ(`logs/`)は永続化のため `docker-compose.yaml` と同階層に bind mount されます。ソースビルドやアップグレード / 初期化 / リバースプロキシ / カスタム証明書などの詳細は [`deploy/docker/README.md`](deploy/docker/README.md) を参照してください。

### コネクター導入

ターゲット端末に応じて 2 つのインストール経路から選択してください。

#### オプション A — Liaison Desktop (GUI、macOS / Windows)

コネクターをラップするメニューバー / トレイアプリ。ワンクリックサインイン、ステータス表示、一時停止 / 再開、ダッシュボードへのワンクリックアクセスを提供します。ノート PC やワークステーションに最適。

<div align="center">

| macOS | Windows |
|:---:|:---:|
| <img src="docs/images/desktop-client/popup-macos.png" alt="Liaison Desktop on macOS" width="360" /> | <img src="docs/images/desktop-client/popup-windows.png" alt="Liaison Desktop on Windows" width="360" /> |

</div>

- **ワンクリックサインイン** — ブラウザ仲介の OAuth フロー、PAT は OS のキーチェーン (macOS Keychain、Windows 資格情報マネージャー) に保存
- **マルチデプロイ対応** — デフォルトは `liaison.cloud`、左下の歯車アイコンから任意のプライベートデプロイへ即座に切替可能（再インストール不要）
- **ハートビート連動のステータス** — 接続中 → オンライン遷移は実際のトンネル状態を反映 (プロセス生存だけではない)
- **再起動を跨いで保持される一時停止** — ユーザー意図がディスクに永続化され、再起動後も Paused のまま

**ダウンロード (ローリングプレリリース、`feat/desktop-client` の最新ビルド)：**

| プラットフォーム | ファイル |
|:---|:---|
| macOS (Apple Silicon + Intel ユニバーサル) | [`Liaison_0.1.0_universal.dmg`](https://github.com/liaisonio/liaison/releases/download/desktop-latest/Liaison_0.1.0_universal.dmg) |
| Windows (.msi インストーラー) | [`Liaison_0.1.0_x64_en-US.msi`](https://github.com/liaisonio/liaison/releases/download/desktop-latest/Liaison_0.1.0_x64_en-US.msi) |
| Windows (.exe NSIS、アンインストール時にキーチェーンクリーンアップ) | [`Liaison_0.1.0_x64-setup.exe`](https://github.com/liaisonio/liaison/releases/download/desktop-latest/Liaison_0.1.0_x64-setup.exe) |

> v0.1 のインストーラーは未署名です。macOS Gatekeeper と Windows SmartScreen が初回起動時に警告を出します — macOS は右クリック → 開く、Windows は「詳細情報」→「実行」を選択してください。Windows には WebView2 Runtime が必要です (Win10 1803+ と Win11 にプリインストール済み)。

#### オプション B — CLI インストールコマンド (Linux / ヘッドレス)

Web コンソールで **コネクターを新規作成** し、UI から対象プラットフォーム用のインストールコマンドをコピーして対象デバイスで実行すると、コネクターがコンソールに自動で現れます。

---

## 動作要件

| コンポーネント | 要件 |
|:---|:---|
| **サーバー** | Linux(Ubuntu 20.04+ または CentOS 7+ 推奨) |
| **コネクター** | Linux / macOS / Windows(x86_64 および ARM64) |
| **ブラウザ** | Chrome 90+、Firefox 88+、Safari 14+、Edge 90+ |

---

## アーキテクチャ

<img src="./docs/diagrams/liaison.png" width="80%">

Liaison は Frontier ですべてのコネクターを管理する中央集権型アーキテクチャを採用しています。

**コンポーネント**

- **Liaison** — Web UI と API、アプリケーションのエントリーポイント
- **Frontier** — コネクターの接続とトラフィックルーティングを担うゲートウェイ
- **Edge** — 対象デバイス上で動作するコネクタークライアント

---

## 機能紹介

| 機能 | スクリーンショット |
|:---:|:---:|
| デバイス管理 | ![Device](docs/pages/device_en.png) |
| アプリケーション管理 | ![Application](docs/pages/application_en.png) |
| プロキシ設定 | ![Proxy](docs/pages/proxy_en.png) |
| コネクター管理 | ![Edge](docs/pages/edge_en.png) |

---

## ドキュメント

- [ビジネスフロー](./docs/biz_sequence.md)
- [API](./docs/swagger/)

---

## コントリビュート

コントリビュート歓迎です。

- [バグ報告](https://github.com/liaisonio/liaison/issues/new?template=bug_report.md)
- [機能提案](https://github.com/liaisonio/liaison/issues/new?template=feature_request.md)
- [PR を開く](https://github.com/liaisonio/liaison/pulls)
- [ドキュメント改善](https://github.com/liaisonio/liaison/issues/new?template=documentation.md)

1. リポジトリを Fork
2. ブランチを作成(`git checkout -b feature/AmazingFeature`)
3. コミット(`git commit -m 'Add some AmazingFeature'`)
4. Push(`git push origin feature/AmazingFeature`)
5. Pull Request を開く

---

## ライセンス

[Apache License 2.0](LICENSE)

---

<div align="center">

**プロジェクトが役立ったら ⭐ Star をお願いします!**

Made with ❤️ by [Liaison Contributors](https://github.com/liaisonio/liaison/graphs/contributors)

[GitHub](https://github.com/liaisonio/liaison) • [Issues](https://github.com/liaisonio/liaison/issues) • [Discussions](https://github.com/liaisonio/liaison/discussions)

</div>
