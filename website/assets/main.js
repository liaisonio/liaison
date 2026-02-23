const LOCALE_KEY = 'liaison-website-locale'

const i18n = {
  'zh-CN': {
    page_title: 'Liaison｜让网络马上通达',
    meta_description: 'Liaison 是一套中心化管理 + 边缘连接器的安全通达平台，让你在不暴露内网的前提下，轻松连接分布在不同位置的设备与应用。',
    og_title: 'Liaison｜让网络马上通达',
    og_description: '中心化管理 + 边缘连接器，安全通达内网设备与应用。',
    nav_aria: '主导航',
    footer_aria: '页脚链接',
    nav_features: '优势',
    nav_architecture: '架构',
    nav_scenarios: '场景',
    nav_quickstart: '快速开始',
    hero_badge: '中心化管理 + 边缘连接器',
    hero_title: '让网络马上通达，轻松连接分布在不同位置的设备与应用',
    hero_subtitle:
      'Liaison 通过 TLS 加密与“内网不暴露”的连接策略，把内网应用安全地映射为可访问入口，并提供可视化控制台完成连接器、设备、应用与流量的全生命周期管理。',
    hero_cta_start: '一分钟上手',
    hero_cta_features: '了解优势',
    kpi_tls: '端到端加密',
    kpi_zero_value: '零配置',
    kpi_zero_label: '自动发现应用',
    kpi_platform_value: '跨平台',
    chip_entry: '访问',
    features_title: '核心优势',
    features_desc: '从安全、体验到运维效率，面向真实互联网环境打造。',
    feature_security_title: '安全可靠',
    feature_security_desc: 'TLS 加密传输，内网不暴露；支持 JWT 鉴权与权限边界。',
    feature_security_li1: '连接建立可控，可随时开启关闭',
    feature_security_li2: '面向公网访问场景的默认安全策略',
    feature_simple_title: '简单易用',
    feature_simple_desc: '控制台完成全流程操作；一键生成安装命令，秒级上线连接器。',
    feature_simple_li1: '统一入口管理连接器、设备、应用与访问',
    feature_simple_li2: '快速开始与脚本分发开箱即用',
    feature_discovery_title: '自动发现',
    feature_discovery_desc: '设备上报 + 远程扫描闭环，自动发现内网应用与端口。',
    feature_discovery_li1: '减少手动录入与错误配置',
    feature_discovery_li2: '适配家庭、企业与边缘场景',
    feature_platform_title: '跨平台分发',
    feature_platform_desc: '连接器支持 Linux/macOS/Windows，覆盖 x86_64 与 ARM64，适用于服务器、工控与边缘设备。',
    feature_monitor_title: '可视化监控',
    feature_monitor_desc: '设备在线状态、延迟、流量统计可视化呈现，为运维与容量评估提供数据依据。',
    arch_title: '架构一目了然',
    arch_desc: '中心化控制平面统一管理，边缘连接器负责最后一公里的通达。',
    arch_manager_desc: '控制台 + API：连接器管理、访问配置、任务下发、数据可视化。',
    arch_frontier_desc: '连接器网关：统一承载连接与通信，简化公网入口与运维。',
    arch_edge_desc: '边缘连接器：设备信息上报、端口扫描、流量采集与应用转发。',
    arch_path_title: '典型访问路径',
    arch_path_body: '互联网客户端 → 访问入口（Entry）→ 连接器（Edge）→ 内网应用 (IP:Port)',
    arch_path_link: '查看业务时序',
    scene_title: '适用场景',
    scene_desc: '从个人到企业，覆盖典型的“外网访问内网”与边缘设备管理需求。',
    scene_remote_title: '远程办公',
    scene_remote_desc: '安全访问办公室内网服务，开发调试与资料共享更顺畅。',
    scene_nas_title: 'NAS 伴侣',
    scene_nas_desc: '随时随地访问家庭 NAS 与自建服务，避免暴露内网拓扑。',
    scene_multi_title: '多机房/多地域',
    scene_multi_desc: '统一连接不同地域的设备与应用，降低跨域运维成本。',
    scene_edge_title: '边缘计算',
    scene_edge_desc: '连接与监控边缘节点，支持自动发现与任务下发。',
    quick_title: '快速开始',
    quick_desc: '在一台 Linux 机器安装 Liaison 服务端，随后通过 Web 控制台完成配置。',
    quick_install_title: '安装 Liaison（服务端）',
    quick_install_desc: '需要一台 Linux 机器（推荐 Ubuntu / Debian / CentOS），具备可被访问的公网 IP 或域名。',
    quick_install_guide: '查看安装指南',
    cta_title: '准备好开始了吗？',
    cta_desc: '安装完成后访问控制台，开始创建连接器与访问配置。',
    cta_release: '下载 Release',
    cta_docs: '阅读文档',
    faq_title: '常见问题',
    faq_desc: '面向互联网部署时最常见的疑问。',
    faq_q1: 'Liaison 和传统“内网穿透”有什么区别？',
    faq_a1: 'Liaison 以“管理中心 + 连接器 + 访问入口”的体系化方式提供可视化控制、任务下发、自动发现与流量统计，适合管理多设备、多应用的长期场景。',
    faq_q2: '是否会暴露内网地址与拓扑？',
    faq_a2: '默认策略强调不暴露内网，外部访问通过访问入口完成转发，并提供 TLS 加密保障链路安全。',
    faq_q3: '适合个人使用还是企业使用？',
    faq_a3: '两者都适合：个人可用于 NAS/远程办公；企业可用于多机房通达、边缘设备管理与运维监控。',
    footer_tagline: '让网络马上通达',
    footer_sequence: '业务时序',
    footer_feedback: '反馈与建议',
  },
  'en-US': {
    page_title: 'Liaison | Make Networks Instantly Reachable',
    meta_description:
      'Liaison is a centralized management and edge-connector platform that securely connects distributed devices and applications without exposing your internal network.',
    og_title: 'Liaison | Make Networks Instantly Reachable',
    og_description: 'Centralized management + edge connectors for secure intranet access.',
    nav_aria: 'Main navigation',
    footer_aria: 'Footer links',
    nav_features: 'Features',
    nav_architecture: 'Architecture',
    nav_scenarios: 'Scenarios',
    nav_quickstart: 'Quick Start',
    hero_badge: 'Centralized management + edge connectors',
    hero_title: 'Make networks instantly reachable and connect distributed devices and applications with ease',
    hero_subtitle:
      'Liaison uses TLS encryption and a non-exposed intranet strategy to expose internal applications through secure access entries, with a visual console for full-lifecycle management of connectors, devices, applications, and traffic.',
    hero_cta_start: 'Start in 1 minute',
    hero_cta_features: 'Explore Features',
    kpi_tls: 'End-to-end encryption',
    kpi_zero_value: 'Zero config',
    kpi_zero_label: 'Auto app discovery',
    kpi_platform_value: 'Cross-platform',
    chip_entry: 'Entry',
    features_title: 'Core Strengths',
    features_desc: 'Built for real-world internet environments, from security to usability and ops efficiency.',
    feature_security_title: 'Secure & Reliable',
    feature_security_desc: 'TLS-encrypted transport with non-exposed intranet and JWT-based authorization boundaries.',
    feature_security_li1: 'Connection lifecycle is fully controllable',
    feature_security_li2: 'Secure-by-default policy for public access',
    feature_simple_title: 'Simple to Use',
    feature_simple_desc: 'Operate everything from the console and bring connectors online in seconds with one command.',
    feature_simple_li1: 'Unified management for connectors, devices, applications, and access entries',
    feature_simple_li2: 'Quick-start scripts ready out of the box',
    feature_discovery_title: 'Auto Discovery',
    feature_discovery_desc: 'Closed-loop reporting and remote scanning to discover internal apps and ports automatically.',
    feature_discovery_li1: 'Reduce manual input and config errors',
    feature_discovery_li2: 'Fits home, enterprise, and edge scenarios',
    feature_platform_title: 'Cross-platform Delivery',
    feature_platform_desc: 'Connectors support Linux/macOS/Windows on x86_64 and ARM64 for servers and edge devices.',
    feature_monitor_title: 'Visual Monitoring',
    feature_monitor_desc: 'Online status, latency, and traffic metrics are visualized for operations and capacity planning.',
    arch_title: 'Architecture at a Glance',
    arch_desc: 'A centralized control plane with edge connectors delivering last-mile reachability.',
    arch_manager_desc: 'Console + API for connector management, access configuration, task dispatch, and data visualization.',
    arch_frontier_desc: 'Connector gateway that unifies connectivity and communication with simpler public ingress.',
    arch_edge_desc: 'Edge connector for device reporting, port scanning, traffic collection, and app forwarding.',
    arch_path_title: 'Typical Access Path',
    arch_path_body: 'Internet Client → Access Entry (Entry) → Connector (Edge) → Internal App (IP:Port)',
    arch_path_link: 'View Business Sequence',
    scene_title: 'Use Cases',
    scene_desc: 'From individuals to enterprises, covering typical external-to-internal access and edge management needs.',
    scene_remote_title: 'Remote Work',
    scene_remote_desc: 'Securely access office intranet services for smoother development and collaboration.',
    scene_nas_title: 'NAS Companion',
    scene_nas_desc: 'Access home NAS and self-hosted services anytime without exposing internal topology.',
    scene_multi_title: 'Multi-DC / Multi-Region',
    scene_multi_desc: 'Unify connectivity across regions to reduce cross-domain operations cost.',
    scene_edge_title: 'Edge Computing',
    scene_edge_desc: 'Connect and monitor edge nodes with auto discovery and task dispatch.',
    quick_title: 'Quick Start',
    quick_desc: 'Install the Liaison server on Linux, then complete setup through the web console.',
    quick_install_title: 'Install Liaison (Server)',
    quick_install_desc: 'Requires a Linux host (Ubuntu / Debian / CentOS recommended) with a public IP or domain.',
    quick_install_guide: 'Installation Guide',
    cta_title: 'Ready to get started?',
    cta_desc: 'After installation, open the console and start creating connectors and access entries.',
    cta_release: 'Download Release',
    cta_docs: 'Read Docs',
    faq_title: 'FAQ',
    faq_desc: 'Most common questions for internet-facing deployment.',
    faq_q1: 'How is Liaison different from traditional intranet tunneling tools?',
    faq_a1: 'Liaison provides a systemized model of management center + connector + access entry with visual control, task dispatch, auto discovery, and traffic metrics for long-term multi-device and multi-app operations.',
    faq_q2: 'Will internal addresses and topology be exposed?',
    faq_a2: 'No. The default strategy avoids exposing the intranet. External traffic is forwarded through access entries with TLS protection.',
    faq_q3: 'Is it suitable for personal or enterprise use?',
    faq_a3: 'Both. Individuals can use it for NAS and remote work; enterprises can use it for multi-datacenter connectivity and edge operations.',
    footer_tagline: 'Make networks instantly reachable',
    footer_sequence: 'Business Sequence',
    footer_feedback: 'Feedback',
  },
}

