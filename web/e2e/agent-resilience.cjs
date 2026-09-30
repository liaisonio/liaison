const assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const origin=process.env.E2E_UI_URL||'http://127.0.0.1:5303';
const access={id:'a'.repeat(32),name:'Resilience test',kind:'codex',edge_id:1,installation_id:'b'.repeat(32),project:'/workspace/demo'};
const main='c'.repeat(32),second='d'.repeat(32);
const delay=ms=>new Promise(r=>setTimeout(r,ms));
(async()=>{
 const browser=await chromium.launch(process.env.CHROME_EXECUTABLE?{executablePath:process.env.CHROME_EXECUTABLE}:undefined);
 try{
  for(const locale of ['zh-CN','en-US'])for(const theme of ['light','dark'])for(const width of [1280,390]){
   const page=await browser.newPage({viewport:{width,height:950},ignoreHTTPSErrors:true});
   let owner=1,failSend=true,searches=0,changed=false,requestFailure=false,sendDelay=200,sendCount=0;
   const errors=[];page.on('pageerror',e=>errors.push(e.message));
   const snapshots=new Map([main,second].map((id,i)=>[id,{version:1,status:'ok',session_id:id,title:i?'Second':'First',project:access.project,running:false,closed:false,messages:[{role:'assistant',text:'Ready.'}],history_persistent:true}]));
   await page.addInitScript(({locale,theme})=>{localStorage.setItem('token','synthetic-test');localStorage.setItem('liaison-locale',locale);localStorage.setItem('liaison-theme-preference',theme);},{locale,theme});
   await page.route('**/api/v1/**',async route=>{
    const path=new URL(route.request().url()).pathname;let data={};
    if(path==='/api/v1/iam/account')data={id:owner,name:'Tester'};
    else if(path==='/api/v1/iam/permissions')data={'ai.access.use':true};
    else if(path.startsWith('/api/v1/agent-accesses/'))data=access;
    else if(path==='/api/v1/edge-agents/connectors')data=[{id:1,name:'Connector',online:true}];
    else if(path==='/api/v1/edge-agents'){
     const req=route.request().postDataJSON();
     if(req.action==='sessions'){
      if(requestFailure)return route.fulfill({status:503,json:{code:503}});
      const rows=req.history_search?Array.from({length:10},(_,i)=>({...snapshots.get(main),session_id:`archived-${i}`,title:'Archived '+i,running:changed,project:'/workspace/archive',updated_at:'2026-01-01T00:00:00Z'})):[...snapshots.values()];
      if(req.history_search)searches++;
      data={version:1,status:'ok',sessions_available:true,history_persistent:true,history_search_available:true,history_total:rows.length,sessions:rows};
     }else if(req.action==='poll')data=snapshots.get(req.session_id);
     else if(req.action==='transcript')data={version:1,status:'ok',history_pages:[]};
     else if(req.action==='send'){
      sendCount++;await delay(sendDelay);
      if(failSend)return route.fulfill({status:503,json:{code:503}});
      const previous=snapshots.get(req.session_id);data={...previous,request_service_ms:100,turn_timing:{dispatch_ms:5,first_reply_ms:60,finished_ms:80},messages:[...previous.messages,{role:'user',text:req.text},{role:'assistant',text:'Finished.'}]};snapshots.set(req.session_id,data);
     }else throw Error('Unexpected mutation '+req.action);
    }
    await route.fulfill({json:{code:200,data}});
   });
   const url=`${origin}/access/agents?access=${access.id}&session=${main}&view=full`;
   const input=()=>page.getByRole('textbox',{name:locale==='zh-CN'?'消息':'Message',exact:true});
   const waitFor=async(fn)=>{for(let n=0;n<100;n++){if(await fn())return;await delay(100);}throw Error('Condition timed out');};
   const showList=async()=>{const button=page.getByRole('button',{name:locale==='zh-CN'?'显示会话列表':'Show conversations',exact:true});if(await button.isVisible())await button.click();};
   const hideList=async()=>{const button=page.getByRole('button',{name:locale==='zh-CN'?'收起会话列表':'Hide conversations',exact:true});if(width<1000&&await button.isVisible())await button.click();};
   await page.goto(url);await input().waitFor();await waitFor(()=>input().isEnabled());await input().fill('Draft A');
   await showList();await page.locator('.edge-agent-session-item').filter({hasText:'Second'}).click();await waitFor(()=>input().isEnabled());assert.equal(await input().inputValue(),'');await input().fill('Draft B');
   await showList();await page.locator('.edge-agent-session-item').filter({hasText:'First'}).click();await waitFor(async()=>await input().inputValue()==='Draft A');
   await page.reload();await input().waitFor();await waitFor(async()=>await input().inputValue()==='Draft A');
   owner=2;await page.reload();await input().waitFor();await waitFor(()=>input().isEnabled());assert.equal(await input().inputValue(),'');
   owner=1;await page.reload();await input().waitFor();await waitFor(async()=>await input().inputValue()==='Draft A');
   await hideList();await page.getByRole('button',{name:locale==='zh-CN'?'发送':'Send',exact:true}).click();
   await page.getByRole('alert').waitFor();assert.equal(await input().inputValue(),'Draft A');
   failSend=false;await page.getByRole('button',{name:locale==='zh-CN'?'发送':'Send',exact:true}).click();await waitFor(async()=>await input().inputValue()==='');
   assert.equal(await page.evaluate(()=>Object.keys(sessionStorage).filter(k=>k.startsWith('liaison:agent-draft:')&&k.endsWith('c'.repeat(32))).length),0);
   await page.getByRole('button',{name:locale==='zh-CN'?'更多会话操作':'More conversation actions',exact:true}).click();
   await page.getByRole('menuitem',{name:locale==='zh-CN'?'会话详情':'Session details',exact:true}).click();
   const timing=page.locator('.edge-agent-timing').first();await timing.waitFor();assert.equal(await timing.locator('code').count(),4);assert(!(await timing.innerText()).includes('—'));assert.equal(await page.locator('.edge-agent-timing').nth(1).locator('code').count(),3);
   assert(await page.getByRole('dialog').evaluate(el=>el.scrollWidth<=el.clientWidth+1));await timing.scrollIntoViewIfNeeded();await page.screenshot({path:`/tmp/agent-timing-${locale}-${theme}-${width}.png`});
   await page.locator('.edge-agent-timing').nth(1).scrollIntoViewIfNeeded();await page.screenshot({path:`/tmp/agent-edge-timing-${locale}-${theme}-${width}.png`});
   await page.getByRole('dialog').getByRole('button',{name:locale==='zh-CN'?'关闭':'Close',exact:true}).last().click();
   if(width===1280){
    await input().fill('Delayed draft');sendDelay=1200;const before=sendCount;
    await page.getByRole('button',{name:locale==='zh-CN'?'发送':'Send',exact:true}).click();await waitFor(async()=>sendCount>before);
    await showList();await page.locator('.edge-agent-session-item').filter({hasText:'Second'}).click();await waitFor(async()=>await input().inputValue()==='Draft B');
    await waitFor(async()=>await page.evaluate(()=>Object.keys(sessionStorage).filter(k=>k.startsWith('liaison:agent-draft:')&&k.endsWith('c'.repeat(32))).length)===0);
    assert.equal(await input().inputValue(),'Draft B','late send success must not clear another conversation draft');
   }
   await showList();const search=page.getByRole('searchbox');await search.fill('archive');await page.getByText('Archived 9',{exact:true}).waitFor();assert.equal(await page.locator('.edge-agent-show-more').count(),0);
   changed=true;await page.evaluate(()=>{const old=Date.now;Date.now=()=>old()+31000;});
   await waitFor(async()=>searches>=2);await waitFor(async()=>await page.locator('.edge-agent-session-item .is-running').count()===10);
   // Fast status refresh errors must disappear after recovery without clearing search results.
   requestFailure=true;await page.getByRole('button',{name:locale==='zh-CN'?'刷新会话':'Refresh sessions',exact:true}).click();await page.getByText(locale==='zh-CN'?'无法加载会话列表，请检查连接器版本、连接状态与访问权限。':'Cannot load sessions. Check the connector version, connection and access permissions.').waitFor();
   requestFailure=false;await page.getByRole('button',{name:locale==='zh-CN'?'刷新会话':'Refresh sessions',exact:true}).click();await page.getByText('Archived 9',{exact:true}).waitFor();
   if(width===1280&&locale==='zh-CN'&&theme==='light'){
    const notice=page.locator('.edge-agent-sessions-page > .liaison-notice');
    requestFailure=true;await notice.waitFor();requestFailure=false;await notice.waitFor({state:'hidden'});
    assert.equal(await page.locator('.edge-agent-session-item').count(),10,'fast status failure/recovery preserves search rows');
   }
   assert.deepEqual(errors,[]);console.log('PASS scoped drafts, failed/successful send, timings, search refresh',locale,theme,width);await page.close();
  }
 }finally{await browser.close();}
})().catch(e=>{console.error(e);process.exitCode=1;});
