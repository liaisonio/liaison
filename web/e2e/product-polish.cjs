const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const assert=require('node:assert/strict');
(async()=>{
 const browser=await chromium.launch();
 try{for(const locale of ['zh-CN','en-US'])for(const theme of ['dark','light']){
  const ctx=await browser.newContext({viewport:{width:1440,height:1000}}),zh=locale==='zh-CN';let writes=0,offline=false,existing=false,model=false,failed=false;
  await ctx.addInitScript(({locale,theme})=>{localStorage.setItem('liaison-locale',locale);localStorage.setItem('liaison-theme-preference',theme)},{locale,theme});
  await ctx.route('**/api/v1/**',async route=>{
   const req=route.request(),path=new URL(req.url()).pathname;let data={};
   if(req.method()!=='GET')writes++;
   if(failed&&(path.endsWith('/agent/status')||path.includes('/edges')))return route.fulfill({status:503,json:{code:503}});
   if(path.endsWith('/agent/status'))data={enabled:model,models:model?[{provider_id:'local',model:'local-model',label:'Local model'}]:[]};
   else if(path.endsWith('/events'))return route.fulfill({contentType:'text/event-stream',body:': keepalive\n\n'});
   else if(path==='/api/v1/agent/sessions')data={items:[]};
   else if(path.includes('/agent/sessions/')){
    if(offline)return route.abort('failed');
    data={session:{id:'session-fixture',kind:'access',version:1,status:0},attachments:[],turns:[],messages:[],steps:[],approvals:[]};
   }else if(path.includes('/edges'))data={edges:existing?[{id:1,name:'existing'}]:[]};
   else if(path.includes('/devices'))data={devices:[]};
   else if(path.includes('/applications'))data={applications:[]};
   else if(path.includes('/proxies'))data={proxies:[]};
   if(req.method()==='POST'&&path==='/api/v1/agent/sessions'){
    if(offline)return route.abort('failed');
    data={session:{id:'session-fixture',kind:'access',version:1,status:0},attachments:[],turns:[],messages:[],steps:[],approvals:[]};
   }
   await route.fulfill({json:{code:200,data}});
  });
  const page=await ctx.newPage(),errors=[];page.on('pageerror',e=>errors.push(e.message));
  await page.goto(`${process.env.E2E_UI_URL}/e2e/product-polish.html`);
  await page.getByRole('heading',{name:zh?'开始使用':'Get started',exact:true}).waitFor();
  const setupModel=()=>page.getByRole('link',{name:zh?'配置模型':'Set up model',exact:true});
  await setupModel().waitFor();
  const create=()=>page.getByRole('link',{name:zh?'创建连接器':'Create connector',exact:true});
  await page.screenshot({path:`/tmp/product-polish-home-${locale}-${theme}.png`});
  await create().click();await page.getByRole('dialog').waitFor();assert.equal(writes,0,'Opening connector wizard must not create resources');
  existing=true;await page.goto(`${process.env.E2E_UI_URL}/e2e/product-polish.html`);
  await setupModel().waitFor();assert.equal(await create().count(),0,'Existing connector needs no guide');
  await page.setViewportSize({width:390,height:844});await page.screenshot({path:`/tmp/product-polish-home-${locale}-${theme}-mobile.png`});
  assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
  model=true;await page.goto(`${process.env.E2E_UI_URL}/e2e/product-polish.html`);
  await page.locator('.management-agent-presets').waitFor();assert.equal(await page.locator('.management-agent-onboarding').count(),0,'Fully configured has no onboarding');
  existing=false;await page.goto(`${process.env.E2E_UI_URL}/e2e/product-polish.html`);
  await create().waitFor();assert.equal(await setupModel().count(),0,'Configured model needs no guide');
  model=false;await page.goto(`${process.env.E2E_UI_URL}/e2e/product-polish.html`);
  await setupModel().waitFor();await create().waitFor();
  await page.screenshot({path:`/tmp/product-polish-setup-${locale}-${theme}-mobile.png`});
  failed=true;await page.goto(`${process.env.E2E_UI_URL}/e2e/product-polish.html`);await page.locator('[role=alert]').waitFor();
  assert.equal(await page.locator('.management-agent-onboarding').count(),0,'API failure is not empty configuration');failed=false;
  await page.goto(`${process.env.E2E_UI_URL}/e2e/product-polish.html?llm`);
  await page.getByText(zh?'接入本地模型':'Connect a local model',{exact:true}).waitFor();
  assert.equal(writes,0);
  // Fail the initial session load, then retry without resending a user message.
  await ctx.route('**/api/v1/agent/status',route=>route.fulfill({json:{code:200,data:{enabled:true,models:[]}}}));
  offline=true;await page.goto(`${process.env.E2E_UI_URL}/e2e/session-model.html`);
  const retry=page.getByRole('button',{name:zh?'重试':'Retry',exact:true});await retry.waitFor();
  assert(!(await page.locator('body').innerText()).includes('Failed to fetch'));
  offline=false;await retry.click();await retry.waitFor({state:'hidden'});
  await page.locator('.agent-workspace textarea').fill('keep this draft');
  offline=true;await retry.waitFor({timeout:12000});
  offline=false;await retry.waitFor({state:'hidden',timeout:12000});
  assert.equal(await page.locator('.agent-workspace textarea').inputValue(),'keep this draft');
  await page.screenshot({path:`/tmp/product-polish-recovered-${locale}-${theme}.png`});
  assert.deepEqual(errors,[]);console.log('PASS',locale,theme,'onboarding, no implicit writes, network retry and recovery');await ctx.close();
 }}finally{await browser.close()}
})().catch(e=>{console.error(e);process.exitCode=1});
