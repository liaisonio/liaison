const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const assert=require('node:assert/strict');
(async()=>{const browser=await chromium.launch();try{
 for(const locale of ['zh-CN','en-US'])for(const theme of ['light','dark']){
  const zh=locale==='zh-CN',context=await browser.newContext({viewport:{width:1440,height:1000}});
  await context.addInitScript(({locale,theme})=>{localStorage.setItem('liaison-locale',locale);localStorage.setItem('liaison-theme-preference',theme);},{locale,theme});
  let mode='empty';
  await context.route('**/api/v1/**',async route=>{
   const path=new URL(route.request().url()).pathname;let data=[];
   if(path.endsWith('/workspace'))data={name:'Zero usage',enabled:true,models:['alpha'],can_manage:true,external_protocol:'openai',external_protocols:['openai']};
   if(path.endsWith('/accesses/1'))data={models:{alpha:'alpha'},enabled:true};
   if(path.endsWith('/usage')){
    if(mode==='failed')return route.fulfill({status:500,json:{code:500}});
    data={summary:mode==='unknown'?{requests:1,unknown_requests:1}:mode==='zero'?{requests:1,unknown_requests:0,input_tokens:0,output_tokens:0}:{requests:0,unknown_requests:0},records:[]};
   }
   await route.fulfill({json:{code:200,data}});
  });
  const page=await context.newPage();await page.goto(process.env.E2E_UI_URL+'/e2e/ollama.html?workspace');
  await page.getByRole('button',{name:zh?'统计':'Statistics',exact:true}).click();
  const chart=page.locator('.ai-token-trend');await chart.waitFor();
  for(const hours of [1,6,24,168,720]){
   const label=hours<=24?(zh?`${hours} 小时`:`${hours}h`):(zh?`${hours/24} 天`:`${hours/24} days`);
   const loaded=page.waitForResponse(response=>new URL(response.url()).pathname.endsWith('/usage')&&new URL(response.url()).searchParams.get('hours')===String(hours));
   await page.getByRole('button',{name:label,exact:true}).click();await loaded;
   await chart.locator('.ai-token-point').first().waitFor();
   assert.equal(await chart.locator('.ai-token-line').count(),1);
   const dates=await chart.locator('.ai-token-point').evaluateAll(nodes=>nodes.map(n=>n.getAttribute('aria-label')));
   assert(dates.length>1);assert(dates.every(d=>d.endsWith(': 0 Token')));
   assert.equal(Date.parse(dates.at(-1).split(': 0')[0])-Date.parse(dates[0].split(': 0')[0]),hours*3600000);
  }
  for(const width of [1440,390]){await page.setViewportSize({width,height:1000});await page.screenshot({path:`/tmp/token-zero-${locale}-${theme}-${width}.png`,fullPage:true});assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));}
  const refresh=page.getByRole('button',{name:zh?'刷新用量':'Refresh usage',exact:true});
  mode='unknown';await refresh.click();await page.getByText(zh?'暂无已确认用量':'No confirmed usage yet',{exact:true}).waitFor();assert.equal(await chart.count(),1);assert.equal(await chart.locator('.ai-token-line,.ai-token-point').count(),0);
  await page.getByRole('button',{name:zh?'概览':'Overview',exact:true}).click();
  await page.getByRole('button',{name:zh?'统计':'Statistics',exact:true}).click();await chart.waitFor();
  await page.getByText(zh?'暂无已确认用量':'No confirmed usage yet',{exact:true}).waitFor();
  for(const width of [1440,390]){await page.setViewportSize({width,height:1000});await page.screenshot({path:`/tmp/token-unknown-${locale}-${theme}-${width}.png`,fullPage:true});}
  mode='failed';await refresh.click();await page.getByText(zh?'用量加载失败，请重试。':'Could not load usage. Please retry.',{exact:true}).waitFor();assert.equal(await chart.count(),0);
  mode='zero';await refresh.click();await chart.waitFor();assert.equal(await chart.locator('.ai-token-line').count(),1);
  await context.close();console.log('PASS zero trend ranges, unknown/error',locale,theme);
 }
}finally{await browser.close();}})().catch(e=>{console.error(e);process.exitCode=1});
