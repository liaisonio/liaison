const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const assert=require('node:assert/strict');
(async()=>{
 const browser=await chromium.launch(process.env.CHROME_EXECUTABLE?{executablePath:process.env.CHROME_EXECUTABLE}:undefined);
 try { for(const locale of ['zh-CN','en-US']) for(const theme of ['light','dark']) {
  const context=await browser.newContext({viewport:{width:1440,height:1000}});
  await context.addInitScript(({locale,theme})=>{localStorage.setItem('liaison-locale',locale);localStorage.setItem('liaison-theme-preference',theme);},{locale,theme});
  let stage='connector',fail=false,writes=0;const zh=locale==='zh-CN';
  await context.route('**/api/v1/**',async route=>{
   const path=new URL(route.request().url()).pathname;
   if(route.request().method()!=='GET')writes++;
   if(fail&&path.endsWith('/edges'))return route.fulfill({status:503,json:{code:503}});
   let data={};
   if(path.endsWith('/workspace'))data={models:['local-model'],upstream_protocol:'openai',external_protocol:'openai',enabled:true};
   if(path.endsWith('/edges'))data={edges:stage==='connector'?[]:[{id:1,name:'Connector'}]};
   if(path.endsWith('/applications'))data={applications:['access','existing'].includes(stage)?[{id:1,name:'Local model',application_type:'openai'}]:[]};
   if(path.endsWith('/proxies'))data={proxies:stage==='existing'?[{id:1,name:'Local API',access_type:'openai',access_protocol:'openai',status:'running',application:{id:1,name:'Local model',application_type:'openai'}}]:[]};
   await route.fulfill({json:{code:200,data}});
  });
  const page=await context.newPage();const errors=[];page.on('pageerror',error=>errors.push(error.message));
  const visit=()=>page.goto(`${process.env.E2E_UI_URL}/e2e/product-polish.html?llm`);
  for(const [next,cn,en] of [['connector','创建连接器','Create connector'],['application','添加模型应用','Add model application'],['access','新建访问','Create access']]){
   stage=next;await visit();
   const empty=page.locator('.liaison-list-panel .liaison-llm-empty');await empty.getByRole('button',{name:zh?cn:en,exact:true}).waitFor();
   assert.equal(await empty.locator('button').count(),2);assert.equal(await page.locator('.liaison-notice').count(),0);
   if(stage==='access'){await empty.getByRole('button',{name:zh?cn:en,exact:true}).click();await page.getByRole('dialog').waitFor();await page.getByRole('button',{name:zh?'取消':'Cancel',exact:true}).click();}
  }
  for(const width of [1440,390]){
   await page.setViewportSize({width,height:width===390?844:1000});
   await page.screenshot({path:`/tmp/llm-empty-${locale}-${theme}-${width}.png`,fullPage:true});
   assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
  }
  await page.locator('.liaison-compound input').fill('no-match');
  await page.locator('.liaison-llm-empty').waitFor({state:'hidden'});
  stage='existing';await visit();await page.getByText('Local API',{exact:true}).waitFor();
  assert.equal(await page.locator('.liaison-llm-empty').count(),0);
  await page.locator('.liaison-compound input').fill('missing');await page.getByText('Local API',{exact:true}).waitFor({state:'hidden'});
  assert.equal(await page.locator('.liaison-llm-empty').count(),0);
  stage='connector';fail=true;await visit();await page.locator('.liaison-llm-empty [role=alert]').waitFor();
  fail=false;await page.getByRole('button',{name:zh?'重试':'Retry',exact:true}).click();
  await page.locator('.liaison-llm-empty').getByRole('button',{name:zh?'创建连接器':'Create connector',exact:true}).waitFor();
  assert.equal(writes,0);assert.deepEqual(errors,[]);console.log('PASS',locale,theme,'next step, existing access, filters, retry, mobile');await context.close();
 }}finally{await browser.close();}
})().catch(error=>{console.error(error);process.exitCode=1;});
