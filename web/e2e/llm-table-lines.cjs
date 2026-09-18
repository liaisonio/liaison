const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const assert=require('node:assert/strict');
(async()=>{const browser=await chromium.launch();try{
 for(const locale of ['zh-CN','en-US'])for(const theme of ['light','dark']){
  const c=await browser.newContext({viewport:{width:1440,height:900}});
  await c.addInitScript(({locale,theme})=>{localStorage.setItem('liaison-locale',locale);localStorage.setItem('liaison-theme-preference',theme);},{locale,theme});
  await c.route('**/api/v1/**',route=>{
   const path=new URL(route.request().url()).pathname;let data=[];
   if(path.endsWith('/workspace'))data={name:'Table fixture',enabled:true,models:['alpha'],can_manage:true,external_protocol:'openai'};
   if(path.endsWith('/accesses/1'))data={enabled:true,models:{alpha:'alpha'}};
   if(path.endsWith('/keys'))data=[{id:1,name:'Example',models:['alpha'],expires_at:'2099-01-01',used_tokens:10}];
   if(path.endsWith('/requests'))data=[1,2,3].map(i=>({request_id:'request-fixture-'+i,key_id:1,model:'alpha',status:200,complete:true,duration_ms:12,input_tokens:5,output_tokens:5}));
   return route.fulfill({json:{code:200,data}});
  });
  const page=await c.newPage();await page.goto(process.env.E2E_UI_URL+'/e2e/ollama.html?workspace');
  for(const tab of locale==='zh-CN'?['请求记录','API 密钥']:['Request records','API keys']){
   await page.getByRole('button',{name:tab,exact:true}).click();await page.locator('.ai-api-table td').first().waitFor();
   assert(await page.locator('.ai-api-table th,.ai-api-table td').evaluateAll(nodes=>nodes.every(n=>getComputedStyle(n).borderBottomColor.endsWith('0.55)'))));
   for(const width of [1440,390]){await page.setViewportSize({width,height:900});await page.screenshot({path:`/tmp/llm-lines-${locale}-${theme}-${tab==='请求记录'||tab==='Request records'?'requests':'keys'}-${width}.png`,fullPage:true});assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));}
  }
  console.log('PASS table lines',locale,theme);await c.close();
 }
}finally{await browser.close();}})().catch(e=>{console.error(e);process.exitCode=1});
