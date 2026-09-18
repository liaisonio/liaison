const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const assert=require('node:assert/strict');
(async()=>{const browser=await chromium.launch();try{for(const locale of ['zh-CN','en-US'])for(const theme of ['dark','light']){
 const c=await browser.newContext({viewport:{width:1440,height:900}});
 await c.addInitScript(({locale,theme})=>{localStorage.setItem('liaison-locale',locale);localStorage.setItem('liaison-theme-preference',theme)},{locale,theme});
 const protocols=['openai','anthropic','ark','qwen','gemini','ollama','openai-compatible'];
 const apps=protocols.map((p,i)=>({id:i+1,name:p+' app',application_type:p,ip:'127.0.0.1',port:8080}));apps.push({id:8,name:'Legacy app',application_type:'llm',ip:'127.0.0.1',port:8080});
 const proxies=apps.map(a=>({id:a.id,name:a.name+' access',application:a,access_protocol:'aiapi',status:'running'}));
 await c.route('**/api/v1/**',r=>{const path=new URL(r.request().url()).pathname;
  if(path.endsWith('/workspace')){const id=Number(path.split('/')[5]);return r.fulfill({json:{code:200,data:{upstream_protocol:id===8?'qwen':protocols[id-1],external_protocol:id===8?'qwen':protocols[id-1],models:['public-model'],enabled:true}}});}
  if(path==='/api/v1/applications')return r.fulfill({json:{code:200,data:{applications:apps}}});
  if(path==='/api/v1/proxies')return r.fulfill({json:{code:200,data:{proxies}}});
  return r.fulfill({json:{code:200,data:{}}});
 });
 const page=await c.newPage();const errors=[];page.on('pageerror',e=>errors.push(e.message));await page.goto(process.env.E2E_UI_URL+'/e2e/llm-access-types.html');
 await page.getByText('Legacy app access',{exact:true}).waitFor();
 assert.equal(await page.getByRole('button',{name:'Anthropic',exact:true}).count(),1);
 assert.equal(await page.getByText('Anthropic Messages',{exact:true}).count(),0);
 assert.equal(await page.locator('[title="Anthropic Messages"]').count(),0);
 for(const width of [1440,390]){await page.setViewportSize({width,height:900});await page.screenshot({path:`/tmp/llm-access-types-${locale}-${theme}-${width}.png`});assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));}
 await page.setViewportSize({width:1440,height:900});
 assert.equal(await page.getByRole('button',{name:'OpenAI',exact:true}).count(),1);
 await page.getByRole('button',{name:'OpenAI',exact:true}).click();
 await page.getByText('openai app access',{exact:true}).waitFor();await page.getByText('openai-compatible app access',{exact:true}).waitFor();
 await page.getByRole('button',{name:'QWen',exact:true}).click();
 await page.getByText('qwen app access',{exact:true}).waitFor();await page.getByText('Legacy app access',{exact:true}).waitFor();assert.equal(await page.getByText('openai app access',{exact:true}).count(),0);
 await page.getByRole('button',{name:locale==='zh-CN'?'新建访问':'Create access',exact:true}).click();
 const dialog=page.getByRole('dialog');await dialog.waitFor();assert.equal(await dialog.locator('select').first().inputValue(),'qwen');
 const options=await dialog.locator('select').nth(1).locator('option').allTextContents();assert(options.some(t=>t.includes('qwen app')));assert(!options.some(t=>t.includes('openai app')));
 await dialog.getByRole('button',{name:locale==='zh-CN'?'取消':'Cancel',exact:true}).click();assert.deepEqual(errors,[]);console.log('PASS vendor access tabs, legacy resolution, matching applications',locale,theme);await c.close();
}}finally{await browser.close()}})().catch(e=>{console.error(e);process.exitCode=1});
