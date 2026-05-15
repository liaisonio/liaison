# <img src="docs/diagrams/liaison-logo.svg" height="40" align="absmiddle" alt="" /> Liaison

> **Accès par connecteur aux équipements et applications derrière NAT**

[![Go](https://github.com/liaisonio/liaison/actions/workflows/go.yml/badge.svg)](https://github.com/liaisonio/liaison/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/liaisonio/liaison)](https://goreportcard.com/report/github.com/liaisonio/liaison)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Tech](https://img.shields.io/badge/Tech-Go%20%7C%20TypeScript%20%7C%20React-blue)](#)
[![Version](https://img.shields.io/badge/Version-v1.8.0-green)](#)

[简体中文](./README.md) | [English](./README_en.md) | [日本語](./README_ja.md) | [한국어](./README_ko.md) | [Español](./README_es.md) | Français | [Deutsch](./README_de.md)

![Dashboard](docs/pages/home_en.png)

| Jellyfin (diffuser vos films maison partout) | OpenClaw (utiliser votre IA maison partout) |
|:---:|:---:|
| ![Jellyfin](docs/pages/jellyfin-ss.png) | ![OpenClaw](docs/pages/openclaw-ss.png) |

[Démarrage rapide](#démarrage-rapide) • [Introduction](#introduction) • [Documentation](#documentation) • [Contribuer](#contribuer)

---

## Introduction

Liaison est une solution d'accès applicatif de niveau entreprise, que l'on peut activer ou désactiver à tout moment, sans exposer de ports sur votre LAN ou votre réseau domestique. Elle fournit un ensemble complet de fonctionnalités : découverte automatique des applications sur les équipements connectés, métriques de trafic en temps réel et transport chiffré TLS.

Ce projet adresse :

- **Accès au réseau privé** — Atteindre les équipements et services derrière NAT depuis l'Internet public avec un minimum de configuration
- **Gestion multi-équipements** — Administrer des équipements répartis sur plusieurs sites, avec support Linux/macOS/Windows
- **Connectivité sécurisée** — Transport chiffré TLS sans exposer de ports sur le LAN ou le réseau domestique
- **Pare-feu par entrée** — Liste blanche CIDR d'IP source sur chaque entrée TCP ou HTTP, appliquée à l'acceptation de la connexion
- **Supervision du trafic** — État des équipements et métriques de trafic en temps réel, pour l'exploitation et la planification de capacité
- **Proxy applicatif** — TCP, HTTP/HTTPS, WebSocket et d'autres protocoles
- **Automatisation de l'API** — Personal Access Tokens (PAT) pour CLI / scripts, avec un flux de connexion via navigateur sur `/cli-auth`

Cas d'usage :

<div align="center">

| **💼 Télétravail & Dev** | **🧑‍💻 Studio personnel** | **🏠 Réseau domestique / NAS** | **🌐 Multi-DC / multi-région** | **⚡ Edge & Ops** |
|:---:|:---:|:---:|:---:|:---:|
| Connecter les équipements bureau et domicile pour développement et débogage à distance | Connecter de manière sécurisée postes de travail et environnements privés avec gestion unifiée | Accéder au NAS domestique et aux services maison depuis Internet | Connectivité unifiée entre serveurs et applications de différentes régions et DC | Connecter et superviser les applications edge avec contrôles à distance |

</div>

---

## Démarrage rapide

Choisissez l'une des deux options de déploiement serveur, puis installez un connecteur.

### Installer le serveur — Option 1 : Binaire + systemd

**1. Télécharger**

```bash
wget https://github.com/liaisonio/liaison/releases/download/v1.8.0/liaison-1.8.0-linux-amd64.tar.gz
tar -xzf liaison-1.8.0-linux-amd64.tar.gz
cd liaison-1.8.0-linux-amd64
```

**2. Exécuter le script d'installation**

```bash
sudo ./install.sh
```

L'IP publique ou le domaine vous seront demandés ; sans saisie dans les 30 secondes, l'IP publique détectée est utilisée.

**3. Ouvrir la console Web**

Visitez `https://votre-ip-publique` pour accéder à la console Web.

> **Astuce :** Les identifiants admin par défaut apparaissent dans la sortie de install.sh ou dans la configuration.

### Installer le serveur — Option 2 : Docker Compose

Nécessite Docker 20.10+ et le plugin `docker compose`. Le bundle fournit `liaison` (console Web + API) et `frontier` (passerelle des connecteurs) en deux conteneurs ; les images sont pré-construites — aucun registre ou checkout des sources n'est requis.

```bash
wget https://github.com/liaisonio/liaison/releases/download/v1.8.0/liaison-1.8.0-docker-amd64.tar.gz
tar -xzf liaison-1.8.0-docker-amd64.tar.gz
cd liaison-1.8.0-docker-amd64
./load.sh
```

`load.sh` détecte automatiquement votre IP publique (avec une confirmation de 30 secondes), charge les images, démarre la stack et imprime le mot de passe admin à usage unique dès que liaison est prêt. Enregistrez ce mot de passe, puis ouvrez `https://<ip-publique>` pour vous connecter.

Les données (`data/` SQLite), les certificats TLS (`certs/`) et les logs (`logs/`) sont montés en bind à côté de `docker-compose.yaml` pour la persistance. Voir [`deploy/docker/README.md`](deploy/docker/README.md) pour la compilation depuis les sources, mise à jour / reset / reverse proxy / certificat personnalisé.

### Installer un connecteur

Deux chemins d'installation, choisissez celui adapté à la machine cible.

#### Option A — Liaison Desktop (GUI, macOS / Windows)

Application de barre de menus / barre d'état système qui encapsule le connecteur et offre une connexion en un clic, un indicateur d'état, pause / reprise et un accès au tableau de bord en un clic. Idéal pour les ordinateurs portables et postes de travail.

<div align="center">

| macOS | Windows |
|:---:|:---:|
| <img src="docs/images/desktop-client/popup-macos.png" alt="Liaison Desktop on macOS" width="360" /> | <img src="docs/images/desktop-client/popup-windows.png" alt="Liaison Desktop on Windows" width="360" /> |

</div>

- **Connexion en un clic** — flux OAuth via navigateur, PAT stocké dans le trousseau du système (Keychain sur macOS, Gestionnaire d'identifiants sur Windows)
- **Multi-déploiement** — par défaut sur `liaison.cloud` ; l'icône d'engrenage en bas à gauche permet de basculer vers n'importe quel déploiement privé sans réinstaller
- **État sensible au heartbeat** — les transitions Connexion → En ligne reflètent l'état réel du tunnel, pas seulement la vivacité du processus
- **Pause qui survit à la fermeture** — l'intention est persistée sur disque, une session en pause le reste après relance

**Télécharger (pré-release roulante, dernier build de `feat/desktop-client`) :**

| Plateforme | Fichier |
|:---|:---|
| macOS (Apple Silicon + Intel, universel) | [`Liaison_0.1.0_universal.dmg`](https://github.com/liaisonio/liaison/releases/download/desktop-latest/Liaison_0.1.0_universal.dmg) |
| Windows (installateur .msi) | [`Liaison_0.1.0_x64_en-US.msi`](https://github.com/liaisonio/liaison/releases/download/desktop-latest/Liaison_0.1.0_x64_en-US.msi) |
| Windows (.exe NSIS, avec nettoyage du trousseau à la désinstallation) | [`Liaison_0.1.0_x64-setup.exe`](https://github.com/liaisonio/liaison/releases/download/desktop-latest/Liaison_0.1.0_x64-setup.exe) |

> Les installateurs v0.1 ne sont pas signés. Gatekeeper macOS et Windows SmartScreen avertiront au premier lancement — clic droit → Ouvrir sur macOS, ou « Informations supplémentaires » → « Exécuter quand même » sur Windows. WebView2 Runtime est requis sur Windows ; Win10 1803+ et Win11 l'incluent.

#### Option B — Commande d'installation CLI (Linux / headless)

**Créez un nouveau connecteur** dans la console Web, copiez la commande d'installation correspondant à votre plateforme depuis l'interface et exécutez-la sur la machine cible. Le connecteur apparaîtra automatiquement dans la console.

---

## Prérequis

| Composant | Exigences |
|:---|:---|
| **Serveur** | Linux (Ubuntu 20.04+ ou CentOS 7+ recommandés) |
| **Connecteur** | Linux / macOS / Windows (x86_64 et ARM64) |
| **Navigateur** | Chrome 90+, Firefox 88+, Safari 14+, Edge 90+ |

---

## Architecture

<img src="./docs/diagrams/liaison.png" width="80%">

Liaison utilise une architecture centralisée avec Frontier qui gère tous les connecteurs.

**Composants**

- **Liaison** — UI Web et API, plus les points d'entrée applicatifs
- **Frontier** — Passerelle des connecteurs, gère les connexions et le routage du trafic
- **Edge** — Client connecteur déployé sur les machines cibles

---

## Fonctionnalités

| Fonction | Capture |
|:---:|:---:|
| Gestion des équipements | ![Device](docs/pages/device_en.png) |
| Gestion des applications | ![Application](docs/pages/application_en.png) |
| Configuration des proxys | ![Proxy](docs/pages/proxy_en.png) |
| Gestion des connecteurs | ![Edge](docs/pages/edge_en.png) |

---

## Documentation

- [Flux métier](./docs/biz_sequence.md)
- [API](./docs/swagger/)

---

## Contribuer

Les contributions sont bienvenues.

- [Signaler un bug](https://github.com/liaisonio/liaison/issues/new?template=bug_report.md)
- [Proposer une fonctionnalité](https://github.com/liaisonio/liaison/issues/new?template=feature_request.md)
- [Ouvrir une PR](https://github.com/liaisonio/liaison/pulls)
- [Améliorer la doc](https://github.com/liaisonio/liaison/issues/new?template=documentation.md)

1. Fork du dépôt
2. Créer une branche (`git checkout -b feature/AmazingFeature`)
3. Commit (`git commit -m 'Add some AmazingFeature'`)
4. Push (`git push origin feature/AmazingFeature`)
5. Ouvrir une Pull Request

---

## Licence

[Apache License 2.0](LICENSE).

---

<div align="center">

**Si ce projet vous aide, laissez une ⭐ Star !**

Made with ❤️ by [Liaison Contributors](https://github.com/liaisonio/liaison/graphs/contributors)

[GitHub](https://github.com/liaisonio/liaison) • [Issues](https://github.com/liaisonio/liaison/issues) • [Discussions](https://github.com/liaisonio/liaison/discussions)

</div>
