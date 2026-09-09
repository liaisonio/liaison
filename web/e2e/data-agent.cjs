// Real staging E2E. Invoked with a Playwright APIRequestContext and credentials
// supplied by the runner. Never hard-code passwords or approve arbitrary commands.
exports.run = async function run(request, baseURL, email, password, protocol) {
  const assert = require('node:assert/strict');
  let token = '';
  const checks = [];
  const api = async (path, method = 'GET', data) => {
    const response = await request.fetch(baseURL + path, { method, data, timeout: 180000, headers: token ? { Authorization: 'Bearer ' + token } : {} });
    const value = await response.json();
    if (!response.ok()) throw Error(`${method} ${path.replace(/sessions\/[^/]+/, 'sessions/[id]')}: ${response.status()} ${value.message}`);
    return value.data;
  };
  token = (await api('/api/v1/iam/login', 'POST', { email, password })).token;
  const apps = (await api('/api/v1/applications?page=1&page_size=100')).applications;
  const app = apps.find(item => item.application_type === protocol && item.name === 'Agent Demo ' + protocol);
  assert.ok(app?.proxy?.id, 'Acceptance application missing');
  const targetPath = '/api/v1/webdata/proxies/' + app.proxy.id;
  const target = await api(targetPath);
  const credential = target.credentials.find(item => item.name === 'Agent demo ' + protocol);
  assert.ok(credential, 'Saved acceptance connection missing');
  let connection, session;
  try {
    connection = await api(targetPath + '/session', 'POST', { protocol, credential_id: credential.id });
    checks.push('saved connection');
    const metadata = await api('/api/v1/webdata/sessions/' + connection.token + '/metadata');
    assert.ok(metadata.nodes.length); checks.push('native metadata');
    session = await api('/api/v1/agent/sessions', 'POST', { handle_id: connection.token, title: 'E2E data-agent ' + protocol });
    const path = '/api/v1/agent/sessions/' + session.session.id;
    const get = () => api(path);
    const toolResults = detail => detail.messages.filter(m => m.value.role === 'tool').map(m => ({...m.value, parsed:JSON.parse(m.value.content)}));
    await api(path + '/turns', 'POST', { prompt: '请调用 data.schema 查看当前连接的数据结构，不执行 data.query，也不要执行其他协议工具。简短概括。' });
    let detail = await get();
    assert.ok(toolResults(detail).some(m => m.tool_name === 'data.schema' || m.name === 'data.schema' || m.parsed.Content?.nodes));
    assert.equal(detail.approvals.length, 0); checks.push('schema without approval');
    const good = protocol === 'redis' ? 'PING' : protocol === 'mongodb' ? '{"ping":1}' : 'SELECT 734 AS agent_e2e';
    async function query(statement, decision = 'approve') {
      await api(path + '/turns', 'POST', { prompt: '请使用 data.query 执行下面这条原始语句，不修改、不附加其他命令。执行后简短报告成功或错误，不重试：\n' + statement });
      const pending = (await get()).approvals.filter(a => a.status === 0);
      assert.equal(pending.length, 1, 'Expected exactly one approval');
      assert.equal(pending[0].tool_id.namespace, 'data'); assert.equal(pending[0].tool_id.name, 'query');
      if (protocol === 'mongodb') assert.deepEqual(JSON.parse(pending[0].input.statement), JSON.parse(statement), 'Refusing an unexpected JSON command');
      else assert.equal(pending[0].input.statement.trim(), statement, 'Refusing an unexpected statement');
      await api(path + '/approvals/' + pending[0].id, 'POST', { decision, note:'E2E exact non-mutating statement verified' });
      return get();
    }
    detail = await query(good);
    assert.ok(toolResults(detail).some(m => m.parsed.Kind === 'table' || m.parsed.Content?.rows));
    assert.equal(detail.turns.at(-1).status, 3); checks.push('approved query and model summary');
    const before = toolResults(detail).length;
    detail = await query(good, 'deny');
    assert.equal(detail.approvals.at(-1).status === 0, false);
    assert.ok(toolResults(detail).slice(before).every(m => m.parsed.IsError || !m.parsed.Content?.rows));
    checks.push('rejected query does not execute');
    const invalid = protocol === 'redis' ? 'LIAISON_E2E_UNKNOWN_COMMAND' : protocol === 'mongodb' ? '{"liaison_e2e_unknown_command":1}' : 'SELECT * FROM liaison_e2e_missing_table_734';
    detail = await query(invalid);
    assert.ok(toolResults(detail).some(m => m.parsed.IsError && m.parsed.Content?.error));
    assert.equal(detail.turns.at(-1).status, 3); checks.push('database error explained without runtime failure');
    const object = 'liaison_agent_e2e_' + Date.now();
    const native = statement => api('/api/v1/webdata/sessions/'+connection.token+'/execute','POST',{statement});
    // Only uniquely-created E2E objects are mutated or removed.
    const cleanup = protocol === 'redis' ? 'DEL '+object : protocol === 'mongodb' ? JSON.stringify({drop:object}) : 'DROP TABLE '+object;
    let created = false;
    try {
      const create = protocol === 'redis' ? 'SET '+object+' initial EX 300' : protocol === 'mongodb' ? JSON.stringify({insert:object,documents:[{value:'initial'}]}) : 'CREATE TABLE '+object+' (value VARCHAR(32))';
      // Cleanup remains safe even if the result is lost after remote execution.
      created = true; detail = await query(create);
      if (protocol !== 'redis' && protocol !== 'mongodb') await query("INSERT INTO "+object+" (value) VALUES ('initial')");
      const update = protocol === 'redis' ? 'SET '+object+' verified EX 300' : protocol === 'mongodb' ? JSON.stringify({update:object,updates:[{q:{value:'initial'},u:{$set:{value:'verified'}}}]}) : "UPDATE "+object+" SET value='verified' WHERE value='initial'";
      await query(update);
      const read = protocol === 'redis' ? 'GET '+object : protocol === 'mongodb' ? JSON.stringify({find:object,filter:{},limit:1}) : 'SELECT value FROM '+object+' LIMIT 1';
      const result = await native(read); assert.ok(JSON.stringify(result).includes('verified')); checks.push('approved create/insert/update verified on native connection');
      const type = protocol === 'mongodb' ? 'collection' : protocol === 'redis' ? 'key' : 'table';
      const objectPath = [type,protocol === 'redis' ? String(credential.redis_db||0) : credential.database||'',protocol === 'postgresql' ? credential.schema||'public' : '',object];
      await api(path+'/turns','POST',{prompt:'只使用 data.schema 工具查看这个对象详情，path 参数必须是 '+JSON.stringify(objectPath)+'。不要执行查询，简短概括索引/字段/TTL信息。'});
      detail = await get();
      assert.ok(toolResults(detail).some(m=>m.parsed.Content?.object_type===type)); checks.push('object details via schema tool');
    } finally { if (created) await native(cleanup); }
    const editor = 'e2e-' + Date.now();
    try {
      const draft = protocol === 'redis' ? 'GET ' : protocol === 'mongodb' ? '{"find":' : 'SELECT ';
      const suggestion = await api('/api/v1/assistance/suggestions', 'POST', {handle_id:connection.token,editor_id:editor,revision:1,text:draft,cursor:Buffer.byteLength(draft)});
      assert.equal(suggestion.revision, 1); assert.equal(typeof suggestion.text, 'string');
      const after = await get(); assert.equal(after.turns.length, detail.turns.length); checks.push('isolated draft completion');
    } finally { await api('/api/v1/assistance/suggestions','DELETE',{handle_id:connection.token,editor_id:editor}); }
    await api('/api/v1/webdata/sessions/' + connection.token, 'DELETE'); connection = undefined;
    let rejected = false;
    try { await api(path+'/turns','POST',{prompt:'再次读取数据结构'}); } catch { rejected = true; }
    assert.ok(rejected, 'Closed handle must reject execution'); checks.push('disconnected handle rejected');
    return { protocol, checks, passed: true };
  } finally {
    if (session) {
      const path = '/api/v1/agent/sessions/'+session.session.id;
      const current = await api(path);
      await api(path,'DELETE',{version:current.session.version});
    }
    if (connection) await api('/api/v1/webdata/sessions/'+connection.token,'DELETE');
  }
};

