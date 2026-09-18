const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const assert=require('node:assert/strict');
(async()=>{const browser=await chromium.launch();try{
 for(const locale of ['zh-CN','en-US'])for(const theme of ['dark','light'])for(const width of [1440,390]){
  const ctx=await browser.newContext({viewport:{width,height:900}});let turns=0,fail=false;
  await ctx.addInitScript(({locale,theme})=>{localStorage.setItem('liaison-locale',locale);localStorage.setItem('liaison-theme-preference',theme);},{locale,theme});
  const entries=[{id:'12',name:'Analytics database with a long display name',type:theme==='light'?'web':'webmysql',state:'running'},{id:'13',name:'Disabled database',type:'webmysql',state:'stopped'}];
  await ctx.route('**/api/v1/**',route=>{const p=new URL(route.request().url()).pathname;let data={};
   if(p.endsWith('/events'))return route.fulfill({contentType:'text/event-stream',body:': keepalive\n\n'});
   if(p.endsWith('/turns'))turns++;
   if(p.includes('/webdata/proxies/')){if(fail)return route.fulfill({status:403,json:{code:403,message:'denied'}});data={protocol:'mysql',credentials:[{id:4}]};}
   else if(p==='/api/v1/proxies')data={proxies:[{id:12,name:entries[0].name,access_protocol:'web',status:'running',application:{application_type:'mysql'}}]};
   else if(p.endsWith('/status'))data={enabled:true,models:[]};
   else if(p.includes('/agent/sessions')){const home=p.includes('session_home');data={session:{id:home?'session_home':'session_target',kind:home?'management':'access'},messages:home?[{id:'u',sequence:1,value:{role:'user',content:'Find my databases'}},{id:'t',sequence:2,value:{role:'tool',tool_name:'access.list',content:JSON.stringify({Content:{items:entries}})}}]:[],steps:[],turns:[],attachments:[],approvals:[]};}
   return route.fulfill({json:{code:200,data}});
  });
  const page=await ctx.newPage(),errors=[];page.on('pageerror',e=>errors.push(e.message));await page.goto(`${process.env.E2E_UI_URL}/e2e/agent-handoff.html`);
  const cards=page.locator('.agent-access-result');await cards.first().waitFor();assert.equal(await cards.count(),2);assert(await cards.nth(1).getByRole('button').isDisabled());
  await cards.first().getByRole('button').click();const dialog=page.getByRole('dialog');await dialog.waitFor();assert.equal(await dialog.getByRole('checkbox').isChecked(),false);
  await dialog.getByRole('checkbox').check();await dialog.locator('textarea').fill('Analyze slow queries without changing data');
  await page.screenshot({path:`/tmp/handoff-dialog-${locale}-${theme}-${width}.png`});
  fail=true;await dialog.getByRole('button',{name:locale==='zh-CN'?'打开':'Open',exact:true}).click();await dialog.locator('.liaison-notice').waitFor();assert.equal(turns,0);
  fail=false;await dialog.getByRole('button',{name:locale==='zh-CN'?'打开':'Open',exact:true}).click();await page.locator('.agent-handoff-preview').waitFor();assert.equal(turns,0);
  await page.locator('.agent-handoff-preview').getByRole('button',{name:locale==='zh-CN'?'填入草稿':'Use draft'}).click();
  const input=page.locator('.agent-composer textarea');assert.equal(await input.inputValue(),'Existing draft\n\nAnalyze slow queries without changing data');assert.equal(await page.locator('.agent-handoff-preview').count(),0);assert.equal(turns,0);
  await page.screenshot({path:`/tmp/handoff-draft-${locale}-${theme}-${width}.png`});
  assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));assert.deepEqual(errors,[]);
  await page.evaluate(()=>{const t=window.handoffTest;const id=t.stageAccessDraft({accessId:12,name:'A',prompt:'private'});if(t.accessDraft(id,13))throw Error('cross access');t.discardAccessDraft(id);if(t.accessDraft(id,12))throw Error('replay');const other=t.stageAccessDraft({accessId:12,name:'A',prompt:'private'});t.setToken('other-user');if(t.accessDraft(other,12))throw Error('cross user');if(t.accessResults(JSON.stringify({IsError:true,Content:{items:[{id:'12',name:'A',type:'webmysql',state:'running'}]}})).length)throw Error('failed result');});
  await ctx.close();console.log('PASS handoff consent, retry, draft append, no execution, isolation',locale,theme,width);
 }
}finally{await browser.close();}})().catch(e=>{console.error(e);process.exitCode=1;});
