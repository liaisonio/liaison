const assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
(async()=>{
 const browser=await chromium.launch();
 try {
  const page=await browser.newPage({viewport:{width:1000,height:1000}});
  const errors=[];page.on('pageerror',e=>errors.push(e.message));
  await page.goto((process.env.UI_URL||'http://127.0.0.1:8017')+'/e2e/markdown.html');
  await page.locator('h1').waitFor();
  assert.equal(await page.locator('h2').count(),1);
  assert.equal(await page.locator('ul ul').count(),1);
  assert.equal(await page.locator('ol li').count(),2);
  assert.equal(await page.locator('blockquote').count(),1);
  assert.equal(await page.locator('table tbody td').count(),2);
  assert.equal(await page.locator('.agent-code').count(),2);
  assert.equal(await page.locator('img, a[href^="javascript:"]').count(),0);
  assert.equal(await page.locator('#stream code').innerText(),'SELECT');
  await page.screenshot({path:'/tmp/liaison-markdown.png'});
  await page.setViewportSize({width:390,height:844});
  assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
  assert.deepEqual(errors,[]);
  console.log('PASS headings, nested lists, tables, code, incomplete stream, HTML/URL/image safety, mobile width');
 }finally{await browser.close();}
})().catch(e=>{console.error(e);process.exitCode=1;});
