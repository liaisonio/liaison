const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const assert=require('node:assert/strict');
(async()=>{
 const browser=await chromium.launch();
 try {for(const locale of ['zh-CN','en-US'])for(const theme of ['light','dark']){
  const zh=locale==='zh-CN',context=await browser.newContext({viewport:{width:1440,height:1000}});
  await context.addInitScript(({locale,theme})=>{localStorage.setItem('liaison-locale',locale);localStorage.setItem('liaison-theme-preference',theme);},{locale,theme});
  let writes=[],state='compatible',apps=[];
  await context.route('**/api/v1/**',async route=>{
   const req=route.request(),path=new URL(req.url()).pathname;let data={};
   if(req.method()!=='GET')writes.push({path,data:req.postDataJSON()});
   if(path.endsWith('/edges'))data={edges:[{id:1,name:'Development connector'},{id:2,name:'Other connector'}]};
   if(path.endsWith('/applications'))data={applications:apps};
   if(path.endsWith('/proxies'))data={proxies:[]};
   if(path.endsWith('/probe'))data={state,models:state==='compatible'?['qwen3','model-with-a-long-identifier-for-layout-verification-12345678901234567890']:[]};
   if(path==='/api/v1/ai/setup')data={id:9,application_id:10};
   if(path==='/api/v1/ai/applications/10')data={application_type:'openai',protocol:'openai-compatible',base_path:'/v1',tls:false};
   await route.fulfill({json:{code:200,data}});
  });
  const page=await context.newPage(),errors=[];page.on('pageerror',e=>errors.push(e.message));
  const visit=()=>page.goto(`${process.env.E2E_UI_URL}/e2e/product-polish.html?llm`);
  const open=async()=>{await page.locator('.liaison-llm-empty .liaison-button').click();await page.getByRole('dialog').waitFor();assert.equal(await page.getByRole('combobox',{name:zh?'连接器':'Connector',exact:true}).count(),0);};
  const application=()=>page.getByRole('dialog').getByLabel(zh?'应用':'Application',{exact:true});
  const source=()=>page.getByRole('dialog').getByLabel(zh?'应用来源':'Application source',{exact:true});
  const newMode=()=>({click:async()=>{await source().selectOption('new');await page.getByLabel(zh?'连接器':'Connector',{exact:true}).selectOption('1');}});
  const existingMode=()=>({click:()=>source().selectOption('existing')});
  const cancel=()=>page.getByRole('button',{name:zh?'取消':'Cancel',exact:true}).click();
  const fill=async()=>{await newMode().click();await page.getByLabel(zh?'应用地址':'Application address',{exact:false}).fill('http://127.0.0.1:8000/v1');};
  const probe=()=>page.getByRole('button',{name:zh?'检测连接并获取模型':'Check connection & fetch models'}).click();
  await visit();await open();
  assert(await page.getByRole('dialog').getByLabel(zh?'访问协议':'Protocol',{exact:true}).isVisible());
  assert(await page.getByRole('dialog').getByLabel(zh?'访问名称':'Access name',{exact:true}).isVisible());
  assert(await page.getByRole('dialog').getByRole('button',{name:zh?'创建访问':'Create access',exact:true}).isDisabled());
  await page.screenshot({path:`/tmp/llm-fields-${locale}-${theme}.png`});
  await fill();
  assert(!(await page.locator('input[type=password]').isVisible()));
  await existingMode().click();await newMode().click();assert.equal(await page.locator('input[type=url]').inputValue(),'http://127.0.0.1:8000/v1');
  await probe();await page.getByText('qwen3',{exact:true}).waitFor();await page.getByRole('checkbox',{name:'qwen3',exact:true}).check();
  assert(writes.every(v=>v.path==='/api/v1/ai/setup/probe'));
  for(const width of [1440,390]){
   await page.setViewportSize({width,height:width===390?844:1000});
   await page.screenshot({path:`/tmp/llm-create-${locale}-${theme}-${width}.png`});
   assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
   assert(await page.locator('.liaison-modal-body').evaluate(el=>el.scrollWidth<=el.clientWidth+1));
  }
  await page.setViewportSize({width:1440,height:1000});
  await page.locator('input[type=url]').fill('http://127.0.0.1:8001/v1');await page.getByText('qwen3',{exact:true}).waitFor({state:'hidden'});
  state='auth_required';await probe();await page.getByText(zh?'认证失败，请检查上游 API 密钥。':'Authentication failed. Check the upstream API key.').waitFor();
  await cancel();assert(writes.every(v=>v.path==='/api/v1/ai/setup/probe'));
  state='compatible';await open();await newMode().click();assert.equal(await page.locator('input[type=url]').inputValue(),'');await fill();await probe();await page.getByRole('checkbox',{name:'qwen3',exact:true}).check();
  await page.getByRole('button',{name:zh?'创建访问':'Create access',exact:true}).click();await page.getByRole('dialog').waitFor({state:'hidden'});
  const saved=writes.filter(v=>v.path==='/api/v1/ai/setup');assert.equal(saved.length,1);assert.deepEqual(saved[0].data.models,{qwen3:'qwen3'});assert.match(saved[0].data.name,/^Access-[a-f0-9]{8}$/);assert.equal(saved[0].data.edge_id,1);
  apps=[{id:10,name:'Existing model',edge_id:1,ip:'127.0.0.1',port:8000,application_type:'openai'}];await visit();await open();assert(await page.getByRole('option',{name:'Existing model · 127.0.0.1:8000'}).count());
  await newMode().click();await fill();await page.getByRole('button',{name:zh?'使用已有应用':'Use existing application'}).click();assert.equal(await application().inputValue(),'10');
  await page.getByRole('checkbox',{name:'qwen3',exact:true}).waitFor();assert(await page.getByRole('checkbox',{name:'qwen3',exact:true}).isChecked());
  assert(!writes.some(v=>v.path==='/api/v1/ai/applications/10'));
  await newMode().click();await page.getByLabel(zh?'连接器':'Connector',{exact:true}).selectOption('2');await existingMode().click();assert.equal(await application().inputValue(),'');assert.equal(await application().locator('option[value="10"]').count(),1);
  await cancel();assert.deepEqual(errors,[]);console.log('PASS',locale,theme,'discovery, cancellation, draft retention, atomic submission, reuse, responsive');await context.close();
 }}finally{await browser.close();}
})().catch(e=>{console.error(e);process.exitCode=1;});
