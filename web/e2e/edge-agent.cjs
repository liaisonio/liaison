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
   await page.route('**/api/v1/edge-agents**',async route=>{
    const req=route.request();let data;
    if(req.method()==='GET')data=[{id:6,name:'Mac mini',device:'Mac-mini.local',online:!offline}];
    else{
     const body=req.postDataJSON();
     if(body.action==='watch')await new Promise(resolve=>setTimeout(resolve,150));
     if(revoked&&['poll','watch'].includes(body.action))return route.fulfill({status:403,json:{message:'Forbidden'}});
     if(body.session_id)snapshot=sessions.get(body.session_id);
     if(body.action==='sessions')data={version:1,status:'ok',sessions_available:true,session_management:true,sessions:[...sessions.values()].map(s=>({session_id:s.session_id,thread_id:s.thread_id,title:s.title||s.messages[0]?.text||s.project.split('/').pop(),project:s.project,running:s.running,closed:s.closed,status:s.status,updated_at:'2026-09-21T03:00:00Z'}))};
     else if(body.action==='discover')data={version:1,status:'ok',installations:[{id:'a'.repeat(32),path:'/opt/homebrew/bin/codex',kind:'codex'}],default_project:'/Users/local/project'};
     else if(body.action==='directories'){const dir=body.directory||'/Users/local';data={version:1,status:dir==='/outside'?'invalid_request':'ok',directory:dir,parent_directory:'/Users/local',directory_roots:['/Users/local'],directories:dir.endsWith('/src')?[]:[{name:'src',path:'/Users/local/project/src'}]};}
     else if(body.action==='start'){starts++;data=snapshot={version:1,status:'ok',revision:1,session_id:String(starts).padStart(32,'b'),thread_id:'thread-'+starts,model:'configured-model',project:body.working_directory||body.project,started_at:'2026-09-21T03:00:00Z',skills_available:true,skills:[{id:'c'.repeat(32),name:'review-code',description:'Review the current project'}],running:false,closed:false,messages:[]};}
     else if(body.action==='resume'){data=snapshot={...snapshot,status:'ok',revision:snapshot.revision+1,running:false,closed:false,archived:false};}
     else if(body.action==='transcript'){data=transcripts.get(body.session_id+':'+(Number(body.history_before)-1))||{version:1,status:'not_found'};}
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
       {id:'file',kind:'fileChange',status:'completed',message_index:1,duration_ms:0,changes:[{path:'web/src/components/workspace.tsx',kind:'update',diff:'@@ -1,2 +1,2 @@\n-const title = "Old";\n+const title = "Workspace";\n export default title;'}]},
       {id:'input',kind:'userInput',status:'completed',message_index:1,duration_ms:0,output:'Layout?\n→ Compact\nName?\n→ Workspace'}
      ]};
     }
     else if(body.action==='rename'){data=snapshot={...snapshot,title:body.title};}
     else if(body.action==='delete'){sessions.delete(body.session_id);data={version:1,status:'ok'};}
     else if(body.action==='discard'){if(snapshot.running||snapshot.messages.length)data={version:1,status:'busy'};else{stops++;sessions.delete(body.session_id);data={version:1,status:'ok'};}}
     else if(body.action==='stop'){stops++;data=snapshot={...snapshot,closed:true};}
     else data=snapshot;
     if(body.action==='send'){snapshot.revision++;snapshot.activities=[{id:'safe-tool-id',kind:'commandExecution',status:'completed',message_index:1,duration_ms:85}];}
     if(body.action==='send'&&body.text==='__approval_test__'){snapshot.title='project';snapshot.running=true;snapshot.approvals=[{id:'f'.repeat(32),command:'ls /private/tmp/liaison-approval-test',directory:'/private/tmp/liaison-approval-test',reason:'Inspect a temporary test directory'}];}
     if(body.action==='send'&&body.text==='__input_test__'){snapshot.title='project';snapshot.running=true;snapshot.input_requests=[{id:'e'.repeat(32),blocking:true,questions:[{id:'layout',header:'Layout',question:'Which layout should we use?',is_other:true,options:[{label:'Compact',description:'Keep the workspace focused.'},{label:'Detailed',description:'Show more information.'}]},{id:'name',header:'Name',question:'What should the workspace be called?'}]}];}
     if(snapshot){snapshot.permissions_available=true;snapshot.permission_mode ||= 'read-only';}
     if(['permissions','approve','answer'].includes(body.action)&&snapshot)sessions.set(snapshot.session_id,snapshot);
     if(['start','resume','send','stop','model','rename'].includes(body.action)&&snapshot)sessions.set(snapshot.session_id,snapshot);
    }
    if(data&&!Array.isArray(data)){data={...data,history_persistent:true};if(offline&&data.session_id)data={...data,archived:true,closed:true,running:false};if(offline&&data.sessions)data.sessions=data.sessions.map(s=>({...s,closed:true,running:false}));}
    await route.fulfill({json:{data}});
   });
   await page.goto((process.env.E2E_UI_URL||'http://127.0.0.1:5298')+'/e2e/edge-agent.html');
   const zh=locale==='zh-CN';
   await page.getByRole('button',{name:zh?'新建访问':'Create access',exact:true}).click();
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
   await page.getByRole('button',{name:zh?'会话详情':'Session details',exact:true}).click();
   await page.getByText('thread-1',{exact:true}).waitFor();
   await page.getByRole('dialog').getByRole('button',{name:zh?'关闭':'Close',exact:true}).last().click();
   await page.getByRole('textbox',{name:zh?'消息':'Message',exact:true}).fill('project');
   await page.getByRole('button',{name:zh?'发送':'Send',exact:true}).click();
   await page.getByRole('heading',{name:'Project',exact:true}).waitFor();
   await page.getByRole('button',{name:zh?'只读':'Read-only',exact:true}).click();
   await page.getByRole('menuitem',{name:zh?'允许工作区写入':'Allow workspace writes',exact:true}).click();
   await page.getByRole('button',{name:zh?'工作区写入':'Workspace write',exact:true}).waitFor();
   await page.getByRole('textbox',{name:zh?'消息':'Message',exact:true}).fill('__approval_test__');
   await page.getByRole('button',{name:zh?'发送':'Send',exact:true}).click();
   await page.getByRole('button',{name:zh?'允许一次':'Allow once',exact:true}).waitFor();
   await page.screenshot({path:`/tmp/agent-approval-${locale}-${dark}-${width}.png`,fullPage:true});
   await page.getByRole('button',{name:zh?'允许一次':'Allow once',exact:true}).click();
   await page.getByRole('button',{name:zh?'允许一次':'Allow once',exact:true}).waitFor({state:'detached'});
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
   await page.locator('.edge-agent-activity > summary').click();
   await page.locator('.edge-agent-activity li.has-detail > details > summary').filter({hasText:zh?'执行命令':'Run command'}).click();
   await page.getByLabel(zh?'输出':'Output',{exact:true}).filter({hasText:'PASS'}).waitFor();
   await page.locator('.edge-agent-activity li.has-detail > details > summary').filter({hasText:zh?'文件操作':'File operation'}).click();
   await page.locator('.edge-agent-file-change > summary').click();
   await page.getByRole('region',{name:zh?'代码差异':'Code diff'}).waitFor();
   await page.getByRole('region',{name:zh?'代码差异':'Code diff'}).scrollIntoViewIfNeeded();
   assert.equal(await page.locator('.edge-agent-diff-line.added').count(),1);
   assert.equal(await page.locator('.edge-agent-diff-line.removed').count(),1);
   await page.screenshot({path:`/tmp/agent-details-${locale}-${dark}-${width}.png`,fullPage:true});
   await page.getByRole('button',{name:zh?'更早一轮':'Previous round',exact:true}).click();
   await page.getByRole('button',{name:zh?'返回最新':'Back to latest',exact:true}).waitFor();
   assert((await page.locator('.agent-message.is-user').innerText()).includes('approval_test'));
   await page.getByRole('button',{name:zh?'更早一轮':'Previous round',exact:true}).click();
   await page.waitForFunction(()=>document.querySelector('.agent-message.is-user')?.textContent?.endsWith('project'));
   assert(await page.getByRole('button',{name:zh?'更早一轮':'Previous round',exact:true}).isDisabled());
   await page.screenshot({path:`/tmp/agent-round-history-${locale}-${dark}-${width}.png`,fullPage:true});
   await page.getByRole('button',{name:zh?'返回最新':'Back to latest',exact:true}).click();
   await page.waitForFunction(()=>document.querySelector('.agent-message.is-user')?.textContent?.endsWith('input_test'));
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
   await page.getByRole('heading',{name:'Project',exact:true}).waitFor();
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
   await page.getByRole('heading',{name:'Project',exact:true}).waitFor();
   assert.equal(starts,2,'Reload must reconnect without launching a new process');
   const toggle=page.getByRole('button',{name:zh?/^(展开|收起)会话$/:/^(Show|Hide) sessions$/});
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
   await page.getByRole('heading',{name:'Project',exact:true}).waitFor();
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
   await page.getByRole('heading',{name:'Project',exact:true}).waitFor();
   snapshot={...snapshot,closed:true,running:false,status:'session_closed'};sessions.set(snapshot.session_id,snapshot);
   offline=true;
   await page.goto((process.env.E2E_UI_URL||'http://127.0.0.1:5298')+'/e2e/edge-agent.html');
   await page.getByRole('button',{name:zh?'去访问':'Open',exact:true}).click();
   await page.getByText(zh?'正在查看服务端保存的历史。若原设备上的 Codex 会话仍在，可以恢复续聊。':'Viewing server history. You can resume if the native Codex conversation is still available on the original device.',{exact:true}).waitFor();
   await page.getByRole('heading',{name:'Project',exact:true}).waitFor();
   assert(await page.getByRole('button',{name:zh?'发送':'Send',exact:true}).isDisabled());
   assert.equal(starts,4,'Offline history must not launch a new process');
   assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
   await page.screenshot({path:`/tmp/agent-history-${locale}-${dark}-${width}.png`,fullPage:true});
   offline=false;await page.reload();
   await page.getByRole('heading',{name:'Project',exact:true}).waitFor();
   await page.getByRole('button',{name:zh?'恢复会话':'Resume conversation',exact:true}).click();
   await page.getByRole('textbox',{name:zh?'消息':'Message',exact:true}).waitFor();
   revoked=true;
   await page.getByRole('alert').filter({hasText:zh?'会话访问权限已失效':'Session access is no longer available'}).waitFor();
   assert(await page.getByRole('button',{name:zh?'发送':'Send',exact:true}).isDisabled());
   assert.deepEqual(errors,[]);await page.close();
  }
  console.log('PASS 8 locale/theme/viewport combinations; discovery, chat, safe Markdown, permission revocation, overflow');
 }finally{await browser.close();}
})().catch(e=>{console.error(e);process.exitCode=1;});
