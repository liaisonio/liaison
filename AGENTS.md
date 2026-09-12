# Liaison contributor instructions

## UI consistency (required)

- Treat the existing console as the design system. Before changing UI, inspect
  `web/src/styles/index.css`, `web/src/components/ui`, and a comparable existing
  page. Use Settings for configuration forms and AgentWorkspace for AI output.
  Do not introduce a separate visual system for a new feature.
- For access child pages, compare against WebData's connection manager, not
  Settings or a marketing landing page. Reuse AppLayout's context-only back
  header and its body padding; use breadcrumb, compact toolbar, then content.
  Do not add nested page padding, centered max-width shells or hero copy.
  Access-context headings use the existing 13px section-title scale. Typography
  numbers below apply to their respective components, not every heading globally.
- Visual acceptance must compare the reference and changed page at the SAME
  viewport, browser zoom, theme and sidebar state. Compare content left edge,
  return navigation, breadcrumb, heading baseline and content density, not just
  font-family and build success.
- Inherit the global UI font stack. Use a consistent monospace stack only for
  commands, code, model identifiers, endpoints and request/session IDs:
  `ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace`.
  Never rely on browser-default serif fonts for code blocks.
- Follow existing typography: page title 18px/680, configuration section title
  17px/680, normal body 13px, descriptions and tab labels 12px. Reuse the shared
  component's own sizing for buttons, inputs and dense tables. Do not globally
  shrink/enlarge controls to match a new page. Keep chat output readable and do
  not let description styles override Markdown paragraphs.
- Use existing semantic tokens: `--ink`, `--muted`, `--accent`, `--line`,
  `--canvas`, `--surface`, `--panel`, `--surface-radius`, `--control-radius`.
  Do not hard-code a brand purple or use undefined tokens. Selected, hover and
  focus states must work with user-selected accent colors and light/dark themes.
- Reuse shared Button, Field, Input, Select, Notice and confirmation components.
  Keep labels above fields, with help text adjacent. Align form columns and
  action rows; prefer 16/20/24px spacing and existing component heights.
- Use one name per concept across navigation, section headings and actions.
  Use LLM / LLM 协议 (LLM protocol) for the access protocol display name;
  retain existing wire identifiers and SDK paths for compatibility.
  LLM workspace terminology: 概览/Overview, 在线体验/Playground, API 密钥/API keys,
  请求记录/Request records, 访问配置/Access configuration. Use 发送/Send for
  chat, not testing terminology. Translate surrounding UI labels with `tr`;
  preserve technical identifiers (API, Token, model IDs) without inventing aliases.
- Write short, factual UI copy. Describe the action or current state, not
  implementation jargon or promotional claims. Differentiate loading, empty,
  disabled and failed states. Loading must disappear after completion.
- Render model output with the shared safe Markdown renderer. Never enable raw
  model HTML or remote model-supplied images. Display safe errors and copyable
  diagnostic identifiers, never upstream credentials or raw provider errors.
- Permission-sensitive UI must follow backend capabilities; hiding a control is
  not authorization. Never fetch upstream configuration for consumer-only UI.
- Verify changed screens at desktop and mobile widths, in Chinese and English,
  light and dark modes. Check focus visibility, long IDs, Markdown, dropdowns,
  loading/error states and page overflow. Inspect screenshots rather than relying
  on a successful build. Update E2E selectors when intentionally changing labels.
- Keep this document free of credentials, server IPs and environment-specific
  deployment commands. Preserve unrelated worktree changes.
