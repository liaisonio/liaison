const assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
(async()=>{
 const browser=await chromium.launch(process.env.CHROME_EXECUTABLE?{executablePath:process.env.CHROME_EXECUTABLE}:undefined);
 try {for(const locale of ['zh-CN','en-US'])for(const theme of ['light','dark'])for(const width of [1440,390]){
  const page=await browser.newPage({viewport:{width,height:900}});const zh=locale==='zh-CN';let fail=false,withConnector=true,writes=0;const errors=[];
  page.on('pageerror',e=>errors.push(e.message));
  await page.addInitScript(({locale,theme})=>{localStorage.setItem('liaison-locale',locale);localStorage.setItem('liaison-theme-preference',theme);document.addEventListener('DOMContentLoaded',()=>{document.documentElement.classList.add(theme);});},{locale,theme});
  await page.route('**/api/v1/**',async route=>{
   const path=new URL(route.request().url()).pathname;if(route.request().method()!=='GET')writes++;
   if(fail&&(/proxies$|agent-accesses$/.test(path)))return route.fulfill({status:403,json:{code:403,message:'Forbidden'}});
   let data={};
   if(path.endsWith('/proxies'))data={proxies:[]};
   if(path.endsWith('/applications'))data={applications:[{id:1,name:'Local model',application_type:'openai'}]};
   if(path.endsWith('/edges'))data={edges:[{id:1,name:'Connector'}]};
   if(path.endsWith('/agent-accesses'))data={items:[],total:0};
   if(path.endsWith('/connectors'))data=withConnector?[{id:1,name:'Connector',online:true}]:[];
   await route.fulfill({json:{code:200,data}});
  });
  const base=process.env.E2E_UI_URL;
  for(const category of ['web','ssh','database','cache','storage','desktop','tcp','llm','agent']){
   const url=category==='agent'?`${base}/e2e/edge-agent.html`:`${base}/e2e/product-polish.html?entry=${encodeURIComponent('/proxy?category='+category)}`;
   await page.goto(url);
   const empty=page.locator('.liaison-access-empty');await empty.getByRole('button',{name:zh?'新建访问':'Create access',exact:true}).waitFor();
   assert.equal(await page.locator('.liaison-list-header').getByRole('button',{name:zh?'新建访问':'Create access',exact:true}).count(),0);
   if(category==='llm'){
    assert.equal(await empty.locator('details').count(),0);
    const help=empty.getByRole('button',{name:zh?'与助理模型有什么区别？':'How is this different from assistant models?'});
    await help.focus();await page.getByRole('tooltip').waitFor();await page.keyboard.press('Escape');await page.getByRole('tooltip').waitFor({state:'hidden'});
    await help.click();await page.getByRole('tooltip').waitFor();await page.screenshot({path:`/tmp/access-help-${locale}-${theme}-${width}.png`});await page.keyboard.press('Tab');
   }
   await page.screenshot({path:`/tmp/access-empty-${category}-${locale}-${theme}-${width}.png`,fullPage:true});
   assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),'No horizontal overflow');
   await empty.getByRole('button',{name:zh?'新建访问':'Create access',exact:true}).click();await page.getByRole('dialog').waitFor();await page.getByRole('button',{name:zh?'取消':'Cancel',exact:true}).click();
   await page.locator('.liaison-compound input').first().fill('missing');await empty.getByRole('button',{name:zh?'重置筛选':'Reset filters'}).waitFor();
   assert.equal(await empty.getByRole('button',{name:zh?'新建访问':'Create access',exact:true}).count(),0);
   await empty.getByRole('button',{name:zh?'重置筛选':'Reset filters'}).click();await empty.getByRole('button',{name:zh?'新建访问':'Create access',exact:true}).waitFor();
  }
  withConnector=false;await page.reload();await page.locator('.liaison-access-empty').getByRole('button',{name:zh?'创建连接器':'Create connector'}).waitFor();
  fail=true;await page.reload();await page.getByText(zh?'无法加载 Agent 访问，请检查权限或重试。':'Cannot load Agent access. Check permissions or retry.').waitFor();assert.equal(await page.locator('.liaison-access-empty').count(),0);
  await page.goto(`${base}/e2e/product-polish.html?entry=${encodeURIComponent('/proxy?category=web')}`);await page.getByText('Forbidden',{exact:true}).waitFor();assert.equal(await page.locator('.liaison-access-empty').count(),0);assert.equal(await page.getByRole('button',{name:zh?'新建访问':'Create access',exact:true}).count(),0);
  assert.equal(writes,0);assert.deepEqual(errors,[]);console.log('PASS',locale,theme,width,'all access categories, create, filters, help, permissions');await page.close();
 }}finally{await browser.close();}
})().catch(e=>{console.error(e);process.exitCode=1;});
