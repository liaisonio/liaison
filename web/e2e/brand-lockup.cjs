const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const assert = require('node:assert/strict');
(async () => {
  const browser = await chromium.launch();
  try {
    for (const locale of ['zh-CN', 'en-US']) for (const theme of ['dark', 'light']) {
      const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } });
      await context.addInitScript(({ locale, theme }) => {
        localStorage.setItem('liaison-locale', locale);
        localStorage.setItem('liaison-theme-preference', theme);
      }, { locale, theme });
      await context.route('**/api/v1/**', route => {
        const path = new URL(route.request().url()).pathname;
        const data = path.endsWith('/status') ? { enabled: true, models: [] } : path.endsWith('/sessions') ? { items: [] } : {};
        return route.fulfill({ json: { code: 200, data } });
      });
      const page = await context.newPage();
      const errors = []; page.on('pageerror', e => errors.push(e.message));
      const base = process.env.E2E_UI_URL;
      await page.goto(base + '/login');
      await page.locator('.login-form-title .liaison-brand-lockup').waitFor();
      assert.equal(await page.locator('.login-brand-message h1').innerText(), locale === 'zh-CN'
        ? '面向本地大模型与应用的 AI 原生零信任访问。'
        : 'AI-native zero-trust access for local LLMs and applications.');
      assert.equal(await page.locator('.login-brand-message p').innerText(), locale === 'zh-CN'
        ? '支持私有化部署、安全 API 分享、浏览器工作台与上下文感知的 AI Agent。'
        : 'Self-hosted, with secure API sharing, browser workspaces, and context-aware AI Agents.');
      assert.equal(await page.locator('.login-brand-topline .liaison-brand-lockup').count(), 0);
      assert.equal(await page.locator('.login-brand-wordmark').innerText(), 'Liaison');
      assert.equal(await page.locator('.login-brand-logo').count(), 1);
      for (const width of [1440, 390]) {
        await page.setViewportSize({ width, height: 1000 });
        assert(await page.locator(`.login-form-title .liaison-brand-${theme}`).isVisible());
        assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
        await page.screenshot({ path: `/tmp/brand-login-${locale}-${theme}-${width}.png` });
      }
      await page.goto(base + '/cli-auth');
      await page.locator('.cli-auth-header .liaison-brand-lockup').waitFor();
      await page.screenshot({ path: `/tmp/brand-cli-${locale}-${theme}.png` });
      await page.setViewportSize({ width: 1440, height: 1000 });
      await page.goto(base + '/e2e/brand-lockup.html');
      await page.locator('.liaison-wordmark').waitFor();
      assert(await page.locator('.liaison-brand-mark').isVisible());
      assert.equal(await page.locator('.liaison-brand-copy small').innerText(), 'ZERO TRUST AI ACCESS');
      assert.equal(await page.locator('.liaison-wordmark path').count(), 1);
      assert.equal(await page.locator('.liaison-wordmark').getAttribute('width'), '66');
      const aiStyle = await page.locator('.liaison-wordmark tspan').evaluate(node => ({
        fill: getComputedStyle(node).fill,
        opacity: getComputedStyle(node).fillOpacity,
      }));
      assert.deepEqual(aiStyle, theme === 'dark'
        ? { fill: 'rgb(181, 122, 239)', opacity: '1' }
        : { fill: 'rgb(134, 44, 211)', opacity: '0.65' });
      await page.screenshot({ path: `/tmp/brand-home-${locale}-${theme}.png` });
      await page.getByRole('button', { name: locale === 'zh-CN' ? '收起侧栏' : 'Collapse sidebar', exact: true }).click();
      assert(await page.locator('.liaison-brand-mark').isVisible());
      assert.equal(await page.locator('.liaison-wordmark').count(), 0);
      await page.screenshot({ path: `/tmp/brand-collapsed-${locale}-${theme}.png` });
      await page.getByRole('button', { name: locale === 'zh-CN' ? '展开侧栏' : 'Expand sidebar', exact: true }).click();
      await page.setViewportSize({ width: 390, height: 844 });
      assert(!(await page.locator('.liaison-wordmark').isVisible()));
      assert(await page.locator('.liaison-brand-mark').isVisible());
      assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
      await page.screenshot({ path: `/tmp/brand-mobile-${locale}-${theme}.png` });
      assert.deepEqual(errors, []);
      await context.close();
      console.log('PASS brand, login exception, CLI, collapsed and mobile:', locale, theme);
    }
  } finally { await browser.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
