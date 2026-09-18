const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const assert=require('node:assert/strict');
(async()=>{const browser=await chromium.launch();try{
for(const locale of ['zh-CN','en-US'])for(const theme of ['light','dark']){
 const zh=locale==='zh-CN',context=await browser.newContext({viewport:{width:1440,height:1000}});
 await context.addInitScript(({locale,theme})=>{localStorage.setItem('liaison-locale',locale);localStorage.setItem('liaison-theme-preference',theme);window.copyFails=false;Object.defineProperty(navigator,'clipboard',{value:{writeText:async()=>{if(window.copyFails)throw Error('clipboard denied')}}});},{locale,theme});
 await context.route('**/api/v1/**',async route=>{const path=new URL(route.request().url()).pathname;let data=[];
 if(path.endsWith('/workspace'))data={name:'Usage fixture',enabled:true,models:['alpha'],can_manage:true,external_protocol:'openai',external_protocols:['openai']};
 if(path.endsWith('/accesses/1'))data={models:{alpha:'alpha'},enabled:true};
 if(path.endsWith('/keys')&&route.request().method()==='POST')data={id:1,name:'Test',secret:'fixture-not-a-real-key',models:['alpha'],expires_at:'2099-01-01',used_tokens:0};
 if(path.endsWith('/usage'))data={summary:{requests:5,input_tokens:100,output_tokens:200},records:[0,1,2,4,5].map((minute,i)=>({created_at:`2026-09-16T12:0${minute}:00Z`,input_tokens:20,output_tokens:[10,80,30,60,20][i]}))};
 await route.fulfill({json:{code:200,data}});});
 const page=await context.newPage();const errors=[];page.on('pageerror',e=>errors.push(e.message));
 await page.goto(process.env.E2E_UI_URL+'/e2e/ollama.html?workspace');
 await page.getByRole('button',{name:zh?'统计':'Statistics',exact:true}).click();
 await page.getByRole('button',{name:zh?'1 小时':'1h',exact:true}).click();
 const svg=page.locator('.ai-token-trend svg');await svg.waitFor();
 assert.equal(await svg.locator('.ai-token-point').count(),5);assert.equal(await svg.locator('.ai-token-line').count(),2);
 assert((await svg.locator('.ai-token-line').first().getAttribute('d')).includes('C'));
 await svg.focus();await page.keyboard.press('Home');assert((await page.locator('.ai-token-trend-legend').innerText()).includes('12:00'));await page.keyboard.press('ArrowRight');assert((await page.locator('.ai-token-trend-legend').innerText()).includes('12:01'));
 for(const width of [1440,390]){await page.setViewportSize({width,height:1000});await page.waitForTimeout(200);await svg.evaluate(e=>e.blur());await page.screenshot({path:`/tmp/token-trend-${locale}-${theme}-${width}.png`,fullPage:true});assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));}
 await page.getByRole('button',{name:zh?'API 密钥':'API keys',exact:true}).click();await page.getByRole('button',{name:zh?'创建密钥':'Create key',exact:true}).click();
 const dialog=page.getByRole('dialog');await dialog.getByLabel(zh?'名称':'Name',{exact:true}).fill('Test');await dialog.getByRole('checkbox').check();await dialog.getByRole('button',{name:zh?'创建':'Create',exact:true}).click();
 const copy=dialog.getByRole('button',{name:zh?'复制密钥':'Copy key',exact:true}),feedback=dialog.getByText(zh?'密钥已复制':'Key copied',{exact:true});
 await copy.click();await feedback.waitFor();await page.waitForTimeout(1500);await copy.click();await page.waitForTimeout(1500);assert(await feedback.isVisible());await feedback.waitFor({state:'hidden',timeout:3000});
 await copy.click();await feedback.waitFor();await page.evaluate(()=>window.copyFails=true);await copy.click();const error=dialog.locator('.liaison-notice');await page.waitForTimeout(2700);assert((await error.innerText()).includes(zh?'操作失败':'Operation failed'));
 assert.deepEqual(errors,[]);console.log('PASS trend buckets/gaps/keyboard and copy reset/error persistence',locale,theme);await context.close();
}
}finally{await browser.close()}})().catch(e=>{console.error(e);process.exitCode=1});
