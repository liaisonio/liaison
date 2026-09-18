const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const assert=require('node:assert/strict');
(async()=>{
 const browser=await chromium.launch();
 try{for(const locale of ['zh-CN','en-US'])for(const theme of ['light','dark']){
  const context=await browser.newContext({viewport:{width:1440,height:900}});
  await context.addInitScript(({locale,theme})=>{localStorage.setItem('liaison-locale',locale);localStorage.setItem('liaison-theme-preference',theme);},{locale,theme});
  let rows=[{id:1,name:'Offline fixture',online:2,os:'Linux',cpu:4,memory:8192,interfaces:[]},{id:2,name:'Online fixture',online:1,os:'Linux',cpu:4,memory:8192,interfaces:[]}],deletes=[],fail=false,reconnected=false;
  await context.route('**/api/v1/devices**',route=>{
   const request=route.request(),id=Number(new URL(request.url()).pathname.split('/').pop());
   if(request.method()==='DELETE'){
    deletes.push(id);if(fail)return route.fulfill({json:{code:500,message:'Failure'}});
    rows=rows.filter(row=>row.id!==id);return route.fulfill({json:{code:200}});
   }
   return route.fulfill({json:{code:200,data:id?{...rows.find(row=>row.id===id),...(reconnected?{online:1}:{})}:{devices:rows}}});
  });
  const page=await context.newPage(),errors=[];page.on('pageerror',e=>errors.push(e.message));
  await page.goto(`${process.env.E2E_UI_URL}/e2e/device-language.html`);
  const globe=page.locator('.liaison-header-language-button'),menu=page.locator('.liaison-language-menu');
  assert.equal((await globe.innerText()).trim(),'');assert.equal(await globe.locator('svg').count(),1);
  await globe.hover();await menu.waitFor();await page.keyboard.press('Escape');await menu.waitFor({state:'hidden'});
  await globe.focus();await page.keyboard.press('Enter');await menu.waitFor();
  await menu.getByRole('button',{name:locale==='zh-CN'?'English':'简体中文',exact:true}).click();
  await globe.click();await menu.getByRole('button',{name:locale==='zh-CN'?'简体中文':'English',exact:true}).click();
  const label=locale==='zh-CN'?'删除':'Delete',cancel=locale==='zh-CN'?'取消':'Cancel';
  const offline=page.getByRole('row').filter({hasText:'Offline fixture'}),online=page.getByRole('row').filter({hasText:'Online fixture'});
  await offline.waitFor();assert(await online.getByRole('button',{name:label,exact:true}).isDisabled());
  const statusFilter=page.locator('.liaison-compound').filter({hasText:locale==='zh-CN'?'在线状态':'Online'}).locator('select');
  await statusFilter.selectOption('2');await online.waitFor({state:'hidden'});await offline.waitFor();
  await statusFilter.selectOption('');await online.waitFor();
  for(const width of [1440,390]){
   await page.setViewportSize({width,height:900});await offline.waitFor();
   const action=offline.locator('td.is-fixed-right'),before=await action.boundingBox();
   await page.locator('.liaison-table-scroll').hover();await page.mouse.wheel(500,0);
   await page.waitForTimeout(200);
   const after=await action.boundingBox();assert(Math.abs(before.x-after.x)<2,'Actions stay fixed while scrolling');
   assert(after.x>=0&&after.x+after.width<=width,'Actions remain in viewport');
   await page.screenshot({path:`/tmp/device-list-${locale}-${theme}-${width}.png`});
   await globe.hover();await menu.waitFor();await page.screenshot({path:`/tmp/device-globe-${locale}-${theme}-${width}.png`});await page.keyboard.press('Escape');
  }
  await page.setViewportSize({width:1440,height:900});
  const dialog=page.getByRole('dialog');
  await offline.getByRole('button',{name:label,exact:true}).click();await dialog.waitFor();
  for(const width of [1440,390]){
   await page.setViewportSize({width,height:900});await page.screenshot({path:`/tmp/device-language-${locale}-${theme}-${width}.png`});
   assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),'No page overflow');
  }
  await dialog.getByRole('button',{name:cancel,exact:true}).click();assert.deepEqual(deletes,[]);
  await page.setViewportSize({width:1440,height:900});
  await offline.getByRole('button',{name:label,exact:true}).click();reconnected=true;
  await dialog.getByRole('button',{name:label,exact:true}).click();
  await dialog.getByText(locale==='zh-CN'?'设备已上线，请刷新列表后重试。':'The device is now online. Refresh the list and try again.').waitFor();assert.deepEqual(deletes,[]);
  reconnected=false;fail=true;await dialog.getByRole('button',{name:label,exact:true}).click();
  await dialog.getByText(locale==='zh-CN'?'删除失败，请检查权限或刷新后重试。':'Could not delete the device. Check permissions or refresh and retry.').waitFor();
  assert.deepEqual(deletes,[1]);fail=false;await dialog.getByRole('button',{name:label,exact:true}).click();await dialog.waitFor({state:'hidden'});await offline.waitFor({state:'hidden'});
  assert.deepEqual(deletes,[1,1]);assert.deepEqual(errors,[]);console.log('PASS',locale,theme,'globe, locale, cancel, offline delete, reconnect, retry, responsive');await context.close();
 }}finally{await browser.close();}
})().catch(e=>{console.error(e);process.exitCode=1;});
