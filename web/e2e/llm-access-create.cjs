const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const assert=require('node:assert/strict');
(async()=>{
 const browser=await chromium.launch();
 try{for(const locale of ['zh-CN','en-US'])for(const theme of ['dark','light']){
  const ctx=await browser.newContext({viewport:{width:1440,height:1000}}),zh=locale==='zh-CN';
  await ctx.addInitScript(({locale,theme})=>{localStorage.setItem('liaison-locale',locale);localStorage.setItem('liaison-theme-preference',theme)},{locale,theme});
  const app={id:41,name:'Model service',application_type:'openai',ip:'model.example',port:8000};
  const rows=[{id:64,name:'Incomplete access',application:app,access_protocol:'aiapi',status:'running',expose_public_port:false}];
  const writes=[],configs={},errors=[];let fail=true,deny=false,probeCount=0;
  await ctx.route('**/api/v1/**',async route=>{
   const req=route.request(),path=new URL(req.url()).pathname,method=req.method();let data={};
   if(method!=='GET')writes.push({path,method,body:req.postData()?req.postDataJSON():undefined});
   if(path==='/api/v1/applications')data={applications:[app]};
   else if(path==='/api/v1/proxies'){if(method==='GET')data={proxies:rows};else{data={id:65,name:'New model access',application:app,access_protocol:'aiapi',status:'running'};rows.push(data);}}
   else if(path==='/api/v1/ai/applications/41/probe'){probeCount++;data=probeCount===1?{state:'auth_required'}:{state:'compatible',models:['discovered-model']};}
   else if(path==='/api/v1/ai/applications/41'){
    if(deny){await route.fulfill({status:403,json:{code:403,message:'Forbidden'}});return;}
    data=method==='PUT'?{...req.postDataJSON(),api_key:undefined,has_api_key:true}:{protocol:'openai-compatible',base_path:'/v1',tls:false,has_api_key:true};
   }else if(path.endsWith('/workspace')){const id=path.split('/').at(-2),c=configs[id];data={upstream_protocol:c?'openai-compatible':'',external_protocol:c?'openai-compatible':'',models:c?Object.keys(c.models):[],can_manage:true,enabled:!!c};}
   else if(/^\/api\/v1\/ai\/accesses\/\d+$/.test(path)){
    const id=path.split('/').at(-1);
    if(method==='PUT'){if(fail){fail=false;await route.fulfill({status:503,json:{code:503,message:'fixture failure'}});return;}configs[id]=req.postDataJSON();}
    data=configs[id]||{enabled:false,models:{},external_protocol:'openai-compatible'};
   }
   await route.fulfill({json:{code:200,data}});
  });
  const page=await ctx.newPage();page.on('pageerror',e=>errors.push(e.message));
  const btn=(cn,en)=>page.getByRole('button',{name:zh?cn:en,exact:true});
  await page.goto(`${process.env.E2E_UI_URL}/e2e/access.html?entry=${encodeURIComponent('/proxy?category=llm')}`);
  await page.getByRole('link',{name:zh?'待配置':'Set up',exact:true}).first().waitFor();
  await btn('新建访问','Create access').click();
  await page.locator('#create-proxy .liaison-access-identity select').nth(1).selectOption('41');
  await page.locator('#create-proxy .liaison-initial-connection select').first().waitFor();
  assert.equal(await page.getByPlaceholder('••••••••').count(),1,'saved key must be masked');
  await btn('保存上游并获取模型列表','Save upstream & fetch models').click();
  await page.getByText(zh?'上游认证失败，请检查上游 API 密钥。':'Upstream authentication failed. Check the upstream API key.',{exact:true}).waitFor();
  await btn('保存上游并获取模型列表','Save upstream & fetch models').click();
  const discovered=page.getByRole('checkbox',{name:'discovered-model',exact:true});await discovered.waitFor();
  assert(!(await discovered.isChecked()),'discovered models must not be exposed automatically');
  await discovered.check();
  await page.locator('.liaison-llm-mappings summary').click();
  assert.equal(await page.getByLabel(zh?'上游模型 1':'Upstream model 1',{exact:true}).inputValue(),'discovered-model');
  await page.getByLabel(zh?'可调用模型 1':'Available model 1',{exact:true}).fill('public-model');
  await page.getByLabel(zh?'上游模型 1':'Upstream model 1',{exact:true}).fill('private-model');
  await page.screenshot({path:`/tmp/llm-create-${locale}-${theme}.png`});
  await btn('确定','Create').click();
  await page.getByText(zh?'访问已创建，但连接配置未保存。请修正后重试，不会重复创建访问。':'Access was created, but its connection was not saved. Correct the configuration and retry; no duplicate access will be created.',{exact:true}).waitFor();
  assert.equal(writes.filter(w=>w.path==='/api/v1/proxies').length,1);
  await btn('保存连接','Save connection').click();
  await page.getByText('public-model',{exact:true}).waitFor();
  await page.getByText(zh?'访问已创建':'Access created',{exact:true}).waitFor({state:'hidden',timeout:5000});
  assert.equal(writes.filter(w=>w.path==='/api/v1/proxies').length,1,'retry duplicated access');
  assert.deepEqual(configs[65].models,{'public-model':'private-model'});
  assert.equal(configs[65].external_protocol,'openai-compatible');assert.equal(configs[65].enabled,true);
  const first=page.getByRole('row').filter({hasText:'Incomplete access'});
  await first.getByRole('button',{name:zh?'编辑':'Edit',exact:true}).click();
  await page.getByLabel(zh?'可调用模型 1':'Available model 1',{exact:true}).fill('repaired');
  await page.getByLabel(zh?'上游模型 1':'Upstream model 1',{exact:true}).fill('private-model');
  await page.setViewportSize({width:390,height:844});
  await page.screenshot({path:`/tmp/llm-create-mobile-${locale}-${theme}.png`});
  assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));
  await btn('确定','Save').click();await page.getByText('repaired',{exact:true}).waitFor();
  assert.deepEqual(configs[64].models,{repaired:'private-model'});
  deny=true;await btn('新建访问','Create access').click();await page.locator('#create-proxy .liaison-access-identity select').nth(1).selectOption('41');
  await page.getByText(zh?'无法读取模型配置，请检查应用配置权限。':'Cannot load model configuration. Check application configuration permissions.',{exact:false}).waitFor();
  const count=writes.length;await btn('确定','Create').click();await page.waitForTimeout(100);assert.equal(writes.length,count,'denied configuration must not create access');
  assert.deepEqual(errors,[]);console.log('PASS LLM creation, partial retry, repair, permissions, layout',locale,theme);await ctx.close();
 }}finally{await browser.close();}
})().catch(e=>{console.error(e);process.exitCode=1;});
