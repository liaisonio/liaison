// Run in a browser context already signed into the staging environment.
async (page) => {
  const browser = page.context().browser();
  const existing = browser.contexts().flatMap(c=>c.pages()).find(p=>p.url().includes('/webdata/'));
  const context = existing.context();
  const baseURL = new URL(existing.url()).origin;
  const token = await existing.evaluate(()=>localStorage.getItem('token'));
  const api = async(path,method='GET',data) => {
    const r = await context.request.fetch(baseURL+path,{method,data,headers:{Authorization:'Bearer '+token}});
    if(!r.ok()) throw Error('API status '+r.status()); return (await r.json()).data;
  };
  const report=[];
  const apps=(await api('/api/v1/applications?page=1&page_size=100')).applications;
  for(const protocol of ['mysql','postgresql','mongodb','redis']) {
    const app=apps.find(a=>a.application_type===protocol&&a.name==='Agent Demo '+protocol);
    const id=app.proxy.id;
    const target=await api('/api/v1/webdata/proxies/'+id);
    const credential=target.credentials.find(c=>c.name==='Agent demo '+protocol).id;
    const p=await context.newPage(); await p.setViewportSize({width:1440,height:1000});
    let sessionID,handle;
    const pending=[];
    p.on('response',r=>{
      if(r.request().method()!=='POST')return;
      if(r.url().endsWith('/api/v1/agent/sessions')) pending.push(r.json().then(v=>sessionID=v.data?.session.id));
      if(r.url().includes('/webdata/proxies/')&&r.url().endsWith('/session'))pending.push(r.json().then(v=>handle=v.data?.token));
    });
    try {
      await p.goto(`${baseURL}/webdata/${id}/connections/${credential}`);
      await p.getByRole('button',{name:'Agent',exact:true}).click({timeout:20000});
      await p.locator('.agent-composer textarea').waitFor();
      await p.waitForFunction(()=>!document.querySelector('.agent-workspace-state'));
      const editor=p.locator('.webdata-statement-textarea');
      const statement=protocol==='redis'?'PING':protocol==='mongodb'?'{}':'SELECT 734 AS ui_e2e';
      await editor.fill(statement);
      const metrics=await p.evaluate(()=>{
        const r=s=>document.querySelector(s).getBoundingClientRect();
        return {header:r('.webdata-header').bottom-r('.agent-workspace>header').bottom,overflow:document.documentElement.scrollWidth>innerWidth};
      });
      if(metrics.header!==0||metrics.overflow)throw Error('Invalid desktop layout '+JSON.stringify(metrics));
      const composer=p.locator('.agent-composer textarea');await composer.fill('这是未发送草稿');await p.waitForTimeout(700);
      if(!await composer.evaluate(e=>e===document.activeElement))throw Error('Composer lost focus');
      const divider=p.getByRole('separator',{name:'调整 Agent 宽度'});
      await divider.focus();await divider.press('ArrowLeft');await p.waitForTimeout(100);
      if(await p.locator('.agent-workspace').evaluate(e=>e.getBoundingClientRect().width)!==440)throw Error('Resize failed');
      await divider.press('ArrowRight');
      await p.screenshot({path:`/tmp/liaison-${protocol}-agent-final.png`});
      await p.setViewportSize({width:1000,height:900});await p.waitForTimeout(150);
      if(await p.locator('.agent-workspace-root.is-docked').count())throw Error('Narrow layout must use overlay');
      await p.setViewportSize({width:390,height:844});await p.waitForTimeout(150);
      const mobile=await p.locator('.agent-workspace').evaluate(e=>({left:e.getBoundingClientRect().left,right:e.getBoundingClientRect().right}));
      if(mobile.left<0||mobile.right>390)throw Error('Mobile overflow');
      await p.setViewportSize({width:1440,height:1000});await p.waitForTimeout(150);
      await p.getByRole('button',{name:'Agent',exact:true}).click();
      await p.locator('.agent-workspace').waitFor({state:'detached'});
      if(await p.locator('.agent-workspace').count())throw Error('Panel did not close');
      if(await editor.inputValue()!==statement)throw Error('Editor draft lost');
      await p.getByRole('button',{name:'Agent',exact:true}).click();
      await composer.waitFor();
      if(await composer.inputValue()!=='这是未发送草稿')throw Error('Chat draft lost');
      const command=protocol==='redis'?'PING':protocol==='mongodb'?'{"ping":1}':'SELECT 734 AS ui_e2e';
      await composer.fill('只用 data.query 执行下面这条语句，不修改，不调用其他协议工具，执行后简短报告：'+command);
      await p.getByRole('button',{name:'发送',exact:true}).click();
      await p.getByRole('button',{name:'允许一次',exact:true}).waitFor({timeout:60000});
      await Promise.all(pending);
      const detail=await api('/api/v1/agent/sessions/'+sessionID);
      const approval=detail.approvals.find(a=>a.status===0);
      if(approval?.tool_id.namespace!=='data'||approval?.tool_id.name!=='query')throw Error('Unexpected approval');
      if(protocol==='mongodb' ? JSON.stringify(JSON.parse(approval.input.statement))!==command : approval.input.statement.trim()!==command)throw Error('Unexpected statement, refusing approval');
      const approvalResponse=p.waitForResponse(r=>r.request().method()==='POST'&&r.url().includes('/approvals/'),{timeout:90000});
      await p.getByRole('button',{name:'允许一次',exact:true}).click();
      if(!(await approvalResponse).ok())throw Error('Approval request failed');
      await p.waitForFunction(()=>!document.querySelector('.agent-workspace-state')&&!document.querySelector('.agent-approval'),{},{timeout:60000});
      if(await p.locator('.agent-workspace-error').count())throw Error('Agent UI error');
      const result=p.locator('.agent-tool-result').filter({hasText:'执行查询'}).last();
      await result.locator('summary').first().click({timeout:15000});
      if(await p.locator('.agent-data-table').count()===0)throw Error('Missing structured result table');
      await p.screenshot({path:`/tmp/liaison-${protocol}-agent-result.png`});
      report.push({protocol,passed:true,checks:['header alignment','no horizontal overflow','focus retained','resize','tablet overlay','mobile bounds','draft preserved across close/reopen','UI send/approve/structured result']});
    } finally {
      await Promise.all(pending);
      if(sessionID){
        const path='/api/v1/agent/sessions/'+sessionID;
        for(let attempt=0;attempt<20;attempt++){
          const s=await api(path);
          if(s.turns.some(t=>t.status<3)){await p.waitForTimeout(300);continue;}
          try {await api(path,'DELETE',{version:s.session.version});break;}catch(error){if(attempt===19)throw error;}
        }
      }
      if(handle)await api('/api/v1/webdata/sessions/'+handle,'DELETE');
      await p.close();
    }
  }
  return report;
}
