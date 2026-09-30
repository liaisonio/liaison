const assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
(async()=>{
 const browser=await chromium.launch(process.env.CHROME_EXECUTABLE?{executablePath:process.env.CHROME_EXECUTABLE}:undefined);
 try{
  for(const locale of ['zh-CN','en-US'])for(const dark of [false,true])for(const width of [1280,390]){
   const page=await browser.newPage({viewport:{width,height:900}});const errors=[];let snapshot;let revoked=false;let offline=false;let entries=[];let sentSkill='';let starts=0;let stops=0;const sessions=new Map();const transcripts=new Map();
   page.on('pageerror',e=>errors.push(e.message));
   await page.addInitScript(({locale,dark})=>{localStorage.setItem('liaison-locale',locale);document.addEventListener('DOMContentLoaded',()=>{document.documentElement.classList.toggle('dark',dark);document.documentElement.classList.toggle('light',!dark);});},{locale,dark});
   await page.route('**/api/v1/agent-accesses**',async route=>{
    const req=route.request();const id=new URL(req.url()).pathname.split('/')[4];let data;
    if(req.method()==='POST'){data={...req.postDataJSON(),id:'d'.repeat(32),session_count:0};entries.push(data);}
    else if(req.method()==='PUT'){data={...req.postDataJSON(),id};entries=[data];}
    else if(req.method()==='DELETE'){entries=[];data=null;}
    else {const query=new URL(req.url()).searchParams;const matched=entries.filter(e=>(!query.get('kind')||e.kind===query.get('kind'))&&e.name.toLowerCase().includes((query.get('name')||'').toLowerCase()));data=id?entries.find(e=>e.id===id):{items:matched,total:matched.length};}
    await route.fulfill({json:{data}});
   });
   let applications=[];
   await page.route('**/api/v1/agent-applications**',async route=>{
    if(route.request().method()==='POST'){const app={...route.request().postDataJSON(),id:'f'.repeat(32),access_count:0};applications=[app];return route.fulfill({json:{data:app}});}
    await route.fulfill({json:{data:{items:applications,total:applications.length}}});
   });
   const actions=[];
   const fileTransfers=new Map(),uploadedFiles=new Map();let fileSerial=0,failFile=false,slowFile=false;
   await page.route('**/api/v1/edge-agents**',async route=>{
    const req=route.request();let data;
    if(req.method()==='GET')data=[{id:6,name:'Mac mini',device:'Mac-mini.local',online:!offline}];
    else{
     const body=req.postDataJSON();
     actions.push(body);
     if(body.action.startsWith('file_')){
      const f=body.file;let reply={version:1,status:'ok'};
      if(failFile&&body.action==='file_begin')return route.fulfill({status:403,json:{message:'Forbidden'}});
      if(slowFile&&body.action==='file_write')await new Promise(resolve=>setTimeout(resolve,250));
      if(body.action==='file_write'&&!fileTransfers.has(f.transfer_id))return route.fulfill({json:{data:{version:1,status:'not_found'}}});
      if(body.action==='file_begin'){const id=String(++fileSerial).padStart(32,'0');fileTransfers.set(id,{name:f.name,size:f.size,data:Buffer.alloc(0)});reply.transfer_id=id;}
      if(body.action==='file_write'){const t=fileTransfers.get(f.transfer_id);assert.equal(t.data.length,f.offset);t.data=Buffer.concat([t.data,Buffer.from(f.data,'base64')]);reply.file_offset=t.data.length;}
      if(body.action==='file_commit'){const t=fileTransfers.get(f.transfer_id);assert.equal(t.data.length,t.size);const path='.liaison-attachments/'+body.session_id+'/'+f.transfer_id+'-'+t.name;uploadedFiles.set(path,t);reply.file={name:t.name,path,size:t.size};fileTransfers.delete(f.transfer_id);}
      if(body.action==='file_cancel')fileTransfers.delete(f.transfer_id);
      if(body.action==='file_list'){reply.directory='.';reply.files=[...uploadedFiles].map(([path,t])=>({name:t.name,path,size:t.size}));}
      if(body.action==='file_read'){const t=uploadedFiles.get(f.path);if(!t)reply.status='invalid_request';else Object.assign(reply,{transfer_id:'1'.repeat(32),file_data:t.data.toString('base64'),file_offset:t.size,file_done:true});}
      return route.fulfill({json:{data:reply}});
     }
     if(body.action==='watch')await new Promise(resolve=>setTimeout(resolve,150));
     if(revoked&&['poll','watch'].includes(body.action))return route.fulfill({status:403,json:{message:'Forbidden'}});
     if(body.session_id)snapshot=sessions.get(body.session_id);
     if(body.action==='sessions')data={version:1,status:'ok',sessions_available:true,session_management:true,sessions:[...sessions.values()].map(s=>({session_id:s.session_id,thread_id:s.thread_id,title:s.title||s.messages[0]?.text||s.project.split('/').pop(),project:s.project,running:s.running,closed:s.closed,status:s.status,updated_at:'2026-09-21T03:00:00Z'}))};
     else if(body.action==='discover')data={version:1,status:'ok',installations:[{id:'a'.repeat(32),path:'/opt/homebrew/bin/codex',kind:'codex'}],default_project:'/Users/local/project'};
     else if(body.action==='directories'){const dir=body.directory||'/Users/local';data={version:1,status:dir==='/outside'?'invalid_request':'ok',directory:dir,parent_directory:'/Users/local',directory_roots:['/Users/local'],directories:dir.endsWith('/src')?[]:[{name:'src',path:'/Users/local/project/src'},{name:'项目文档',path:'/Users/local/project/docs'}]};}
     else if(body.action==='start'){starts++;data=snapshot={version:1,status:'ok',revision:1,session_id:String(starts).padStart(32,'b'),thread_id:'thread-'+starts,model:'configured-model',project:body.working_directory||body.project,started_at:'2026-09-21T03:00:00Z',skills_available:true,skills:[{id:'c'.repeat(32),name:'review-code',description:'Review the current project'}],running:false,closed:false,messages:[]};}
     else if(body.action==='resume'){data=snapshot={...snapshot,status:'ok',revision:snapshot.revision+1,running:false,closed:false,archived:false};}
     else if(body.action==='transcript'){const pages=[...transcripts.values()].filter(p=>p.session_id===body.session_id&&(p.window||0)<Number(body.history_before)).sort((a,b)=>(b.window||0)-(a.window||0)).slice(0,Number(body.history_limit||1));data={version:1,status:'ok',session_id:body.session_id,history_pages:pages,history_before:pages.at(-1)?.window?String(pages.at(-1).window):''};}
     else if(body.action==='send'){if(snapshot.messages.length){transcripts.set(snapshot.session_id+':'+(snapshot.window||0),{...snapshot,window:snapshot.window||0,closed:true,running:false,archived:true});snapshot={...snapshot,window:(snapshot.window||0)+1};}sentSkill=body.skill_id||'';data=snapshot={...snapshot,messages:[{role:'user',text:body.text},{role:'assistant',text:'## Project\n\nUse `go test ./...` to verify.\n\n<script>alert(1)</script>\n\n![remote](https://invalid.example/image.png)'}]};
     }
     else if(body.action==='models')data={...snapshot,models_available:true,models:[{id:'configured-model',name:'Configured model'},{id:'native-other',name:'Native other'}]};
     else if(body.action==='model'){data=snapshot={...snapshot,model:body.model};}
     else if(body.action==='permissions'){data=snapshot={...snapshot,permission_mode:body.permission_mode,revision:snapshot.revision+1};}
     else if(body.action==='approve'){assert(['accept','decline'].includes(body.decision));assert.equal(body.approval_id,'f'.repeat(32));data=snapshot={...snapshot,approvals:[],running:false,revision:snapshot.revision+1};}
     else if(body.action==='answer'){
      assert.equal(body.input_id,'e'.repeat(32));assert.deepEqual(body.answers,[{question_id:'layout',answers:['Compact']},{question_id:'name',answers:['Workspace']}]);
      data=snapshot={...snapshot,input_requests:[],running:false,revision:snapshot.revision+1,activities:[
       {id:'command',kind:'commandExecution',status:'completed',message_index:1,duration_ms:52,command:'go test ./...',directory:'/Users/local/project',output:'PASS\n<script>not executable</script>',exit_code:0},
       {id:'file',kind:'fileChange',status:'completed',message_index:1,duration_ms:0,changes:[{path:'web/src/components/'+('long-directory-'.repeat(30))+'/workspace.tsx',kind:'update',diff:'@@ -1,2 +1,2 @@\n-const title = "Old";\n+const title = "'+('Workspace'.repeat(80))+'";\n export default title;'}]},
       {id:'plan',kind:'turnPlan',status:'completed',message_index:1,duration_ms:0,output:'Implementation and verification',plan:[{step:'Inspect the existing code',status:'completed'},{step:'Verify the changes',status:'completed'}]},
       {id:'search',kind:'webSearch',status:'completed',message_index:1,duration_ms:0,label:'SQLite WAL documentation'},
       {id:'mcp',kind:'mcpToolCall',status:'completed',message_index:1,duration_ms:0,label:'docs / search'},
       {id:'input',kind:'userInput',status:'completed',message_index:1,duration_ms:0,output:'Layout?\n→ Compact\nName?\n→ Workspace'}
      ]};
     }
     else if(body.action==='rename'){data=snapshot={...snapshot,title:body.title};}
     else if(body.action==='delete'){sessions.delete(body.session_id);data={version:1,status:'ok'};}
     else if(body.action==='discard'){if(snapshot.running||snapshot.messages.length)data={version:1,status:'busy'};else{stops++;sessions.delete(body.session_id);data={version:1,status:'ok'};}}
     else if(body.action==='stop'){stops++;data=snapshot={...snapshot,closed:true};}
     else if(body.action==='interrupt'){data=snapshot={...snapshot,running:false,input_requests:[],approvals:[],revision:snapshot.revision+1};sessions.set(snapshot.session_id,snapshot);}
     else data=snapshot;
     if(body.action==='send'){snapshot.revision++;snapshot.activities=[{id:'safe-tool-id',kind:'commandExecution',status:'completed',message_index:1,duration_ms:85}];}
     if(body.action==='send'&&body.text==='__approval_test__'){snapshot.title='project';snapshot.running=true;snapshot.approvals=[{id:'f'.repeat(32),command:'ls /private/tmp/liaison-approval-test',directory:'/private/tmp/liaison-approval-test',reason:'Inspect a temporary test directory'}];}
     if(body.action==='send'&&body.text==='__input_test__'){snapshot.title='project';snapshot.running=true;snapshot.input_requests=[{id:'e'.repeat(32),blocking:true,questions:[{id:'layout',header:'Layout',question:'Which layout should we use?',is_other:true,options:[{label:'Compact',description:'Keep the workspace focused.'},{label:'Detailed',description:'Show more information.'}]},{id:'name',header:'Name',question:'What should the workspace be called?'}]}];}
     if(body.action==='send'&&body.attachments?.length){snapshot.messages[0].attachments=body.attachments.map(path=>{const f=uploadedFiles.get(path);assert(f,'Send references uploaded files, never client-only paths');return {name:f.name,path,size:f.size};});}
     if(snapshot){snapshot.permissions_available=true;snapshot.permission_mode ||= 'read-only';snapshot.files_available=true;snapshot.files_upload_available=true;}
     if(['permissions','approve','answer'].includes(body.action)&&snapshot)sessions.set(snapshot.session_id,snapshot);
     if(['start','resume','send','stop','model','rename'].includes(body.action)&&snapshot)sessions.set(snapshot.session_id,snapshot);
    }
    if(data&&!Array.isArray(data)){data={...data,history_persistent:true};if(offline&&data.session_id)data={...data,archived:true,closed:true,running:false};if(offline&&data.sessions)data.sessions=data.sessions.map(s=>({...s,closed:true,running:false}));}
    await route.fulfill({json:{data}});
   });
   await page.goto((process.env.E2E_UI_URL||'http://127.0.0.1:5298')+'/e2e/edge-agent.html');
   const zh=locale==='zh-CN';
   await page.getByRole('button',{name:zh?'新建访问':'Create access',exact:true}).click();
   await page.getByRole('combobox',{name:zh?'应用来源':'Application source'}).selectOption('new');
   await page.getByRole('button',{name:zh?'发现已安装的 Agent':'Discover installed Agents'}).click();
   await page.getByRole('button',{name:zh?'浏览':'Browse',exact:true}).click();
   await page.getByRole('button',{name:'src',exact:true}).waitFor();
   await page.screenshot({path:`/tmp/agent-directory-${locale}-${dark}-${width}.png`,fullPage:true});
   const remotePath=page.getByRole('dialog').getByLabel(zh?'远端目录':'Remote directory',{exact:false});
   await remotePath.fill('/outside');
   await page.getByRole('dialog').getByRole('button',{name:zh?'打开':'Open',exact:true}).click();
   await page.getByText(zh?'无法打开目录。请检查路径、目录范围和连接器版本。':'Cannot open this folder. Check the path, allowed roots and connector version.').waitFor();
   assert(await page.getByRole('button',{name:zh?'选择此目录':'Select folder',exact:true}).isDisabled());
   await remotePath.fill('/Users/local/project');
   await page.getByRole('dialog').getByRole('button',{name:zh?'打开':'Open',exact:true}).click();
   await page.getByRole('button',{name:zh?'选择此目录':'Select folder',exact:true}).click();
   await page.getByRole('dialog').getByRole('button',{name:zh?'创建访问':'Create access',exact:true}).click();
   await page.getByRole('button',{name:zh?'去访问':'Open',exact:true}).waitFor();
   const tabs=page.getByRole('navigation',{name:zh?'访问类型':'Access types'});
   await tabs.getByRole('button',{name:'Codex',exact:true}).click();
   const search=page.getByPlaceholder(zh?'输入访问名称':'Access name', {exact:true});
   await search.fill('no-matching-access');
   await page.getByText(zh?'暂无匹配访问':'No matching access',{exact:true}).waitFor();
   await page.getByRole('button',{name:zh?'重置':'Reset',exact:true}).click();
   await page.getByRole('button',{name:zh?'去访问':'Open',exact:true}).waitFor();
   await tabs.getByRole('button',{name:zh?'全部':'All',exact:true}).click();
   await page.getByRole('columnheader',{name:zh?'会话数量':'Conversations',exact:true}).waitFor();
   assert.equal(await page.getByRole('columnheader',{name:zh?'项目目录':'Project directory',exact:true}).count(),0);
   await page.screenshot({path:`/tmp/agent-list-${locale}-${dark}-${width}.png`,fullPage:true});
   await page.reload();
   await page.getByRole('button',{name:zh?'去访问':'Open',exact:true}).click();
   await page.getByRole('button',{name:zh?'选择项目目录':'Choose project folder',exact:true}).click();
   const folderSearch=page.getByRole('textbox',{name:zh?'定位文件夹':'Find folder',exact:true});
   await page.getByRole('button',{name:'src',exact:true}).waitFor();
   await page.getByRole('button',{name:'src',exact:true}).focus();
   await page.keyboard.press('s');
   assert.equal(await folderSearch.inputValue(),'s');
   await folderSearch.fill('文档');
   await page.locator('.edge-agent-directory-list button.is-located').getByText('项目文档',{exact:true}).waitFor();
   await folderSearch.fill('missing-folder');
   await page.getByText(zh?'没有匹配的文件夹。':'No matching folders.',{exact:true}).waitFor();
   await folderSearch.fill('SR');
   await page.locator('.edge-agent-directory-list button.is-located').getByText('src',{exact:true}).waitFor();
   await page.screenshot({path:`/tmp/agent-folder-${locale}-${dark}-${width}.png`});
   await folderSearch.press('Enter');
   await page.waitForFunction(()=>document.querySelector('.edge-agent-directory-path input')?.value.endsWith('/src'));
   await page.getByRole('button',{name:zh?'上一级':'Parent folder',exact:true}).click();
   await page.getByRole('dialog').getByRole('button',{name:zh?'在此目录新建':'Create in this folder',exact:true}).click();
   await page.getByRole('button',{name:zh?'更多会话操作':'More conversation actions',exact:true}).click();
   await page.getByRole('menuitem',{name:zh?'会话详情':'Session details',exact:true}).click();
   await page.getByText('thread-1',{exact:true}).waitFor();
   await page.getByRole('dialog').getByText(zh?'历史保存在服务端，不自动过期。空闲时仅结束运行实例。':'History is stored on the server without automatic expiration. Idle instances may stop.',{exact:true}).waitFor();
   await page.getByRole('dialog').getByRole('button',{name:zh?'关闭':'Close',exact:true}).last().click();
   assert.equal(await page.locator('.edge-agent-session-list footer').count(),0);
   assert.equal(await page.locator('.edge-agent-project .edge-agent-codex-label').count(),0);
   assert.equal(await page.locator('.edge-agent-breadcrumb .edge-agent-kind').innerText(),'Codex');
   assert.equal(await page.locator('.edge-agent-breadcrumb .edge-agent-kind svg').count(),0);
   if(process.env.E2E_SCROLL_ONLY==='1'){
    snapshot={...snapshot,revision:(snapshot.revision||0)+1,running:false,messages:[{role:'user',text:'Scroll check'},{role:'assistant',text:Array.from({length:60},(_,i)=>`Paragraph ${i}. Read this historical message.`).join('\n\n')}]};sessions.set(snapshot.session_id,snapshot);
    await page.getByText('Paragraph 59. Read this historical message.',{exact:true}).waitFor();
    assert.equal(await page.locator('.edge-agent-messages .agent-markdown p').last().evaluate(el=>getComputedStyle(el).fontSize),'13px');
    assert.equal(await page.locator('.edge-agent-messages .agent-markdown p').last().evaluate(el=>getComputedStyle(el).lineHeight),'22.75px');
    assert(await page.locator('.edge-agent-workspace > header').evaluate(el=>el.getBoundingClientRect().height<90),'Compact workspace header');
    const messages=page.locator('.edge-agent-messages'),input=page.getByRole('textbox',{name:zh?'消息':'Message',exact:true});
    const position=()=>page.evaluate(()=>{const e=document.querySelector('.edge-agent-messages');return {top:e.scrollTop,height:e.clientHeight,outer:Array.from(document.querySelectorAll('body,main,.edge-agent-sessions-page,.edge-agent-session-layout,.edge-agent-session-main,.edge-agent-workspace')).map(n=>({name:n.className||n.tagName,top:n.scrollTop,y:n.getBoundingClientRect().y}))};});
    await messages.evaluate(e=>{e.scrollTop=700;});await page.waitForTimeout(200);const before=await position();
    await input.click();await page.waitForTimeout(200);assert.deepEqual(await position(),before,'Clicking input preserves scroll and layout');
    await input.fill('Line one\nLine two\nLine three\nLine four\nLine five');await page.waitForTimeout(200);assert.equal((await position()).top,before.top);
    await input.fill('/');await page.waitForTimeout(200);assert.equal((await position()).top,before.top);
    await input.fill('');
    for(const decision of ['accept','decline']){
     const prior=await position();
     const approval=decision==='accept'?{id:'f'.repeat(32),reason:'Inspect the project environment',command:'pwd',directory:'/project'}:{id:'f'.repeat(32),kind:'fileChange',reason:'Review the proposed change',directory:'/project',changes:[{path:'notes.txt',kind:'update',diff:'@@ -1 +1 @@\n-old\n+new'}]};
     snapshot={...snapshot,revision:snapshot.revision+1,running:true,approvals:[approval],activities:decision==='accept'?[{id:'pending-command',kind:'commandExecution',status:'running',message_index:1,command:'pwd',duration_ms:0}]:[]};sessions.set(snapshot.session_id,snapshot);
     const dialog=page.locator('.edge-agent-workspace .edge-agent-approval');await dialog.waitFor();
     if(decision==='accept'){
      const details=page.locator('.edge-agent-activity li > details');
      assert(!(await details.evaluate(el=>el.open)),'Awaiting approval does not expand duplicate command details');
      assert((await details.locator('summary').innerText()).includes(zh?'等待确认':'Awaiting approval'));
     }
     assert.equal(await page.getByRole('dialog').count(),0,'Approval is an inline card without a modal');
     assert(await input.evaluate(e=>e===document.activeElement),'Approval does not steal focus');
     assert.equal((await position()).top,prior.top,'Approval preserves the historical reading position');
     await input.click();assert.equal(await dialog.count(),1,'Clicking outside does not decide');
     await page.keyboard.press('Escape');assert.equal(await dialog.count(),1,'Escape never rejects the request');
     if(decision==='decline'){await dialog.locator('summary').click();await dialog.locator('.edge-agent-diff-line.added').waitFor();}
     await page.screenshot({path:`/tmp/agent-approval-card-${locale}-${dark}-${width}.png`});
     await dialog.getByRole('button',{name:decision==='accept'?(zh?'确认':'Confirm'):(zh?'取消':'Cancel'),exact:true}).click();
     await dialog.waitFor({state:'detached'});
     assert.equal(actions.filter(a=>a.action==='approve').at(-1).decision,decision==='accept'?'accept':'decline');
     assert.equal((await position()).top,prior.top,'Closing approval preserves the transcript');
     await input.focus();
    }
    snapshot={...snapshot,revision:snapshot.revision+1,running:false,closed:true,status:'session_closed'};sessions.set(snapshot.session_id,snapshot);
    await page.locator('.edge-agent-composer').waitFor({state:'detached'});
    await page.screenshot({path:`/tmp/agent-scroll-${locale}-${dark}-${width}.png`,fullPage:true});await page.close();continue;
   }
   if(process.env.E2E_HISTORY_HEADER_ONLY==='1'){
    snapshot={...snapshot,closed:true,running:false,status:'session_closed',archived:true,messages:[{role:'user',text:'Earlier request'},{role:'assistant',text:'Saved history'}]};sessions.set(snapshot.session_id,snapshot);
    await page.reload();
    const resume=page.getByRole('button',{name:zh?'恢复会话':'Resume conversation',exact:true});await resume.waitFor();
    assert.equal(await page.locator('.edge-agent-setup').count(),0);
    assert.equal(await page.locator('.edge-agent-session-page > .liaison-notice').count(),0);
    assert.equal(await page.locator('.edge-agent-workspace > header').count(),1);
    await page.getByRole('button',{name:zh?'全页面展开':'Expand to full page',exact:true}).click();
    assert(await page.locator('.edge-agent-sessions-page').evaluate(el=>{const r=el.getBoundingClientRect();return r.x===0&&r.y===0&&r.width===innerWidth&&r.height===innerHeight;}));
    const fullPageURL=page.url();
    assert.equal(new URL(fullPageURL).searchParams.get('view'),'full');
    await page.reload();await resume.waitFor();
    assert.equal(page.url(),fullPageURL,'Reload must preserve access, session and view');
    assert(await page.locator('.is-page-expanded').evaluate(el=>{const r=el.getBoundingClientRect();return r.x===0&&r.y===0&&r.width===innerWidth&&r.height===innerHeight;}));
    await page.screenshot({path:`/tmp/agent-full-page-reload-${locale}-${dark}-${width}.png`,fullPage:true});
    await page.keyboard.press('Escape');
    assert.equal(await page.locator('.is-page-expanded').count(),0);
    assert.equal(new URL(page.url()).searchParams.has('view'),false);
    await page.reload();await resume.waitFor();
    assert.equal(await page.locator('.is-page-expanded').count(),0,'Escape must persist normal view');
    await page.getByRole('button',{name:zh?'全页面展开':'Expand to full page',exact:true}).click();
    await page.getByRole('button',{name:zh?'退出全页面':'Exit full page',exact:true}).click();
    await page.reload();await resume.waitFor();
    assert.equal(await page.locator('.is-page-expanded').count(),0,'Exit button must persist normal view');
    await page.screenshot({path:`/tmp/agent-history-header-${locale}-${dark}-${width}.png`,fullPage:true});
    offline=true;await page.reload();await page.getByText('Saved history',{exact:true}).waitFor();
    assert.equal(await resume.count(),0);
    await page.getByTitle(zh?'连接器离线，暂时无法恢复。':'The connector is offline. Resume is temporarily unavailable.',{exact:true}).waitFor();
    assert.equal(starts,1);assert.deepEqual(errors,[]);await page.close();continue;
   }
   if(process.env.E2E_FILES_ONLY==='1'){
    const content=Buffer.from('Project specification\n'.repeat(10000));
    const uploader=page.locator('input[type=file]').first();
    await uploader.setInputFiles({name:'specification.txt',mimeType:'text/plain',buffer:content});
    await page.locator('.edge-agent-attachments').getByText('specification.txt',{exact:true}).waitFor();
    assert.deepEqual([...uploadedFiles.values()][0].data,content);assert.equal(fileTransfers.size,0);
    const png=Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jP1sAAAAASUVORK5CYII=','base64');
    await page.locator('input[type=file]').last().setInputFiles({name:'design.png',mimeType:'image/png',buffer:png});
    await page.getByRole('img',{name:'design.png'}).waitFor();
    await page.locator('.edge-agent-composer textarea').evaluate((el,png)=>{const data=new DataTransfer();data.items.add(new window.File([Uint8Array.from(atob(png),c=>c.charCodeAt(0))],'clipboard.png',{type:'image/png'}));el.dispatchEvent(new ClipboardEvent('paste',{clipboardData:data,bubbles:true,cancelable:true}));},png.toString('base64'));
    await page.getByRole('img',{name:'clipboard.png'}).waitFor();
    await page.getByRole('button',{name:(zh?'移除附件':'Remove attachment')+' clipboard.png',exact:true}).click();
    await page.locator('.edge-agent-composer').evaluate(el=>{const data=new DataTransfer();data.items.add(new window.File(['dropped'],'drop.txt',{type:'text/plain'}));el.dispatchEvent(new DragEvent('drop',{dataTransfer:data,bubbles:true,cancelable:true}));});
    await page.locator('.edge-agent-attachments').getByText('drop.txt',{exact:true}).waitFor();
    await page.getByRole('button',{name:(zh?'移除附件':'Remove attachment')+' drop.txt',exact:true}).click();
    failFile=true;await uploader.setInputFiles({name:'denied.txt',mimeType:'text/plain',buffer:Buffer.from('denied')});
    await page.getByText(zh?'传输未完成。请检查连接器、访问权限、文件大小及路径后重试。':'Transfer failed. Check the connector, permissions, file size and path, then retry.',{exact:true}).waitFor();
    assert.equal(await page.locator('.edge-agent-attachments').getByText('denied.txt',{exact:true}).count(),0);failFile=false;
    await page.getByRole('button',{name:zh?'关闭提示':'Dismiss',exact:true}).click();
    slowFile=true;await uploader.setInputFiles({name:'cancel.txt',mimeType:'text/plain',buffer:Buffer.alloc(1024*1024,1)});
    await page.locator('.edge-agent-transfer').getByRole('button',{name:zh?'取消':'Cancel',exact:true}).click();
    await page.locator('.edge-agent-transfer').waitFor({state:'hidden'});slowFile=false;
    assert.equal(await page.locator('.edge-agent-attachments').getByText('cancel.txt',{exact:true}).count(),0);
    assert(await page.getByRole('button',{name:zh?'发送':'Send',exact:true}).isEnabled(),'Attachment-only messages can be sent');
    await page.screenshot({path:`/tmp/agent-files-${locale}-${dark}-${width}.png`,fullPage:true});
    await page.getByRole('button',{name:zh?'添加与工具':'Add and tools',exact:true}).click();
    await page.getByRole('menuitem',{name:zh?'项目文件':'Project files',exact:true}).click();
    await page.getByRole('dialog').getByText('specification.txt',{exact:true}).waitFor();
    await page.screenshot({path:`/tmp/agent-project-files-${locale}-${dark}-${width}.png`,fullPage:true});
    const downloadEvent=page.waitForEvent('download');
    await page.getByRole('dialog').getByRole('button',{name:(zh?'下载':'Download')+' specification.txt',exact:true}).click();
    const downloaded=await downloadEvent;assert.equal(downloaded.suggestedFilename(),'specification.txt');
    const stream=await downloaded.createReadStream();const chunks=[];for await(const part of stream)chunks.push(part);assert.deepEqual(Buffer.concat(chunks),content);
    await page.getByRole('dialog').getByRole('button').filter({hasText:'drop.txt'}).click();
    await page.locator('.agent-source-code').waitFor();
    assert((await page.locator('.agent-source-code').innerText()).includes('dropped'),'File names open a preview');
    await page.getByRole('dialog').getByRole('button',{name:'Close',exact:true}).click();
    await page.getByRole('button',{name:(zh?'移除附件':'Remove attachment')+' design.png',exact:true}).click();
    assert.equal(await page.getByRole('img',{name:'design.png'}).count(),0);
    await page.getByRole('button',{name:zh?'发送':'Send',exact:true}).click();
    await page.locator('.edge-agent-message-files').getByText('specification.txt',{exact:true}).waitFor();
    assert.equal(await page.locator('.edge-agent-attachments').count(),0,'Accepted send clears the composer attachments');
    assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),'Attachments do not overflow');
    assert.deepEqual(errors,[]);await page.close();continue;
   }
   const root=page.locator('.edge-agent-sessions-page'),composer=page.getByRole('textbox',{name:zh?'消息':'Message',exact:true});
   await composer.fill('Keep this draft');
   const originalList=await page.locator('.edge-agent-session-list').isVisible(),originalStarts=starts;
   await page.getByRole('button',{name:zh?'全页面展开':'Expand to full page',exact:true}).click();
   const bounds=await root.boundingBox();assert.equal(bounds.x,0);assert.equal(bounds.y,0);assert.equal(bounds.width,width);assert.equal(bounds.height,900);
   assert.equal(await page.locator('.edge-agent-session-list').isVisible(),originalList);
   await page.getByRole('button',{name:originalList?(zh?'收起会话列表':'Hide conversations'):(zh?'显示会话列表':'Show conversations'),exact:true}).click();
   assert.equal(await page.locator('.edge-agent-session-list').isVisible(),!originalList);
   await page.getByRole('button',{name:!originalList?(zh?'收起会话列表':'Hide conversations'):(zh?'显示会话列表':'Show conversations'),exact:true}).click();
   await page.getByRole('button',{name:zh?'更多会话操作':'More conversation actions',exact:true}).click();
   await page.getByRole('menuitem',{name:zh?'会话详情':'Session details',exact:true}).click();
   await page.getByRole('dialog').waitFor();await page.keyboard.press('Escape');
   await page.getByRole('dialog').waitFor({state:'hidden'});
   assert(await root.evaluate(el=>el.classList.contains('is-page-expanded')),'Closing a dialog must not exit full page');
   await page.screenshot({path:`/tmp/agent-full-page-${locale}-${dark}-${width}.png`,fullPage:true});
   await page.keyboard.press('Escape');
   assert.equal(await root.evaluate(el=>el.classList.contains('is-page-expanded')),false);
   assert.equal(await composer.inputValue(),'Keep this draft');assert.equal(starts,originalStarts,'View changes never restart the session');
   assert.equal(await page.locator('.edge-agent-session-list').isVisible(),originalList);
   await composer.fill('');
   await page.getByRole('button',{name:zh?'全页面展开':'Expand to full page',exact:true}).click();
   await page.getByRole('button',{name:zh?'退出全页面':'Exit full page',exact:true}).click();
   assert.equal(await root.evaluate(el=>el.classList.contains('is-page-expanded')),false);
   assert.equal(await page.locator('.edge-agent-footnote').count(),0);
   assert.equal(await page.locator('.edge-agent-composer .edge-agent-permission').count(),1);
   assert.equal(await page.locator('.edge-agent-composer .edge-agent-keyhint').count(),1);
   assert.equal(await page.locator('.edge-agent-keyhint').isVisible(),width>640);
   if(process.env.E2E_FOOTNOTE_ONLY==='1'){
    const message=page.getByRole('textbox',{name:zh?'消息':'Message',exact:true});
    const slash=page.locator('.edge-agent-slash');
    const tools=page.getByRole('button',{name:zh?'添加与工具':'Add and tools',exact:true});
    await message.fill('/');await slash.waitFor();
    await page.locator('.edge-agent-project strong').click();
    await slash.waitFor({state:'hidden'});
    assert.equal(await message.inputValue(),'/','Outside click preserves the draft');
    await message.fill('/review');await slash.waitFor();
    await tools.focus();await slash.waitFor({state:'hidden'});
    await message.fill('/');await slash.waitFor();
    await message.press('Escape');await slash.waitFor({state:'hidden'});
    await message.fill('Keep this draft');
    await tools.click();await page.keyboard.press('Escape');
    assert(await tools.evaluate(el=>document.activeElement===el),'Tools restore focus on Escape');
    await tools.click();await page.getByRole('menuitem',{name:zh?'命令与技能':'Commands and skills'}).click();
    await slash.waitFor();
    const popup=await slash.boundingBox();
    assert(popup.x>=0&&popup.y>=0&&popup.x+popup.width<=width+1,'Skill menu fits the viewport');
    await page.screenshot({path:`/tmp/agent-menu-${locale}-${dark}-${width}.png`,fullPage:true});
    await page.getByRole('option',{name:'review-code Review the current project'}).click();
    await slash.waitFor({state:'hidden'});
    assert.equal(await message.inputValue(),'Keep this draft');
    assert(await message.evaluate(el=>document.activeElement===el),'Skill selection restores composer focus');
    await page.getByRole('button',{name:zh?'移除技能':'Remove skill'}).click();
    await message.fill('/review');await page.getByRole('option',{name:'review-code Review the current project'}).waitFor();await message.press('Enter');await slash.waitFor({state:'hidden'});
    await page.getByRole('button',{name:zh?'移除技能':'Remove skill'}).click();
    await message.fill('/');await slash.waitFor();
    await page.getByRole('button',{name:zh?'切换模型':'Switch model',exact:true}).click();
    await slash.waitFor({state:'hidden'});await page.getByRole('dialog').waitFor();
    await page.keyboard.press('Escape');await page.getByRole('dialog').waitFor({state:'hidden'});
    await message.fill('');
    const separator=page.getByRole('separator',{name:zh?'调整会话列表宽度':'Resize session list'});
    if(width>=1000){
     // Shared CSS can arrive after page CSS in the production lazy-loaded chunks.
     await page.addStyleTag({content:'.agent-workspace-divider { left:-3px; }'});
     const sidebar=await page.locator('.edge-agent-session-list').boundingBox(),main=await page.locator('.edge-agent-session-main').boundingBox();
     assert(Math.abs(sidebar.x+sidebar.width-main.x)<1,'Sidebar and conversation share a panel without a gap');
     const handle=await separator.boundingBox();
     assert(Math.abs(handle.x+handle.width/2-main.x)<2,'Drag hit area stays on the visible divider regardless of stylesheet order');
     assert.equal(await page.locator('.edge-agent-workspace').evaluate(el=>getComputedStyle(el).borderTopWidth),'0px');
     await separator.waitFor();const box=await separator.boundingBox();
     await page.mouse.move(box.x+box.width/2,box.y+80);await page.mouse.down();await page.mouse.move(box.x+box.width/2+90,box.y+80,{steps:8});await page.mouse.up();
     await page.mouse.move(20,20);await page.waitForTimeout(200);
     assert.equal(await separator.evaluate(el=>getComputedStyle(el,'::after').backgroundColor),'rgba(0, 0, 0, 0)','Pointer drag must not leave a highlighted separator');
     assert.equal(Math.round((await page.locator('.edge-agent-session-list').boundingBox()).width),320);
     await page.reload();await separator.waitFor();
     assert.equal(Math.round((await page.locator('.edge-agent-session-list').boundingBox()).width),320,'Sidebar width survives reload');
     await separator.focus();await page.keyboard.press('ArrowLeft');assert.equal(await separator.getAttribute('aria-valuenow'),'310');
     await page.keyboard.press('End');assert.equal(await separator.getAttribute('aria-valuenow'),'420');
     await page.keyboard.press('Home');assert.equal(await separator.getAttribute('aria-valuenow'),'200');
     await separator.dblclick();assert.equal(await separator.getAttribute('aria-valuenow'),'230');
     await page.keyboard.press('ArrowRight');
     await page.getByRole('textbox',{name:zh?'消息':'Message',exact:true}).focus();
    }else{
     await page.getByRole('button',{name:zh?'显示会话列表':'Show conversations',exact:true}).click();
     assert.equal(await separator.isVisible(),false,'Mobile keeps drawer without drag handle');
     await page.getByRole('button',{name:zh?'收起会话列表':'Hide conversations',exact:true}).click();
    }
    await page.mouse.move(10,10);await page.waitForTimeout(200);
    assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),'Resize does not cause page overflow');
    await page.screenshot({path:`/tmp/agent-footnote-${locale}-${dark}-${width}.png`,fullPage:true});
    assert.deepEqual(errors,[]);await page.close();continue;
   }
   await page.getByRole('textbox',{name:zh?'消息':'Message',exact:true}).fill('project');
   await page.getByRole('button',{name:zh?'发送':'Send',exact:true}).click();
   await page.getByRole('heading',{name:'Project',exact:true}).last().waitFor();
   assert.equal(await page.locator('.agent-message > .edge-agent-codex-label svg').count(),0,'Message authors are text only');
   await page.getByRole('button',{name:zh?'按需授权':'Approval on request',exact:true}).click();
   await page.getByRole('menuitem',{name:zh?'允许工作区写入':'Allow workspace writes',exact:true}).click();
   await page.getByRole('button',{name:zh?'工作区写入':'Workspace write',exact:true}).waitFor();
   await page.getByRole('textbox',{name:zh?'消息':'Message',exact:true}).fill('__approval_test__');
   await page.getByRole('button',{name:zh?'发送':'Send',exact:true}).click();
   await page.locator('.edge-agent-approval').getByRole('button',{name:zh?'确认':'Confirm',exact:true}).waitFor();
   await page.screenshot({path:`/tmp/agent-approval-${locale}-${dark}-${width}.png`,fullPage:true});
   await page.locator('.edge-agent-approval').getByRole('button',{name:zh?'确认':'Confirm',exact:true}).click();
   await page.locator('.edge-agent-approval').waitFor({state:'detached'});
   await page.getByRole('textbox',{name:zh?'消息':'Message',exact:true}).fill('__input_test__');
   await page.getByRole('button',{name:zh?'发送':'Send',exact:true}).click();
   const inputForm=page.getByRole('form',{name:zh?'回答问题':'Answer questions'});
   await inputForm.waitFor();
   assert(await inputForm.getByRole('button',{name:zh?'提交回答':'Submit answers'}).isDisabled(),'Never preselect a user answer');
   await inputForm.getByRole('radio',{name:'Compact',exact:false}).check();
   await inputForm.getByRole('textbox',{name:zh?'你的回答':'Your answer',exact:false}).fill('Workspace');
   await inputForm.getByRole('button',{name:zh?'提交回答':'Submit answers'}).scrollIntoViewIfNeeded();
   await page.screenshot({path:`/tmp/agent-input-${locale}-${dark}-${width}.png`,fullPage:true});
   await inputForm.getByRole('button',{name:zh?'提交回答':'Submit answers'}).click();
   await inputForm.waitFor({state:'detached'});
   await page.locator('.edge-agent-activity > summary').last().click();
   const commandSummary=page.locator('.edge-agent-activity li.has-detail > details > summary').filter({hasText:zh?'执行命令':'Run command'});
   assert.equal(await commandSummary.locator('.edge-agent-disclosure').evaluate(el=>getComputedStyle(el).transform),'none','Closed command points right');
   assert.equal(await commandSummary.locator('.lucide-check').evaluate(el=>getComputedStyle(el).transform),'none','Status check never rotates with parent');
   await page.locator('.edge-agent-activity li.has-detail > details > summary').filter({hasText:zh?'执行命令':'Run command'}).click();
   assert.equal(await commandSummary.locator('.edge-agent-disclosure').evaluate(el=>getComputedStyle(el).transform),'matrix(0, 1, -1, 0, 0, 0)','Open command points down');
   await page.getByLabel(zh?'输出':'Output',{exact:true}).filter({hasText:'PASS'}).waitFor();
   await page.locator('.edge-agent-activity li.has-detail > details > summary').filter({hasText:zh?'文件操作':'File operation'}).click();
   await page.locator('.edge-agent-file-change > summary').click();
   await page.getByRole('region',{name:zh?'代码差异':'Code diff'}).waitFor();
   await page.getByRole('region',{name:zh?'代码差异':'Code diff'}).scrollIntoViewIfNeeded();
   assert.equal(await page.locator('.edge-agent-diff-line.added').count(),1);
   assert.equal(await page.locator('.edge-agent-diff-line.removed').count(),1);
   await page.getByRole('region',{name:zh?'任务计划':'Task plan'}).waitFor();
   await page.getByText('SQLite WAL documentation',{exact:true}).waitFor();
   await page.getByText('docs / search',{exact:true}).waitFor();
   assert(await page.locator('.edge-agent-messages').evaluate(el=>el.scrollWidth<=el.clientWidth+1),'Only code/diff containers may scroll horizontally, never the conversation');
   if(process.env.E2E_ACTIVITY_ONLY==='1'){
    const publish=patch=>{snapshot={...snapshot,...patch,revision:snapshot.revision+1};sessions.set(snapshot.session_id,snapshot);};
    publish({window:200,messages:[{role:'assistant',text:'Executing checks'}],activities:[{id:'live-command',kind:'commandExecution',status:'running',message_index:0,duration_ms:0,command:'go test ./...',output:'Running tests…'}],running:true});
    const activity=page.locator('.edge-agent-activity').last(),detail=activity.locator('li.has-detail > details').first();
    await page.locator('.edge-agent-command-output').filter({hasText:'Running tests…'}).waitFor();
    assert(await activity.evaluate(el=>el.open),'Live activity opens');assert(await detail.evaluate(el=>el.open),'Live command output opens');
    publish({running:false,activities:snapshot.activities.map(a=>({...a,status:'completed',output:'PASS',exit_code:0}))});
    await page.waitForFunction(()=>Array.from(document.querySelectorAll('.edge-agent-activity')).at(-1)?.open===false);
    assert(!(await detail.evaluate(el=>el.open)),'Completed command collapses');
    await activity.locator(':scope > summary').click();await detail.locator(':scope > summary').click();
    publish({running:true,activities:snapshot.activities.map(a=>({...a,status:'running'}))});
    await page.waitForTimeout(350);publish({running:false,activities:snapshot.activities.map(a=>({...a,status:'failed'}))});
    await detail.getByText(zh?'失败':'Failed',{exact:true}).waitFor();
    assert(await activity.evaluate(el=>el.open),'Manual expansion survives completion');assert(await detail.evaluate(el=>el.open),'Manual detail expansion survives completion');
    for(const decision of ['accept','decline']){
     publish({running:true,approvals:[{id:'f'.repeat(32),kind:'fileChange',directory:'/project',reason:'Review this patch',command:'',changes:[{path:'notes.txt',kind:'update',diff:'@@ -1 +1 @@\n-old\n+new'}]}]});
     const approval=page.locator('.edge-agent-approval');await approval.getByText(zh?'确认文件修改':'Confirm file changes',{exact:true}).waitFor();
     await approval.locator('summary').click();
     await approval.locator('.edge-agent-diff-line.added').waitFor();
     assert(!(await approval.innerText()).includes('越出当前沙箱'),'File approval uses file-specific copy');
     await page.screenshot({path:`/tmp/agent-file-approval-${locale}-${dark}-${width}.png`,fullPage:true});
     await approval.getByRole('button',{name:decision==='accept'?(zh?'确认':'Confirm'):(zh?'取消':'Cancel'),exact:true}).click();await approval.waitFor({state:'detached'});
    }
    publish({running:true,input_requests:[{id:'e'.repeat(32),blocking:true,questions:[{id:'choice',question:'Choose or enter an answer',is_other:true,is_secret:true,options:[{label:'Default',description:'Use the default'}]}]}]});
    await inputForm.waitFor();assert(await inputForm.getByRole('button',{name:zh?'提交回答':'Submit answers'}).isDisabled());
    await inputForm.getByRole('radio',{name:zh?'其他回答':'Other answer',exact:true}).check();
    const secret=inputForm.locator('input[type=password]');await secret.fill('isolated-test-value');assert(!(await inputForm.getByRole('button',{name:zh?'提交回答':'Submit answers'}).isDisabled()));
    await inputForm.getByRole('button',{name:zh?'停止本轮':'Stop turn',exact:true}).click();await inputForm.waitFor({state:'detached'});
    publish({running:false,activities:[{id:'cancelled',kind:'userInput',status:'cancelled',message_index:0,duration_ms:0},{id:'ended',kind:'reasoning',status:'ended',message_index:0,duration_ms:0},{id:'failed',kind:'commandExecution',status:'failed',message_index:0,duration_ms:0,command:'exit 1',output:'Test failure',exit_code:1}]});
    await activity.getByText(zh?'已取消':'Cancelled',{exact:true}).waitFor();
    assert.equal(await activity.locator('.edge-agent-spinner').count(),0,'Ended history has no spinning indicators');
    assert(await page.locator('.edge-agent-messages').evaluate(el=>el.scrollWidth<=el.clientWidth+1));
    await page.screenshot({path:`/tmp/agent-disclosure-${locale}-${dark}-${width}.png`,fullPage:true});
    assert.deepEqual(errors,[]);await page.close();continue;
   }
   await page.screenshot({path:`/tmp/agent-details-${locale}-${dark}-${width}.png`,fullPage:true});
   await page.waitForFunction(()=>document.querySelectorAll('.agent-message.is-user').length===3);
   const roundUsers=await page.locator('.agent-message.is-user').allTextContents();
   assert(roundUsers[0].endsWith('project'));
   assert(roundUsers[1].includes('approval_test'));
   assert(roundUsers[2].includes('input_test'));
   assert.equal(await page.getByRole('button',{name:zh?'加载更早记录':'Load earlier messages',exact:true}).count(),0);
   await page.screenshot({path:`/tmp/agent-round-history-${locale}-${dark}-${width}.png`,fullPage:true});
   await page.getByRole('button',{name:zh?'选择工作目录':'Choose working directory',exact:true}).click();
   await page.getByRole('button',{name:'src',exact:true}).click();
   await page.getByText(zh?'此目录下没有可显示的子目录。':'No visible subfolders in this directory.').waitFor();
   await page.getByRole('button',{name:zh?'选择此目录':'Select folder',exact:true}).click();
   await page.getByRole('dialog').getByRole('button',{name:zh?'取消':'Cancel',exact:true}).click();
   assert.equal(starts,1);
   await page.getByRole('button',{name:zh?'选择工作目录':'Choose working directory',exact:true}).click();
   await page.getByRole('button',{name:'src',exact:true}).click();
   await page.getByRole('button',{name:zh?'选择此目录':'Select folder',exact:true}).click();
   await page.getByRole('dialog').getByRole('button',{name:zh?'新建会话':'New conversation',exact:true}).click();
   await page.locator('.edge-agent-project strong').filter({hasText:'src'}).waitFor();
   assert.equal(entries[0].project,'/Users/local/project','Switching must not change the saved access default');
   const input=page.getByRole('textbox',{name:zh?'消息':'Message',exact:true});
   await page.screenshot({path:`/tmp/agent-empty-${locale}-${dark}-${width}.png`,fullPage:true});
   const workspaceBox=await page.locator('.edge-agent-workspace').boundingBox();
   for(const button of await page.locator('.edge-agent-session-actions button').all()){
    const box=await button.boundingBox();assert(box.x>=workspaceBox.x&&box.x+box.width<=workspaceBox.x+workspaceBox.width,'Header actions must not be clipped');
   }
   await page.getByRole('button',{name:zh?'切换模型':'Switch model',exact:true}).click();
   const modelSelect=page.getByRole('dialog').getByRole('combobox');
   await modelSelect.selectOption('native-other');
   await page.getByRole('dialog').getByRole('button',{name:zh?'切换':'Switch',exact:true}).click();
   await page.locator('.edge-agent-model').filter({hasText:'native-other'}).waitFor();
   const before=await input.boundingBox();
   await input.fill('Keep this draft');
   const tools=page.getByRole('button',{name:zh?'添加与工具':'Add and tools',exact:true});
   await tools.click();
   await page.getByRole('menuitem',{name:zh?'命令与技能':'Commands and skills'}).waitFor();
   const toolMenu=await page.getByRole('menu').boundingBox(),triggerBox=await tools.boundingBox();
   assert(toolMenu.y>=0&&toolMenu.y+toolMenu.height<=triggerBox.y,'Tools open above the trigger');
   await page.screenshot({path:`/tmp/agent-tools-${locale}-${dark}-${width}.png`,fullPage:true});
   await page.keyboard.press('Escape');
   assert(await tools.evaluate(el=>document.activeElement===el),'Escape returns focus to the tools trigger');
   await tools.click();
   await page.getByRole('menuitem',{name:zh?'命令与技能':'Commands and skills'}).click();
   await page.getByRole('option',{name:'review-code Review the current project'}).click();
   assert.equal(await input.inputValue(),'Keep this draft','Choosing a skill from tools must preserve the draft');
   await page.getByRole('button',{name:zh?'移除技能':'Remove skill'}).click();
   await input.fill('/');
   await page.getByRole('listbox').waitFor();
   const after=await input.boundingBox(),menu=await page.locator('.edge-agent-slash').boundingBox();
   assert(Math.abs(before.y-after.y)<2,'Slash menu must not move the composer');
   assert(menu.y>=0&&menu.y+menu.height<=after.y,'Slash menu must open above the composer');
   assert(after.y+after.height<900,'Composer remains in viewport');
   await page.screenshot({path:`/tmp/agent-slash-${locale}-${dark}-${width}.png`,fullPage:true});
   await input.press('Escape');
   assert.equal(await page.getByRole('listbox').count(),0);
   await input.fill('/review');await page.getByRole('option',{name:'review-code Review the current project'}).waitFor();await input.press('Enter');
   await page.getByRole('button',{name:zh?'移除技能':'Remove skill'}).waitFor();
   await page.getByRole('textbox',{name:zh?'消息':'Message',exact:true}).fill('Explain this project');
   await page.getByRole('button',{name:zh?'发送':'Send',exact:true}).click();
   await page.getByRole('heading',{name:'Project',exact:true}).last().waitFor();
   assert(await input.evaluate(el=>document.activeElement===el),'Composer retains focus after send');
   assert.equal(sentSkill,'c'.repeat(32));
   assert.equal(starts,2);
   await page.locator('.edge-agent-activity summary').click();
   await page.getByText(zh?'执行命令':'Run command',{exact:true}).waitFor();
   assert.equal(await page.locator('.edge-agent-messages script,.edge-agent-messages img').count(),0);
   assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
   await page.screenshot({path:`/tmp/liaison-edge-agent-${locale}-${dark?'dark':'light'}-${width}.png`,fullPage:true});
   assert.equal(stops,0,'Directory switch must not stop the original session');
   await page.reload();
   await page.getByRole('heading',{name:'Project',exact:true}).last().waitFor();
   assert.equal(starts,2,'Reload must reconnect without launching a new process');
   const toggle=page.getByRole('button',{name:zh?/^(显示|收起)会话列表$/:/^(Show|Hide) conversations$/});
   if(await toggle.getAttribute('aria-expanded')==='false')await toggle.click();
   await page.getByRole('button',{name:zh?'刷新会话':'Refresh sessions'}).click();
   const groups=page.locator('.edge-agent-project-group');
   assert.equal(await groups.count(),2,'Group sessions by their full project directory');
   const group=groups.filter({has:page.locator('.edge-agent-group-path',{hasText:'/Users/local/project/src'})});
   const collapse=group.locator('.edge-agent-project-group-heading button').first();
   await collapse.click();
   assert.equal(await group.locator('.edge-agent-session-item').count(),0);
   await page.getByRole('button',{name:zh?'刷新会话':'Refresh sessions'}).click();
   assert.equal(await collapse.getAttribute('aria-expanded'),'false','Refreshing must retain collapsed projects');
   await collapse.click();
   await page.screenshot({path:`/tmp/agent-session-list-${locale}-${dark}-${width}.png`,fullPage:true});
   await page.locator('.edge-agent-session-item').filter({hasText:'project'}).filter({hasNotText:'Explain this project'}).click();
   await page.locator('.edge-agent-project strong').filter({hasText:'project'}).waitFor();
   assert.equal(starts,2);
   if(await toggle.getAttribute('aria-expanded')==='false')await toggle.click();
   await page.locator('.edge-agent-session-item').filter({hasText:'Explain this project'}).click();
   await page.getByRole('heading',{name:'Project',exact:true}).last().waitFor();
   await page.screenshot({path:`/tmp/agent-sessions-${locale}-${dark}-${width}.png`,fullPage:true});
   assert.equal(stops,0);
   if(await toggle.getAttribute('aria-expanded')==='false')await toggle.click();
   const selectedRow=page.locator('.edge-agent-session-row').filter({hasText:'Explain this project'});
   await selectedRow.getByRole('button',{name:zh?'会话操作':'Conversation actions'}).click();
   await page.getByRole('menuitem',{name:zh?'重命名':'Rename',exact:true}).click();
   await page.getByRole('dialog').getByRole('textbox').fill('Renamed conversation');
   await page.getByRole('dialog').getByRole('button',{name:zh?'保存':'Save',exact:true}).click();
   const renamed=page.locator('.edge-agent-session-row').filter({hasText:'Renamed conversation'});await renamed.waitFor();
   await page.screenshot({path:`/tmp/agent-controls-${locale}-${dark}-${width}.png`,fullPage:true});
   await renamed.getByRole('button',{name:zh?'会话操作':'Conversation actions'}).click();
   await page.getByRole('menuitem',{name:zh?'删除':'Delete',exact:true}).click();
   await page.getByRole('dialog').getByRole('button',{name:zh?'取消':'Cancel',exact:true}).click();
   assert.equal(sessions.size,2,'Cancel preserves session');
   await renamed.getByRole('button',{name:zh?'会话操作':'Conversation actions'}).click();
   await page.getByRole('menuitem',{name:zh?'删除':'Delete',exact:true}).click();
   await page.getByRole('dialog').getByRole('button',{name:zh?'删除':'Delete',exact:true}).click();
   await page.getByText(zh?'选择一个会话，或新建会话开始。':'Select a conversation or start a new one.',{exact:true}).waitFor();
   assert.equal(sessions.size,1);assert.equal(starts,2,'Deleting selected session must not auto-start another');
   await page.locator('.edge-agent-session-item').click();
   await page.getByRole('button',{name:zh?'切换模型':'Switch model',exact:true}).waitFor();
   // Replacing a new empty conversation removes only that conversation.
   await page.locator('.edge-agent-session-actions').getByRole('button',{name:zh?'新建会话':'New conversation',exact:true}).click();
   await page.getByText(zh?'从这个项目开始':'Start with this project',{exact:true}).waitFor();
   const emptyID=String(starts).padStart(32,'b');
   await page.getByRole('button',{name:zh?'选择工作目录':'Choose working directory',exact:true}).click();
   await page.getByRole('button',{name:'src',exact:true}).click();
   await page.getByRole('button',{name:zh?'选择此目录':'Select folder',exact:true}).click();
   await page.getByText(zh?'当前空会话将移除，其他会话不受影响。':'The current empty conversation will be removed. Other conversations are unaffected.').waitFor();
   await page.screenshot({path:`/tmp/agent-replace-${locale}-${dark}-${width}.png`,fullPage:true});
   await page.getByRole('dialog').getByRole('button',{name:zh?'新建会话':'New conversation',exact:true}).click();
   await page.locator('.edge-agent-project strong').filter({hasText:'src'}).waitFor();
   assert(!sessions.has(emptyID),'Discard the original empty session');
   assert(sessions.has('1'.padStart(32,'b')),'Retain existing conversation content');
   assert.equal(sessions.size,2);assert.equal(starts,4);
   // Server history is readable with an offline connector, without starting a process.
   if(await toggle.getAttribute('aria-expanded')==='false')await toggle.click();
   await page.getByRole('button',{name:zh?'刷新会话':'Refresh sessions'}).click();
   await page.locator('.edge-agent-session-item').filter({hasText:'project'}).click();
   await page.getByRole('heading',{name:'Project',exact:true}).last().waitFor();
   snapshot={...snapshot,closed:true,running:false,status:'session_closed'};sessions.set(snapshot.session_id,snapshot);
   offline=true;
   await page.goto((process.env.E2E_UI_URL||'http://127.0.0.1:5298')+'/e2e/edge-agent.html');
   await page.getByRole('button',{name:zh?'去访问':'Open',exact:true}).click();
   await page.getByTitle(zh?'连接器离线，暂时无法恢复。':'The connector is offline. Resume is temporarily unavailable.',{exact:true}).waitFor();
   await page.getByRole('heading',{name:'Project',exact:true}).last().waitFor();
   assert(await page.getByRole('button',{name:zh?'发送':'Send',exact:true}).isDisabled());
   assert.equal(starts,4,'Offline history must not launch a new process');
   assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
   await page.screenshot({path:`/tmp/agent-history-${locale}-${dark}-${width}.png`,fullPage:true});
   offline=false;await page.reload();
   await page.getByRole('heading',{name:'Project',exact:true}).last().waitFor();
   await page.getByRole('button',{name:zh?'恢复会话':'Resume conversation',exact:true}).click();
   await page.getByRole('textbox',{name:zh?'消息':'Message',exact:true}).waitFor();
   revoked=true;
   await page.getByRole('alert').filter({hasText:zh?'会话访问权限已失效':'Session access is no longer available'}).waitFor();
   assert(await page.getByRole('button',{name:zh?'发送':'Send',exact:true}).isDisabled());
   assert.deepEqual(errors,[]);await page.close();
  }
  console.log(process.env.E2E_FILES_ONLY==='1'?'PASS 8 locale/theme/viewport combinations; uploads, native-image attachment UI, paste, drag/drop, cancellation, permission failures, byte-exact download, attachment-only send':process.env.E2E_FOOTNOTE_ONLY==='1'?'PASS 8 locale/theme/viewport combinations; history info in details, clean footnote, responsive keyboard hint':'PASS 8 locale/theme/viewport combinations; discovery, chat, safe Markdown, permission revocation, overflow');
 }finally{await browser.close();}
})().catch(e=>{console.error(e);process.exitCode=1;});
