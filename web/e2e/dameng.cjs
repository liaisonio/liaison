// Fixture UI acceptance only; does not claim a real DM8 connection.
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const assert=require('node:assert/strict');
(async()=>{
 const browser=await chromium.launch();
 try {
  for(const enabled of [false,true])for(const locale of ['zh-CN','en-US'])for(const theme of ['light','dark'])for(const width of [1440,390]){
   const context=await browser.newContext({viewport:{width,height:1000}});
   await context.addInitScript(({locale,theme})=>{localStorage.setItem('liaison-locale',locale);localStorage.setItem('liaison-theme-preference',theme);},{locale,theme});
   await context.route('**/api/v1/**',async route=>{
    const path=new URL(route.request().url()).pathname;let data={};
    if(path.endsWith('/webdata/capabilities'))data={dameng:enabled};
    else if(path.includes('applications'))data={applications:[{id:1,name:'DM8 example',application_type:'dameng',ip:'db.example',port:5236}]};
    else if(path.includes('proxies'))data={proxies:[]};
    await route.fulfill({json:{code:200,message:'success',data}});
   });
   const page=await context.newPage(),errors=[];page.on('pageerror',e=>errors.push(e.message));
   await page.goto(`${process.env.E2E_UI_URL||'http://127.0.0.1:5175'}/e2e/dameng.html`);
   await page.getByRole('button',{name:locale==='zh-CN'?'新建访问':'Create access',exact:true}).click();
   const dialog=page.getByRole('dialog');await dialog.waitFor();
   await page.waitForTimeout(150);
   const options=await page.evaluate(()=>window.damengChecks.availableApplicationTypes().map(x=>x.value));
   assert.equal(options.includes('dameng'),enabled);
   if(enabled){
    await dialog.getByLabel('Schema',{exact:true}).waitFor();
    assert.equal(await dialog.getByLabel(locale==='zh-CN'?'默认数据库':'Default database',{exact:true}).count(),0);
    await dialog.getByLabel(locale==='zh-CN'?'保存密码':'Save password',{exact:true}).uncheck();
    assert.equal(await dialog.locator('input[type=password]').count(),0);
    await dialog.getByLabel(locale==='zh-CN'?'保存密码':'Save password',{exact:true}).check();
    await dialog.getByLabel('Schema',{exact:true}).fill('Mixed_Case_Schema');
    await dialog.getByLabel('Schema',{exact:true}).focus();
    const checks=await page.evaluate(()=>{const c=window.damengChecks;return [c.isSQLProtocol('dameng'),c.sqlQualifiedName('dameng',{name:'A"B',schema:'Test'}),c.sqlQuoteIdent('dameng','A"B')];});
    assert.deepEqual(checks,[true,'"Test"."A""B"','"A""B"']);
    await page.screenshot({path:`/tmp/dameng-${locale}-${theme}-${width}.png`,fullPage:true});
   }else{assert.equal(await dialog.locator('option[value=webdameng]').count(),0);}
   assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));
   assert.deepEqual(errors,[]);await context.close();
  }
  console.log('Dameng UI: 16 capability/locale/theme/viewport cases passed.');
 } finally {await browser.close();}
})().catch(err=>{console.error(err);process.exit(1)});
