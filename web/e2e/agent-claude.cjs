// Synthetic API fixtures only. Native bridge coverage lives in Go opt-in tests.
const assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const origin=process.env.E2E_UI_URL||'http://127.0.0.1:5303';
const pause=ms=>new Promise(r=>setTimeout(r,ms));
(async()=>{
 const browser=await chromium.launch(process.env.CHROME_EXECUTABLE?{executablePath:process.env.CHROME_EXECUTABLE}:undefined);
 try{
  for(const locale of ['zh-CN','en-US'])for(const theme of ['light','dark'])for(const width of [1280,390]){
   const page=await browser.newPage({viewport:{width,height:900}}),zh=locale==='zh-CN';
   const errors=[],saved=[],actions=[];let rejectRecent=false,disconnect=false;
   page.on('pageerror',e=>errors.push(e.message));
   const app={id:'d'.repeat(32),name:'Claude workspace',kind:'claude',edge_id:1,installation_id:'b'.repeat(32),access_count:0};
   const access={id:'a'.repeat(32),name:'Claude access',kind:'claude',edge_id:1,installation_id:app.installation_id,application_id:app.id,application_name:app.name,project:'/workspace/demo',session_count:1};
   let revision=1,snapshot={version:1,status:'ok',session_id:'c'.repeat(32),thread_id:'native-claude-session',project:access.project,title:'Claude verification',model:'deepseek-flash',agent_version:'test',running:false,closed:false,history_persistent:true,messages:[{role:'user',text:'Hello'},{role:'assistant',text:'Ready to help.\n\n```go\nfmt.Println("hello")\n```'}]};
   await page.addInitScript(({locale,theme})=>{localStorage.setItem('token','synthetic-agent-test');localStorage.setItem('liaison-locale',locale);localStorage.setItem('liaison-theme-preference',theme);},{locale,theme});
   await page.route('**/api/v1/**',async route=>{
    const path=new URL(route.request().url()).pathname,method=route.request().method();let data={};
    if(path==='/api/v1/iam/account')data={id:1,name:'UI tester'};
    else if(path==='/api/v1/iam/permissions')data={'ai.access.use':true,'application.read':true,'application.create':true};
    else if(path==='/api/v1/edge-agents/connectors')data=[{id:1,name:'Test connector',device:'Test device',online:true}];
    else if(path==='/api/v1/agent-applications'){
     if(method==='POST'){const body=route.request().postDataJSON();assert.equal(body.kind,'claude');saved.push(body);data={...app,...body};}
     else data={items:[app,{...app,id:'e'.repeat(32),kind:'codex',name:'Codex workspace'}],total:2};
    }else if(path==='/api/v1/agent-accesses'){
     if(method==='POST'){const body=route.request().postDataJSON();assert.equal(body.kind,'claude');saved.push(body);data={...access,...body};}
     else data={items:[access],total:1};
    }else if(path.startsWith('/api/v1/agent-accesses/'))data=access;
    else if(path==='/api/v1/edge-agents'){
     const req=route.request().postDataJSON();actions.push(req);const action=req.action;
     if(disconnect&&(action==='poll'||action==='watch'))return route.abort('failed');
     if(action==='discover')data={version:1,status:'ok',default_project:access.project,installations:[{id:app.installation_id,kind:'claude',path:'/opt/bin/claude',source:'PATH'},{id:'e'.repeat(32),kind:'codex',path:'/opt/bin/codex',source:'PATH'}]};
     else if(action==='directories')data={version:1,status:rejectRecent?'invalid_request':'ok',directory:access.project,directories:[]};
     else if(action==='sessions')data={version:1,status:'ok',sessions_available:true,session_management:true,sessions:[{...snapshot,updated_at:'2026-01-01T00:00:00Z'}]};
     else if(action==='transcript')data={version:1,status:'ok',session_id:snapshot.session_id,history_pages:[]};
     else{
      if(action==='watch')await pause(120);
      if(action==='send'){assert.equal(req.skill_id,'9'.repeat(32));assert.equal(req.text,'Please run the greeting');snapshot={...snapshot,running:true,approvals:[{id:'f'.repeat(32),kind:'commandExecution',command:'printf hello',directory:access.project,reason:'Print a greeting'}]};revision++;}
      if(action==='approve'){assert.equal(req.decision,'accept');snapshot={...snapshot,approvals:[],input_requests:[{id:'f'.repeat(32),blocking:true,questions:[{id:'q1',header:'Style',question:'Choose a style',is_other:true,is_secret:false,options:[{label:'Brief',description:'Short response'},{label:'Detailed',description:'More detail'}]}]}]};revision++;}
      if(action==='answer'){assert.deepEqual(req.answers,[{question_id:'q1',answers:['Brief']}]);snapshot={...snapshot,running:false,input_requests:[],messages:[...snapshot.messages,{role:'assistant',text:'Done. Brief response.'}]};revision++;}
      if(action==='start'){assert.equal(req.installation_id,app.installation_id);snapshot={...snapshot,messages:[],running:false};revision++;}
      if(action==='models')snapshot={...snapshot,models_available:true,models:[{id:'local-model',name:'Local model'}]};
      if(action==='model'){assert.equal(req.model,'local-model');snapshot={...snapshot,model:req.model};revision++;}
      if(action==='interrupt'){snapshot={...snapshot,running:false};revision++;}
      assert(['poll','watch','send','approve','answer','start','models','model','interrupt'].includes(action),'unexpected Agent operation: '+action);
      data={...snapshot,revision};
     }
    }
    await route.fulfill({json:{code:200,data}});
   });
   await page.goto(origin+'/access/agents');
   await page.getByText('Claude access',{exact:true}).waitFor();
   await page.getByRole('button',{name:zh?'新建访问':'Create access',exact:true}).click();
   let dialog=page.getByRole('dialog');
   await dialog.locator('select').nth(0).selectOption('claude');
   await dialog.locator('select').nth(3).selectOption(app.id);
   assert.equal(await dialog.getByRole('option',{name:'Codex workspace'}).count(),0);
   await dialog.getByPlaceholder('/path/to/project').fill(access.project);
   await page.screenshot({path:`/tmp/claude-form-${locale}-${theme}-${width}.png`});
   await dialog.getByRole('button',{name:zh?'创建访问':'Create access',exact:true}).click();
   await dialog.waitFor({state:'hidden'});
   await page.getByRole('button',{name:zh?'新建访问':'Create access',exact:true}).click();dialog=page.getByRole('dialog');
   await dialog.locator('select').nth(0).selectOption('claude');
   await dialog.locator('select').nth(2).selectOption('new');
   await dialog.getByRole('button',{name:zh?'发现已安装的 Agent':'Discover installed Agents'}).click();
   await dialog.getByRole('option',{name:'/opt/bin/claude'}).waitFor({state:'attached'});
   assert.equal(await dialog.getByRole('option',{name:'/opt/bin/codex'}).count(),0);
   await dialog.getByRole('button',{name:zh?'创建访问':'Create access',exact:true}).click();await dialog.waitFor({state:'hidden'});
   assert.equal(saved.length,3);
   await page.goto(origin+'/resource/app?category=agent');
   await page.getByRole('button',{name:zh?'新建应用':'Create application',exact:true}).click();
   dialog=page.getByRole('dialog');
   await dialog.locator('select').nth(0).selectOption('claude');
   await dialog.getByRole('button',{name:zh?'发现已安装的 Agent':'Discover installed Agents'}).click();
   await dialog.getByRole('option',{name:'/opt/bin/claude'}).waitFor({state:'attached'});
   assert.equal(await dialog.getByRole('option',{name:'/opt/bin/codex'}).count(),0);
   await page.screenshot({path:`/tmp/claude-application-${locale}-${theme}-${width}.png`});
   await dialog.getByRole('button',{name:zh?'保存':'Save',exact:true}).click();
   await dialog.waitFor({state:'hidden'});
   assert.equal(saved.length,4);
   snapshot={...snapshot,skills_available:true,skills:[{id:'9'.repeat(32),name:'/review',description:'Review the supplied request'}]};
   await page.goto(`${origin}/access/agents?access=${access.id}&session=${snapshot.session_id}&view=full`);
   await page.getByText('Ready to help.',{exact:true}).waitFor();
   assert.equal(await page.locator('.edge-agent-kind').innerText(),'Claude Code');
   assert.equal(await page.locator('.edge-agent-codex-label').last().innerText(),'Claude Code');
   await page.getByRole('button',{name:zh?'切换模型':'Switch model',exact:true}).click();
   const modelDialog=page.getByRole('dialog');
   await modelDialog.getByText(zh?'这里是 Claude Code 提供的模型选项，可能包含别名。实际调用模型由设备配置映射；选择 Sonnet 等选项不代表一定调用 Anthropic。':'These are model options from Claude Code and may include aliases. Device settings determine the actual model; choosing Sonnet does not necessarily call Anthropic.',{exact:true}).waitFor();
   await modelDialog.getByRole('combobox').selectOption('local-model');
   await page.screenshot({path:`/tmp/claude-model-${locale}-${theme}-${width}.png`});
   await modelDialog.getByRole('button',{name:zh?'切换':'Switch',exact:true}).click();
   await page.locator('.edge-agent-model').getByText('local-model',{exact:true}).waitFor();
   assert.equal(await page.locator('.edge-agent-permission').innerText(),zh?'默认权限':'Default permissions');
   assert(await page.locator('.edge-agent-permission span[title]').count());
   await page.locator('textarea').fill('/');
   await page.getByRole('option',{name:/\/status/}).waitFor();
   await page.getByText(zh?'Liaison 快捷操作':'Liaison shortcuts',{exact:true}).waitFor();
   await page.getByText(zh?'Claude Code 原生命令与技能':'Claude Code native commands and skills',{exact:true}).waitFor();
   await page.screenshot({path:`/tmp/claude-commands-${locale}-${theme}-${width}.png`});
   await page.locator('textarea').fill('/review');
   await page.locator('textarea').press('Enter');
   await page.locator('.edge-agent-selected-skill').getByText('/review',{exact:true}).waitFor();
   assert(await page.getByRole('button',{name:zh?'发送':'Send',exact:true}).isDisabled());
   await page.locator('textarea').fill('Please run the greeting');
   await page.locator('textarea').press('Enter');
   dialog=page.locator('.edge-agent-approval');await dialog.waitFor();
   await dialog.getByText('printf hello',{exact:true}).waitFor();
   await page.screenshot({path:`/tmp/claude-approval-${locale}-${theme}-${width}.png`});
   await dialog.getByRole('button',{name:zh?'确认':'Confirm',exact:true}).click();
   const question=page.getByRole('form',{name:zh?'回答问题':'Answer questions'});await question.waitFor();
   await question.getByRole('radio',{name:/Brief/}).check();
   await question.getByRole('button',{name:zh?'提交回答':'Submit answers'}).click();
   await page.getByText('Done. Brief response.',{exact:true}).waitFor();
   await page.locator('textarea').fill('/new');
   await page.locator('textarea').press('Enter');
   const directoryDialog=page.getByRole('dialog');
   const recentProject=directoryDialog.getByRole('button',{name:`${zh?'打开最近项目':'Open recent project'} ${access.project}`,exact:true});
   await recentProject.click();
   assert(actions.some(a=>a.action==='directories'&&a.directory===access.project&&a.access_id===access.id));
   await directoryDialog.getByRole('button',{name:zh?'在此目录新建':'Create in this folder',exact:true}).waitFor();
   await page.screenshot({path:`/tmp/claude-recent-${locale}-${theme}-${width}.png`});
   rejectRecent=true;
   await recentProject.click();
   await directoryDialog.getByText(zh?'无法打开目录。请检查路径、目录范围和连接器版本。':'Cannot open this folder. Check the path, allowed roots and connector version.',{exact:true}).waitFor();
   assert(await directoryDialog.getByRole('button',{name:zh?'在此目录新建':'Create in this folder',exact:true}).isDisabled());
   await directoryDialog.getByRole('button',{name:zh?'取消':'Cancel',exact:true}).click();
   rejectRecent=false;
   // Reload/reconnect observes the same active turn; neither action replays Send.
   snapshot={...snapshot,running:true};revision++;
   const sends=actions.filter(a=>a.action==='send').length;
   await page.reload();
   const stop=page.getByRole('button',{name:zh?'停止':'Stop',exact:true});await stop.waitFor();
   disconnect=true;
   const interrupted=page.getByText(zh?'连接暂时中断，正在重连。不会重发未确认的操作；切换页面不会中断任务。':'Connection interrupted. Reconnecting without replaying operations. Switching pages does not stop a running task.',{exact:true});
   await interrupted.waitFor();disconnect=false;await interrupted.waitFor({state:'hidden'});
   assert.equal(actions.filter(a=>a.action==='send').length,sends,'reload/reconnect must not replay Send');
   await stop.click();await page.getByRole('button',{name:zh?'发送':'Send',exact:true}).waitFor();
   assert.equal(actions.filter(a=>a.action==='interrupt').length,1);
   await page.screenshot({path:`/tmp/claude-workspace-${locale}-${theme}-${width}.png`});
   assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),'page overflow');
   assert.deepEqual(errors,[]);
   await page.close();console.log(`PASS Claude ${locale} ${theme} ${width}: forms, provider routing, chat, approval, question`);
  }
 }finally{await browser.close();}
})().catch(e=>{console.error(e);process.exitCode=1;});
