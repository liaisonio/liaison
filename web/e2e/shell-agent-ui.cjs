const assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
(async()=>{
 const browser=await chromium.launch();
 try{
  const page=await browser.newPage({viewport:{width:1280,height:760}});const errors=[];
  page.on('pageerror',e=>errors.push(e.message));
  await page.goto((process.env.UI_FIXTURE_URL||'http://127.0.0.1:8017')+'/e2e/shell-agent-ui.html');
  await page.getByLabel('Shell Agent',{exact:true}).waitFor();
  assert.equal(await page.getByText('Shell Agent',{exact:true}).count(),1);
  assert.equal(await page.getByText('AI 命令提示',{exact:true}).count(),0);
  assert.equal(await page.locator('.webssh-completion.is-risky').count(),1);
  assert.equal(await page.locator('.command-risk').evaluate(e=>getComputedStyle(e).color),'rgb(243, 200, 107)');
  assert.equal(await page.locator('.typed').evaluate(e=>getComputedStyle(e).whiteSpace),'pre');
  await page.screenshot({path:'/tmp/liaison-shell-toolbar.png'});
  await page.getByRole('listbox',{name:'命令补全'}).getByRole('option').click();
  assert.equal(await page.getByLabel('Accepted candidate').innerText(),'rm -rf /tmp/obsolete-cache');
  await page.getByRole('button',{name:'切换候选'}).click();
  assert.equal(await page.locator('.webssh-completion.is-risky').count(),0);
  await page.getByRole('button',{name:'建议与诊断',exact:true}).click();
  assert(await page.getByRole('textbox',{name:'Shell 分析问题',exact:true}).isVisible());
  await page.getByRole('button',{name:'收起',exact:true}).click();
  await page.setViewportSize({width:390,height:844});
  assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
  await page.screenshot({path:'/tmp/liaison-shell-toolbar-mobile.png'});
  assert.deepEqual(errors,[]);
  console.log('PASS one toolbar, risk color/text, unchanged candidate, expandable analysis, mobile layout; no commands executed');
 }finally{await browser.close();}
})().catch(e=>{console.error(e);process.exitCode=1;});
