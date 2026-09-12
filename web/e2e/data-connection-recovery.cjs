// Explicit staging target and credentials; never stores passwords or session tokens.
const assert = require('node:assert/strict');
const {request, chromium} = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
(async () => {
  const baseURL = process.env.E2E_BASE_URL;
  assert(baseURL && process.env.E2E_CONNECTION_PATH);
  const api = await request.newContext({baseURL, ignoreHTTPSErrors:true});
  const login = await (await api.post('/api/v1/iam/login', {data:{email:process.env.E2E_EMAIL,password:process.env.E2E_PASSWORD}})).json();
  assert(login.data?.token);
  const browser = await chromium.launch();
  const context = await browser.newContext({ignoreHTTPSErrors:true,viewport:{width:1440,height:1000}});
  let nativeToken;
  try {
    await context.addInitScript(token => {localStorage.setItem('token',token);localStorage.setItem('locale','zh-CN');localStorage.setItem('liaison-theme-preference','dark');}, login.data.token);
    const page=await context.newPage();
    const pattern='**/api/v1/webdata/proxies/*/session';
    let attempts=0;
    await page.route(pattern,route=>{attempts++;return route.fulfill({status:400,contentType:'application/json',body:JSON.stringify({code:400,message:'测试：目标不可达'})});});
    await page.goto(baseURL+process.env.E2E_CONNECTION_PATH);
    const state=page.getByTestId('webdata-connection-state');
    const retry=state.getByRole('button',{name:'重新连接',exact:true});
    await retry.waitFor();
    assert((await state.innerText()).includes('测试：目标不可达'));
    await page.waitForTimeout(1000);assert.equal(attempts,1,'must not automatically loop retries');
    await retry.click();await retry.waitFor();assert.equal(attempts,2);
    await page.screenshot({path:'/tmp/liaison-data-connection-failed.png'});
    await state.getByRole('button',{name:'返回连接',exact:true}).click();
    await page.getByText('匿名访问',{exact:true}).first().waitFor();
    await page.goto(baseURL+process.env.E2E_CONNECTION_PATH);
    await retry.waitFor();
    if(process.env.E2E_FAILURE_ONLY==='1') {console.log('PASS: failure ends loading; no auto retry; explicit retry');return;}
    await page.unroute(pattern);
    page.on('response',async response=>{
      if(response.url().endsWith('/session') && response.request().method()==='POST' && response.ok()) {
        nativeToken=(await response.json()).data?.token;
      }
    });
    await retry.click();
    await page.locator('.webdata-statement-textarea').waitFor({timeout:60000});
    assert.equal(await state.count(),0);
    await page.screenshot({path:'/tmp/liaison-data-connection-recovered.png'});
    console.log('PASS: failure ends loading; no auto retry; explicit retry; real connection recovers');
  } finally {
    await browser.close();
    if(nativeToken)await api.delete('/api/v1/webdata/sessions/'+nativeToken,{headers:{Authorization:'Bearer '+login.data.token}});
    await api.dispose();
  }
})().catch(error=>{console.error(error);process.exitCode=1;});
