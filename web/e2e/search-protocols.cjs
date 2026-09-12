// Real staging acceptance. Targets and credentials must be explicitly provided.
// Creates a demo application/connection for review, and cleans up its temporary index.
const assert = require('node:assert/strict');
const {request, chromium} = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const fs = require('node:fs');
const {randomBytes} = require('node:crypto');

(async () => {
 const base = process.env.E2E_BASE_URL, protocol = process.env.E2E_SEARCH_PROTOCOL;
 assert(base && ['elasticsearch','opensearch'].includes(protocol));
 const req = await request.newContext({baseURL:base, ignoreHTTPSErrors:true});
 let token, connection, agent, browser, peerID, indexCreated=false;
 const index='liaison-e2e-'+Date.now();
 const api=async(path,method='GET',data)=>{
  const response=await req.fetch(path,{method,data,timeout:180000,headers:token?{Authorization:'Bearer '+token}:{}});
  const value=await response.json();
  if(!response.ok()||value.code<200||value.code>=300)throw Error(`${method} ${path.replace(/sessions\/[^/]+/g,'sessions/[id]')} ${response.status()} ${value.message}`);
  return value.data;
 };
 const checks=[];
 checks.push=(...items)=>{console.log('[PASS]',...items);return Array.prototype.push.apply(checks,items);};
 try {
  token=(await api('/api/v1/iam/login','POST',{email:process.env.E2E_EMAIL,password:process.env.E2E_PASSWORD})).token;
  const apps=(await api('/api/v1/applications?page=1&page_size=100')).applications;
  const reference=apps.find(a=>a.application_type==='clickhouse');assert(reference,'reference connector missing');
  const title=protocol==='elasticsearch'?'Elasticsearch Demo':'OpenSearch Demo';
  let app=apps.find(a=>a.name===title);
  if(!app)app=await api('/api/v1/applications','POST',{name:title,description:'Search workspace acceptance',ip:process.env.E2E_SEARCH_HOST,port:Number(process.env.E2E_SEARCH_PORT),application_type:protocol,edge_id:reference.edge_id,device_id:reference.device.id});
  let proxy=app.proxy;
  if(!proxy?.id)proxy=await api('/api/v1/proxies','POST',{application_id:app.id,name:'Web '+title,access_protocol:'web',expose_public_port:false,port:0});
  const targetPath='/api/v1/webdata/proxies/'+proxy.id;
  connection=await api(targetPath+'/session','POST',{protocol,username:process.env.E2E_SEARCH_USERNAME||'',password:process.env.E2E_SEARCH_PASSWORD||'',tls_mode:process.env.E2E_SEARCH_TLS||'disable',save_credential:true});
  const sessionPath='/api/v1/webdata/sessions/'+connection.token;
  const organizations=(await api('/api/v1/iam/organizations')).organizations;
  const organization=organizations.find(o=>o.is_root)||organizations[0];assert(organization);
  const peerEmail='search-e2e-'+Date.now()+'@example.test', peerPassword='E2e!'+randomBytes(20).toString('base64url');
  peerID=(await api('/api/v1/iam/users','POST',{organization_id:organization.id,email:peerEmail,password:peerPassword,role:'user',name:'Temporary search isolation test'})).user.id;
  const peerToken=(await api('/api/v1/iam/login','POST',{email:peerEmail,password:peerPassword})).token;
  const foreign=await req.get(sessionPath+'/metadata',{headers:{Authorization:'Bearer '+peerToken}});
  assert([401,403,404].includes(foreign.status()));checks.push('second user cannot read another user session');
  const execute=async(method,path,body)=>{
   const result=await api(sessionPath+'/execute','POST',{statement:JSON.stringify({method,path,body})});assert(!result.error,result.error);return result;
  };
  await execute('PUT','/'+index,{mappings:{properties:{message:{type:'text'},value:{type:'integer'}}}});indexCreated=true;
  await execute('PUT','/'+index+'/_doc/1',{message:'中文 acceptance',value:3});
  let result=await execute('GET','/'+index+'/_doc/1');assert.equal(JSON.parse(result.message)._source.value,3);
  await execute('POST','/'+index+'/_update/1',{doc:{value:4}});
  result=await execute('GET','/'+index+'/_doc/1');assert.equal(JSON.parse(result.message)._source.value,4);checks.push('connector document create/read/update');
  const metadata=await api(sessionPath+'/metadata');assert(metadata.nodes.some(n=>n.title===index));
  const mapping=await api(sessionPath+'/object?type=index&name='+index);assert.equal(mapping.columns.length,2);assert(mapping.ddl.includes('message'));checks.push('index navigation and Mapping fields');
  const query={method:'POST',path:'/'+index+'/_search',body:{size:20,query:{match_all:{}},aggs:{value_sum:{sum:{field:'value'}}}}};
  for(let i=0;i<20;i++){result=await execute(query.method,query.path,query.body);if(result.rows?.length)break;await new Promise(r=>setTimeout(r,500));}
  assert.equal(result.rows.length,1);assert.equal(JSON.parse(result.message).aggregations.value_sum.value,4);checks.push('search hits and aggregation');
  const invalid=await api(sessionPath+'/execute','POST',{statement:'{"method":"POST","path":"/_reindex","body":{}}'});assert(invalid.error);await execute('GET','/'+index+'/_doc/1');checks.push('blocked endpoint and error recovery');
  if(process.env.E2E_SEARCH_AGENT!=='0'){
   agent=await api('/api/v1/agent/sessions','POST',{handle_id:connection.token,title:'Search acceptance '+protocol});
   const path='/api/v1/agent/sessions/'+agent.session.id;
   await api(path+'/turns','POST',{prompt:'只用 data.schema 查看索引 '+index+' 的 Mapping，path=["index","","","'+index+'"]，不执行查询。'});
   let detail=await api(path);assert.equal(detail.approvals.length,0);assert(detail.messages.some(m=>m.value.role==='tool'&&m.value.content.includes('message')));checks.push('real Agent Mapping without approval');
   for(const decision of ['approve','deny']){
    await api(path+'/turns','POST',{prompt:'请用 data.query 执行以下原始 JSON 请求，不修改、不增加其他命令，结果简短概括，不重试：'+JSON.stringify(query)});
    detail=await api(path);const pending=detail.approvals.filter(a=>a.status===0);assert.equal(pending.length,1);assert.equal(pending[0].tool_id.namespace,'data');assert.equal(pending[0].tool_id.name,'query');assert.deepEqual(JSON.parse(pending[0].input.statement),query);
    await api(path+'/approvals/'+pending[0].id,'POST',{decision,note:'Verified exact read-only search query'});
    detail=await api(path);assert.equal(detail.approvals.filter(a=>a.status===0).length,0);
    if(decision==='approve')assert(detail.messages.some(m=>m.value.role==='tool'&&m.value.content.includes('value_sum')));
    checks.push('real Agent '+decision);
   }
  }
  const target=await api(targetPath);
  const credential=target.credentials?.at(-1)||await api(targetPath+'/credential','POST',{name:title,protocol,username:process.env.E2E_SEARCH_USERNAME||'',password:process.env.E2E_SEARCH_PASSWORD||'',tls_mode:process.env.E2E_SEARCH_TLS||'disable'});assert(credential.id);
  browser=await chromium.launch();const context=await browser.newContext({ignoreHTTPSErrors:true,viewport:{width:1440,height:1000}});
  await context.addInitScript(t=>{localStorage.setItem('token',t);localStorage.setItem('locale','zh-CN');localStorage.setItem('liaison-theme-preference','dark');},token);
  const page=await context.newPage();const errors=[];page.on('pageerror',e=>errors.push(e.message));
  const url=base+'/webdata/'+proxy.id+'/connections/'+credential.id;
  await page.goto(url);const editor=page.locator('.webdata-statement-textarea');await editor.waitFor();
  await page.getByText(index,{exact:true}).first().dblclick();
  await editor.fill(JSON.stringify(query,null,2));await page.getByRole('button',{name:'执行',exact:true}).click();
  await page.getByText('完整 JSON 响应（含聚合）',{exact:true}).waitFor();
  await page.getByText('完整 JSON 响应（含聚合）',{exact:true}).click();
  await editor.evaluate(e=>{e.scrollTop=0;});
  await page.waitForTimeout(1500);
  assert(await page.locator('.webdata-search-results .liaison-table tbody tr').first().isVisible());
  const rowBounds=await page.locator('.webdata-search-results .liaison-table tbody tr').first().boundingBox();
  assert(rowBounds && rowBounds.height>=20,'result row compressed after JSON expansion');
  await page.screenshot({path:'/tmp/liaison-'+protocol+'-workspace.png'});
  assert.equal(errors.length,0,errors.join('\n'));assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));checks.push('browser editor/query/full JSON/no page errors');
  await page.setViewportSize({width:390,height:844});assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));checks.push('mobile page width');
  await execute('DELETE','/'+index+'/_doc/1');checks.push('document delete');
  await execute('DELETE','/'+index);indexCreated=false;
  await api(sessionPath,'DELETE');connection=undefined;
  const invalidSession=await req.get(sessionPath+'/metadata',{headers:{Authorization:'Bearer '+token}});assert(!invalidSession.ok());checks.push('closed session rejected');
  fs.writeFileSync('/tmp/liaison-'+protocol+'-acceptance.json',JSON.stringify({protocol,checks,url},null,2));console.log(JSON.stringify({protocol,checks,url},null,2));
 }finally{
  if(connection&&indexCreated)await api('/api/v1/webdata/sessions/'+connection.token+'/execute','POST',{statement:JSON.stringify({method:'DELETE',path:'/'+index})});
  if(agent){const path='/api/v1/agent/sessions/'+agent.session.id;const detail=await api(path);await api(path,'DELETE',{version:detail.session.version});}
  if(connection)await api('/api/v1/webdata/sessions/'+connection.token,'DELETE');
  if(peerID)await api('/api/v1/iam/users/'+peerID,'DELETE');
  await browser?.close();await req.dispose();
 }
})().catch(error=>{console.error(error);process.exitCode=1});
