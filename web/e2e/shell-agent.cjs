// Disposable staging connection, normal login. No credentials in this file.
const assert = require('node:assert/strict');
const {chromium, request} = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
(async () => {
  const baseURL = process.env.STAGING_URL;
  const api = await request.newContext({baseURL, ignoreHTTPSErrors:true});
  const login = await (await api.post('/api/v1/iam/login', {data:{email:process.env.STAGING_EMAIL,password:process.env.STAGING_PASSWORD}})).json();
  assert(login.data?.token, 'Login failed');
  const token = login.data.token;
  const browser = await chromium.launch();
  const sessions = new Set();
  const call = async (path, method='GET', data) => {
    const r=await api.fetch(path,{method,data,headers:{Authorization:'Bearer '+token},timeout:120000});
    assert(r.ok(), `${method} ${path}: ${r.status()}`);return (await r.json()).data;
  };
  let page;
  try {
    const context=await browser.newContext({ignoreHTTPSErrors:true,viewport:{width:1440,height:1000}});
    await context.addInitScript(t=>{localStorage.setItem('token',t);localStorage.setItem('locale','zh-CN');localStorage.setItem('liaison-theme-preference','dark');},token);
    page=await context.newPage();
    const errors=[],sent=[];let requests=0,terminalOutput='';
    page.on('websocket',ws=>{
      ws.on('framesent',e=>{try{const v=JSON.parse(e.payload.toString());if(v.type==='input')sent.push(v.data);}catch{}});
      ws.on('framereceived',e=>{try{terminalOutput+=JSON.parse(e.payload.toString()).data||'';}catch{}});
    });
    page.on('pageerror',e=>errors.push(e.message));
    page.on('request',r=>{if(r.method()==='POST'&&r.url().endsWith('/turns'))requests++;});
    page.on('response',r=>{if(r.request().method()==='POST'&&r.url().endsWith('/agent/sessions'))r.json().then(v=>{if(v.data)sessions.add(v.data.session.id);}).catch(()=>{});});
    await page.goto(baseURL+(process.env.STAGING_WEBSSH_PATH||'/webssh/2/connections/1'));
    const shell=page.getByLabel('Shell Agent',{exact:true});await shell.waitFor();
    await page.locator('.webssh-terminal-screen').click();
    await page.keyboard.type("printf 'shell-agent-fixture\\n'; false",{delay:30});await page.keyboard.press('Enter');
    await shell.getByText('命令退出码 1').waitFor();assert.equal(requests,0,'Failure must not silently share context');
    await shell.getByRole('button',{name:'分析最近结果',exact:true}).click();
    await page.waitForResponse(r=>r.request().method()==='POST'&&r.url().endsWith('/turns'),{timeout:120000});
    await shell.locator('article').first().waitFor();
    assert.equal(sessions.size,1);const id=[...sessions][0];
    let detail=await call('/api/v1/agent/sessions/'+id);
    assert.equal(detail.session.kind,'shell');assert(detail.messages.some(m=>m.value.role==='tool'&&(m.value.content||'').includes('shell_context')));
    assert.equal(detail.approvals.length,0);
    const input=shell.getByRole('textbox',{name:'Shell 分析问题',exact:true});
    await input.fill("请执行诊断命令 printf 'shell-agent-approved'，只运行这一条，不运行其他命令。");
    await shell.getByRole('button',{name:'分析',exact:true}).click();
    const allow=shell.getByRole('button',{name:'允许一次',exact:true});await allow.waitFor({timeout:120000});
    const command=await shell.locator('.shell-agent-approval pre').innerText();
    assert.match(command,/^printf\s+['"]shell-agent-approved['"]\s*$/,'Refuse to approve an unexpected command');
    await allow.scrollIntoViewIfNeeded();await page.screenshot({path:process.env.SHELL_AGENT_SCREENSHOT||'/tmp/liaison-shell-agent.png'});
    await allow.click();await allow.waitFor({state:'hidden',timeout:120000});
    detail=await call('/api/v1/agent/sessions/'+id);
    assert(detail.messages.some(m=>m.value.role==='tool'&&(m.value.content||'').includes('shell-agent-approved')));
    // A unique value discussed only with Shell Agent must reach completion.
    // No file is created, and the suggested text is never executed.
    const marker='shell-memory-'+Date.now().toString(36);
    await input.fill(`后续我们用 printf 打印测试标记 ${marker}。请记住并回复这个标记即可，不读取终端，不调用工具，不执行命令。`);
    let completed=page.waitForResponse(r=>r.request().method()==='POST'&&r.url().endsWith('/turns'),{timeout:120000});
    await shell.getByRole('button',{name:'分析',exact:true}).click();await completed;
    detail=await call('/api/v1/agent/sessions/'+id);
    assert(detail.messages.some(m=>m.value.role==='assistant'&&(m.value.content||'').includes(marker)));
    const turnsBefore=detail.turns.length;
    const prefix="printf 'shell-memory-";
    await page.locator('.webssh-terminal-screen').click();await page.keyboard.type(prefix,{delay:40});
    const echoDeadline=Date.now()+5000;
    while(!terminalOutput.includes(prefix)&&Date.now()<echoDeadline)await page.waitForTimeout(50);
    assert(terminalOutput.includes(prefix),'Wait for remote draft echo before requesting completion');
    const suggestion=page.waitForResponse(r=>r.request().method()==='POST'&&r.url().endsWith('/assistance/suggestions'),{timeout:20000});
    await page.keyboard.press('Control+Space');
    const response=await suggestion;assert.equal(response.status(),200);
    assert.equal(response.request().postDataJSON().agent_session_id,id);
    const insertion=(await response.json()).data.text;
    assert((prefix+insertion).includes(marker),`Completion must use the conclusion from the same Shell Agent session; got ${JSON.stringify(insertion)}`);
    await page.locator('.webssh-completion').waitFor();
    await page.screenshot({path:process.env.SHELL_AGENT_SCREENSHOT||'/tmp/liaison-unified-shell.png'});
    const sentBefore=sent.length;await page.keyboard.press('Tab');
    const acceptedDeadline=Date.now()+5000;
    while(sent.length===sentBefore&&Date.now()<acceptedDeadline)await page.waitForTimeout(50);
    assert.equal(sent.slice(sentBefore).join(''),insertion,'Tab must send only the suggested suffix, without execution');
    await page.keyboard.press('Control+c');
    detail=await call('/api/v1/agent/sessions/'+id);
    assert.equal(detail.turns.length,turnsBefore,'Completion must not persist drafts as chat turns');
    assert.equal(sessions.size,1,'Completion and analysis must reuse one Shell session');
    console.log('PASS same Shell session and real-model memory across analysis/completion, no draft turns');
    await page.getByRole('button',{name:'Agent',exact:true}).click();
    await page.locator('.agent-workspace').waitFor();
    const deadline=Date.now()+20000;
    while(sessions.size<2&&Date.now()<deadline)await page.waitForTimeout(100);
    if(sessions.size!==2)console.error(await page.locator('.agent-workspace').innerText());
    assert.equal(sessions.size,2,'Shell and Sidepanel must have independent sessions');
    const chatId=[...sessions].find(x=>x!==id);const chat=await call('/api/v1/agent/sessions/'+chatId);
    assert.equal(chat.session.kind,'access');assert.equal(chat.messages.length,0);
    assert.deepEqual(errors,[]);
    console.log('PASS failure opt-in, Shell context analysis, approval-before-exec, real diagnostic result, separate Sidepanel history');
  } finally {
    for(const id of sessions){
      const path='/api/v1/agent/sessions/'+id;let d=await call(path);
      for(const a of d.approvals||[])if(a.status===0)await call(path+'/approvals/'+a.id,'POST',{decision:'deny'});
      d=await call(path);await call(path,'DELETE',{version:d.session.version});
    }
    if(page){const disconnect=page.getByRole('button',{name:'断开',exact:true});if(await disconnect.count())await disconnect.click();}
    await browser.close();await api.dispose();
  }
})().catch(e=>{console.error(e);process.exitCode=1;});
