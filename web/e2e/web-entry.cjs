const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const assert=require('node:assert/strict');
(async()=>{const browser=await chromium.launch();try{
 for(const locale of ['zh-CN','en-US'])for(const theme of ['dark','light']){
  const context=await browser.newContext({viewport:{width:1440,height:1000}});
  await context.addInitScript(({locale,theme})=>{localStorage.setItem('liaison-locale',locale);localStorage.setItem('liaison-theme-preference',theme)},{locale,theme});
  const app={id:1,name:'Private website',application_type:'http',ip:'service.example',port:8080};
  let created,domain=false,updated;
  await context.route('**/api/v1/**',async route=>{
   const path=new URL(route.request().url()).pathname;
   if(path.endsWith('/capabilities'))return route.fulfill({json:{code:200,data:{domain}}});
   if(path.includes('/applications'))return route.fulfill({json:{code:200,data:{applications:[app],total:1}}});
   if(route.request().method()==='POST'&&path.endsWith('/proxies')){created=route.request().postDataJSON();return route.fulfill({json:{code:200,data:{id:2,...created,application:app}}});}
   if(route.request().method()==='PUT'&&path.endsWith('/proxies/1')){updated=route.request().postDataJSON();return route.fulfill({json:{code:200}});}
   if(path.includes('/proxies'))return route.fulfill({json:{code:200,data:{proxies:[{id:1,name:'Legacy website',application:app,access_protocol:'http',port:9443,status:'running',expose_public_port:true}],total:1}}});
   return route.fulfill({json:{code:200,data:{}}});
  });
  const page=await context.newPage();const errors=[];page.on('pageerror',e=>errors.push(e.message));
  await page.goto(process.env.E2E_UI_URL+'/e2e/web-entry.html');
  await page.getByRole('button',{name:locale==='zh-CN'?'新建访问':'Create access',exact:true}).click();
  const mode=page.getByLabel(locale==='zh-CN'?'入口方式':'Entry mode',{exact:false});
  assert.equal(await mode.inputValue(),'path');assert.equal(await mode.locator('option[value="domain"]').count(),0);
  await page.locator('#create-proxy select').nth(1).selectOption('1');
  assert.equal(await page.locator('input[type="number"]').count(),0);
  for(const width of [1440,390]){await page.setViewportSize({width,height:1000});await page.screenshot({path:`/tmp/web-entry-${locale}-${theme}-${width}.png`});assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));}
  await mode.selectOption('port');assert.equal(await page.locator('input[type="number"]').count(),1);
  await mode.selectOption('path');
  await page.getByRole('button',{name:locale==='zh-CN'?'确定':'Create',exact:true}).click();
  await page.waitForFunction(()=>!document.querySelector('[role="dialog"]'));
  assert.equal(created.http_entry_mode,'path');assert.equal(created.expose_public_port,false);assert.equal(created.port,undefined);
  domain=true;await page.reload();await page.getByRole('button',{name:locale==='zh-CN'?'新建访问':'Create access',exact:true}).click();
  assert.equal(await mode.locator('option[value="domain"]').count(),1);await mode.selectOption('domain');assert.equal(await page.locator('input[type="number"]').count(),0);
  await page.getByRole('button',{name:locale==='zh-CN'?'取消':'Cancel',exact:true}).click();
  await page.getByRole('button',{name:locale==='zh-CN'?'编辑':'Edit',exact:true}).click();
  assert.equal(await mode.inputValue(),'port');assert.equal(await page.locator('input[type="number"]').inputValue(),'9443');
  await mode.selectOption('path');await page.getByRole('button',{name:locale==='zh-CN'?'确定':'Save',exact:true}).click();
  await page.waitForFunction(()=>!document.querySelector('[role="dialog"]'));
  assert.equal(updated.http_entry_mode,'path');assert.equal(updated.expose_public_port,false);
  assert.deepEqual(errors,[]);console.log('PASS Web entry UI',locale,theme);await context.close();
 }
}finally{await browser.close()}})().catch(e=>{console.error(e);process.exitCode=1});
