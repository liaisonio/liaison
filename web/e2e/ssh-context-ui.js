// Authenticated Playwright page whose URL is a saved, disposable WebSSH connection.
// Executes only cd /tmp, printf, false and a session-local history setting.
async (page) => {
  const base = new URL(page.url()).origin;
  const route = page.url();
  const token = await page.evaluate(() => localStorage.getItem('token'));
  const api = async (path, method = 'GET', data) => {
    const r = await page.context().request.fetch(base + path, {method, data, timeout:90000, headers:{Authorization:'Bearer '+token}});
    if (!r.ok()) throw Error(`API ${r.status()} ${path}`);
    return (await r.json()).data;
  };
  const assert = (condition, message) => {if(!condition) throw Error(message);};
  let output = '', handle, session;
  const errors = [], requests = [];
  page.on('pageerror', e => errors.push(e.message));
  page.on('request', r => {if(r.method()==='POST'&&r.url().endsWith('/assistance/suggestions')) requests.push(r.postDataJSON());});
  page.on('response', r => {if(r.request().method()==='POST'&&r.url().includes('/webssh/proxies/')&&r.url().endsWith('/session')) r.json().then(v=>handle=v.data?.token).catch(()=>{});});
  page.on('websocket', ws => ws.on('framereceived', e => {try {const m=JSON.parse(e.payload.toString());if(m.data)output+=m.data;}catch{}}));
  const until = async condition => {const end=Date.now()+25000;while(Date.now()<end){if(condition())return;await page.waitForTimeout(80);}throw Error('Timed out waiting for shell marker');};
  const prompts = () => (output.match(/633;B/g)||[]).length;
  const send = async command => {
    await page.locator('.webssh-terminal-screen').click();
    const start=output.length,n=prompts();await page.keyboard.type(command,{delay:15});await page.keyboard.press('Enter');
    await until(()=>prompts()>n);return output.slice(start);
  };
  try {
    await page.goto(route);await until(()=>prompts()>0&&handle);
    const mode=page.getByRole('combobox',{name:'AI 上下文共享',exact:true});
    assert(await mode.inputValue()==='none','Context must start disabled');
    await send('cd /tmp');
    const failed=await send("printf 'context-fixture\\n'; false");
    assert(failed.includes('633;E;'),'Shell did not report command history');
    assert(failed.includes('633;D;1'),'Failed command status lost');
    assert(failed.includes('633;P;Cwd=/tmp'),'Directory was not reported');
    await send('HISTCONTROL=ignorespace');
    const ignored=await send(" printf 'ignored-fixture\\n'");
    assert(!ignored.includes('633;E;'),'Ignored history reused another command');
    session=await api('/api/v1/agent/sessions','POST',{handle_id:handle,title:'E2E Shell context'});
    const path='/api/v1/agent/sessions/'+session.session.id;
    await api(path+'/turns','POST',{prompt:'请只调用 terminal.read 读取最近终端输出，简短说明最近命令结果，不执行任何命令，不调用其他工具。'});
    const detail=await api(path);assert(detail.approvals.length===0,'Read unexpectedly requested execution');
    const contexts=detail.messages.filter(m=>m.value.role==='tool').map(m=>JSON.parse(m.value.content).Content?.shell_context).filter(Boolean);
    assert(contexts.length>0,'Structured shell context missing from terminal.read');
    const snapshot=contexts.at(-1);
    assert(snapshot.directory==='/tmp','Wrong connection directory');
    assert(snapshot.recent_commands.some(c=>c.exit_code===1&&c.output.includes('context-fixture')),'Failed command/output association missing');
    assert(snapshot.recent_commands.at(-1).source==='unknown','Ignored command should be explicitly unknown');
    for (const value of ['commands','output','none']) {
      await mode.selectOption(value);await page.locator('.webssh-terminal-screen').click();
      await page.keyboard.type('ls ',{delay:40});await page.waitForTimeout(250);
      const response=page.waitForResponse(r=>r.request().method()==='POST'&&r.url().endsWith('/assistance/suggestions'));
      await page.keyboard.press('Control+Space');assert((await response).status()===200,'Context suggestion request failed');
      assert((requests.at(-1).context_mode||'none')===value,'Wrong context sharing scope');
      assert(!('shell_context' in requests.at(-1)),'Client must not upload server context');
      if(value==='output')await page.screenshot({path:'/tmp/liaison-shell-context.png'});
      const n=prompts();await page.keyboard.press('Control+c');await until(()=>prompts()>n);
    }
    assert(errors.length===0,errors.join('\n'));
    return {passed:true,checks:['cwd','failed exit status','history report','ignored history not reused','Agent structured context','three explicit sharing modes','server-authored context','no automatic execution']};
  } finally {
    if(session){const path='/api/v1/agent/sessions/'+session.session.id;const d=await api(path);await api(path,'DELETE',{version:d.session.version});}
    const disconnect=page.getByRole('button',{name:'断开',exact:true});if(await disconnect.count())await disconnect.click();
  }
}
