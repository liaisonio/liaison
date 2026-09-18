// Mock-backed UI regression; real SMB integration is a separate Go test.
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright'),assert=require('node:assert/strict');
(async()=>{const browser=await chromium.launch();try{
 for(const locale of ['zh-CN','en-US'])for(const theme of ['dark','light']){
  const ctx=await browser.newContext({viewport:{width:1440,height:1000}}),zh=locale==='zh-CN';let fail=true,saved=true;const sessions=[],errors=[];
  await ctx.addInitScript(({locale,theme})=>{localStorage.setItem('liaison-locale',locale);localStorage.setItem('liaison-theme-preference',theme)},{locale,theme});
  await ctx.route('**/api/v1/webdata/**',async route=>{
   const req=route.request(),path=new URL(req.url()).pathname;let data;
   if(path==='/api/v1/webdata/proxies/1')data={proxy_id:1,proxy_name:'SMB files',protocol:'smb',target_host:'files.example.test',target_port:445,credentials:[{id:7,username:'demo',database:'Shared',saved}]};
   else if(req.method()==='POST'){
    assert(path.endsWith('/session'),'no write operation');sessions.push(req.postDataJSON());if(fail){await route.fulfill({status:502,json:{code:502}});return;}
    data={token:'fixture-session',expires_at:'2099-01-01T00:00:00Z'};
   }else if(path.endsWith('/list'))data=[{name:'readme.txt',directory:false,mode:'-r--------',size:12}];
   else if(path.endsWith('/preview'))data={text:'Example SMB file'};
   else if(path.endsWith('/download')){await route.fulfill({body:'Example SMB file',headers:{'Content-Disposition':'attachment; filename="readme.txt"'}});return;}
   await route.fulfill({json:{code:200,data}});
  });
  const page=await ctx.newPage();page.on('pageerror',e=>errors.push(e.message));
  await page.goto(`${process.env.E2E_UI_URL}/e2e/websmb.html`);await page.getByRole('button',{name:zh?'重试':'Retry',exact:true}).waitFor();
  fail=false;await page.getByRole('button',{name:zh?'重试':'Retry',exact:true}).click();
  await page.getByRole('button',{name:'readme.txt',exact:true}).waitFor();assert.equal(sessions.at(-1).credential_id,7);
  await page.getByRole('button',{name:'readme.txt',exact:true}).dblclick();await page.getByText('Example SMB file',{exact:true}).waitFor();
  assert.equal(await page.locator('input[type=file]').count(),0);
  const download=page.waitForEvent('download');await page.getByRole('button',{name:zh?'下载':'Download',exact:true}).click();assert.equal((await download).suggestedFilename(),'readme.txt');
  await page.screenshot({path:`/tmp/websmb-${locale}-${theme}.png`});
  await page.setViewportSize({width:390,height:844});await page.screenshot({path:`/tmp/websmb-${locale}-${theme}-mobile.png`});assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
  saved=false;await page.goto(`${process.env.E2E_UI_URL}/e2e/websmb.html`);
  await page.locator('input[type=password]').fill('temporary-fixture');
  await page.getByRole('button',{name:zh?'连接':'Connect',exact:true}).click();await page.getByRole('button',{name:'readme.txt',exact:true}).waitFor();
  assert.equal(sessions.at(-1).password,'temporary-fixture');assert.equal(sessions.at(-1).database,'Shared');
  assert(!(await page.evaluate(()=>JSON.stringify({...localStorage,...sessionStorage}))).includes('temporary-fixture'));
  assert.deepEqual(errors,[]);console.log('PASS',locale,theme,'SMB retry/preview/read-only/unsaved password/mobile');await ctx.close();
 }
}finally{await browser.close();}})().catch(e=>{console.error(e);process.exitCode=1});
