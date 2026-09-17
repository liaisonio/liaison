<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/assets/liaison-brand-rays-dark.svg" />
  <img src="docs/assets/liaison-brand-rays-light.svg" width="270" height="88" alt="Liaison" />
</picture>

> **Acceso de confianza cero nativo de IA para LLM locales y aplicaciones.**

Autohospedado, con uso compartido seguro de API, espacios de trabajo en el navegador y agentes de IA conscientes del contexto.

[![CI](https://github.com/liaisonio/liaison/actions/workflows/go.yml/badge.svg)](https://github.com/liaisonio/liaison/actions/workflows/go.yml)
[![Release](https://img.shields.io/github/v/release/liaisonio/liaison?display_name=tag&sort=semver)](https://github.com/liaisonio/liaison/releases)
[![Downloads](https://img.shields.io/github/downloads/liaisonio/liaison/total)](https://github.com/liaisonio/liaison/releases)
[![License](https://img.shields.io/badge/License-AGPLv3-blue.svg?logo=gnu)](LICENSE)
[![Go](https://img.shields.io/github/go-mod/go-version/liaisonio/liaison?logo=go&logoColor=white)](go.mod)
[![React](https://img.shields.io/badge/React-20232A?logo=react&logoColor=61DAFB)](web/package.json)
[![TypeScript](https://img.shields.io/badge/TypeScript-3178C6?logo=typescript&logoColor=white)](web/package.json)
[![Vite](https://img.shields.io/badge/Vite-646CFF?logo=vite&logoColor=white)](web/vite.config.ts)
[![Tailwind CSS](https://img.shields.io/badge/Tailwind_CSS-06B6D4?logo=tailwindcss&logoColor=white)](web/tailwind.config.cjs)

[简体中文](./README_zh.md) | [English](./README.md) | [日本語](./README_ja.md) | [한국어](./README_ko.md) | Español | [Français](./README_fr.md) | [Deutsch](./README_de.md)

![Dashboard](docs/pages/home_en.png)

| Jellyfin (ver películas caseras en cualquier lugar) | OpenClaw (usar la IA doméstica en cualquier lugar) |
|:---:|:---:|
| ![Jellyfin](docs/pages/jellyfin-ss.png) | ![OpenClaw](docs/pages/openclaw-ss.png) |

[Inicio rápido](#inicio-rápido) • [Introducción](#introducción) • [Documentación](#documentación) • [Contribuir](#contribuir)

---

## Introducción

Conecta servidores, bases de datos, escritorios y aplicaciones privadas. Trabaja con un Agent de IA en sesiones SSH y de bases de datos, o consulta tus conectores, dispositivos y aplicaciones desde el inicio. Autohospedado, con herramientas sujetas a permisos de usuario.

- ✨ **IA en tu flujo de trabajo** — El Agent de la conexión ayuda a interpretar la terminal, redactar comandos y consultar bases de datos. Las operaciones que requieren aprobación esperan tu confirmación.
- 💬 **Consulta tus recursos** — Busca conectores, dispositivos y aplicaciones desde el Agent de inicio, dentro de los permisos del usuario.

Este proyecto resuelve:

- **Acceso a red privada** — Alcanzar dispositivos y servicios detrás de NAT desde internet público con configuración mínima
- **Gestión multi-dispositivo** — Administrar dispositivos en varias ubicaciones con soporte para Linux/macOS/Windows
- **Conectividad segura** — Transporte cifrado con TLS sin exponer puertos en tu LAN o red doméstica
- **Firewall por entrada** — Lista de permitidos por CIDR de IP de origen en cada entrada TCP o HTTP, aplicada al aceptar la conexión
- **Monitoreo de tráfico** — Estado del dispositivo y métricas de tráfico en tiempo real para operaciones y planificación de capacidad
- **Proxy de aplicaciones** — Protocolos TCP, HTTP/HTTPS, WebSocket y otros
- **Automatización de la API** — Personal Access Tokens (PAT) para CLI / scripts con un flujo de inicio de sesión mediado por navegador en `/cli-auth`

Casos de uso:

<div align="center">

| **💼 Trabajo remoto & Dev** | **🧑‍💻 Estudio personal** | **🏠 Red doméstica / NAS** | **🌐 Multi-DC / Multi-región** | **⚡ Edge & Ops** |
|:---:|:---:|:---:|:---:|:---:|
| Conectar dispositivos de oficina y hogar para desarrollo y depuración remotos | Conectar de forma segura estaciones de trabajo y entornos privados con gestión unificada | Acceder al NAS doméstico y servicios smart-home desde internet | Conectividad unificada entre servidores y aplicaciones en regiones y DCs | Conectar y monitorizar aplicaciones edge con chequeos remotos |

</div>

---

## Inicio rápido

Instala el paquete tar.gz del servidor y después instala un conector.

### Instalar el servidor — tar.gz

**1. Descargar**

```bash
wget https://github.com/liaisonio/liaison/releases/download/v1.13.0/liaison-1.13.0-linux-amd64.tar.gz
tar -xzf liaison-1.13.0-linux-amd64.tar.gz
cd liaison-1.13.0-linux-amd64
```

**2. Ejecutar el script de instalación**

```bash
./install.sh
```

Se te pedirá una IP pública o dominio; si no introduces nada en 30 segundos, se usará la IP pública detectada.

**3. Abrir la consola web**

Visita `https://tu-ip-publica` para acceder a la consola web.

> **Sugerencia:** Las credenciales de admin por defecto aparecen en la salida de install.sh o en el archivo de configuración.

### Instalar el conector

Dos rutas de instalación, elige la que se adapte al dispositivo objetivo.

#### Opción A — Liaison Desktop (GUI, macOS / Windows)

App de barra de menú / bandeja del sistema que envuelve al conector y ofrece inicio de sesión con un clic, indicador de estado, pausar / reanudar y acceso al dashboard con un clic. Ideal para portátiles y estaciones de trabajo.

<div align="center">

| macOS | Windows |
|:---:|:---:|
| <img src="docs/images/desktop-client/popup-macos.png" alt="Liaison Desktop on macOS" width="360" /> | <img src="docs/images/desktop-client/popup-windows.png" alt="Liaison Desktop on Windows" width="360" /> |

</div>

- **Inicio de sesión con un clic** — flujo OAuth mediado por navegador, PAT almacenado en el llavero del SO (Keychain en macOS, Administrador de credenciales en Windows)
- **Multi-despliegue** — por defecto en `liaison.cloud`; el icono de engranaje en la esquina inferior izquierda permite cambiar a cualquier despliegue privado sin reinstalar
- **Estado consciente del heartbeat** — las transiciones Conectando → En línea reflejan el estado real del túnel, no solo la vida del proceso
- **Pausa que sobrevive al cierre** — la intención se persiste en disco, así que una sesión pausada permanece pausada tras relanzar

**Descarga (pre-release rodante, último build de `feat/desktop-client`):**

| Plataforma | Archivo |
|:---|:---|
| macOS (Apple Silicon + Intel, universal) | [`Liaison_0.1.0_universal.dmg`](https://github.com/liaisonio/liaison/releases/download/desktop-latest/Liaison_0.1.0_universal.dmg) |
| Windows (instalador .msi) | [`Liaison_0.1.0_x64_en-US.msi`](https://github.com/liaisonio/liaison/releases/download/desktop-latest/Liaison_0.1.0_x64_en-US.msi) |
| Windows (.exe NSIS, con limpieza de keychain al desinstalar) | [`Liaison_0.1.0_x64-setup.exe`](https://github.com/liaisonio/liaison/releases/download/desktop-latest/Liaison_0.1.0_x64-setup.exe) |

> Los instaladores de v0.1 no están firmados. Gatekeeper de macOS y Windows SmartScreen avisarán al primer arranque — clic derecho → Abrir en macOS, o "Más información" → "Ejecutar de todos modos" en Windows. WebView2 Runtime es necesario en Windows; Win10 1803+ y Win11 lo incluyen.

#### Opción B — Comando de instalación CLI (Linux / sin cabeza)

**Crea un nuevo conector** en la consola web, copia el comando de instalación para tu plataforma desde la UI y ejecútalo en el dispositivo objetivo. El conector aparecerá automáticamente en la consola.

---

## Requisitos del sistema

| Componente | Requisitos |
|:---|:---|
| **Servidor** | Linux (se recomienda Ubuntu 20.04+ o CentOS 7+) |
| **Conector** | Linux / macOS / Windows (x86_64 y ARM64) |
| **Navegador** | Chrome 90+, Firefox 88+, Safari 14+, Edge 90+ |

---

## Arquitectura

<img src="./docs/diagrams/liaison.png" width="80%">

Liaison usa una arquitectura centralizada con Frontier gestionando todos los conectores.

**Componentes**

- **Liaison** — UI web y API, junto con los puntos de entrada de la aplicación
- **Frontier** — Gateway de conectores que maneja conexiones y enrutamiento de tráfico
- **Edge** — Cliente conector en los dispositivos objetivo

---

## Galería de funciones

| Función | Captura |
|:---:|:---:|
| Gestión de dispositivos | ![Device](docs/pages/device_en.png) |
| Gestión de aplicaciones | ![Application](docs/pages/application_en.png) |
| Configuración de proxy | ![Proxy](docs/pages/proxy_en.png) |
| Gestión de conectores | ![Edge](docs/pages/edge_en.png) |

---

## Documentación

- [Flujo de negocio](./docs/biz_sequence.md)
- [API](./docs/swagger/)

---

## Contribuir

Las contribuciones son bienvenidas.

- [Reportar un bug](https://github.com/liaisonio/liaison/issues/new?template=bug_report.md)
- [Sugerir una función](https://github.com/liaisonio/liaison/issues/new?template=feature_request.md)
- [Abrir un PR](https://github.com/liaisonio/liaison/pulls)
- [Mejorar la documentación](https://github.com/liaisonio/liaison/issues/new?template=documentation.md)

1. Haz fork del repositorio
2. Crea una rama (`git checkout -b feature/AmazingFeature`)
3. Haz commit (`git commit -m 'Add some AmazingFeature'`)
4. Push (`git push origin feature/AmazingFeature`)
5. Abre un Pull Request

---

## Licencia

[GNU Affero General Public License v3.0](LICENSE).

---

<div align="center">

**Si este proyecto te ayuda, dale una ⭐ Star!**

Made with ❤️ by [Liaison Contributors](https://github.com/liaisonio/liaison/graphs/contributors)

[GitHub](https://github.com/liaisonio/liaison) • [Issues](https://github.com/liaisonio/liaison/issues) • [Discussions](https://github.com/liaisonio/liaison/discussions)

</div>
