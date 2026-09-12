// Explicit staging only. Protocol fixture must run behind the selected connector.
// Creates and removes its own applications/accesses/user/keys. Never logs secrets.
const assert = require('node:assert/strict');
const { randomBytes } = require('node:crypto');
const { request, chromium } = require(process.env.PLAYWRIGHT_MODULE ||
  'playwright');

(async () => {
  const baseURL = process.env.E2E_BASE_URL;
  assert(
    baseURL &&
      process.env.E2E_AI_EDGE_ID &&
      process.env.E2E_AI_HOST &&
      process.env.E2E_AI_PORT,
  );
  const req = await request.newContext({
    baseURL,
    ignoreHTTPSErrors: true,
    timeout: 45000,
  });
  let token, browser, peerID;
  const apps = [],
    proxies = [];
  let checks = 0;
  const pass = (s) => {
    checks++;
    console.log('PASS', s);
  };
  const api = async (path, method = 'GET', data) => {
    const response = await req.fetch(path, {
      method,
      data,
      headers: token ? { Authorization: `Bearer ${token}` } : {},
    });
    const value = await response.json();
    assert(
      response.ok() && value.code >= 200 && value.code < 300,
      `${method} ${path}: ${response.status()}`,
    );
    return value.data;
  };
  try {
    token = (
      await api('/api/v1/iam/login', 'POST', {
        email: process.env.E2E_EMAIL,
        password: process.env.E2E_PASSWORD,
      })
    ).token;
    assert(token);
    let firstProxy, firstKey;
    for (const protocol of ['openai-compatible', 'anthropic']) {
      const app = await api('/api/v1/applications', 'POST', {
        name: 'AI protocol fixture ' + Date.now(),
        ip: process.env.E2E_AI_HOST,
        port: Number(process.env.E2E_AI_PORT),
        application_type: 'llm',
        edge_id: Number(process.env.E2E_AI_EDGE_ID),
      });
      apps.push(app.id);
      const appPath = `/api/v1/ai/applications/${app.id}`;
      const config = await api(appPath, 'PUT', {
        protocol,
        base_path: protocol === 'anthropic' ? '/anthropic' : '/v1',
        tls: false,
        api_key: 'fixture-not-a-real-secret',
      });
      assert(config.has_api_key);
      assert(!config.api_key);
      pass(protocol + ' write-only credential');
      const probe = await api(appPath + '/probe', 'POST');
      assert.equal(probe.state, 'compatible');
      assert(probe.models.includes('fixture-chat'));
      pass(protocol + ' metadata through real connector');
      const proxy = await api('/api/v1/proxies', 'POST', {
        name: 'AI API fixture ' + protocol,
        application_id: app.id,
        access_protocol: 'aiapi',
        port: 0,
        expose_public_port: false,
      });
      proxies.push(proxy.id);
      assert.equal(proxy.port, 0);
      const path = `/api/v1/ai/accesses/${proxy.id}`;
      await api(path, 'PUT', {
        enabled: true,
        models: { chat: 'fixture-chat' },
        external_protocol: 'openai-compatible',
      });
      const workspace = await api(path + '/workspace');
      assert.deepEqual(workspace.models, ['chat']);
      assert(!JSON.stringify(workspace).includes('fixture-chat'));
      pass(protocol + ' consumer workspace exposes aliases only');
      const key = await api(path + '/keys', 'POST', {
        name: 'E2E scoped key',
        models: ['chat'],
        expires_in_days: 1,
      });
      assert(key.secret);
      const keys = await api(path + '/keys');
      assert(keys.every((k) => !k.secret));
      const auth = { Authorization: 'Bearer ' + key.secret };
      const list = await req.get(path + '/v1/models', { headers: auth });
      assert(list.ok());
      assert.deepEqual(
        (await list.json()).data.map((m) => m.id),
        ['chat'],
      );
      pass(protocol + ' scoped model discovery');
      const noAuth = await req.get(path + '/v1/models');
      assert.equal(noAuth.status(), 401);
      const jwt = await req.get(path + '/v1/models', {
        headers: { Authorization: 'Bearer ' + token },
      });
      assert.equal(jwt.status(), 401);
      const denied = await req.post(path + '/v1/chat/completions', {
        headers: auth,
        data: {
          model: 'fixture-chat',
          messages: [{ role: 'user', content: 'hello' }],
        },
      });
      assert.equal(denied.status(), 403);
      pass('virtual key only and model alias enforcement');
      assert.equal((await req.get(path+'/v1/models?target=other', {headers:auth})).status(),400);
      assert.equal((await req.post(path+'/v1/responses', {headers:auth,data:{}})).status(),405);
      assert.equal((await req.post(path+'/v1/chat/completions',{headers:auth,data:' '.repeat((1<<20)+1)})).status(),413);
      pass('query overrides, unsupported endpoints and oversized bodies rejected');
      for (const stream of [false, true]) {
        const response = await req.post(path + '/v1/chat/completions', {
          headers: auth,
          data: {
            model: 'chat',
            messages: [{ role: 'user', content: 'hello' }],
            max_tokens: 200,
            stream,
          },
        });
        const text = await response.text();
        assert(response.ok(), text);
        assert(response.headers()['x-request-id']);
        assert(text.includes('connector forwarding verified'));
        assert(!text.includes('fixture-chat'));
        if (stream) assert(text.includes('[DONE]'));
        else assert.equal(JSON.parse(text).model, 'chat');
        pass(
          protocol +
            (stream ? ' streaming' : ' JSON') +
            ' conversion and model alias',
        );
      }
      if (protocol === 'anthropic') {
        const invalid = await req.post(path + '/v1/chat/completions', {
          headers: auth,
          data: {
            model: 'chat',
            messages: [{ role: 'user', content: 'hi' }],
            tools: [],
          },
        });
        assert.equal(invalid.status(), 400);
        pass('unsupported conversion fields rejected');
      }
      await api(path, 'PUT', {
        enabled: false,
        models: { chat: 'fixture-chat' },
      });
      assert.equal(
        (await req.get(path + '/v1/models', { headers: auth })).status(),
        503,
      );
      await api(path, 'PUT', {
        enabled: true,
        models: { chat: 'fixture-chat' },
      });
      pass('disable and enable access');
      const records = await api(path + '/requests');
      assert(
        records.some(
          (r) => r.complete && r.key_id === key.id && r.input_tokens === 7 && r.output_tokens === 9,
        ),
      );
      assert(records.every((r) => !r.prompt && !r.response));
      pass('bounded metadata audit, no conversation contents');
      // Quotas use lifetime settled input+output, independent of log retention.
      const metered = (await api(path + '/keys')).find(k => k.id === key.id);
      assert(metered.used_tokens >= 32);
      await api(path + '/keys/' + key.id + '/quota', 'PUT', {token_limit: metered.used_tokens});
      const exhausted = await req.post(path + '/v1/chat/completions', {headers:auth,data:{model:'chat',messages:[{role:'user',content:'quota'}]}});
      assert.equal(exhausted.status(),429);
      assert.equal((await exhausted.json()).error.code,'TOKEN_QUOTA_EXHAUSTED');
      for (const response of await Promise.all(Array.from({length:4},()=>req.post(path+'/v1/chat/completions',{headers:auth,data:{model:'chat',messages:[{role:'user',content:'quota'}]}})))) {
        assert.equal(response.status(),429);
      }
      await api(path + '/keys/' + key.id + '/quota', 'PUT', {token_limit: metered.used_tokens+1});
      const lastCall = await req.post(path + '/v1/chat/completions',{headers:auth,data:{model:'chat',messages:[{role:'user',content:'quota'}],stream:true}});
      assert.equal(lastCall.status(),200);
      assert((await lastCall.text()).includes('[DONE]'));
      let settled;
      for (let attempt=0;attempt<20;attempt++) {
        settled=(await api(path+'/keys')).find(k=>k.id===key.id);
        if(settled.used_tokens>metered.used_tokens) break;
        await new Promise(resolve=>setTimeout(resolve,100));
      }
      assert(settled.used_tokens>settled.token_limit);
      assert.equal(settled.remaining_tokens,0);
      await api(path + '/keys/' + key.id + '/quota', 'PUT', {token_limit:null});
      const unlimited=(await api(path+'/keys')).find(k=>k.id===key.id);
      assert.equal(unlimited.token_limit,null);
      assert.equal(unlimited.used_tokens,settled.used_tokens);
      pass(protocol + ' lifetime quota, concurrent rejection, streaming settlement and adjustment');
      const unknownKey=await api(path+'/keys','POST',{name:'Unreported usage fixture',models:['chat'],expires_in_days:1,token_limit:1000});
      const unknownAuth={Authorization:'Bearer '+unknownKey.secret};
      const unreported=await req.post(path+'/v1/chat/completions',{headers:unknownAuth,data:{model:'chat',messages:[{role:'user',content:'missing-usage'}]}});
      assert.equal(unreported.status(),200);
      await unreported.text();
      for(let attempt=0;attempt<20;attempt++) {
        const k=(await api(path+'/keys')).find(k=>k.id===unknownKey.id);
        if(k.unknown_requests>0) break;
        await new Promise(resolve=>setTimeout(resolve,100));
      }
      const deniedUnknown=await req.post(path+'/v1/chat/completions',{headers:unknownAuth,data:{model:'chat',messages:[{role:'user',content:'quota'}]}});
      assert.equal(deniedUnknown.status(),429);
      assert.equal((await deniedUnknown.json()).error.code,'TOKEN_USAGE_UNCONFIRMED');
      await api(path+'/keys/'+unknownKey.id,'DELETE');
      pass(protocol+' missing usage pauses limited key');
      if (!firstProxy) {
        firstProxy = proxy;
        firstKey = key;
      } else {
        await api(path + '/keys/' + key.id, 'DELETE');
        assert.equal(
          (await req.get(path + '/v1/models', { headers: auth })).status(),
          401,
        );
        pass('key revocation');
      }
    }
    const organizations = (await api('/api/v1/iam/organizations'))
      .organizations;
    const organization =
      organizations.find((o) => o.is_root) || organizations[0];
    assert(organization);
    const email = 'ai-e2e-' + Date.now() + '@example.test',
      password = 'Ai!' + randomBytes(20).toString('base64url');
    peerID = (
      await api('/api/v1/iam/users', 'POST', {
        organization_id: organization.id,
        email,
        password,
        role: 'user',
        name: 'Temporary AI isolation test',
      })
    ).user.id;
    const peer = (await api('/api/v1/iam/login', 'POST', { email, password }))
      .token;
    for (const suffix of ['', '/workspace', '/keys', '/requests', '/usage']) {
      const response = await req.get(
        `/api/v1/ai/accesses/${firstProxy.id}${suffix}`,
        { headers: { Authorization: 'Bearer ' + peer } },
      );
      assert([403, 404].includes(response.status()));
    }
    pass(
      'separate user cannot read access configuration, keys or request audit',
    );
    browser = await chromium.launch();
    const context = await browser.newContext({
      ignoreHTTPSErrors: true,
      viewport: { width: 1440, height: 1000 },
    });
    await context.addInitScript((t) => {
      localStorage.setItem('token', t);
      if (!localStorage.getItem('liaison-locale'))
        localStorage.setItem('liaison-locale', 'zh-CN');
      localStorage.setItem('liaison-theme-preference', 'dark');
    }, token);
    const page = await context.newPage();
    const errors = [];
    page.on('pageerror', (e) => errors.push(e.message));
    await page.goto(baseURL + '/ai/' + firstProxy.id);
    await page.locator('.ai-api-tabs').waitFor();
    assert.equal(await page.getByText('正在加载…', {exact: true}).count(), 0);
    await page.screenshot({path: '/tmp/liaison-ai-api-overview.png', fullPage: true});
    await page.locator('.ai-api-tabs').getByRole('button', { name: '在线体验', exact: true }).click();
    await page.getByLabel('消息', { exact: true }).fill('hello');
    await page.getByRole('button', { name: '发送', exact: true }).click();
    await page
      .locator('.ai-api-answer')
      .filter({ hasText: 'connector forwarding verified' })
      .waitFor();
    await page.getByRole('button', { name: '停止', exact: true }).waitFor({state: 'hidden'});
    await page.locator('.ai-api-page header').scrollIntoViewIfNeeded();
    await page.screenshot({
      path: '/tmp/liaison-ai-api-desktop.png',
      fullPage: true,
    });
    assert(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    );
    assert.equal(errors.length, 0);
    pass('Chinese UI, actual stream rendering, no page errors');
    await page.setViewportSize({ width: 390, height: 844 });
    await page.waitForTimeout(300);
    assert(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    );
    assert(
      await page
        .locator('.ai-api-page')
        .evaluate((el) => el.getBoundingClientRect().width >= 260),
    );
    await page.screenshot({
      path: '/tmp/liaison-ai-api-mobile.png',
      fullPage: true,
    });
    pass('mobile layout without page overflow');
    await page.evaluate(() => localStorage.setItem('liaison-locale', 'en-US'));
    await page.reload();
    await page.getByRole('button', { name: 'Playground', exact: true }).click();
    pass('English UI');
    await page.getByLabel('Message',{exact:true}).fill('slow-fixture');
    await page.getByRole('button',{name:'Send',exact:true}).click();
    await page.locator('.ai-api-answer').filter({hasText:'connector forwarding verified'}).waitFor();
    await page.getByRole('button',{name:'Stop',exact:true}).click();
    await page.waitForFunction(()=>!Array.from(document.querySelectorAll('button')).find(b=>b.textContent==='Send')?.disabled);
    pass('UI Stop cancels inference and restores input controls');
    const path = `/api/v1/ai/accesses/${firstProxy.id}`;
    const slowRequest = {
      headers: { Authorization: 'Bearer ' + firstKey.secret },
      data: {
        model: 'chat',
        messages: [{ role: 'user', content: 'slow-fixture' }],
        stream: true,
      },
    };
    const streaming = Array.from({length:8},()=>req.post(path+'/v1/chat/completions',slowRequest));
    await page.waitForTimeout(1200);
    const saturated = await req.post(path+'/v1/chat/completions',slowRequest);
    assert.equal(saturated.status(),429);
    pass('concurrent request safety limit rejects overflow');
    await api(path + '/keys/' + firstKey.id, 'DELETE');
    for(const stopped of await Promise.all(streaming)){
      const text = await stopped.text();
      assert(!text.includes('[DONE]'));
      assert(text.includes('Stream interrupted'));
    }
    pass('revocation cancels all active streams');
    console.log(`AI gateway acceptance: ${checks} checks passed`);
  } finally {
    if (browser) await browser.close();
    if (peerID) await api('/api/v1/iam/users/' + peerID, 'DELETE');
    for (const id of proxies.reverse())
      await api('/api/v1/proxies/' + id, 'DELETE');
    for (const id of apps.reverse())
      await api('/api/v1/applications/' + id, 'DELETE');
    await req.dispose();
  }
})().catch((e) => {
  console.error(e.message);
  process.exitCode = 1;
});
