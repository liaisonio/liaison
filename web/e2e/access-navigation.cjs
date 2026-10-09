const assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
(async()=>{const browser=await chromium.launch();try{
for(const locale of ['zh-CN','en-US'])for(const theme of ['light','dark'])for(const width of [1440,390]){
const page=await browser.newPage({viewport:{width,height:900}});
await page.addInitScript(({locale,theme})=>{localStorage.setItem('liaison-locale',locale);localStorage.setItem('liaison-theme-preference',theme);},{locale,theme});
await page.route('**/api/v1/**',r=>r.fulfill({json:{code:200,data:{}}}));
await page.goto(`${process.env.E2E_UI_URL||'http://127.0.0.1:5303'}/e2e/brand-lockup.html`);
await page.locator('.liaison-nav-toggle').first().waitFor();
await page.evaluate(async()=>{const {usePermissions}=await import('/src/store/permissions.tsx');usePermissions.setState({owner:'brand-fixture',loaded:true,grants:{'ai.access.use':true}});});
await page.locator('.liaison-nav-toggle').first().click();
const links=page.locator('.liaison-nav-children').first().locator('a');
assert.deepEqual(await links.allTextContents(),[locale==='zh-CN'?'全部访问':'All access','Web','LLM','Agent','IDE','SSH/SFTP','TCP','Database','Cache','Desktop','Storage']);
assert.equal(await links.nth(3).getAttribute('href'),'/access/agents');
assert.equal(await links.nth(4).getAttribute('href'),'/access/webide');
assert.equal(await links.nth(6).getAttribute('href'),'/proxy?category=tcp');
await page.screenshot({path:`/tmp/access-order-${locale}-${theme}-${width}.png`});
await page.evaluate(async()=>{const {usePermissions}=await import('/src/store/permissions.tsx');usePermissions.setState({grants:{}});});
await page.waitForFunction(()=>!Array.from(document.querySelectorAll('.liaison-nav-child')).some(e=>e.textContent==='Agent'));
console.log('PASS',locale,theme,width,'order, links, permissions');await page.close();
}}finally{await browser.close();}})().catch(e=>{console.error(e);process.exitCode=1;});
