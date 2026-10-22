<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/assets/liaison-brand-rays-dark.svg" />
  <img src="docs/assets/liaison-brand-rays-light.svg" width="270" height="88" alt="Liaison" />
</picture>

> **Accès auto-hébergé à vos applications privées, LLM et agents IA.**

Espaces de travail dans le navigateur, partage d’API à portée limitée et assistance IA contextuelle, avec des connecteurs qui établissent uniquement des connexions sortantes depuis vos services privés.

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

[English](./README.md) | [简体中文](./README_zh.md) | [日本語](./README_ja.md) | [한국어](./README_ko.md) | [Español](./README_es.md) | Français | [Deutsch](./README_de.md)

[Site web](https://liaison.cloud) · [Documentation](https://liaison.cloud/docs/get-started/introduction) · [Fonctionnalités](#features) · [Installation](#install) · [Protocoles d’accès](#supported-access-protocols) · [Modèles de l’Agent](#agent-model-providers) · [Présentation](#product-tour)

![Démonstration de Liaison avec des données d’exemple.](docs/assets/readme/liaison-product-demo-full-v2.gif)

<a id="features"></a>

## Fonctionnalités

- 🤖 **Accès aux LLM locaux** — Partagez des modèles locaux via des API authentifiées et un Playground dans le navigateur.
- 🔑 **Partage d’API contrôlé** — Limitez les clés à certains modèles, définissez des quotas de tokens, révoquez l’accès et suivez l’utilisation.
- 🤖 **Accès aux agents IA** — Utilisez Codex et Claude Code sur vos appareils depuis le navigateur, avec reprise des sessions, suivi de l’exécution et demandes d’autorisation.
- 💻 **IDE dans le navigateur** — Modifiez vos projets et utilisez terminaux et extensions avec code-server.
- ✨ **IA dans votre travail** — Examinez les sorties du terminal, préparez des commandes et interrogez des bases de données dans la limite des droits de l’utilisateur, avec approbation si nécessaire.
- 💬 **Recherche de ressources** — Retrouvez les connecteurs, appareils et applications auxquels vous avez accès.
- 🔌 **Connecteurs à connexions sortantes** — Connectez vos appareils privés sans y ouvrir de ports entrants.
- 🔐 **Accès aux applications** — Accédez aux services TCP, HTTP, HTTPS, WebSocket et SSH avec des contrôles propres à chaque accès.
- 🖥️ **Espaces de travail dans le navigateur** — Utilisez terminaux, bureaux distants, bases de données et caches depuis votre navigateur.
- 📁 **Fichiers et objets** — Gérez les fichiers via SFTP et parcourez le stockage compatible S3 (en lecture seule).
- 🔎 **Détection d’applications** — Détectez les services existants sur les appareils dotés d’un connecteur et enregistrez-les depuis la console.
- 👥 **Gestion des identités et des accès** — Gérez utilisateurs et ressources avec une autorisation fondée sur Casbin.
- 🛡️ **Politiques de pare-feu** — Restreignez les accès TCP et HTTP par IP source ou CIDR.
- 📋 **Journaux et audit** — Consultez les actions d’administration et les sessions des applications prises en charge.
- 📦 **Auto-hébergement** — Exécutez l’intégralité du plan de contrôle sur votre propre serveur Linux.

<a id="install"></a>

## Installation

Téléchargez l’archive Docker autonome, décompressez-la et lancez l’installateur (Docker 20.10 ou ultérieur avec Compose est requis) :

```bash
wget https://github.com/liaisonio/liaison/releases/download/v1.15.4/liaison-1.15.4-linux-amd64.tar.gz
tar -xzf liaison-1.15.4-linux-amd64.tar.gz
cd liaison-1.15.4-linux-amd64
./install.sh
```

Après l’installation, ouvrez `https://<server-address>`. L’installateur affiche les identifiants de connexion initiaux.

<a id="supported-access-protocols"></a>

## Protocoles d’accès pris en charge

Accédez à vos services privés existants via Liaison.

<table>
  <tr><th align="left">SSH / SFTP</th><td><img src="docs/assets/integrations/readme/ssh.svg" width="40" height="40" alt="SSH / WebSSH" title="SSH / WebSSH" />&nbsp;&nbsp; <img src="docs/assets/integrations/readme/sftp.svg" width="40" height="40" alt="WebSFTP" title="WebSFTP" /></td></tr>
  <tr><th align="left">Bureau</th><td><img src="docs/assets/integrations/windows.svg" width="40" height="40" alt="WebRDP" title="WebRDP" />&nbsp;&nbsp; <img src="docs/assets/integrations/vnc.png" width="40" height="40" alt="WebVNC" title="WebVNC" /></td></tr>
  <tr><th align="left">Bases de données SQL</th><td><img src="docs/assets/integrations/mysql.svg" width="40" height="40" alt="WebMySQL" title="WebMySQL" />&nbsp;&nbsp; <img src="docs/assets/integrations/mariadb.svg" width="40" height="40" alt="WebMariaDB" title="WebMariaDB" />&nbsp;&nbsp; <img src="docs/assets/integrations/postgresql.svg" width="40" height="40" alt="WebPostgreSQL" title="WebPostgreSQL" />&nbsp;&nbsp; <img src="docs/assets/integrations/sqlserver.svg" width="40" height="40" alt="WebSQLServer" title="WebSQLServer" />&nbsp;&nbsp; <img src="docs/assets/integrations/oracle.svg" width="40" height="40" alt="WebOracle" title="WebOracle" />&nbsp;&nbsp; <img src="docs/assets/integrations/clickhouse.svg" width="40" height="40" alt="WebClickHouse" title="WebClickHouse" /></td></tr>
  <tr><th align="left">Données et recherche</th><td><img src="docs/assets/integrations/mongodb.svg" width="40" height="40" alt="WebMongoDB" title="WebMongoDB" />&nbsp;&nbsp; <img src="docs/assets/integrations/elasticsearch.svg" width="40" height="40" alt="WebElasticsearch" title="WebElasticsearch" />&nbsp;&nbsp; <img src="docs/assets/integrations/opensearch.svg" width="40" height="40" alt="WebOpenSearch" title="WebOpenSearch" /></td></tr>
  <tr><th align="left">Cache</th><td><img src="docs/assets/integrations/redis.svg" width="40" height="40" alt="WebRedis" title="WebRedis" />&nbsp;&nbsp; <img src="docs/assets/integrations/memcached.svg" width="40" height="40" alt="WebMemcached" title="WebMemcached" /></td></tr>
  <tr><th align="left">Stockage</th><td><img src="docs/assets/integrations/readme/s3.svg" width="40" height="40" alt="WebS3 (S3-compatible)" title="WebS3 (S3-compatible)" /></td></tr>
  <tr><th align="left">Protocoles LLM en amont</th><td><picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/integrations/readme/openai-dark.svg" /><img src="docs/assets/integrations/readme/openai-light.svg" width="40" height="40" alt="OpenAI-compatible" title="OpenAI-compatible" /></picture>&nbsp;&nbsp; <img src="docs/assets/integrations/anthropic.svg" width="40" height="40" alt="Anthropic Messages" title="Anthropic Messages" />&nbsp;&nbsp; <picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/integrations/readme/ollama-dark.svg" /><img src="docs/assets/integrations/readme/ollama-light.svg" width="40" height="40" alt="Ollama" title="Ollama" /></picture></td></tr>
  <tr><th align="left">Web / TCP</th><td><img src="docs/assets/integrations/readme/web.svg" width="40" height="40" alt="HTTP / HTTPS / WebSocket" title="HTTP / HTTPS / WebSocket" />&nbsp;&nbsp; <img src="docs/assets/integrations/readme/tcp.svg" width="40" height="40" alt="TCP" title="TCP" /></td></tr>
</table>

Les services LLM en amont prennent en charge **OpenAI-compatible**, **Anthropic Messages** et **Ollama**. Les clients utilisent des endpoints OpenAI-compatible ; les services Anthropic exposent aussi des endpoints Messages natifs. Les logos désignent des services existants auxquels vous pouvez accéder, et non des produits inclus ou déployés par Liaison. Les logos de bases de données et de bureaux désignent les espaces de travail du navigateur, et non des serveurs natifs de bases de données, RDP ou VNC en écoute.

<a id="agent-model-providers"></a>

## Fournisseurs de modèles de l’Agent

Configurez le modèle de l’Agent intégré à Liaison indépendamment de l’accès aux services.

<table>
  <tr>
    <th align="left">Modèles</th>
    <td><picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/integrations/readme/openai-dark.svg" /><img src="docs/assets/integrations/readme/openai-light.svg" width="40" height="40" alt="OpenAI" title="OpenAI" /></picture>&nbsp;&nbsp; <img src="docs/assets/integrations/anthropic.svg" width="40" height="40" alt="Anthropic / Claude" title="Anthropic / Claude" />&nbsp;&nbsp; <img src="docs/assets/integrations/gemini.svg" width="40" height="40" alt="Google Gemini" title="Google Gemini" />&nbsp;&nbsp; <img src="docs/assets/integrations/deepseek.svg" width="40" height="40" alt="DeepSeek" title="DeepSeek" />&nbsp;&nbsp; <picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/integrations/readme/zhipu-dark.svg" /><img src="docs/assets/integrations/readme/zhipu-light.svg" width="40" height="40" alt="Z.ai / GLM" title="Z.ai / GLM" /></picture>&nbsp;&nbsp; <picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/integrations/readme/kimi-dark.svg" /><img src="docs/assets/integrations/readme/kimi-light.svg" width="40" height="40" alt="Moonshot / Kimi" title="Moonshot / Kimi" /></picture>&nbsp;&nbsp; <img src="docs/assets/integrations/minimax.svg" width="40" height="40" alt="MiniMax" title="MiniMax" />&nbsp;&nbsp; <picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/integrations/readme/mimo-dark.svg" /><img src="docs/assets/integrations/readme/mimo-light.svg" width="40" height="40" alt="Xiaomi MiMo" title="Xiaomi MiMo" /></picture></td>
  </tr>
</table>

Huit fournisseurs préconfigurés, ainsi que des services OpenAI-compatible personnalisés.

<a id="product-tour"></a>

## Présentation du produit

### Accès aux LLM locaux

![Exemple de configuration des modèles LLM locaux, des protocoles et de l’authentification par clé API.](docs/assets/readme/local-llm-access-models-dark-v2.png)

### Web SSH

Travaillez dans un terminal web avec audit et un Agent capable d’examiner les sorties de la session et de préparer des commandes.

![Web SSH dans Liaison](docs/assets/readme/web-ssh-dark.png)

### Applications web privées

Accédez à vos serveurs multimédias et autres applications web internes via Liaison.

![Jellyfin via Liaison](docs/pages/jellyfin-ss.png)

### Assistants IA privés

Utilisez vos outils IA internes sans exposer le réseau privé.

![OpenClaw via Liaison](docs/pages/openclaw-ss.png)

### Web MySQL

Parcourez les schémas, exécutez du SQL et demandez à l’Agent de la session d’effectuer des requêtes et d’expliquer les résultats, avec approbation si nécessaire.

![Espace de travail MySQL de Liaison](docs/assets/readme/web-mysql-dark.png)

### Web MongoDB

Explorez les collections et demandez à l’Agent d’exécuter des requêtes approuvées et d’expliquer les documents, sans client local.

![Espace de travail MongoDB de Liaison](docs/assets/readme/web-mongodb-dark.png)

### Web VNC

Ouvrez un bureau VNC privé dans une session web gérée.

![Bureau VNC dans Liaison](docs/assets/readme/web-vnc-dark.png)

### Web RDP

Connectez-vous à un bureau RDP avec le même parcours d’accès.

![Bureau RDP dans Liaison](docs/assets/readme/web-rdp-dark-v2.png)

<a id="documentation"></a>

## Documentation

- [Déploiement Docker](deploy/docker/README.md)
- [Référence API](docs/swagger/swagger.yaml)
- [Notes de version](https://github.com/liaisonio/liaison/releases)
- [Tickets](https://github.com/liaisonio/liaison/issues)
- [Discussions](https://github.com/liaisonio/liaison/discussions)

<a id="contributing"></a>

## Contribuer

Liaison évolue en continu. Nous vous invitons à contribuer au développement de l’édition auto-hébergée de ce dépôt.

- Agents IA : intégrations, gestion des sessions et collaboration.
- Accès aux applications : SSH, fichiers, bases de données, IDE, bureaux à distance et API de modèles.
- Compatibilité multiplateforme : installation et fonctionnement sous Linux, macOS et Windows.
- Expérience utilisateur : améliorations de l’interface, traductions et documentation.

Pour les changements importants, ouvrez d’abord une issue afin de définir le périmètre. Limitez chaque PR à un sujet et précisez les vérifications effectuées. N’incluez aucun identifiant secret, donnée privée ou détail de déploiement propre à un environnement. Vous pouvez participer directement sur GitHub ; rejoindre un groupe de discussion est facultatif.

Les signalements de bugs, propositions de fonctionnalités, améliorations de la documentation et pull requests sont les bienvenus. Commencez par les [Issues](https://github.com/liaisonio/liaison/issues) ou ouvrez une [Pull Request](https://github.com/liaisonio/liaison/pulls).

<a id="license"></a>

## Licence

Liaison est distribué sous la licence [GNU Affero General Public License v3.0](LICENSE).
