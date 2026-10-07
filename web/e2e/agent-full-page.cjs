// Exercise the real AppLayout, including authentication and access-loading gaps.
const assert = require('node:assert/strict');
const {chromium} = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const origin = process.env.E2E_UI_URL || 'http://127.0.0.1:5303';
const access = {id:'a'.repeat(32),name:'Layout verification',kind:'codex',edge_id:1,installation_id:'b'.repeat(32),project:'/workspace/demo'};
const answer = 'Layout ready.\n\n' + '检查文件与模型的对应关系。 Verify the project configuration and request handling. '.repeat(12) + '\n\n```go\nimport "example.org/project/log"\n\nopt.SetLog(log.New())\n'+ '// Example line\n'.repeat(20)+'// ' + 'long-output-'.repeat(100) + '\n```\n\n| File | Description |\n| --- | --- |\n| `src/example.ts` | Responsive preview |';
const session = {version:1,status:'ok',session_id:'c'.repeat(32),thread_id:'test-thread',project:access.project,title:'Layout verification',closed:true,running:false,messages:[{role:'user',text:'Check the layout. 检查布局。'},{role:'assistant',text:answer}],history_persistent:true};
const delay = ms => new Promise(resolve => setTimeout(resolve,ms));
(async()=>{
 const browser = await chromium.launch(process.env.CHROME_EXECUTABLE?{executablePath:process.env.CHROME_EXECUTABLE}:undefined);
 try {
  for(const locale of ['zh-CN','en-US'])for(const theme of ['light','dark'])for(const width of [1280,390]){
   const page=await browser.newPage({viewport:{width,height:900},ignoreHTTPSErrors:process.env.E2E_IGNORE_HTTPS_ERRORS==='1'});
   let failAccess=false,deny=false,replyVersion=1,needsApproval=true,emptyHistory=false,connectorOnline=true,activeConversation=false;const errors=[],starts=[];
   page.on('pageerror',()=>errors.push('browser error'));
   await page.addInitScript(({locale,theme})=>{
    localStorage.setItem('token','synthetic-layout-test');
    localStorage.setItem('liaison-locale',locale);
    localStorage.setItem('liaison-theme-preference',theme);
    window.shellFlashes=0;
    window.observeFullPage=true;
    const sample=()=>{
     const query=new URLSearchParams(location.search);
     if(window.observeFullPage&&location.pathname==='/access/agents'&&query.get('access')&&query.get('view')==='full'){
      if([...document.querySelectorAll('.liaison-global-header,.liaison-sidebar,.liaison-page-header')].some(el=>el.getClientRects().length&&getComputedStyle(el).visibility!=='hidden'))window.shellFlashes++;
     }
     requestAnimationFrame(sample);
    };requestAnimationFrame(sample);
   },{locale,theme});
   await page.route('**/api/v1/**',async route=>{
    const path=new URL(route.request().url()).pathname;
    let data={};
    if(path==='/api/v1/iam/account'){await delay(450);data={id:1,name:'Layout tester'};}
    else if(path==='/api/v1/iam/permissions'){await delay(750);data={'ai.access.use':!deny};}
    else if(path.startsWith('/api/v1/agent-accesses/')){
     await delay(450);if(failAccess)return route.fulfill({status:503,json:{code:503,message:'Unavailable'}});data=access;
    }else if(path==='/api/v1/agent-accesses')data={items:[],total:0};
    else if(path==='/api/v1/edge-agents/connectors')data=[{id:1,name:'Test connector',device:'Test device',online:connectorOnline}];
    else if(path==='/api/v1/edge-agents'){
     const {action,history_search,session_id}=route.request().postDataJSON();
     if(process.env.E2E_NEW_DIRECTORY==='1'&&action==='directories'){
      const directory=route.request().postDataJSON().directory||access.project;
      return route.fulfill({json:{code:200,data:{version:1,status:'ok',directory,directories:[{name:'another-project',path:'/workspace/another-project'}]}}});
     }
     if(process.env.E2E_NEW_DIRECTORY==='1'&&action==='start'){
      starts.push(route.request().postDataJSON());
      return route.fulfill({json:{code:200,data:{...session,project:starts.at(-1).working_directory||access.project}}});
     }
     assert(['sessions','poll','transcript'].includes(action),'layout tests must not start or modify a session');
     const rows=[{...session,reply_token:'main',updated_at:'2026-01-01T00:00:00Z'},...Array.from({length:11},(_,i)=>({...session,session_id:String(i).padStart(32,'d'),reply_token:`reply-${replyVersion}`,attention:i===0&&needsApproval?'approval':undefined,title:`Conversation ${i+1}`,running:i===0,updated_at:'2026-01-01T00:00:00Z'}))];
     const matches=emptyHistory?[]:history_search?[{...rows[1],title:'Older matching conversation',project:'/workspace/archive'}]:rows;
     data=action==='sessions'?{version:1,status:'ok',history_search_available:true,history_total:matches.length,attention_sessions:rows.filter(s=>s.attention),sessions_available:true,session_management:true,sessions:matches}:action==='transcript'?{version:1,status:'ok',history_pages:[]}:{...session,closed:!activeConversation,session_id:session_id||session.session_id,reply_token:session_id&&session_id!==session.session_id?`reply-${replyVersion}`:'main'};
    }
    await route.fulfill({json:{code:200,data}});
   });
   const url=`${origin}/access/agents?access=${access.id}&session=${session.session_id}&view=full`;
   const hidden=async()=>{
    await page.locator('.liaison-global-header').waitFor({state:'hidden'});
    assert.equal(await page.locator('.liaison-global-header').isVisible(),false);
    assert.equal(await page.locator('.liaison-sidebar').isVisible(),false);
    assert.equal(await page.evaluate(()=>window.shellFlashes),0,'product shell was visible during full-page initialization');
   };
   await page.goto(url);
   await page.locator('.liaison-app-frame').waitFor();
   await hidden();
   await page.getByText('Layout ready.',{exact:true}).waitFor();
   if(process.env.E2E_NEW_DIRECTORY==='1'){
    const label=locale==='zh-CN'?'新建会话':'New conversation';
    if(width===390)await page.getByRole('button',{name:locale==='zh-CN'?'显示会话列表':'Show conversations',exact:true}).click();
    for(const button of [page.locator('.edge-agent-session-list').getByRole('button',{name:label,exact:true}),page.locator('.edge-agent-project-group-heading').getByRole('button',{name:label+' · '+access.project,exact:true})]){
     await button.click();const dialog=page.getByRole('dialog');await dialog.waitFor();
     await dialog.getByRole('button',{name:locale==='zh-CN'?'在此目录新建':'Create in this folder'}).waitFor();
     assert.equal(starts.length,0,'opening folder picker must not create a session');
     await dialog.getByRole('button',{name:locale==='zh-CN'?'取消':'Cancel',exact:true}).click();
     assert(new URL(page.url()).searchParams.get('session')===session.session_id,'cancel preserves selected session');
    }
    if(width===390)await page.getByRole('button',{name:locale==='zh-CN'?'收起会话列表':'Hide conversations',exact:true}).click();
    await page.locator('.edge-agent-session-actions').getByRole('button',{name:label,exact:true}).click();
    const dialog=page.getByRole('dialog');await dialog.getByRole('button',{name:'another-project'}).click();
    await page.waitForFunction(()=>document.querySelector('.edge-agent-directory-path input')?.value==='/workspace/another-project'&&!document.querySelector('.edge-agent-directory-list[aria-busy="true"]'));
    await page.screenshot({path:`/tmp/agent-new-directory-${locale}-${theme}-${width}.png`});
    assert(await dialog.evaluate(el=>el.scrollWidth<=el.clientWidth+1));
    await dialog.getByRole('button',{name:locale==='zh-CN'?'在此目录新建':'Create in this folder'}).click();
    await page.waitForFunction(()=>!document.querySelector('[role="dialog"]'));await delay(900);
    assert.equal(starts.length,1);assert.equal(starts[0].working_directory,'/workspace/another-project');
    emptyHistory=true;await page.goto(`${origin}/access/agents?access=${access.id}&view=full`);await delay(2200);
    assert.equal(starts.length,1,'an empty history must not auto-create a session');
    const welcome=page.locator('.edge-agent-welcome');await welcome.waitFor();
    const chooseProject=welcome.getByRole('button',{name:locale==='zh-CN'?'选择项目目录':'Choose project folder',exact:true});
    assert(await chooseProject.isEnabled());
    assert.equal(await welcome.locator('.edge-agent-welcome-project').getAttribute('title'),access.project);
    await chooseProject.focus();
    await page.screenshot({path:`/tmp/agent-empty-${locale}-${theme}-${width}.png`});
    assert(await welcome.evaluate(el=>el.scrollWidth<=el.clientWidth+1));
    await chooseProject.click();await page.getByRole('dialog').waitFor();
    await page.getByRole('dialog').getByRole('button',{name:locale==='zh-CN'?'取消':'Cancel',exact:true}).click();
    assert.equal(starts.length,1,'canceling the empty-state picker must not create a session');
    connectorOnline=false;await page.reload();await welcome.waitFor();
    assert(await chooseProject.isDisabled());
    assert((await welcome.textContent()).includes(locale==='zh-CN'?'恢复在线':'connector is online'));
    assert.deepEqual(errors,[]);console.log('PASS choose directory before creation, cancel and empty history',locale,theme,width);await page.close();continue;
   }
   await hidden();
   await page.locator('.agent-code .hljs-keyword').first().waitFor();
   const code=page.locator('.agent-code').first();
   const wrap=code.getByRole('button',{name:locale==='zh-CN'?'自动换行':'Wrap lines',exact:true});
   await wrap.click();assert.equal(await wrap.getAttribute('aria-pressed'),'true');
   assert.equal(await code.locator('pre').evaluate(el=>getComputedStyle(el).whiteSpace),'pre-wrap');
   await wrap.click();
   await page.evaluate(()=>{window.copiedCode='';Object.defineProperty(navigator,'clipboard',{configurable:true,value:{writeText:async text=>{window.copiedCode=text;}}});});
   await code.getByRole('button',{name:locale==='zh-CN'?'复制':'Copy',exact:true}).click();
   assert((await page.evaluate(()=>window.copiedCode)).includes('opt.SetLog(log.New())'));
   await code.getByRole('status').waitFor({state:'hidden'});
   await page.evaluate(()=>{navigator.clipboard.writeText=async()=>{throw new Error('Clipboard unavailable');};});
   await code.getByRole('button',{name:locale==='zh-CN'?'复制':'Copy',exact:true}).click();
   assert((await code.getByRole('status').textContent()).includes(locale==='zh-CN'?'复制失败':'Copy failed'));
   await page.evaluate(()=>{navigator.clipboard.writeText=async text=>{window.copiedCode=text;};});
   await code.getByRole('button',{name:locale==='zh-CN'?'复制':'Copy',exact:true}).click();
   await code.getByRole('status').waitFor({state:'hidden'});
   await code.getByRole('button',{name:locale==='zh-CN'?'展开代码':'Expand code',exact:true}).click();
   assert.equal(await code.locator('pre').evaluate(el=>getComputedStyle(el).maxHeight),'none');
   await code.getByRole('button',{name:locale==='zh-CN'?'收起代码':'Collapse code',exact:true}).click();
   assert.equal(await code.locator('pre').evaluate(el=>getComputedStyle(el).maxHeight),'260px');
   if(width===390)await page.getByRole('button',{name:locale==='zh-CN'?'显示会话列表':'Show conversations',exact:true}).click();
   const project=page.locator('.edge-agent-project-name');
   await project.focus();await page.getByRole('tooltip').waitFor();assert.equal(await page.getByRole('tooltip').textContent(),access.project);
   await project.click();await page.getByRole('dialog').waitFor();
   await page.keyboard.press('Tab');
   assert(await page.getByRole('dialog').evaluate(el=>el.contains(document.activeElement)),'path dialog keeps keyboard focus');
   await page.screenshot({path:`/tmp/agent-path-${locale}-${theme}-${width}.png`});
   await page.getByRole('button',{name:locale==='zh-CN'?'复制路径':'Copy path',exact:true}).click();
   assert.equal(await page.evaluate(()=>window.copiedCode),access.project);
   await page.getByRole('dialog').getByRole('button',{name:'Close',exact:true}).click();
   if(process.env.E2E_MULTI_SESSION==='1'){
    const search=page.getByRole('searchbox',{name:locale==='zh-CN'?'搜索全部会话':'Search all conversations'});
    await search.fill('archive');await page.getByText('Older matching conversation',{exact:true}).waitFor();
    assert.equal(await page.locator('.edge-agent-session-item').count(),1);
    assert.equal(await page.locator('.edge-agent-attention').count(),0,'No duplicate attention list above project conversations');
    await search.fill('');await page.getByText('Conversation 7',{exact:true}).waitFor();
    assert.equal(await page.locator('.edge-agent-unread').count(),0);
    replyVersion=2;
    await page.getByRole('button',{name:locale==='zh-CN'?'刷新会话':'Refresh sessions',exact:true}).click();
    await page.locator('.edge-agent-unread').first().waitFor();
    await page.locator('.edge-agent-session-item').filter({hasText:'Conversation 1'}).click();
    if(width===390)await page.getByRole('button',{name:locale==='zh-CN'?'显示会话列表':'Show conversations',exact:true}).click();
    await page.waitForFunction(()=>!document.querySelector('.edge-agent-session-item[aria-current="true"] .edge-agent-unread'));
    needsApproval=false;await page.getByRole('button',{name:locale==='zh-CN'?'刷新会话':'Refresh sessions',exact:true}).click();
    await page.locator('.edge-agent-attention').waitFor({state:'hidden'});
    await page.screenshot({path:`/tmp/agent-multi-${locale}-${theme}-${width}.png`});
    assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));assert.deepEqual(errors,[]);
    console.log(`PASS search/attention/unread/read ${locale} ${theme} ${width}`);await page.close();continue;
   }
   if(width===390)await page.getByRole('button',{name:locale==='zh-CN'?'收起会话列表':'Hide conversations',exact:true}).click();
   if(width===1280){
    assert.equal(await page.locator('.edge-agent-session-item').count(),8);
    assert.equal(await page.locator('.edge-agent-group-path').count(),0);
    assert.equal(await page.locator('.edge-agent-project-group-heading small').count(),0);
    await page.getByRole('button',{name:locale==='zh-CN'?'展开显示':'Show more',exact:true}).click();
    assert.equal(await page.locator('.edge-agent-session-item').count(),12);
    await page.reload();await page.getByText('Layout ready.',{exact:true}).waitFor();
    assert.equal(await page.locator('.edge-agent-session-item').count(),12,'expanded list survives reload');
    await page.getByRole('button',{name:locale==='zh-CN'?'收起':'Show less',exact:true}).click();
    assert.equal(await page.locator('.edge-agent-session-item').count(),8);
    await page.locator('.edge-agent-project-group-heading button').first().click();
    await page.reload();await page.getByText('Layout ready.',{exact:true}).waitFor();
    assert.equal(await page.locator('.edge-agent-session-item').count(),0,'collapsed project survives reload');
    await page.locator('.edge-agent-project-group-heading button').first().click();
   }
   const responsive=async()=>{
    const sizes=await page.locator('.edge-agent-messages').evaluate(el=>{
     const css=getComputedStyle(el),message=el.querySelector('.agent-message:not(.is-user)');
     const rect=message.getBoundingClientRect(),panel=el.getBoundingClientRect(),composer=document.querySelector('.edge-agent-composer')?.getBoundingClientRect();
     return {available:el.clientWidth-parseFloat(css.paddingLeft)-parseFloat(css.paddingRight),message:rect.width,center:Math.abs(rect.x+rect.width/2-panel.x-panel.width/2),composerCenter:composer?Math.abs(composer.x+composer.width/2-rect.x-rect.width/2):null,composerWidth:composer?.width??null,overflow:el.scrollWidth-el.clientWidth,pageOverflow:document.documentElement.scrollWidth-innerWidth};
    });
    assert(Math.abs(sizes.message-sizes.available)<2,JSON.stringify(sizes));
    assert.equal(sizes.composerWidth!==null,activeConversation,'Only active conversations show a composer');
    assert(sizes.message<=881,'reading column must stay bounded on wide screens');
    assert(sizes.center<2,'messages must share a centered reading column');
    if(activeConversation){
     assert(sizes.composerWidth<=881,'composer must stay bounded on wide screens');
     assert(sizes.composerCenter<2,'messages and composer must share a centered reading column');
    }
    assert(sizes.overflow<=1&&sizes.pageOverflow<=1,'conversation must not overflow horizontally');
    return sizes.message;
   };
   await responsive();
   activeConversation=true;await page.reload();await page.getByText('Layout ready.',{exact:true}).waitFor();
   await page.locator('.edge-agent-composer').waitFor();await responsive();await hidden();
   if(width===1280){
    await page.setViewportSize({width:1920,height:900});
    const wide=await responsive();
    await page.screenshot({path:`/tmp/agent-wide-${locale}-${theme}.png`});
    await page.setViewportSize({width:1280,height:900});
    assert(Math.abs(wide-await responsive())<2,'wide screens should keep a comfortable reading width');
   }
   await page.screenshot({path:`/tmp/agent-full-page-${locale}-${theme}-${width}.png`});
   const inputMounted=await page.locator('.edge-agent-workspace').evaluate(el=>{el.dataset.mountCheck='preserved';return true;});assert(inputMounted);
   // This recorder covers document bootstrap, not the intentional normal view
   // before React commits a user-initiated transition. Reload resets recording.
   await page.evaluate(()=>{window.observeFullPage=false;});
   await page.locator('[data-page-expand]').click();
   await page.locator('.liaison-global-header').waitFor({state:'visible'});
   await responsive();
   assert(await page.locator('.liaison-global-header').isVisible());
   assert.equal(await page.locator('.edge-agent-workspace').getAttribute('data-mount-check'),'preserved');
   await page.screenshot({path:`/tmp/agent-normal-page-${locale}-${theme}-${width}.png`});
   await page.locator('[data-page-expand]').click();await hidden();
   await page.reload();await page.getByText('Layout ready.',{exact:true}).waitFor();await hidden();
   failAccess=true;await page.reload();
   await page.getByRole('button',{name:locale==='zh-CN'?'重试':'Retry',exact:true}).waitFor();await hidden();
   await page.screenshot({path:`/tmp/agent-full-page-error-${locale}-${theme}-${width}.png`});
   failAccess=false;await page.getByRole('button',{name:locale==='zh-CN'?'重试':'Retry',exact:true}).click();await page.getByText('Layout ready.',{exact:true}).waitFor();await hidden();
   deny=true;await page.reload();await page.getByRole('alert').waitFor();await hidden();
   deny=false;await page.goto(`${origin}/access/agents?view=full`);await page.locator('.liaison-global-header').waitFor();assert(await page.locator('.liaison-sidebar').isVisible());
   assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));assert.deepEqual(errors,[]);
   console.log(`PASS full-page bootstrap/reload/error/retry/permission/list/exit ${locale} ${theme} ${width}`);
   await page.close();
   if(process.env.E2E_FULLPAGE_QUICK==='1')return;
  }
 }finally{await browser.close();}
})().catch(error=>{console.error(error);process.exitCode=1;});
