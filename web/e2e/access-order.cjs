const assert=require('node:assert/strict');
const {request,chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
(async()=>{
 const baseURL=process.env.E2E_BASE_URL;assert(baseURL);
 const api=await request.newContext({baseURL,ignoreHTTPSErrors:true});
 const browser=await chromium.launch();
 try {
  const login=await (await api.post('/api/v1/iam/login',{data:{email:process.env.E2E_EMAIL,password:process.env.E2E_PASSWORD}})).json();
  assert(login.data?.token);
  const context=await browser.newContext({ignoreHTTPSErrors:true});
  await context.addInitScript(token=>{localStorage.setItem('token',token);localStorage.setItem('locale','zh-CN');},login.data.token);
  const page=await context.newPage();await page.goto(baseURL+'/proxy');
  await page.getByRole('button',{name:'新建访问',exact:true}).click();
  const options=await page.locator('#create-proxy select').first().locator('option').evaluateAll(nodes=>nodes.map(n=>n.value).filter(Boolean));
  const web=options.filter(v=>v.startsWith('web'));
  assert(web.length>0);assert.deepEqual(options.slice(0,web.length),web);
  await page.getByRole('button',{name:'取消',exact:true}).click();
  console.log('PASS: deployed access creation dropdown puts all Web protocols first; no access created');
 }finally{await browser.close();await api.dispose();}
})().catch(e=>{console.error(e);process.exitCode=1;});
