const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const assert=require('node:assert/strict');
(async()=>{
 const browser=await chromium.launch();
 try {
  const context=await browser.newContext({viewport:{width:1440,height:1000}});
  await context.addInitScript(()=>localStorage.setItem('liaison-locale','en-US'));
  const turns=[];let restoreGate;
  await context.route('**/api/v1/**',async route=>{
   const path=new URL(route.request().url()).pathname;
   if(restoreGate && route.request().method()==='GET' && /\/agent\/sessions\/[^/]+$/.test(path))await restoreGate;
   let data={};
   if(path.endsWith('/agent/status'))data={enabled:true};
   else if(path.endsWith('/events'))return route.fulfill({contentType:'text/event-stream',body:': keepalive\n\n'});
   else if(path.endsWith('/turns'))turns.push(path);
   else if(path.includes('/agent/sessions'))data={session:{id:path.includes('session_second')?'session_second':'session_first',kind:'access',status:0},attachments:[{id:'same-handle',access_id:101}],messages:[],turns:[],approvals:[],steps:[]};
   else data={id:'conn_fixture'};
   await route.fulfill({json:{code:200,data}});
  });
  const page=await context.newPage();await page.goto(`${process.env.E2E_UI_URL}/e2e/agent-session-isolation.html`);
  const input=page.locator('.agent-workspace textarea');await input.fill('Private draft in first conversation');
  await page.getByRole('button',{name:'Send',exact:true}).waitFor({state:'visible'});
  await page.waitForFunction(()=>document.querySelector('.agent-composer-toolbar button[aria-label="Send"]')?.disabled===false);
  assert.equal(await input.inputValue(),'Private draft in first conversation','Initial session route binding must preserve the draft');
  await page.getByRole('button',{name:'Toggle panel',exact:true}).click();
  let releaseRestore;
  restoreGate=new Promise(resolve=>{releaseRestore=resolve;});
  const restoring=page.waitForRequest(request=>request.method()==='GET' && /\/agent\/sessions\/[^/]+$/.test(new URL(request.url()).pathname));
  await page.getByRole('button',{name:'Toggle panel',exact:true}).click();
  await restoring;
  assert.equal(await input.inputValue(),'Private draft in first conversation','Closing the same conversation keeps its draft');
  await page.waitForFunction(()=>document.querySelector('.agent-composer-toolbar button[aria-label="Send"]')?.disabled===true);
  assert(await page.getByRole('button',{name:'Send',exact:true}).isDisabled(),'Send must wait for session restoration');
  releaseRestore();restoreGate=undefined;
  await page.waitForFunction(()=>document.querySelector('.agent-composer-toolbar button[aria-label="Send"]')?.disabled===false);
  await page.getByRole('button',{name:'Switch conversation',exact:true}).click();
  await page.waitForFunction(()=>document.querySelector('.agent-workspace textarea')?.value==='');
  assert.equal(turns.length,0,'Switching conversations must not send the previous draft');
  console.log('PASS same-handle conversation draft isolation; initial binding and panel reopening preserve draft');
 }finally{await browser.close();}
})().catch(error=>{console.error(error);process.exitCode=1;});
