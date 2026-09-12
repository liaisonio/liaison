const assert=require('node:assert/strict');
const {request,chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
(async()=>{
 const api=await request.newContext({baseURL:process.env.STAGING_URL,ignoreHTTPSErrors:true,timeout:120000});
 const auth=await(await api.post('/api/v1/iam/login',{data:{email:process.env.STAGING_EMAIL,password:process.env.STAGING_PASSWORD}})).json();assert(auth.data?.token);
 const headers={Authorization:'Bearer '+auth.data.token};
 const call=async(path,method='GET',data)=>{const r=await api.fetch(path,{method,data,headers});assert(r.ok(),`${method} ${path}: ${r.status()}`);return(await r.json()).data;};
 const original=await call('/api/v1/settings/model');
 const payload={enabled:original.enabled,default_provider:original.default_provider,output_language:original.output_language,providers:original.providers.map(({id,type,base_url,model,models})=>({id,type,base_url,model,models}))};
 const sessions=[];let browser;
 try {
  for(const language of ['en','zh']){
   const saved=await call('/api/v1/settings/model','PUT',{...payload,output_language:language});assert.equal(saved.output_language,language);
   assert.equal((await call('/api/v1/settings/model')).output_language,language);
   const session=await call('/api/v1/agent/sessions','POST',{kind:'management',title:'Language verification'});sessions.push(session.session.id);
   const path='/api/v1/agent/sessions/'+session.session.id;
   await call(path+'/turns','POST',{prompt:language==='en'?'请用中文简单问候我，不要使用工具。':'Please greet me briefly in English. Do not use tools.'});
   const detail=await call(path);
   const answer=detail.messages.filter(m=>m.value.role==='assistant').map(m=>m.value.content||'').join(' ');
   assert(answer.trim(),'No answer');
   console.log(JSON.stringify({language,answer}));
   assert.equal(/[\u3400-\u9fff]/u.test(answer),language==='zh');
  }
  browser=await chromium.launch();const page=await browser.newPage({ignoreHTTPSErrors:true});
  await page.addInitScript(token=>{localStorage.setItem('token',token);localStorage.setItem('locale','zh-CN');},auth.data.token);
  await page.goto(process.env.STAGING_URL+'/settings');
  await page.getByRole('button',{name:'模型配置',exact:true}).click();
  await page.getByRole('combobox',{name:'AI 输出语言',exact:true}).waitFor();
  console.log('PASS persisted language, live model policy against opposite-language request, settings selector');
 }finally{
  await call('/api/v1/settings/model','PUT',payload);
  for(const id of sessions){const path='/api/v1/agent/sessions/'+id;const d=await call(path);await call(path,'DELETE',{version:d.session.version});}
  await browser?.close();await api.dispose();
 }
})().catch(e=>{console.error(e);process.exitCode=1;});
