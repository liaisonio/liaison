const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const assert=require('node:assert/strict');
(async()=>{
 const browser=await chromium.launch();
 try{for(const locale of ['zh-CN','en-US'])for(const theme of ['light','dark']){
  const context=await browser.newContext({viewport:{width:1440,height:1000}});
  await context.addInitScript(({locale,theme})=>{localStorage.setItem('liaison-locale',locale);localStorage.setItem('liaison-theme-preference',theme);},{locale,theme});
  let status='reachable',delay=0,writes=0,probes=[];
  await context.route('**/api/v1/**',async route=>{
   const path=new URL(route.request().url()).pathname;
   if(path.endsWith('/applications/probe')){
    probes.push(route.request().postDataJSON());const reply=status;
    if(delay)await new Promise(resolve=>setTimeout(resolve,delay));
    return route.fulfill({json:{code:200,data:{status:reply,duration_ms:12}}}).catch(()=>{});
   }
   if(route.request().method()!=='GET')writes++;
   const data=path.endsWith('/edges')?{edges:[{id:7,name:'Local connector',online:1,status:1}]}:path.endsWith('/applications')?{applications:[]}:{};
   await route.fulfill({json:{code:200,data}});
  });
  const page=await context.newPage();const errors=[];page.on('pageerror',e=>errors.push(e.message));
  await page.goto(`${process.env.E2E_UI_URL}/e2e/product-polish.html?app`);
  const zh=locale==='zh-CN';await page.getByRole('button',{name:zh?'新建应用':'Create application',exact:true}).click();
  const form=page.locator('#create-application');const button=()=>form.getByRole('button',{name:zh?'测试连接':'Test connection',exact:true});
  assert(await button().isDisabled());await form.locator('select').nth(1).selectOption('7');
  await form.locator('input[list]').fill('127.0.0.1');await button().click();
  await form.getByText(/12 ms/).waitFor();assert.deepEqual(probes[0],{edge_id:7,host:'127.0.0.1',port:443});
  for(const width of [1440,390]){
   await page.setViewportSize({width,height:width===390?844:1000});await page.screenshot({path:`/tmp/app-probe-${locale}-${theme}-${width}.png`,fullPage:true});
   assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
  }
  for(const next of ['refused','timeout','connector_offline','edge_upgrade_required']){
   status=next;await button().click();await form.locator('.liaison-notice.is-warning').waitFor();
   await button().waitFor();assert(await form.locator('.liaison-notice').innerText());
  }
  delay=400;status='reachable';await button().click();await form.locator('input[type=number]').fill('8443');
  await page.waitForTimeout(500);assert.equal(await form.locator('.liaison-notice').count(),0);
  assert(await button().isEnabled());assert.equal(writes,0);assert.deepEqual(errors,[]);
  console.log('PASS application probe',locale,theme,'validation, result, errors, stale cancellation, no save, mobile');await context.close();
 }}finally{await browser.close();}
})().catch(e=>{console.error(e);process.exitCode=1;});