const setYear = () => {
  const el = document.getElementById('year')
  if (!el) return
  el.textContent = String(new Date().getFullYear())
}

const enableSmoothAnchors = () => {
  const links = document.querySelectorAll('a[href^="#"]')
  for (const link of links) {
    link.addEventListener('click', (e) => {
      const href = link.getAttribute('href')
      if (!href || href === '#') return
      const target = document.querySelector(href)
      if (!target) return
      e.preventDefault()
      target.scrollIntoView({ behavior: 'smooth', block: 'start' })
      history.pushState(null, '', href)
    })
  }
}

const enableRevealOnScroll = () => {
  const sections = document.querySelectorAll('.section--reveal')
  if (!sections.length) return
  const observer = new IntersectionObserver(
    (entries) => {
      for (const entry of entries) {
        if (entry.isIntersecting) {
          entry.target.classList.add('revealed')
        }
      }
    },
    { rootMargin: '0px 0px -8% 0px', threshold: 0 }
  )
  sections.forEach((section) => observer.observe(section))
}

const applyLocale = (locale) => {
  const lang = locale === 'en-US' ? 'en-US' : 'zh-CN'
  const dict = i18n[lang]

  document.documentElement.lang = lang
  const pageTitle = document.getElementById('i18n-page-title')
  const metaDescription = document.getElementById('i18n-meta-description')
  const ogTitle = document.getElementById('i18n-og-title')
  const ogDescription = document.getElementById('i18n-og-description')
  if (pageTitle) pageTitle.textContent = dict.page_title
  if (metaDescription) metaDescription.setAttribute('content', dict.meta_description)
  if (ogTitle) ogTitle.setAttribute('content', dict.og_title)
  if (ogDescription) ogDescription.setAttribute('content', dict.og_description)

  document.querySelectorAll('[data-i18n]').forEach((el) => {
    const key = el.getAttribute('data-i18n')
    if (key && dict[key]) {
      el.textContent = dict[key]
    }
  })

  document.querySelectorAll('[data-i18n-aria-label]').forEach((el) => {
    const key = el.getAttribute('data-i18n-aria-label')
    if (key && dict[key]) {
      el.setAttribute('aria-label', dict[key])
    }
  })

  const zhBtn = document.getElementById('lang-zh')
  const enBtn = document.getElementById('lang-en')
  if (zhBtn && enBtn) {
    zhBtn.classList.toggle('is-active', lang === 'zh-CN')
    enBtn.classList.toggle('is-active', lang === 'en-US')
  }

  localStorage.setItem(LOCALE_KEY, lang)
}

const initLocaleSwitch = () => {
  const current = localStorage.getItem(LOCALE_KEY) || 'zh-CN'
  applyLocale(current)

  const zhBtn = document.getElementById('lang-zh')
  const enBtn = document.getElementById('lang-en')
  if (zhBtn) {
    zhBtn.addEventListener('click', () => applyLocale('zh-CN'))
  }
  if (enBtn) {
    enBtn.addEventListener('click', () => applyLocale('en-US'))
  }
}

setYear()
enableSmoothAnchors()
enableRevealOnScroll()
initLocaleSwitch()
