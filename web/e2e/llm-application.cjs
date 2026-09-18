const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const assert=require('node:assert/strict');
(async()=>{
 const browser=await chromium.launch();
 try{for(const locale of ['zh-CN','en-US'])for(const theme of ['dark','light']){
  const ctx=await browser.newContext({viewport:{width:1440,height:1000}}),zh=locale==='zh-CN';
  await ctx.addInitScript(({locale,theme})=>{localStorage.setItem('liaison-locale',locale);localStorage.setItem('liaison-theme-preference',theme)},{locale,theme});
  let creates=0,probeCount=0;const writes=[],errors=[];
  await ctx.route('**/api/v1/**',async route=>{
   const r=route.request(),path=new URL(r.url()).pathname;let data={};
   if(r.method()!=='GET')writes.push(path);
   if(path==='/api/v1/applications')data=r.method()==='POST'?(creates++,{id:71,name:'New LLM',application_type:'openai'}):{applications:[]};
   else if(path==='/api/v1/edges')data={edges:[{id:1,name:'Fixture connector'}]};
   else if(path==='/api/v1/proxies')data={proxies:[]};
   else if(path==='/api/v1/ai/applications/71')data=r.method()==='PUT'?r.postDataJSON():{protocol:'openai-compatible',base_path:'/v1',tls:false};
   else if(path.endsWith('/probe')){probeCount++;data={state:'compatible',models:probeCount===1?[]:['fixture-vllm-model']};}
   await route.fulfill({json:{code:200,data}});
  });
  const page=await ctx.newPage();page.on('pageerror',e=>errors.push(e.message));
  const btn=(cn,en)=>page.getByRole('button',{name:zh?cn:en,exact:true});
  await page.goto(`${process.env.E2E_UI_URL}/e2e/llm-application.html`);await btn('新建应用','Create application').click();
  await page.locator('#create-application select').nth(0).selectOption('openai');
  await page.locator('#create-application select').nth(1).selectOption('1');
  await page.locator('#create-application input[list]').fill('127.0.0.1');
  await page.locator('#create-application input[type=number]').fill('8000');
  await btn('确定','Create').click();
  await btn('保存上游并获取模型列表','Save upstream & fetch models').click();
  await page.getByText(zh?'上游返回空列表。可检查服务后重试，或在访问中手动填写模型。':'The upstream returned no models. Check the service and retry, or enter models manually when configuring access.',{exact:true}).waitFor();
  await btn('保存上游并获取模型列表','Save upstream & fetch models').click();
  await page.getByText('fixture-vllm-model',{exact:true}).waitFor();
  assert.equal(await page.getByRole('checkbox',{name:'fixture-vllm-model'}).count(),0,'application discovery must not grant access');
  await page.screenshot({path:`/tmp/llm-application-${locale}-${theme}.png`});
  await page.setViewportSize({width:390,height:844});await page.screenshot({path:`/tmp/llm-application-mobile-${locale}-${theme}.png`});
  assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));
  await btn('完成','Done').click();await page.getByText(zh?'应用和上游配置已保存':'Application and upstream settings saved',{exact:true}).waitFor();
  assert.equal(creates,1);assert(!writes.includes('/api/v1/proxies'));assert.deepEqual(errors,[]);
  console.log('PASS application creation → upstream discovery, empty retry, no automatic access',locale,theme);await ctx.close();
 }}finally{await browser.close();}
})().catch(e=>{console.error(e);process.exitCode=1});
