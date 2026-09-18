const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const assert=require('node:assert/strict');
(async()=>{const browser=await chromium.launch();try{
for(const locale of ['zh-CN','en-US'])for(const theme of ['light','dark']){
const zh=locale==='zh-CN',context=await browser.newContext({viewport:{width:1440,height:900}});
await context.addInitScript(({locale,theme})=>{localStorage.setItem('liaison-locale',locale);localStorage.setItem('liaison-theme-preference',theme);},{locale,theme});
const calls=[],reads=[],windows=[];let fail=true,slow=false;
await context.route('**/api/v1/**',async route=>{
const path=new URL(route.request().url()).pathname;reads.push(path);
if(path.endsWith('/test')){const body=route.request().postDataJSON();calls.push(body);assert(['alpha','beta','gamma'].includes(body.model));if(slow)await new Promise(r=>setTimeout(r,700));if(body.model==='beta'&&fail)return route.fulfill({status:502,json:{error:{code:'UPSTREAM_ERROR'}}});return route.fulfill({contentType:'text/event-stream',headers:{'x-request-id':'request-'+body.model},body:'data: '+JSON.stringify({choices:[{delta:{content:'### Result\n\nSafe **Markdown** from '+body.model}}]})+'\n\ndata: [DONE]\n\n'});}
let data=[];
if(path.endsWith('/workspace'))data={name:'Comparison fixture',enabled:true,models:['alpha','beta','gamma'],can_manage:false,external_protocol:'openai',external_protocols:['openai']};
if(path.endsWith('/requests'))data=[{request_id:'fixture-very-long-request-id-0123456789',model:'alpha',key_id:0,status:200,duration_ms:250,complete:true,input_tokens:10,output_tokens:20},{request_id:'failed-id',model:'beta',key_id:0,status:502,duration_ms:100,complete:false}];
if(path.endsWith('/requests'))data.push(...Array.from({length:20},(_,i)=>({request_id:'page-record-'+i,model:'alpha',status:i%2?502:200,duration_ms:100,complete:i%2===0})));
if(path.endsWith('/usage'))windows.push(new URL(route.request().url()).searchParams.get('hours'));
if(path.endsWith('/usage'))data={summary:{requests:2,unknown_requests:1,input_tokens:10,output_tokens:20},records:[{created_at:'2026-09-13T00:00:00Z',input_tokens:2,output_tokens:8},{created_at:'2026-09-14T00:00:00Z',input_tokens:15,output_tokens:30},{created_at:'2026-09-15T00:00:00Z',input_tokens:10,output_tokens:20}]};
await route.fulfill({json:{code:200,data}});
});
const page=await context.newPage(),errors=[];page.on('pageerror',e=>errors.push(e.message));
await page.goto(`${process.env.E2E_UI_URL}/e2e/ollama.html?workspace`);
await page.locator('.ai-api-example').waitFor();assert.equal(await page.locator('.ai-insights').count(),0);assert(!reads.some(p=>p.endsWith('/usage')),'Overview does not load statistics');
const example=await page.locator('.ai-api-example').boundingBox();assert(example.y+example.height<900,'Example visible above the fold');
await page.locator('.ai-api-tabs').getByRole('button',{name:zh?'统计':'Statistics',exact:true}).click();await page.locator('.ai-token-line').waitFor();assert.equal(await page.locator('.ai-api-example').count(),0);assert((await page.locator('.ai-insights-metrics').innerText()).includes('50.0%'));
await page.getByRole('group',{name:zh?'用量时间范围':'Usage time range'}).getByRole('button',{name:zh?'6 小时':'6h',exact:true}).click();await page.waitForFunction(()=>document.querySelector('.ai-insights-metrics')?.textContent.includes('6'));assert(windows.includes('6'));await page.getByRole('group',{name:zh?'用量时间范围':'Usage time range'}).getByRole('button',{name:zh?'30 天':'30 days',exact:true}).click();await page.locator('.ai-token-line').waitFor();
for(const width of [1440,390]){await page.setViewportSize({width,height:900});await page.screenshot({path:`/tmp/insights-${locale}-${theme}-${width}.png`});assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));}
await page.locator('.ai-api-tabs').getByRole('button',{name:zh?'请求记录':'Request records',exact:true}).click();assert.equal(await page.locator('.ai-api-table tbody tr').count(),10);await page.getByRole('button',{name:'Page 3',exact:true}).click();assert.equal(await page.locator('.ai-api-table tbody tr').count(),2);await page.getByRole('button',{name:'Next page',exact:true}).isDisabled().then(assert);await page.getByRole('button',{name:'Page 1',exact:true}).click();await page.getByRole('button',{name:'fixture-very'}).click();await page.locator('.liaison-drawer').waitFor();assert((await page.locator('.liaison-drawer').innerText()).includes('fixture-very-long-request-id-0123456789'));await page.keyboard.press('Escape');
await page.locator('.ai-api-tabs').getByRole('button',{name:zh?'在线体验':'Playground',exact:true}).click();await page.getByRole('button',{name:zh?'模型对比':'Compare models',exact:true}).click();
const input=page.getByLabel(zh?'对比问题':'Comparison prompt');await input.fill('Explain this');await page.getByRole('button',{name:zh?'发送对比':'Send comparison',exact:true}).click();await page.getByText(zh?'调用失败，可重新发送':'Request failed; send again to retry',{exact:true}).waitFor();await page.getByRole('button',{name:zh?'发送对比':'Send comparison',exact:true}).waitFor();assert.equal(calls.length,2);assert.equal(calls[0].messages[0].content,calls[1].messages[0].content);
fail=false;await input.press('Enter');await page.waitForFunction(()=>[...document.querySelectorAll('.ai-compare-results [role=status]')].every(e=>['Complete','已完成'].includes(e.textContent)));
for(const width of [1440,390]){await page.setViewportSize({width,height:900});await page.screenshot({path:`/tmp/compare-${locale}-${theme}-${width}.png`,fullPage:true});assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));}
slow=true;await input.press('Enter');await page.getByRole('button',{name:zh?'停止全部':'Stop all',exact:true}).click();await page.getByRole('button',{name:zh?'发送对比':'Send comparison',exact:true}).waitFor();
assert(!reads.includes('/api/v1/ai/accesses/1'),'consumer must not fetch upstream configuration');assert.deepEqual(errors,[]);console.log('PASS insights, details, comparison, failure, retry, stop, scope',locale,theme);await context.close();
}}finally{await browser.close();}})().catch(e=>{console.error(e);process.exitCode=1;});
