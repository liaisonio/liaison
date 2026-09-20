const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const assert=require('node:assert/strict');
(async()=>{
const browser=await chromium.launch();
try{for(const locale of ['zh-CN','en-US'])for(const theme of ['dark','light']){
 const ctx=await browser.newContext({viewport:{width:1440,height:1000}}),zh=locale==='zh-CN';
 await ctx.addInitScript(({locale,theme})=>{localStorage.setItem('liaison-locale',locale);localStorage.setItem('liaison-theme-preference',theme)},{locale,theme});
 let session={id:'session-fixture',kind:'access',version:1,model_selection:{provider_id:'',model:''}},fail=false,patches=0;
 const detail=()=>({session,attachments:[],turns:[],steps:[],messages:[],approvals:[]});
 await ctx.route('**/api/v1/**',async route=>{
  const req=route.request(),path=new URL(req.url()).pathname;let data={};
  if(path.endsWith('/agent/status'))data={enabled:true,models:[{provider_id:'one',provider_type:'openai',model:'model-default',is_default:true},{provider_id:'two',provider_type:'anthropic',model:'model-alternate',is_default:false}]};
  else if(path.endsWith('/events'))return route.fulfill({contentType:'text/event-stream',body:': keepalive\n\n'});
  else if(req.method()==='PATCH'){
   patches++;if(fail){fail=false;return route.fulfill({status:409,json:{code:409,message:'Conflict'}})}
   const body=req.postDataJSON();assert.equal(body.version,session.version);session={...session,model_selection:body.model_selection,version:session.version+1};data=session;
  }else if(path.includes('/agent/sessions'))data=detail();
  await route.fulfill({json:{code:200,data}});
 });
 const page=await ctx.newPage(),errors=[];page.on('pageerror',e=>errors.push(e.message));
 const select=()=>page.getByRole('combobox',{name:zh?'对话模型':'Conversation model'});
 const choose=async(name)=>{await select().click();await page.getByRole('option',{name,exact:false}).first().click();await page.waitForTimeout(100);};
 await page.goto(`${process.env.E2E_UI_URL}/e2e/session-model.html`);
 await select().waitFor();await choose('model-alternate');
 await page.waitForFunction(()=>document.querySelector('.agent-model-selector')?.textContent.includes('model-alternate'));
 assert.equal(session.model_selection.model,'model-alternate');assert.equal(patches,1);
 await page.reload();await select().waitFor();assert.match(await select().innerText(),/model-alternate/);
 await page.screenshot({path:`/tmp/session-model-${locale}-${theme}.png`});
 await choose('model-default');assert.equal(session.model_selection.provider_id,'');
 fail=true;await choose('model-alternate');await page.locator('.agent-workspace-error').waitFor();assert.equal(session.model_selection.provider_id,'');
 await page.setViewportSize({width:390,height:844});await page.screenshot({path:`/tmp/session-model-${locale}-${theme}-mobile.png`});
 assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
 await page.goto(`${process.env.E2E_UI_URL}/e2e/session-model.html?shell`);session.kind='shell';
 await choose('model-alternate');assert.equal(session.model_selection.model,'model-alternate');
 await page.screenshot({path:`/tmp/session-model-shell-${locale}-${theme}.png`});
 assert.deepEqual(errors,[]);console.log('PASS',locale,theme,'access/shell switch, persistence, reset, failure, mobile');await ctx.close();
}}finally{await browser.close()}
})().catch(e=>{console.error(e);process.exitCode=1});
