const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const assert=require('node:assert/strict');
(async()=>{const browser=await chromium.launch();try{
 for(const locale of ['zh-CN','en-US'])for(const theme of ['light','dark']){
  const zh=locale==='zh-CN',context=await browser.newContext({viewport:{width:1440,height:1000}});
  await context.addInitScript(({locale,theme})=>{localStorage.setItem('liaison-locale',locale);localStorage.setItem('liaison-theme-preference',theme);},{locale,theme});
  let credentials=[],failSave=false,writes=[];
  const app={id:1,name:'SSH server',application_type:'ssh',ip:'10.0.0.10',port:22},proxy={id:1,name:'SSH access',access_type:'webssh',status:'running',application:app};
  await context.route('**/api/v1/**',async route=>{
   const req=route.request(),path=new URL(req.url()).pathname;let data={};
   if(path==='/api/v1/proxies')data={proxies:[proxy]};
   if(path==='/api/v1/applications')data={applications:[app]};
   if(path==='/api/v1/webssh/proxies/1')data={credentials};
   if(path==='/api/v1/webssh/proxies/1/credential'){
    writes.push(req.postDataJSON());
    if(failSave){await route.fulfill({json:{code:500}});return;}
    credentials=[{id:7,username:'operator',saved:req.postDataJSON().remember_password}];
   }
   if(path==='/api/v1/proxies/1')data=proxy;
   await route.fulfill({json:{code:200,data}});
  });
  const page=await context.newPage(),visit=()=>page.goto(`${process.env.E2E_UI_URL}/e2e/product-polish.html?ssh`);
  const dialog=()=>page.getByRole('dialog'),open=()=>page.getByRole('button',{name:zh?'去访问':'Open',exact:true}).click();
  const fill=async()=>{await dialog().locator('input[autocomplete=username]').fill('operator');await dialog().locator('input[type=password]').fill('fixture-only');};
  const saveOpen=()=>dialog().getByRole('button',{name:zh?'保存并访问':'Save & open',exact:true}).click();
  await visit();await open();await fill();
  failSave=true;await saveOpen();await dialog().getByText(zh?'连接配置保存失败，请检查后重试。':'Unable to save connection configuration. Check it and retry.').waitFor();assert.equal(await page.getByTestId('destination').count(),0);
  failSave=false;await page.screenshot({path:`/tmp/access-save-open-${locale}-${theme}.png`});await saveOpen();
  await page.getByTestId('destination').waitFor();assert.match(await page.getByTestId('destination').textContent(),/^\/webssh\/1\/connections\/7\?from=/);assert(!(await page.getByTestId('destination').textContent()).includes('fixture-only'));
  // Explicit editing remains save-only, even after cancelling an Open flow.
  credentials=[];await visit();await open();await dialog().getByRole('button',{name:zh?'取消':'Cancel',exact:true}).click();
  await page.getByRole('button',{name:zh?'编辑':'Edit',exact:true}).click();await fill();await dialog().getByRole('button',{name:zh?'确定':'Save',exact:true}).click();await dialog().waitFor({state:'hidden'});assert.equal(await page.getByTestId('destination').count(),0);
  // An unsaved password still opens the connection page, which prompts on connect.
  credentials=[];await visit();await open();await dialog().locator('input[autocomplete=username]').fill('operator');await dialog().getByRole('checkbox',{name:zh?'保存密码':'Save password'}).uncheck();await saveOpen();await page.getByTestId('destination').waitFor();assert.equal(writes.at(-1).remember_password,false);assert.equal(writes.at(-1).password,'');
  console.log('PASS',locale,theme,'save-and-open, failure retry, cancel, edit-only, temporary password, safe URL');await context.close();
 }
}finally{await browser.close()}})().catch(e=>{console.error(e);process.exitCode=1});
