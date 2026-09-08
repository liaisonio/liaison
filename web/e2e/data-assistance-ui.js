// Deterministic UI race/acceptance tests; model responses alone are stubbed.
async(page)=>{
  const context=page.context().browser().contexts().find(c=>c.pages().some(p=>p.url().includes('/webdata/')));
  const existing=context.pages().find(p=>p.url().includes('/webdata/'));
  const baseURL=new URL(existing.url()).origin;
  const token=await existing.evaluate(()=>localStorage.getItem('token'));
  const api=async(path,method='GET')=>{const r=await context.request.fetch(baseURL+path,{method,headers:{Authorization:'Bearer '+token}});if(!r.ok())throw Error('API '+r.status());return(await r.json()).data;};
  const apps=(await api('/api/v1/applications?page=1&page_size=100')).applications;
  const results=[];
  for(const protocol of ['mysql','postgresql','mongodb','redis']){
    const app=apps.find(a=>a.name==='Agent Demo '+protocol);
    const target=await api('/api/v1/webdata/proxies/'+app.proxy.id);
    const credential=target.credentials.find(c=>c.name==='Agent demo '+protocol);
    const p=await context.newPage();let handle;let executions=0;let delayed=false;
    const pending=[];
    p.on('request',r=>{if(r.method()==='POST'&&(r.url().endsWith('/execute')||r.url().endsWith('/turns')))executions++;});
    p.on('response',r=>{if(r.request().method()==='POST'&&r.url().includes('/webdata/proxies/')&&r.url().endsWith('/session'))pending.push(r.json().then(v=>handle=v.data.token));});
    await p.route('**/api/v1/assistance/suggestions',async route=>{
      const data=route.request().postDataJSON();
      if(delayed)await p.waitForTimeout(700);
      await route.fulfill({json:{code:200,data:{revision:data.revision,cursor:data.cursor,text:' completed'}}}).catch(()=>{});
    });
    try{
      await p.goto(`${baseURL}/webdata/${app.proxy.id}/connections/${credential.id}`);
      const editor=p.locator('.webdata-statement-textarea');await editor.fill('draft');
      await p.getByRole('button',{name:'补全草稿',exact:true}).click();
      await p.getByRole('button',{name:'应用到编辑器',exact:true}).click();
      if(await editor.inputValue()!=='draft completed'||executions!==0)throw Error('Apply changed or executed draft');
      delayed=true;await editor.fill('old draft');await p.getByRole('button',{name:'补全草稿',exact:true}).click();
      await editor.fill('new draft');await p.waitForTimeout(900);
      if(await p.locator('.data-assistance-candidate').count()||await editor.inputValue()!=='new draft')throw Error('Stale response applied');
      if(!await editor.evaluate(e=>document.activeElement===e))throw Error('Suggestion stole focus');
      delayed=false;await p.getByRole('button',{name:'补全草稿',exact:true}).click();
      await p.getByRole('button',{name:'忽略建议',exact:true}).click();
      if(await p.locator('.data-assistance-candidate').count()||executions!==0)throw Error('Dismiss failed');
      results.push({protocol,passed:true,checks:['accept without execution','typing discards stale completion','focus retained','dismiss without execution']});
    }finally{await Promise.all(pending);if(handle)await api('/api/v1/webdata/sessions/'+handle,'DELETE');await p.close();}
  }
  return results;
}
