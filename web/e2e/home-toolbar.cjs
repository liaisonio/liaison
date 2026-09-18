const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const assert=require('node:assert/strict');
(async()=>{
 const browser=await chromium.launch();
 try{for(const locale of ['zh-CN','en-US'])for(const theme of ['dark','light'])for(const selected of [false,true]){
  const context=await browser.newContext({viewport:{width:1440,height:1000}});
  await context.addInitScript(({locale,theme})=>{localStorage.setItem('liaison-locale',locale);localStorage.setItem('liaison-theme-preference',theme);},{locale,theme});
  await context.route('**/api/v1/**',route=>{
   const path=new URL(route.request().url()).pathname;let data={};
   if(path.endsWith('/status'))data={enabled:true,models:[]};
   else if(path.endsWith('/events'))return route.fulfill({contentType:'text/event-stream',body:': keepalive\n\n'});
   else if(path.endsWith('/sessions'))data={items:[]};
   else if(path.includes('/agent/sessions/'))data={session:{id:'session_abcdef',kind:'management',title:'Fixture'},messages:[],turns:[],steps:[],approvals:[]};
   else if(path.includes('/edges'))data={edges:[]};else if(path.includes('/devices'))data={devices:[]};else if(path.includes('/applications'))data={applications:[{id:1,name:'Example database',application_type:'mysql'}]};
   return route.fulfill({json:{code:200,data}});
  });
  const page=await context.newPage(),errors=[];page.on('pageerror',e=>errors.push(e.message));
  await page.goto(`${process.env.E2E_UI_URL}/e2e/home-toolbar.html${selected?'?selected=1':''}`);
  const history=page.getByRole('button',{name:locale==='zh-CN'?'历史会话':'History',exact:true}),user=page.getByRole('button',{name:locale==='zh-CN'?'打开用户菜单':'Open user menu',exact:true});
  await history.waitFor();assert.equal(await user.count(),1);
  if(!selected){await page.locator('.management-agent-presets').waitFor();assert.equal(await page.locator('.management-agent-presets button').count(),4);await page.getByRole('button',{name:locale==='zh-CN'?/我能调用哪些模型/:/Which models can I call/}).waitFor();}
  for(const width of [1440,390]){
   await page.setViewportSize({width,height:width===390?844:1000});
   const h=await history.boundingBox(),u=await user.boundingBox();assert(h&&u);
   assert(h.x+h.width<=u.x&&Math.abs((h.y+h.height/2)-(u.y+u.height/2))<5,'History and avatar must be in one non-overlapping row');
   assert(h.x>=0&&u.x+u.width<=width,'Toolbar fits viewport');
   await history.click();assert.equal(await history.getAttribute('aria-expanded'),'true');await history.click();
   await user.click();await page.locator('.liaison-header-user-menu').waitFor();await page.keyboard.press('Escape');
   await page.screenshot({path:`/tmp/home-toolbar-${locale}-${theme}-${selected?'session':'home'}-${width}.png`});
   assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));
  }
  assert.deepEqual(errors,[]);await context.close();console.log('PASS home/session history and avatar layout:',locale,theme,selected);
 }}finally{await browser.close();}
})().catch(e=>{console.error(e);process.exitCode=1;});
