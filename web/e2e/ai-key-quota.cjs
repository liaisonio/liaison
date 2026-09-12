// Staging only: creates its own access and keys; credentials come from env.
const assert = require('node:assert/strict');
const {request,chromium} = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
(async()=>{
 const origin=process.env.E2E_BASE_URL;
 assert(origin && process.env.E2E_AI_EDGE_ID && process.env.E2E_AI_HOST && process.env.E2E_AI_PORT);
 const req=await request.newContext({baseURL:origin,ignoreHTTPSErrors:true});
 let token,browser,app,access; const keys=[];
 const api=async(path,method='GET',data)=>{
  const res=await req.fetch(path,{method,data,headers:token?{Authorization:`Bearer ${token}`}:{}});
  assert(res.ok(),`${method} ${path}: ${res.status()}`);
  return (await res.json()).data;
 };
 try {
  token=(await api('/api/v1/iam/login','POST',{email:process.env.E2E_EMAIL,password:process.env.E2E_PASSWORD})).token;
  app=await api('/api/v1/applications','POST',{name:'Quota UI fixture',ip:process.env.E2E_AI_HOST,port:Number(process.env.E2E_AI_PORT),application_type:'llm',edge_id:Number(process.env.E2E_AI_EDGE_ID)});
  await api(`/api/v1/ai/applications/${app.id}`,'PUT',{protocol:'openai-compatible',base_path:'/v1',tls:false});
  access=await api('/api/v1/proxies','POST',{name:'Quota UI fixture',application_id:app.id,access_protocol:'aiapi',port:0,expose_public_port:false});
  const path=`/api/v1/ai/accesses/${access.id}`;
  await api(path,'PUT',{enabled:true,models:{chat:'fixture-chat'}});
  browser=await chromium.launch();
  for(const locale of ['zh-CN','en-US']) for(const theme of ['dark','light']) {
   const zh=locale==='zh-CN';
   const context=await browser.newContext({ignoreHTTPSErrors:true,viewport:{width:1440,height:1000}});
   await context.addInitScript(({token,locale,theme})=>{localStorage.setItem('token',token);localStorage.setItem('liaison-locale',locale);localStorage.setItem('liaison-theme-preference',theme)},{token,locale,theme});
   const page=await context.newPage(), errors=[];
   page.on('pageerror',e=>errors.push(e.message));
   await page.goto(`${origin}/ai/${access.id}`);
   await page.locator('.ai-api-tabs button').filter({hasText:zh?'API 密钥':'API keys'}).click();
   await page.getByRole('button',{name:zh?'创建密钥':'Create key',exact:true}).click();
   const dialog=page.getByRole('dialog');
   await dialog.getByLabel(zh?'名称':'Name',{exact:true}).fill(`Quota ${locale} ${theme}`);
   const limit=dialog.getByLabel(zh?'Token 总额度':'Total token limit');
   await limit.fill('-1');
   assert(await dialog.getByRole('button',{name:zh?'创建':'Create',exact:true}).isDisabled());
   await limit.fill('100000');
   await dialog.getByLabel('chat',{exact:true}).check();
   await page.screenshot({path:`/tmp/liaison-quota-${locale}-${theme}-create.png`,fullPage:true});
   await dialog.getByRole('button',{name:zh?'创建':'Create',exact:true}).click();
   await dialog.getByRole('button',{name:zh?'我已保存，完成':'Saved, done',exact:true}).waitFor();
   const key=(await api(path+'/keys')).find(k=>k.name===`Quota ${locale} ${theme}`);
   assert(key); keys.push(key.id); assert.equal(key.token_limit,100000);
   await dialog.getByRole('button',{name:zh?'我已保存，完成':'Saved, done',exact:true}).click();
   let row=page.locator('.ai-api-table tbody tr').filter({hasText:key.name});
   await row.getByRole('button',{name:zh?'调整额度':'Edit quota'}).click();
   await dialog.getByLabel(zh?'Token 总额度':'Total token limit').fill('0');
   await dialog.getByRole('button',{name:zh?'保存':'Save',exact:true}).click();
   await dialog.waitFor({state:'hidden'});
   await row.getByText(zh?'额度耗尽':'Quota exhausted',{exact:true}).waitFor();
   assert.equal((await api(path+'/keys')).find(k=>k.id===key.id).remaining_tokens,0);
   await page.screenshot({path:`/tmp/liaison-quota-${locale}-${theme}-list.png`,fullPage:true});
   await page.setViewportSize({width:390,height:844});
   await page.waitForTimeout(500);
   assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
   await row.getByRole('button',{name:zh?'调整额度':'Edit quota'}).click();
   await page.screenshot({path:`/tmp/liaison-quota-${locale}-${theme}-mobile.png`,fullPage:true});
   await dialog.getByLabel(zh?'Token 总额度':'Total token limit').fill('');
   await dialog.getByRole('button',{name:zh?'保存':'Save',exact:true}).click();
   await dialog.waitFor({state:'hidden'});
   assert.equal((await api(path+'/keys')).find(k=>k.id===key.id).token_limit,null);
   assert.deepEqual(errors,[]);
   console.log('PASS quota UI',locale,theme,'create/invalid/edit/exhausted/unlimited/mobile');
   await context.close();
  }
 } finally {
  if(access) for(const id of keys) await api(`/api/v1/ai/accesses/${access.id}/keys/${id}`,'DELETE');
  if(access) await api(`/api/v1/proxies/${access.id}`,'DELETE');
  if(app) await api(`/api/v1/applications/${app.id}`,'DELETE');
  await browser?.close(); await req.dispose();
 }
})().catch(e=>{console.error(e.message);process.exitCode=1});
