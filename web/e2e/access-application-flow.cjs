const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const assert=require('node:assert/strict');
(async()=>{const browser=await chromium.launch();try{
 for(const locale of ['zh-CN','en-US'])for(const theme of ['light','dark']){
  const zh=locale==='zh-CN',ctx=await browser.newContext({viewport:{width:1440,height:1000}});
  await ctx.addInitScript(({locale,theme})=>{localStorage.setItem('liaison-locale',locale);localStorage.setItem('liaison-theme-preference',theme);},{locale,theme});
  const writes=[],errors=[];let failAccess=true,applicationType='ssh';
  await ctx.route('**/api/v1/**',async route=>{
   const req=route.request(),path=new URL(req.url()).pathname,method=req.method();let data={};
   if(method!=='GET')writes.push({path,data:req.postData()?req.postDataJSON():undefined});
   if(path==='/api/v1/edges')data={edges:[{id:1,name:'Connector-one',online:1,device:{name:'Mac-mini.local'}},{id:2,name:'Connector-two',online:2}]};
   if(path==='/api/v1/applications')data=method==='GET'?{applications:[{id:1,name:'Primary service',edge_id:1,ip:'127.0.0.1',port:22,application_type:applicationType},{id:2,name:'Other device service',edge_id:2,ip:'127.0.0.1',port:22,application_type:applicationType}]}:{id:10,...req.postDataJSON()};
   if(path==='/api/v1/applications/probe')data={status:'reachable',duration_ms:1};
   if(path==='/api/v1/proxies'){
    if(method==='GET')data={proxies:[]};
    else if(failAccess){failAccess=false;await route.fulfill({status:500,json:{code:500}});return;}
    else data={id:20,name:req.postDataJSON().name};
   }
   if(path==='/api/v1/ai/applications/1')data={protocol:'openai-compatible',base_path:'/v1',tls:false};
   if(path==='/api/v1/ai/applications/1/probe')data={state:'compatible',models:['qwen3']};
   await route.fulfill({json:{code:200,data}});
  });
  const page=await ctx.newPage();page.on('pageerror',e=>errors.push(e.message));
  const dialog=()=>page.getByRole('dialog');
  const select=(cn,en)=>dialog().getByRole('combobox',{name:zh?cn:en,exact:true});
  const button=(cn,en)=>dialog().getByRole('button',{name:zh?cn:en,exact:true});
  const open=async(type)=>{await page.goto(`${process.env.E2E_UI_URL}/e2e/product-polish.html?entry=${encodeURIComponent('/proxy?access_type='+type)}`);await page.getByRole('button',{name:zh?'新建访问':'Create access',exact:true}).click();};
  for(const [type,appType] of [['webssh','ssh'],['websftp','ssh'],['http','http'],['tcp','tcp'],['webmysql','mysql'],['webredis','redis'],['webrdp','rdp'],['webs3','s3'],['websmb','smb']]){
   applicationType=appType;await open(type);
   assert.equal(await select('应用来源','Application source').inputValue(),'existing');
   assert.equal(await select('连接器','Connector').count(),0);
   assert.equal(await select('应用','Application').locator('option[value="2"]').count(),1);
   await select('应用','Application').selectOption('2');
   await select('应用来源','Application source').selectOption('new');
   await select('连接器','Connector').locator('option[value="1"]').waitFor({state:'attached'});
   assert.match(await select('连接器','Connector').textContent(),/Mac-mini.local/);
   assert.match(await select('连接器','Connector').textContent(),zh?/设备未上报.*离线/:/Device not reported.*Offline/);
   await select('连接器','Connector').selectOption('1');
   await select('连接器','Connector').selectOption('2');assert.equal(await select('应用来源','Application source').inputValue(),'new');
   await select('连接器','Connector').selectOption('1');
   await dialog().getByLabel(zh?'应用地址':'Application address',{exact:false}).fill('127.0.0.1:8080');
   await button('测试连接','Test connection').click();await dialog().getByText(/TCP port reachable|端口可达/).waitFor();
   await button('取消','Cancel').click();
  }
  assert(writes.every(w=>w.path==='/api/v1/applications/probe'),'probe and cancel must not create resources');
  applicationType='http';await open('http');await select('应用来源','Application source').selectOption('new');await select('连接器','Connector').selectOption('1');
  await dialog().getByLabel(zh?'应用地址':'Application address',{exact:false}).fill('127.0.0.1:8080');
  for(const width of [1440,390]){
   await page.setViewportSize({width,height:width===390?844:1000});
   await page.screenshot({path:`/tmp/access-flow-${locale}-${theme}-${width}.png`});
   assert(await dialog().evaluate(e=>e.scrollWidth<=e.clientWidth+1));
  }
  await page.setViewportSize({width:1440,height:1000});
  await button('确定','Create').click();await dialog().getByText(/应用已创建，但访问未创建|The application was created, but access was not/).waitFor();
  assert(await select('连接器','Connector').isDisabled());await button('确定','Create').click();await dialog().waitFor({state:'hidden'});
  assert.equal(writes.filter(w=>w.path==='/api/v1/applications').length,1,'retry must reuse created application');
  assert(writes.filter(w=>w.path==='/api/v1/proxies').every(w=>w.data.http_entry_mode==='path'&&w.data.application_id===10));
  applicationType='openai';await open('openai');await select('应用','Application').selectOption('1');
  await dialog().locator('.liaison-llm-models code').getByText('qwen3',{exact:true}).waitFor();
  assert.equal(await dialog().getByRole('checkbox',{name:'qwen3',exact:true}).count(),0);
  assert.equal(await dialog().locator('input[type=password]:visible').count(),0);
  await page.screenshot({path:`/tmp/access-flow-llm-${locale}-${theme}.png`});
  await button('创建访问','Create access').click();await dialog().waitFor({state:'hidden'});
  assert.deepEqual(writes.find(w=>w.path==='/api/v1/ai/accesses/20').data.models,{qwen3:'qwen3'});
  assert(!writes.some(w=>w.path==='/api/v1/ai/applications/1'),'existing upstream must not be modified');
  assert.deepEqual(errors,[]);console.log('PASS',locale,theme,'9 protocols, connector isolation, no-write probe/cancel, retry, single LLM, responsive');await ctx.close();
 }
}finally{await browser.close();}})().catch(e=>{console.error(e);process.exitCode=1;});
