const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright'),assert=require('node:assert/strict');
(async()=>{const browser=await chromium.launch();try{for(const locale of ['zh-CN','en-US'])for(const theme of ['dark','light']){
const ctx=await browser.newContext({viewport:{width:1440,height:1000}});
await ctx.addInitScript(({locale,theme})=>{localStorage.setItem('liaison-locale',locale);localStorage.setItem('liaison-theme-preference',theme)},{locale,theme});
let native=true;const reads=[];
await ctx.route('**/api/v1/**',async r=>{const path=new URL(r.request().url()).pathname;reads.push(path);await r.fulfill({json:{code:200,data:path.endsWith('/workspace')?{name:'Native model access',enabled:true,models:['chat'],can_manage:false,external_protocol:'openai-compatible',external_protocols:native?['openai-compatible','anthropic']:['openai-compatible']}:[]}})});
const page=await ctx.newPage();await page.goto(`${process.env.E2E_UI_URL}/e2e/ollama.html?workspace`);
const select=page.locator('label').filter({has:page.getByText(locale==='zh-CN'?'调用 API':'Request API',{exact:true})}).locator('select');await select.selectOption('anthropic');
const example=page.locator('.ai-api-example');assert((await example.textContent()).includes('/v1/messages'));assert((await example.textContent()).includes('x-api-key: $LIAISON_API_KEY'));assert((await example.textContent()).includes('max_tokens'));assert(!reads.includes('/api/v1/ai/accesses/1'));
await page.screenshot({path:`/tmp/anthropic-workspace-${locale}-${theme}.png`});await page.setViewportSize({width:390,height:844});assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));await page.screenshot({path:`/tmp/anthropic-workspace-mobile-${locale}-${theme}.png`});
await select.selectOption('openai-compatible');assert((await example.textContent()).includes('/v1/chat/completions'));native=false;await page.reload();await example.waitFor();assert.equal(await select.count(),0);
console.log('PASS native capability, examples, consumer isolation, mobile',locale,theme);await ctx.close();
}}finally{await browser.close()}})().catch(e=>{console.error(e);process.exitCode=1});
