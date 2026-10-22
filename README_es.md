<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/assets/liaison-brand-rays-dark.svg" />
  <img src="docs/assets/liaison-brand-rays-light.svg" width="270" height="88" alt="Liaison" />
</picture>

> **Acceso autoalojado a aplicaciones privadas, LLM y agentes de IA.**

Espacios de trabajo en el navegador, API compartidas con permisos delimitados y asistencia de IA contextual, mediante conectores que solo inician conexiones salientes desde tu red privada.

[![CI](https://github.com/liaisonio/liaison/actions/workflows/go.yml/badge.svg)](https://github.com/liaisonio/liaison/actions/workflows/go.yml)
[![Release](https://img.shields.io/github/v/release/liaisonio/liaison?display_name=tag&sort=semver)](https://github.com/liaisonio/liaison/releases)
[![Downloads](https://img.shields.io/github/downloads/liaisonio/liaison/total)](https://github.com/liaisonio/liaison/releases)
[![Go](https://img.shields.io/github/go-mod/go-version/liaisonio/liaison?logo=go&logoColor=white)](go.mod)
[![React](https://img.shields.io/badge/React-20232A?logo=react&logoColor=61DAFB)](web/package.json)
[![TypeScript](https://img.shields.io/badge/TypeScript-3178C6?logo=typescript&logoColor=white)](web/package.json)
[![Vite](https://img.shields.io/badge/Vite-646CFF?logo=vite&logoColor=white)](web/vite.config.ts)
[![Tailwind CSS](https://img.shields.io/badge/Tailwind_CSS-06B6D4?logo=tailwindcss&logoColor=white)](web/tailwind.config.cjs)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](#contributing)

[English](./README.md) | [简体中文](./README_zh.md) | [日本語](./README_ja.md) | [한국어](./README_ko.md) | Español | [Français](./README_fr.md) | [Deutsch](./README_de.md)

[Sitio web](https://liaison.cloud) · [Documentación](https://liaison.cloud/docs/get-started/introduction) · [Funciones](#features) · [Instalación](#install) · [Protocolos de acceso](#supported-access-protocols) · [Modelos del Agent](#agent-model-providers) · [Recorrido](#product-tour)

![Demostración de Liaison con datos de ejemplo.](docs/assets/readme/liaison-product-demo-full-v2.gif)

<a id="features"></a>

## Funciones

- 🤖 **Acceso a LLM locales** — Comparte modelos locales mediante API autenticadas y un Playground en el navegador.
- 🔑 **Control del acceso a las API** — Limita las claves a determinados modelos, establece cuotas de tokens, revoca el acceso y consulta el uso.
- 🤖 **Acceso a agentes de IA** — Utiliza Codex y Claude Code en tus dispositivos desde el navegador, con sesiones reanudables, progreso de ejecución y solicitudes de permisos.
- 💻 **IDE en el navegador** — Edita proyectos y utiliza terminales y extensiones mediante code-server.
- ✨ **IA en tu flujo de trabajo** — Revisa la salida del terminal, prepara comandos y consulta bases de datos dentro de los permisos del usuario, con aprobación cuando sea necesaria.
- 💬 **Consulta tus recursos** — Encuentra conectores, dispositivos y aplicaciones dentro de tu ámbito de acceso.
- 🔌 **Conectores de salida** — Conecta dispositivos privados sin abrir puertos de entrada en ellos.
- 🔐 **Acceso a aplicaciones** — Accede a servicios TCP, HTTP, HTTPS, WebSocket y SSH con controles por acceso.
- 🖥️ **Espacios de trabajo en el navegador** — Trabaja con terminales, escritorios remotos, bases de datos y cachés desde el navegador.
- 📁 **Archivos y objetos** — Gestiona archivos por SFTP y explora almacenamiento compatible con S3 (solo lectura).
- 🔎 **Detección de aplicaciones** — Detecta servicios existentes en los dispositivos con conector y regístralos desde la consola.
- 👥 **Gestión de identidad y acceso** — Gestiona usuarios y recursos con autorización basada en Casbin.
- 🛡️ **Políticas de firewall** — Restringe el acceso TCP y HTTP por IP de origen o CIDR.
- 📋 **Registros y auditoría** — Revisa las acciones administrativas y las sesiones de las aplicaciones compatibles.
- 📦 **Despliegue autoalojado** — Ejecuta el plano de control completo en tu propio servidor Linux.

<a id="install"></a>

## Instalación

Descarga el paquete Docker autónomo, descomprímelo y ejecuta el instalador (requiere Docker 20.10 o posterior y Compose):

```bash
wget https://github.com/liaisonio/liaison/releases/download/v1.15.4/liaison-1.15.4-linux-amd64.tar.gz
tar -xzf liaison-1.15.4-linux-amd64.tar.gz
cd liaison-1.15.4-linux-amd64
./install.sh
```

Tras la instalación, abre `https://<server-address>`. El instalador muestra las credenciales iniciales de acceso.

<a id="supported-access-protocols"></a>

## Protocolos de acceso compatibles

Accede a tus servicios privados existentes a través de Liaison.

<table>
  <tr><th align="left">SSH / SFTP</th><td><img src="docs/assets/integrations/readme/ssh.svg" width="40" height="40" alt="SSH / WebSSH" title="SSH / WebSSH" />&nbsp;&nbsp; <img src="docs/assets/integrations/readme/sftp.svg" width="40" height="40" alt="WebSFTP" title="WebSFTP" /></td></tr>
  <tr><th align="left">Escritorio</th><td><img src="docs/assets/integrations/windows.svg" width="40" height="40" alt="WebRDP" title="WebRDP" />&nbsp;&nbsp; <img src="docs/assets/integrations/vnc.png" width="40" height="40" alt="WebVNC" title="WebVNC" /></td></tr>
  <tr><th align="left">Bases de datos SQL</th><td><img src="docs/assets/integrations/mysql.svg" width="40" height="40" alt="WebMySQL" title="WebMySQL" />&nbsp;&nbsp; <img src="docs/assets/integrations/mariadb.svg" width="40" height="40" alt="WebMariaDB" title="WebMariaDB" />&nbsp;&nbsp; <img src="docs/assets/integrations/postgresql.svg" width="40" height="40" alt="WebPostgreSQL" title="WebPostgreSQL" />&nbsp;&nbsp; <img src="docs/assets/integrations/sqlserver.svg" width="40" height="40" alt="WebSQLServer" title="WebSQLServer" />&nbsp;&nbsp; <img src="docs/assets/integrations/oracle.svg" width="40" height="40" alt="WebOracle" title="WebOracle" />&nbsp;&nbsp; <img src="docs/assets/integrations/clickhouse.svg" width="40" height="40" alt="WebClickHouse" title="WebClickHouse" /></td></tr>
  <tr><th align="left">Datos y búsqueda</th><td><img src="docs/assets/integrations/mongodb.svg" width="40" height="40" alt="WebMongoDB" title="WebMongoDB" />&nbsp;&nbsp; <img src="docs/assets/integrations/elasticsearch.svg" width="40" height="40" alt="WebElasticsearch" title="WebElasticsearch" />&nbsp;&nbsp; <img src="docs/assets/integrations/opensearch.svg" width="40" height="40" alt="WebOpenSearch" title="WebOpenSearch" /></td></tr>
  <tr><th align="left">Caché</th><td><img src="docs/assets/integrations/redis.svg" width="40" height="40" alt="WebRedis" title="WebRedis" />&nbsp;&nbsp; <img src="docs/assets/integrations/memcached.svg" width="40" height="40" alt="WebMemcached" title="WebMemcached" /></td></tr>
  <tr><th align="left">Almacenamiento</th><td><img src="docs/assets/integrations/readme/s3.svg" width="40" height="40" alt="WebS3 (S3-compatible)" title="WebS3 (S3-compatible)" /></td></tr>
  <tr><th align="left">Protocolos LLM de origen</th><td><picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/integrations/readme/openai-dark.svg" /><img src="docs/assets/integrations/readme/openai-light.svg" width="40" height="40" alt="OpenAI-compatible" title="OpenAI-compatible" /></picture>&nbsp;&nbsp; <img src="docs/assets/integrations/anthropic.svg" width="40" height="40" alt="Anthropic Messages" title="Anthropic Messages" />&nbsp;&nbsp; <picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/integrations/readme/ollama-dark.svg" /><img src="docs/assets/integrations/readme/ollama-light.svg" width="40" height="40" alt="Ollama" title="Ollama" /></picture></td></tr>
  <tr><th align="left">Web / TCP</th><td><img src="docs/assets/integrations/readme/web.svg" width="40" height="40" alt="HTTP / HTTPS / WebSocket" title="HTTP / HTTPS / WebSocket" />&nbsp;&nbsp; <img src="docs/assets/integrations/readme/tcp.svg" width="40" height="40" alt="TCP" title="TCP" /></td></tr>
</table>

Los servicios LLM de origen admiten **OpenAI-compatible**, **Anthropic Messages** y **Ollama**. Los clientes utilizan endpoints OpenAI-compatible; los servicios Anthropic también ofrecen endpoints nativos de Messages. Los logotipos identifican servicios existentes a los que puedes acceder, no productos incluidos ni desplegados por Liaison. Los logotipos de bases de datos y escritorios representan espacios de trabajo en el navegador, no servidores nativos de bases de datos, RDP o VNC a la escucha.

<a id="agent-model-providers"></a>

## Proveedores de modelos del Agent

Configura el modelo del Agent integrado en Liaison de forma independiente del acceso a los servicios.

<table>
  <tr>
    <th align="left">Modelos</th>
    <td><picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/integrations/readme/openai-dark.svg" /><img src="docs/assets/integrations/readme/openai-light.svg" width="40" height="40" alt="OpenAI" title="OpenAI" /></picture>&nbsp;&nbsp; <img src="docs/assets/integrations/anthropic.svg" width="40" height="40" alt="Anthropic / Claude" title="Anthropic / Claude" />&nbsp;&nbsp; <img src="docs/assets/integrations/gemini.svg" width="40" height="40" alt="Google Gemini" title="Google Gemini" />&nbsp;&nbsp; <img src="docs/assets/integrations/deepseek.svg" width="40" height="40" alt="DeepSeek" title="DeepSeek" />&nbsp;&nbsp; <picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/integrations/readme/zhipu-dark.svg" /><img src="docs/assets/integrations/readme/zhipu-light.svg" width="40" height="40" alt="Z.ai / GLM" title="Z.ai / GLM" /></picture>&nbsp;&nbsp; <picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/integrations/readme/kimi-dark.svg" /><img src="docs/assets/integrations/readme/kimi-light.svg" width="40" height="40" alt="Moonshot / Kimi" title="Moonshot / Kimi" /></picture>&nbsp;&nbsp; <img src="docs/assets/integrations/minimax.svg" width="40" height="40" alt="MiniMax" title="MiniMax" />&nbsp;&nbsp; <picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/integrations/readme/mimo-dark.svg" /><img src="docs/assets/integrations/readme/mimo-light.svg" width="40" height="40" alt="Xiaomi MiMo" title="Xiaomi MiMo" /></picture></td>
  </tr>
</table>

Ocho proveedores preconfigurados, además de servicios OpenAI-compatible personalizados.

<a id="product-tour"></a>

## Recorrido del producto

### Acceso a LLM locales

![Ejemplo de configuración de modelos LLM locales, protocolos y autenticación con clave API.](docs/assets/readme/local-llm-access-models-dark-v2.png)

### Web SSH

Trabaja en un terminal del navegador con auditoría y un Agent que puede revisar la salida de la sesión y ayudarte a preparar comandos.

![Web SSH en Liaison](docs/assets/readme/web-ssh-dark.png)

### Aplicaciones web privadas

Accede a servidores multimedia y otras aplicaciones web internas a través de Liaison.

![Jellyfin a través de Liaison](docs/pages/jellyfin-ss.png)

### Asistentes de IA privados

Utiliza herramientas internas de IA sin exponer la red privada.

![OpenClaw a través de Liaison](docs/pages/openclaw-ss.png)

### Web MySQL

Explora esquemas, ejecuta SQL y pide al Agent de la sesión que consulte y explique los resultados, con aprobación cuando sea necesaria.

![Espacio de trabajo MySQL de Liaison](docs/assets/readme/web-mysql-dark.png)

### Web MongoDB

Explora colecciones y pide al Agent que ejecute consultas aprobadas y explique documentos, sin un cliente local.

![Espacio de trabajo MongoDB de Liaison](docs/assets/readme/web-mongodb-dark.png)

### Web VNC

Abre un escritorio VNC privado en una sesión administrada del navegador.

![Escritorio VNC en Liaison](docs/assets/readme/web-vnc-dark.png)

### Web RDP

Conéctate a un escritorio RDP mediante el mismo flujo de acceso.

![Escritorio RDP en Liaison](docs/assets/readme/web-rdp-dark-v2.png)

<a id="documentation"></a>

## Documentación

- [Despliegue con Docker](deploy/docker/README.md)
- [Referencia de API](docs/swagger/swagger.yaml)
- [Notas de versión](https://github.com/liaisonio/liaison/releases)
- [Incidencias](https://github.com/liaisonio/liaison/issues)
- [Debates](https://github.com/liaisonio/liaison/discussions)

<a id="contributing"></a>

## Contribuir

Liaison sigue evolucionando. Te invitamos a contribuir al desarrollo de la edición autoalojada de este repositorio.

- Agentes de IA: integraciones, gestión de sesiones y colaboración.
- Acceso a aplicaciones: SSH, archivos, bases de datos, IDE, escritorios remotos y API de modelos.
- Compatibilidad multiplataforma: instalación y funcionamiento en Linux, macOS y Windows.
- Experiencia de usuario: mejoras de interfaz, traducciones y documentación.

Para cambios importantes, abre primero un issue para acordar el alcance. Centra cada PR en un tema y explica cómo lo verificaste. No incluyas credenciales, datos privados ni detalles de despliegue de entornos específicos. Puedes participar directamente en GitHub; unirte a un grupo de chat es opcional.

Se agradecen los informes de errores, las propuestas de funciones, las mejoras de documentación y los pull requests. Participa en [Issues](https://github.com/liaisonio/liaison/issues) o abre un [Pull Request](https://github.com/liaisonio/liaison/pulls).

<a id="license"></a>

## Licencia

Liaison se distribuye bajo la [GNU Affero General Public License v3.0](LICENSE).