if (require.main === module) {
  const https = require('node:https');
  const request = { fetch(url, options) {
    return new Promise((resolve, reject) => {
      const data = options.data === undefined ? '' : JSON.stringify(options.data);
      const req = https.request(url, {method:options.method,rejectUnauthorized:false,headers:{...options.headers,'Content-Type':'application/json','Content-Length':Buffer.byteLength(data)}}, response => {
        let body=''; response.on('data',part=>body+=part); response.on('end',()=>resolve({ok:()=>response.statusCode<400,status:()=>response.statusCode,json:async()=>JSON.parse(body)}));
      });
      req.setTimeout(options.timeout,()=>req.destroy(Error('Request timeout'))); req.on('error',reject); req.end(data);
    });
  }};
  (async()=>{
    if (!process.env.E2E_BASE_URL) throw Error('Set E2E_BASE_URL to an isolated test deployment');
    let password=''; for await(const part of process.stdin) password+=part;
    const protocols=process.argv.slice(2);
    for (const protocol of protocols.length ? protocols : ['mysql','mariadb','postgresql','mongodb','redis']) {
      try { console.log(JSON.stringify(await exports.run(request,process.env.E2E_BASE_URL,process.env.E2E_EMAIL||'default@liaison.com',password.trim(),protocol))); }
      catch(error) {console.error(JSON.stringify({protocol,passed:false,error:error.message,stack:error.stack}));process.exitCode=1;}
    }
  })();
}
