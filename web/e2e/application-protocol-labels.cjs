const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const assert=require('node:assert/strict');
(async()=>{const browser=await chromium.launch();try{
 for(const locale of ['zh-CN','en-US'])for(const theme of ['dark','light']){
  const c=await browser.newContext({viewport:{width:1440,height:1000}});
  await c.addInitScript(({locale,theme})=>{localStorage.setItem('liaison-locale',locale);localStorage.setItem('liaison-theme-preference',theme)},{locale,theme});
  const types=['qwen','anthropic','memcached','elasticsearch','postgresql','openai','ollama','ark','gemini','llm'];
  await c.route('**/api/v1/**',r=>{const path=new URL(r.request().url()).pathname;return r.fulfill({json:{code:200,data:path==='/api/v1/applications'?{applications:types.map((type,i)=>({id:i+1,name:'Example '+type,application_type:type,ip:'127.0.0.1',port:18082}))}:path==='/api/v1/edges'?{edges:[]}:{proxies:[]}}});});
  const page=await c.newPage();await page.goto(process.env.E2E_UI_URL+'/e2e/llm-application.html');await page.locator('.liaison-application-protocol').first().waitFor();
  assert.equal(await page.locator('.liaison-application-protocol').nth(0).innerText(),'QWen');
  assert.equal(await page.locator('.liaison-application-protocol').nth(1).innerText(),'Anthropic');
  assert.equal(await page.locator('.liaison-application-protocol').nth(2).innerText(),'Memcached');
  await page.locator('.liaison-application-protocol').first().hover();assert.equal(await page.locator('.liaison-application-protocol').first().getAttribute('title'),'QWen');
  for(const width of [1440,390]){
   await page.setViewportSize({width,height:1000});
   assert(await page.locator('.liaison-application-protocol .liaison-status').evaluateAll(nodes=>nodes.every(n=>{const a=n.getBoundingClientRect(),b=n.closest('td').getBoundingClientRect();return a.left>=b.left&&a.right<=b.right-10;})),'Badge clipped by protocol cell');
   assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),'Page overflow');
   if(width===390){await page.locator('.liaison-table-scroll').hover();await page.mouse.wheel(190,0);await page.waitForTimeout(150);assert.equal(await page.locator('td.is-fixed-right').first().evaluate(n=>getComputedStyle(n).position),'static');}
   await page.screenshot({path:`/tmp/application-labels-${locale}-${theme}-${width}.png`});
  }
  await c.close();console.log('PASS application labels and bounds',locale,theme);
 }
}finally{await browser.close()}})().catch(e=>{console.error(e);process.exitCode=1});
