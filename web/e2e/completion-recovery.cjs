const assert=require('node:assert/strict');
const {chromium,request}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
(async()=>{
 const api=await request.newContext({baseURL:process.env.STAGING_URL,ignoreHTTPSErrors:true});
 const auth=await(await api.post('/api/v1/iam/login',{data:{email:process.env.STAGING_EMAIL,password:process.env.STAGING_PASSWORD}})).json();assert(auth.data?.token);
 const browser=await chromium.launch();let page;const sessions=new Set();
 try{
  const context=await browser.newContext({ignoreHTTPSErrors:true,viewport:{width:1440,height:1000}});
  await context.addInitScript(t=>{localStorage.setItem('token',t);localStorage.setItem('locale','zh-CN');},auth.data.token);
  page=await context.newPage();let output='';const errors=[];
  page.on('response',r=>{if(r.request().method()==='POST'&&r.url().endsWith('/agent/sessions'))r.json().then(v=>{if(v.data)sessions.add(v.data.session.id);}).catch(()=>{});});
  page.on('pageerror',e=>errors.push(e.message));
  page.on('websocket',ws=>ws.on('framereceived',e=>{try{output+=JSON.parse(e.payload.toString()).data||'';}catch{}}));
  const until=async test=>{const end=Date.now()+15000;while(Date.now()<end){if(test())return;await page.waitForTimeout(60);}throw Error('Missing prompt');};
  const prompts=()=>(output.match(/633;B/g)||[]).length;
  await page.goto(process.env.STAGING_URL+'/webssh/2/connections/1');await until(()=>prompts()>0);
  await page.getByRole('button',{name:'关闭自动提示',exact:true}).waitFor();
  assert.equal(await page.getByRole('button',{name:'提示 · Ctrl+Space',exact:true}).count(),0);
  await page.locator('.webssh-terminal-screen').click();
  const response=()=>page.waitForResponse(r=>r.request().method()==='POST'&&r.url().endsWith('/assistance/suggestions'),{timeout:20000});
  for(const command of ['ip','ls','ip a']){
   const pending=response();await page.keyboard.type(command,{delay:40});const r=await pending;assert.equal(r.status(),200);
   const value=(await r.json()).data.text;console.log(JSON.stringify({command,insertion:value}));
   if(value)await page.locator('.webssh-completion').waitFor();
   const n=prompts();await page.keyboard.press('Control+c');await until(()=>prompts()>n);
  }
  let pending=response();await page.keyboard.type('find . -type ',{delay:40});assert.equal((await pending).status(),200);
  pending=response();await page.setViewportSize({width:1380,height:950});assert.equal((await pending).status(),200);console.log('PASS resize resumes without new prompt');
  await page.getByRole('combobox',{name:'AI 上下文共享',exact:true}).focus();
  pending=response();await page.locator('.webssh-terminal-screen').click();assert.equal((await pending).status(),200);console.log('PASS refocus resumes without extra typing');
  await page.screenshot({path:'/tmp/liaison-completion-recovery.png'});
  assert.deepEqual(errors,[]);
 }finally{
  for(const id of sessions){
   const headers={Authorization:'Bearer '+auth.data.token};
   const path='/api/v1/agent/sessions/'+id;
   const detail=await(await api.get(path,{headers})).json();
   assert((await api.delete(path,{headers,data:{version:detail.data.session.version}})).ok());
  }
  if(page){const d=page.getByRole('button',{name:'断开',exact:true});if(await d.count())await d.click();}await browser.close();await api.dispose();
 }
})().catch(e=>{console.error(e);process.exitCode=1;});
