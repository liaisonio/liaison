// Isolated access creation/navigation QA; no external connections or credentials.
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const assert=require('node:assert/strict');
(async()=>{
 const browser=await chromium.launch();
 try {for(const locale of ['zh-CN','en-US'])for(const theme of ['dark','light']){
  const ctx=await browser.newContext({viewport:{width:1440,height:1000}}),zh=locale==='zh-CN';
  await ctx.addInitScript(({locale,theme})=>{localStorage.setItem('liaison-locale',locale);localStorage.setItem('liaison-theme-preference',theme)},{locale,theme});
  const apps=['ssh','rdp','vnc','mysql','mariadb','postgresql','sqlserver','oracle','clickhouse','elasticsearch','opensearch','redis','mongodb','http','llm'].map((type,i)=>({id:i+1,name:`${type} service`,application_type:type,ip:'server.example',port:22}));
  const types=['webssh','websftp','webrdp','webvnc','webmysql','aiapi','http','ssh','tcp','webredis'];
  const proxies=types.map((type,i)=>({id:i+1,name:`${type} access`,access_protocol:type,application:apps.find(a=>a.application_type===(type==='aiapi'?'llm':type==='websftp'?'ssh':type.replace(/^web/,'')))||apps[0],status:'running',expose_public_port:!type.startsWith('web')&&type!=='aiapi',port:type.startsWith('web')?0:3000,access_url:'https://service.example',created_at:'2026-01-01T00:00:00Z'}));
  const writes=[],errors=[];let failSave=true,failToggle=true;
  await ctx.route('**/api/v1/**',async route=>{
   const req=route.request(),url=new URL(req.url());let data={};
   if(url.pathname.endsWith('/workspace'))data={models:['qwen-demo'],upstream_protocol:'openai-compatible',external_protocol:'openai-compatible',enabled:true};
   else if(url.pathname==='/api/v1/applications')data={applications:apps};
   else if(url.pathname==='/api/v1/proxies'){
    if(req.method()==='GET')data={proxies};
    else{writes.push({path:url.pathname,body:req.postDataJSON()});data={id:99,name:'New access'};}
   }else if(req.method()==='PUT'&&/^\/api\/v1\/proxies\/\d+$/.test(url.pathname)&&req.postDataJSON().status){
    await new Promise(r=>setTimeout(r,180));
    if(failToggle){failToggle=false;await route.fulfill({status:400,json:{code:400,message:'Status update failed'}});return;}
    data={status:req.postDataJSON().status};
   }else if(url.pathname.endsWith('/credential')){
    writes.push({path:url.pathname,body:req.postDataJSON()});
    if(failSave){failSave=false;await route.fulfill({status:400,json:{code:400,message:'fixture error'}});return;}
   }else if(/\/proxies\/\d+$/.test(url.pathname))data={protocol:url.pathname.includes('webdesktop')?'rdp':'mysql',credentials:[{id:7,username:'demo',saved:true,database:'analytics',schema:'reporting',tls_mode:'require',connection_params:'do-not-display-secret',password:'do-not-display-password'}]};
   await route.fulfill({json:{code:200,data}});
  });
  const page=await ctx.newPage();page.on('pageerror',e=>errors.push(e.message));
  const btn=(cn,en)=>page.getByRole('button',{name:zh?cn:en,exact:true});
  const openCategory=async query=>page.goto(`${process.env.E2E_UI_URL}/e2e/access.html?entry=${encodeURIComponent('/proxy?'+query)}`);
  await openCategory('category=database');
  await page.getByText('webmysql access',{exact:true}).waitFor();
  assert.equal(await page.getByText('webssh access',{exact:true}).count(),0);
  const tabs=page.getByRole('navigation',{name:zh?'访问类型':'Access types'});
  assert.equal(await tabs.getByRole('button',{name:'WebMySQL',exact:true}).count(),1);
  assert.equal(await tabs.getByRole('button',{name:'MySQL',exact:true}).count(),0);
  assert.equal(await tabs.getByRole('button',{name:'WebRedis',exact:true}).count(),0);
  assert.equal(await page.getByText('webredis access',{exact:true}).count(),0);
  await tabs.getByRole('button',{name:'WebMongoDB',exact:true}).click();
  assert.equal(await page.getByText('webmysql access',{exact:true}).count(),0);
  await tabs.getByRole('button',{name:'WebMySQL',exact:true}).click();
  await page.getByText('webmysql access',{exact:true}).waitFor();
  await page.screenshot({path:`/tmp/access-groups-${locale}-${theme}.png`});
  await page.setViewportSize({width:390,height:844});
  await page.getByRole('button',{name:zh?'更多协议':'More protocols',exact:true}).waitFor({state:'visible'});
  assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));
  await page.screenshot({path:`/tmp/access-groups-mobile-${locale}-${theme}.png`});
  await page.setViewportSize({width:1440,height:1000});
  for(const query of ['category=cache','access_type=webredis','category=database&access_type=webredis']) {
   await openCategory(query);
   await page.getByText('webredis access',{exact:true}).waitFor();
   assert.equal(await page.getByText('webmysql access',{exact:true}).count(),0);
  }
  await page.screenshot({path:`/tmp/access-cache-${locale}-${theme}.png`});
  await page.setViewportSize({width:390,height:844});
  assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));
  await page.screenshot({path:`/tmp/access-cache-mobile-${locale}-${theme}.png`});
  await page.setViewportSize({width:1440,height:1000});
  await openCategory('category=desktop');
  await page.getByText('webrdp access',{exact:true}).waitFor();
  assert.equal(await tabs.getByRole('button',{name:'WebVNC',exact:true}).count(),1);
  await openCategory('category=ssh');
  await page.getByText('webssh access',{exact:true}).waitFor();
  for(const name of ['WebSSH','WebSFTP','SSH'])assert.equal(await tabs.getByRole('button',{name,exact:true}).count(),1);
  await openCategory('category=database&access_type=webssh');
  await page.getByText('webmysql access',{exact:true}).waitFor();
  assert.equal(await page.getByText('webssh access',{exact:true}).count(),0);
  await page.goto(`${process.env.E2E_UI_URL}/e2e/access.html`);
  await page.getByText('webssh access',{exact:true}).waitFor();
  const accessTypeCell=page.getByRole('row').filter({has:page.getByText('aiapi access',{exact:true})}).getByRole('cell').nth(1);
  assert.equal(await accessTypeCell.textContent(),'OpenAI');
  assert.equal(await accessTypeCell.locator('svg,img').count(),0);
  assert.equal(await page.getByRole('row').filter({has:page.getByText('websftp access',{exact:true})}).locator('svg.lucide-folder-sync').count(),0);
  assert(await page.getByRole('row').filter({has:page.getByText('webssh access',{exact:true})}).locator('svg.lucide-square-terminal').count()>0);
  assert.equal(await btn('更多操作','More actions').count(),0);
  assert.equal(await page.getByRole('columnheader',{name:zh?'我的连接':'My connection',exact:true}).count(),0);
  const appLink=page.getByRole('row').filter({has:page.getByText('webmysql access',{exact:true})}).getByRole('link',{name:'mysql service',exact:true});
  assert.equal(await appLink.getAttribute('href'),'/resource/app?application_id=4');
  assert.equal(await appLink.evaluate(e=>getComputedStyle(e).textDecorationLine),'none');
  await appLink.hover();assert.equal(await appLink.evaluate(e=>getComputedStyle(e).textDecorationLine),'underline');
  await page.goto(`${process.env.E2E_UI_URL}/e2e/access.html?entry=${encodeURIComponent('/proxy?access_type=webmysql')}`);
  const dbRow=page.getByRole('row').filter({has:page.getByText('webmysql access',{exact:true})});
  for(const name of (zh?['用户名','数据库','连接选项']:['Username','Database','Connection options']))await page.getByRole('columnheader',{name,exact:true}).waitFor();
  assert.equal(await page.getByRole('columnheader',{name:zh?'我的连接':'My connection',exact:true}).count(),0);
  await dbRow.getByText('demo',{exact:true}).waitFor();
  assert((await dbRow.textContent()).includes('analytics'));
  assert((await dbRow.textContent()).includes('Schema: reporting'));
  assert((await dbRow.textContent()).includes('TLS: require'));
  assert(!(await page.locator('body').textContent()).includes('do-not-display'));
  await page.screenshot({path:`/tmp/access-columns-${locale}-${theme}.png`});
  const actionLayout=await dbRow.locator('td.is-fixed-right').evaluate(td=>{const group=td.querySelector('.liaison-access-actions');const buttons=[...group.children];return {extra:td.getBoundingClientRect().width-group.getBoundingClientRect().width,gaps:buttons.slice(1).map((b,i)=>b.getBoundingClientRect().left-buttons[i].getBoundingClientRect().right)};});
  assert(actionLayout.extra<=30,JSON.stringify(actionLayout));assert(actionLayout.gaps.every(g=>g>=10&&g<=14),JSON.stringify(actionLayout));
  await page.goto(`${process.env.E2E_UI_URL}/e2e/access.html`);await page.getByText('webssh access',{exact:true}).waitFor();
  for(const type of types){
   const row=page.getByRole('row').filter({has:page.getByText(`${type} access`,{exact:true})});
   assert.equal(await row.getByRole('button',{name:zh?'删除':'Delete',exact:true}).count(),1);
   assert.equal(await page.getByRole('menuitem',{name:zh?'连接管理':'Manage connections',exact:true}).count(),0);
   assert.equal(await row.getByRole('button',{name:zh?'停用':'Disable',exact:true}).count(),0);
   assert.equal(await row.getByRole('switch').getAttribute('aria-checked'),'true');
   assert.equal(await page.getByRole('menu').count(),0);
  }
  await page.screenshot({path:`/tmp/access-menu-${locale}-${theme}.png`});
  const toggle=page.getByRole('row').filter({has:page.getByText('webssh access',{exact:true})}).getByRole('switch');
  assert.deepEqual(await toggle.evaluate(e=>({width:getComputedStyle(e).width,height:getComputedStyle(e).height})),{width:'36px',height:'20px'});
  await toggle.click();assert(await toggle.isDisabled());await page.waitForFunction(()=>!document.querySelector('button[role=switch]:disabled'));assert.equal(await toggle.getAttribute('aria-checked'),'true');
  await toggle.click();await page.waitForFunction(()=>!document.querySelector('button[role=switch]:disabled'));assert.equal(await toggle.getAttribute('aria-checked'),'false');
  await toggle.click();await page.waitForFunction(()=>!document.querySelector('button[role=switch]:disabled'));assert.equal(await toggle.getAttribute('aria-checked'),'true');
  await btn('新建访问','Create access').click();
  const form=page.locator('#create-proxy'),selects=form.locator('select');
  await selects.first().selectOption('webmemcached');
  assert.equal(await form.locator('input[type=password]').count(),0);
  assert.equal(await form.locator('input[autocomplete=username]').count(),0);
  assert.equal(await form.locator('select').last().inputValue(),'disable');
  assert.equal(await selects.first().locator('option[value=openai]').textContent(),'OpenAI');
  const offered=await selects.first().locator('option').evaluateAll(options=>options.map(option=>option.value));
  for(const unsupported of ['rdp','vnc','mysql','mariadb','postgresql','sqlserver','oracle','clickhouse','elasticsearch','opensearch','redis','mongodb'])assert(!offered.includes(unsupported),`Unimplemented native server offered: ${unsupported}`);
  for(const supported of ['tcp','http','ssh','webrdp','webvnc','openai','anthropic','ark','qwen','gemini','ollama'])assert(offered.includes(supported),`Missing supported access: ${supported}`);
  for(const type of ['webssh','websftp','webrdp','webvnc','webmysql','webmariadb','webpostgresql','websqlserver','weboracle','webclickhouse','webelasticsearch','webopensearch','webredis','webmongodb']){
   await selects.first().selectOption(type);
   await form.locator('input[type=password]').waitFor();
   assert.equal(await form.locator('input[type=password]').count(),1);
   assert.equal(await form.locator('input[autocomplete=username]').count(),type==='webvnc'?0:1);
  }
  await selects.first().selectOption('webssh');await selects.nth(1).selectOption('1');
  assert.equal(await form.getByRole('checkbox').count(),1);
  await form.getByRole('checkbox').uncheck();assert.equal(await form.locator('input[type=password]').count(),0);
  await form.getByRole('checkbox').check();await form.locator('input[autocomplete=username]').fill('demo');await form.locator('input[type=password]').fill('fixture-only');
  await page.screenshot({path:`/tmp/access-create-${locale}-${theme}.png`});
  await btn('确定','Create').click();await page.locator('.liaison-modal .liaison-notice').waitFor();
  assert.equal(writes.filter(w=>w.path==='/api/v1/proxies').length,1);
  assert(!JSON.stringify(writes[0]).includes('fixture-only'));
  assert.equal(await form.locator('input[type=password]').inputValue(),'fixture-only');
  await btn('保存连接','Save connection').click();await page.locator('.liaison-modal').waitFor({state:'hidden'});
  assert.equal(writes.filter(w=>w.path==='/api/v1/proxies').length,1);
  assert.equal(writes.filter(w=>w.path.endsWith('/credential')).length,2);
  await page.getByRole('row').filter({has:page.getByText('webssh access',{exact:true})}).getByRole('button',{name:zh?'编辑':'Edit',exact:true}).click();
  const edit=page.locator('#edit-proxy');await edit.locator('input[type=password]').waitFor();
  assert.equal(await edit.locator('input[type=password]').inputValue(),'');assert.equal(await edit.locator('input[type=password]').getAttribute('placeholder'),'••••••••');
  assert.equal(await edit.locator('input[autocomplete=username]').inputValue(),'demo');
  await btn('确定','Save').click();await page.locator('.liaison-modal').waitFor({state:'hidden'});
  assert.equal(writes.at(-1).body.password,'');
  await page.getByRole('row').filter({has:page.getByText('webssh access',{exact:true})}).getByRole('button',{name:zh?'编辑':'Edit',exact:true}).click();
  await edit.getByRole('checkbox').uncheck();await btn('确定','Save').click();await page.locator('.liaison-modal').waitFor({state:'hidden'});
  assert.equal(writes.at(-1).body.remember_password,false);assert.equal(writes.at(-1).body.password,'');
  await page.setViewportSize({width:390,height:844});await btn('新建访问','Create access').click();
  await selects.first().selectOption('webpostgresql');await selects.nth(1).selectOption('6');
  await page.screenshot({path:`/tmp/access-create-${locale}-${theme}-mobile.png`});
  assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
  await btn('取消','Cancel').click();await page.setViewportSize({width:1440,height:1000});
  await page.getByRole('row').filter({hasText:'webssh access'}).getByRole('button',{name:zh?'去访问':'Open',exact:true}).click();
  await page.locator('output').waitFor();assert((await page.locator('output').textContent()).startsWith('/webssh/1/connections/7?from='));
  for(const [type,path] of [['websftp','/websftp/2?connect=1&from='],['webrdp','/webdesktop/3/connections/7?from='],['webvnc','/webdesktop/4/connections/7?from='],['webmysql','/webdata/5/connections/7?from=']]){
   await page.goto(`${process.env.E2E_UI_URL}/e2e/access.html`);
   await page.getByRole('row').filter({has:page.getByText(`${type} access`,{exact:true})}).getByRole('button',{name:zh?'去访问':'Open',exact:true}).click();
   await page.locator('output').waitFor();assert((await page.locator('output').textContent()).startsWith(path));
  }
  for(const [entry,destination] of [['/webssh/1','/webssh/1/connections/7'],['/webdesktop/3','/webdesktop/3/connections/7'],['/webdata/5','/webdata/5/connections/7']]){
   await page.goto(`${process.env.E2E_UI_URL}/e2e/access.html?entry=${encodeURIComponent(entry)}`);await page.locator('output').waitFor();assert.equal(await page.locator('output').textContent(),destination);
  }
  await page.goto(`${process.env.E2E_UI_URL}/e2e/access.html?entry=${encodeURIComponent('/proxy?access_type=aiapi')}`);
  const llmRow=page.getByRole('row').filter({has:page.getByText('aiapi access',{exact:true})});
  await llmRow.getByText('qwen-demo',{exact:true}).waitFor();
  assert.equal(await llmRow.getByText('OpenAI',{exact:true}).count(),3);
  assert.equal(await llmRow.getByRole('button',{name:zh?'去访问':'Open',exact:true}).count(),0);
  await llmRow.getByRole('switch').click();await page.waitForFunction(()=>!document.querySelector('button[role=switch]:disabled'));
  assert.equal(await llmRow.getByRole('switch').getAttribute('aria-checked'),'false');
  const view=llmRow.getByRole('link',{name:zh?'查看':'View',exact:true});assert.equal(await view.getAttribute('href'),'/ai/6?from=%2Fproxy%3Faccess_type%3Daiapi');
  assert.equal(await view.evaluate(e=>getComputedStyle(e).textDecorationLine),'none');
  assert.equal(await llmRow.locator('svg circle').count(),0);
  await page.screenshot({path:`/tmp/access-llm-${locale}-${theme}.png`});
  await view.click();await page.locator('output').waitFor();assert((await page.locator('output').textContent()).startsWith('/ai/6?from='));
  assert.deepEqual(errors,[]);console.log('PASS',locale,theme,'access forms, switch rollback, LLM protocol/models and disabled detail navigation');await ctx.close();
 }}finally{await browser.close();}
})().catch(e=>{console.error(e);process.exitCode=1;});
