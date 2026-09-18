const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const assert=require('node:assert/strict');
// Fixture responses test UI behavior; protocol transport is exercised by Go tests.
(async()=>{const browser=await chromium.launch();try{
 for(const locale of ['zh-CN','en-US'])for(const theme of ['dark','light']){
  const context=await browser.newContext({viewport:{width:1440,height:1000}});
  await context.addInitScript(({locale,theme})=>{localStorage.setItem('liaison-locale',locale);localStorage.setItem('liaison-theme-preference',theme)},{locale,theme});
  let saved=0;
  await context.route('**/api/v1/ai/applications/**',async route=>{
   const path=new URL(route.request().url()).pathname,protocol=path.split('/')[5];
   if(path.endsWith('/probe'))return route.fulfill({json:{code:200,data:{state:['ark','qwen'].includes(protocol)?'unsupported':'compatible',models:['public-model']}}});
   if(route.request().method()==='PUT')saved++;
   return route.fulfill({json:{code:200,data:{protocol,application_type:protocol,base_path:({ark:'/api/v3',qwen:'/api/v1',gemini:'/v1beta',ollama:'/api'})[protocol]||'/v1',tls:true}}});
  });
  const page=await context.newPage(),errors=[];page.on('pageerror',e=>errors.push(e.message));await page.goto(process.env.E2E_UI_URL+'/e2e/native-llm.html');
  const select=page.getByLabel('Application type'),upstream=page.locator('fieldset select').first();
  for(const protocol of ['openai','anthropic','ark','qwen','gemini','ollama','openai-compatible']){
   await select.selectOption(protocol);await page.waitForFunction(p=>Array.from(document.querySelectorAll('select')).some(s=>s.disabled&&s.value===(p==='openai-compatible'?'openai':p)),protocol);
   assert(await upstream.isDisabled());
   if(['openai','openai-compatible'].includes(protocol)){
    const capability=page.getByLabel(locale==='zh-CN'?/^API 能力/:/^API capabilities/);
    await capability.selectOption('openai-compatible');assert.equal(await capability.inputValue(),'openai-compatible');
    await capability.selectOption('openai');assert.equal(await capability.inputValue(),'openai');
   }
   const code=page.locator('pre');const body=await code.innerText();assert(body.includes('$LIAISON_API_KEY'));assert(body.includes('\\\n'));
   if(protocol==='gemini')assert(body.includes(':streamGenerateContent?alt=sse')&&body.includes('x-goog-api-key'));
   if(protocol==='qwen')assert(body.includes('X-DashScope-SSE: enable')&&body.includes('generation'));
   if(protocol==='ark')assert(body.includes('/api/v3/responses'));
   if(protocol==='openai')assert(body.includes('/v1/responses'));
   await page.getByRole('button',{name:locale==='zh-CN'?'保存上游并获取模型列表':'Save upstream & fetch models',exact:true}).click();
   await page.waitForFunction(()=>!document.querySelector('[aria-busy="true"]'));
   for(const width of [1440,390]){await page.setViewportSize({width,height:1000});await page.screenshot({path:`/tmp/native-llm-${protocol}-${locale}-${theme}-${width}.png`});assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));}
   await page.setViewportSize({width:1440,height:1000});
  }
  assert.equal(saved,7);assert.deepEqual(errors,[]);console.log('PASS native LLM UI',locale,theme);await context.close();
 }
}finally{await browser.close()}})().catch(e=>{console.error(e);process.exitCode=1});
