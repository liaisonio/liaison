const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const assert=require('node:assert/strict');
(async()=>{const browser=await chromium.launch();try{
 for(const locale of ['zh-CN','en-US'])for(const theme of ['light','dark']){
  const zh=locale==='zh-CN',context=await browser.newContext({viewport:{width:1440,height:1000}});
  await context.addInitScript(({locale,theme})=>{localStorage.setItem('liaison-locale',locale);localStorage.setItem('liaison-theme-preference',theme);},{locale,theme});
  const app={id:1,name:'SSH server',edge_id:1,application_type:'ssh',ip:'10.0.0.10',port:22};
  const names=['Saved access','Temporary access','Unconfigured access','Failed access'];let fail=true,writes=0;
  await context.route('**/api/v1/**',async route=>{
   const req=route.request(),path=new URL(req.url()).pathname;let data={};
   if(req.method()!=='GET')writes++;
   if(path==='/api/v1/proxies')data={proxies:names.map((name,i)=>({id:i+1,name,access_type:'webssh',status:'running',application:app}))};
   if(path==='/api/v1/applications')data={applications:[app]};
   if(path==='/api/v1/edges')data={edges:[{id:1,name:'Connector',online:1,device:{name:'Server'}}]};
   if(path.startsWith('/api/v1/webssh/proxies/')){
    const id=Number(path.split('/').pop());
    if(id===4&&fail){await route.fulfill({status:500,json:{code:500,message:'failed'}});return;}
    data={credentials:id===3?[]:[{id,username:'operator',saved:id===1}],proxy_id:id};
   }
   await route.fulfill({json:{code:200,data}});
  });
  const page=await context.newPage();await page.goto(`${process.env.E2E_UI_URL}/e2e/product-polish.html?ssh`);
  const row=name=>page.getByRole('row').filter({hasText:name});
  await row('Saved access').getByText(zh?'已保存':'Saved',{exact:true}).waitFor();
  await row('Temporary access').getByText(zh?'未保存':'Not saved',{exact:true}).waitFor();
  assert(await row('Unconfigured access').getByText(zh?'未配置连接':'Not configured',{exact:true}).count());
  assert(await row('Failed access').getByRole('button',{name:zh?'加载失败 · 重试':'Failed · Retry'}).count());
  fail=false;await row('Failed access').getByRole('button',{name:zh?'加载失败 · 重试':'Failed · Retry'}).last().click();
  await row('Failed access').getByText(zh?'未保存':'Not saved',{exact:true}).waitFor();
  await page.screenshot({path:`/tmp/access-password-list-${locale}-${theme}.png`});
  await page.getByRole('button',{name:zh?'新建访问':'Create access',exact:true}).click();
  const dialog=page.getByRole('dialog');await dialog.getByRole('combobox',{name:zh?'应用':'Application',exact:true}).selectOption('1');
  const user=dialog.locator('input[autocomplete=username]'),password=dialog.locator('input[type=password]');
  const baseline=await page.addStyleTag({content:'.liaison-initial-connection-fields { align-items: stretch; }'});
  const oldUser=await user.boundingBox(),oldPassword=await password.boundingBox();assert(Math.abs(oldUser.y-oldPassword.y)>1);
  await page.screenshot({path:`/tmp/access-password-before-${locale}-${theme}.png`});await baseline.evaluate(el=>el.remove());
  for(const width of [1440,390]){
   await page.setViewportSize({width,height:width===390?844:1000});
   if(width===1440){const a=await user.boundingBox(),b=await password.boundingBox();assert(Math.abs(a.y-b.y)<1);assert.equal(a.height,b.height);}
   assert(await dialog.evaluate(el=>el.scrollWidth<=el.clientWidth+1));
   await page.screenshot({path:`/tmp/access-password-form-${locale}-${theme}-${width}.png`});
  }
  const remember=dialog.getByRole('checkbox',{name:zh?'保存密码':'Save password'});
  await remember.uncheck();assert.equal(await password.count(),0);await remember.check();assert(await password.isVisible());
  await user.fill('operator');await password.fill('fixture-only');await password.press('Tab');assert(await remember.evaluate(el=>el===document.activeElement));
  await dialog.getByRole('button',{name:zh?'取消':'Cancel',exact:true}).click();assert.equal(writes,0);
  console.log('PASS',locale,theme,'alignment, saved states, retry, responsive, keyboard, no writes');await context.close();
 }
}finally{await browser.close()}})().catch(e=>{console.error(e);process.exitCode=1});
