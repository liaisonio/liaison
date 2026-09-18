const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const assert=require('node:assert/strict');
(async()=>{
 const browser=await chromium.launch();
 try{for(const locale of ['zh-CN','en-US'])for(const theme of ['dark','light']){
  const context=await browser.newContext({viewport:{width:1440,height:1000}});
  await context.addInitScript(({locale,theme})=>{localStorage.setItem('liaison-locale',locale);localStorage.setItem('liaison-theme-preference',theme);},{locale,theme});
  let executions=0;const errors=[];
  const detail={session:{id:'session_abcdef',kind:'access',status:0},attachments:[{id:'data-fixture',access_id:101}],messages:[{id:'user',value:{role:'user',content:'```sql\nuser draft\n```'}},{id:'answer',value:{role:'assistant',content:'Suggested query:\n```sql\nSELECT 42 AS total;\n```'}}],turns:[],approvals:[],steps:[]};
  await context.route('**/api/v1/**',async route=>{
   const path=new URL(route.request().url()).pathname;let data={};
   if(path.endsWith('/agent/status'))data={enabled:true};
   else if(path.endsWith('/events'))return route.fulfill({contentType:'text/event-stream',body:': keepalive\n\n'});
   else if(path.includes('/agent/sessions'))data=detail;
   else if(path.endsWith('/webdata/proxies/101'))data={protocol:'mysql',proxy_name:'Data workspace',target_host:'data.example',target_port:3306,effective_status:'active',credentials:[{id:7,protocol:'mysql',name:'Fixture',username:'demo',saved:true,database:'demo'}]};
   else if(path.endsWith('/session'))data={token:'data-fixture',protocol:'mysql'};
   else if(path.endsWith('/metadata'))data={nodes:[]};
   else if(path.endsWith('/execute'))executions++;
   await route.fulfill({json:{code:200,data}});
  });
  const page=await context.newPage();page.on('pageerror',e=>errors.push(e.message));
  const zh=locale==='zh-CN',button=(cn,en)=>page.getByRole('button',{name:zh?cn:en,exact:true});
  await page.goto(`${process.env.E2E_UI_URL}/e2e/data-context.html`);
  const editor=page.locator('.webdata-code-editor textarea');await editor.fill('SELECT 1;');await button('Agent','Agent').click();
  // Session binding may remount the Agent portal after the first visible frame.
  // Wait for the exact handoff count instead of reading during that transition.
  await page.waitForFunction(label=>Array.from(document.querySelectorAll('button')).filter(el=>el.textContent.trim()===label).length===1,zh?'预览填入':'Preview in editor');
  await page.screenshot({path:`/tmp/editor-handoff-code-${locale}-${theme}.png`});
  await button('预览填入','Preview in editor').click();const dialog=page.getByRole('dialog');await dialog.waitFor();
  const proposed=dialog.locator('textarea').last();assert.equal(await proposed.inputValue(),'SELECT 42 AS total;');
  await button('取消','Cancel').click();assert.equal(await editor.inputValue(),'SELECT 1;');
  await button('预览填入','Preview in editor').click();
  await editor.evaluate(el=>{Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype,'value').set.call(el,'SELECT 2;');el.dispatchEvent(new Event('input',{bubbles:true}));});
  await page.getByRole('alert').waitFor();assert(await button('替换草稿，不执行','Replace draft, do not run').isDisabled());
  await button('取消','Cancel').click();await button('预览填入','Preview in editor').click();
  await proposed.fill('汉'.repeat(5000));assert(await button('替换草稿，不执行','Replace draft, do not run').isDisabled());
  await proposed.fill('SELECT\u001b 42');assert(await button('替换草稿，不执行','Replace draft, do not run').isDisabled());
  await proposed.fill('SELECT 43 AS total;');await page.waitForFunction(()=>document.querySelectorAll('.liaison-toast').length===0);
  await page.screenshot({path:`/tmp/editor-handoff-preview-${locale}-${theme}.png`});
  await page.setViewportSize({width:390,height:844});await page.screenshot({path:`/tmp/editor-handoff-mobile-${locale}-${theme}.png`});
  assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));
  await button('替换草稿，不执行','Replace draft, do not run').click();await dialog.waitFor({state:'hidden'});
  await page.waitForFunction(()=>document.querySelector('.webdata-code-editor textarea')?.value==='SELECT 43 AS total;');
  await page.locator('.agent-workspace').waitFor({state:'hidden'});assert(await editor.isVisible());assert.equal(executions,0);assert.deepEqual(errors,[]);
  await context.close();console.log('PASS editor handoff, explicit confirmation, stale protection, byte/control limits, no execution:',locale,theme);
 }}finally{await browser.close();}
})().catch(e=>{console.error(e);process.exitCode=1;});
