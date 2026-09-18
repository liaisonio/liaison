const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const assert=require('node:assert/strict');
(async()=>{const browser=await chromium.launch();try{
for(const locale of ['zh-CN','en-US'])for(const theme of ['dark','light']){
 const zh=locale==='zh-CN',ctx=await browser.newContext({viewport:{width:1440,height:1000}});
 await ctx.addInitScript(({locale,theme})=>{localStorage.setItem('liaison-locale',locale);localStorage.setItem('liaison-theme-preference',theme);},{locale,theme});
 const calls=[],errors=[];let fail=false;
 await ctx.route('**/api/v1/**',async r=>{
  const path=new URL(r.request().url()).pathname;
  if(path.endsWith('/test')&&fail){await r.fulfill({status:502,json:{reason:'UPSTREAM_ERROR',message:'Request failed'}});return;}
  if(path.endsWith('/test')){calls.push(r.request().postDataJSON());await r.fulfill({contentType:'text/event-stream',headers:{'x-request-id':'fixture-request-id'},body:'data: '+JSON.stringify({choices:[{delta:{content:zh?'### 连接已就绪\n\n可以通过此模型进行对话。\n\n`model: chat`':'### Connection ready\n\nYou can now chat with this model.\n\n`model: chat`'}}]})+'\n\ndata: [DONE]\n\n'});return;}
  await r.fulfill({json:{code:200,data:path.endsWith('/workspace')?{name:'Model access',enabled:true,models:['chat','reasoner'],can_manage:false,external_protocol:'openai-compatible',external_protocols:['openai-compatible']}:[]}});
 });
 const page=await ctx.newPage();page.on('pageerror',e=>errors.push(e.message));
 await page.goto(`${process.env.E2E_UI_URL}/e2e/ollama.html?workspace`);
 await page.locator('.ai-api-tabs').getByRole('button',{name:zh?'在线体验':'Playground',exact:true}).click();
 const input=page.getByLabel(zh?'消息':'Message',{exact:true});await input.fill(zh?'你好，介绍一下你自己':'Hello, introduce yourself');await input.press('Enter');
 await page.locator('.ai-playground-messages h3').waitFor();await page.waitForFunction(()=>document.querySelectorAll('.ai-api-answer').length===2&&document.querySelector('textarea').value==='');
 assert(await input.evaluate(el=>document.activeElement===el),'Enter send retains focus after reply');
 assert.equal(await page.locator('.ai-playground-composer .ai-api-answer').count(),0);
 await page.getByLabel(zh?'模型':'Model',{exact:true}).selectOption('reasoner');
 assert.equal(await page.locator('.ai-api-answer small').last().textContent(),'chat');
 await input.fill('second');await input.press('Shift+Enter');assert.equal(await input.inputValue(),'second\n');await input.press('Enter');
 await page.waitForFunction(()=>document.querySelectorAll('.ai-api-answer').length===4&&document.querySelector('textarea').value==='');
 assert.equal(calls[1].messages.length,3);assert(!calls[1].messages.some(m=>'model' in m));
 await page.screenshot({path:`/tmp/llm-playground-${locale}-${theme}.png`,fullPage:true});
 await page.setViewportSize({width:390,height:844});assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));await page.screenshot({path:`/tmp/llm-playground-mobile-${locale}-${theme}.png`,fullPage:true});
 await page.getByRole('button',{name:zh?'新对话':'New conversation',exact:true}).click();assert.equal(await page.locator('.ai-api-answer').count(),0);assert.deepEqual(errors,[]);
 fail=true;await input.fill('hi');await input.press('Enter');await page.locator('.ai-playground-notice').waitFor();
 assert.equal(await page.locator('.ai-api-workspace > .liaison-notice').count(),0);
 await page.screenshot({path:`/tmp/llm-playground-error-${locale}-${theme}.png`,fullPage:true});
 fail=false;await page.getByRole('button',{name:zh?'发送':'Send',exact:true}).click();await page.locator('.ai-playground-notice').waitFor({state:'hidden'});await page.waitForFunction(()=>document.querySelector('textarea').value==='');assert(await input.evaluate(el=>document.activeElement===el),'Click send restores focus');
 console.log('PASS conversation, model labels, keyboard, reset, responsive',locale,theme);await ctx.close();
}}finally{await browser.close();}})().catch(e=>{console.error(e);process.exitCode=1;});
