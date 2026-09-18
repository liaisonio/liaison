const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const assert=require('node:assert/strict');
(async()=>{const browser=await chromium.launch();try{
for(const source of ['scan','application'])for(const locale of ['zh-CN','en-US'])for(const theme of ['light','dark']){
 const context=await browser.newContext({viewport:{width:1440,height:1000}});
 await context.addInitScript(({locale,theme})=>{localStorage.setItem('liaison-locale',locale);localStorage.setItem('liaison-theme-preference',theme)},{locale,theme});
 const app={id:1,name:'App-127.0.0.1:5000',application_type:'http',ip:'127.0.0.1',port:5000};let submitted;
 await context.route('**/api/v1/**',async route=>{
  const path=new URL(route.request().url()).pathname;let data={};
  if(path.endsWith('/capabilities'))data={domain:theme==='dark'};
  else if(path.includes('scan'))data={id:1,task_status:'completed',applications:['127.0.0.1:5000:http']};
  else if(path.endsWith('/edges'))data={edges:[{id:1,name:'Test connector',status:1,online:1}]};
  else if(path.endsWith('/applications'))data=route.request().method()==='POST'?app:{applications:[app],total:1};
  else if(path.endsWith('/proxies')){if(route.request().method()==='POST'){submitted=route.request().postDataJSON();data={id:1,...submitted,application:app}}else data={proxies:[],total:0};}
  await route.fulfill({json:{code:200,data}});
 });
 const page=await context.newPage();const errors=[];page.on('pageerror',e=>errors.push(e.message));const zh=locale==='zh-CN';
 await page.goto(process.env.E2E_UI_URL+'/e2e/web-entry-sources.html?'+source);
 if(source==='scan'){
  await page.getByRole('button',{name:zh?'扫描应用':'Scan',exact:true}).click();
  await page.getByRole('button',{name:zh?'添加':'Add',exact:true}).click();
  await page.getByRole('button',{name:zh?'添加并创建访问':'Add and create access',exact:true}).click();
 }else await page.getByRole('button',{name:zh?'创建访问':'Create access',exact:true}).click();
 const form=page.locator(source==='scan'?'#scan-create-access':'#create-access');
 await form.waitFor();
 const mode=form.getByLabel(zh?'入口方式':'Entry mode',{exact:false});assert.equal(await mode.inputValue(),'path');
 assert.equal(await mode.locator('option[value="domain"]').count(),theme==='dark'?1:0);
 await mode.selectOption('port');assert.equal(await form.locator('input[type="number"]').count(),1);await mode.selectOption('path');assert.equal(await form.locator('input[type="number"]').count(),0);
 for(const width of [1440,390]){await page.setViewportSize({width,height:1000});await page.screenshot({path:`/tmp/web-entry-${source}-${locale}-${theme}-${width}.png`});assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));}
 await page.locator(`button[form="${source==='scan'?'scan-create-access':'create-access'}"]`).click();
 await page.waitForFunction(id=>!document.getElementById(id),source==='scan'?'scan-create-access':'create-access');
 assert.equal(submitted.http_entry_mode,'path');assert.equal(submitted.expose_public_port,false);assert.equal(submitted.port,undefined);assert.deepEqual(errors,[]);
 console.log('PASS',source,locale,theme);await context.close();
}
}finally{await browser.close()}})().catch(e=>{console.error(e);process.exitCode=1});
