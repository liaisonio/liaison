const assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
(async()=>{const browser=await chromium.launch();try{
 for(const locale of ['zh-CN','en-US'])for(const dark of [false,true])for(const width of [1280,390]){
  const page=await browser.newPage({viewport:{width,height:900}}),errors=[];
  const tr=(a,b)=>locale==='zh-CN'?a:b;
  page.on('pageerror',e=>errors.push(e.message));
  await page.addInitScript(({locale,dark})=>{localStorage.setItem('liaison-locale',locale);document.addEventListener('DOMContentLoaded',()=>{document.documentElement.classList.toggle('dark',dark);document.documentElement.classList.toggle('light',!dark);});},{locale,dark});
  const network={id:1,name:'Internal SSH',application_type:'ssh',ip:'127.0.0.1',port:22,edge_id:6,device:{name:'Mac mini'},created_at:'2026-09-26'};
  const agent={id:'a'.repeat(32),name:'Local Codex',kind:'codex',edge_id:6,installation_id:'b'.repeat(32),access_count:1};
  await page.route('**/api/v1/**',route=>{
   const path=new URL(route.request().url()).pathname;assert.equal(route.request().method(),'GET','No mutations while browsing');
   const data=path==='/api/v1/applications'?{applications:[network]}:path==='/api/v1/edges'?{edges:[{id:6,name:'Office',device:{name:'Mac mini'}}]}:path==='/api/v1/proxies'?{proxies:[]}:path==='/api/v1/agent-applications'?{items:[agent],total:1}:path==='/api/v1/edge-agents/connectors'?[{id:6,name:'Office',device:'Mac mini',online:true}]:{};
   return route.fulfill({json:{code:200,data}});
  });
  await page.goto((process.env.E2E_UI_URL||'http://127.0.0.1:5301')+'/e2e/application-tabs.html');
  await page.getByRole('cell',{name:'Internal SSH',exact:true}).waitFor();await page.getByRole('cell',{name:'Local Codex',exact:true}).waitFor();
  assert.equal(await page.locator('table').count(),1,'All applications is one table');
  assert(await page.getByRole('columnheader',{name:tr('应用类型','Application type'),exact:true}).isVisible());
  for(const name of ['Internal SSH','Local Codex'])assert.equal(await page.getByRole('cell',{name,exact:true}).locator('img,svg').count(),0,'No logo next to name');
  assert(await page.getByRole('cell',{name:'Codex',exact:true}).locator('img,svg').count()>0,'Logo in protocol column');
  assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));
  await page.screenshot({path:`/tmp/application-tabs-${locale}-${dark}-${width}.png`,fullPage:true});
  const search=page.getByPlaceholder(tr('输入应用名称','Application name'));await search.fill('Local Codex');
  await page.getByRole('cell',{name:'Internal SSH',exact:true}).waitFor({state:'hidden'});await page.getByRole('cell',{name:'Local Codex',exact:true}).waitFor();
  await page.getByRole('button',{name:tr('重置','Reset'),exact:true}).click();await page.getByRole('cell',{name:'Internal SSH',exact:true}).waitFor();
  await page.getByRole('button',{name:tr('新建应用','Create application'),exact:true}).click();
  await page.getByRole('combobox',{name:tr('应用分类','Application category')}).selectOption('agent');
  await page.getByRole('button',{name:tr('下一步','Next'),exact:true}).click();await page.getByRole('button',{name:tr('发现已安装的 Agent','Discover installed Agents')}).waitFor();
  await page.getByRole('button',{name:tr('取消','Cancel'),exact:true}).click();
  const category=async name=>{const button=page.getByRole('button',{name,exact:true});if(!await button.isVisible())await page.getByRole('button',{name:tr('更多协议','More protocols'),exact:true}).click();await page.getByRole('button',{name,exact:true}).click();};
  await category(tr('网络应用','Network applications'));
  await page.getByRole('cell',{name:'Internal SSH',exact:true}).waitFor();assert.equal(await page.getByRole('cell',{name:'Local Codex',exact:true}).count(),0);
  await category('Agent');await page.getByRole('cell',{name:'Local Codex',exact:true}).waitFor();assert.equal(await page.getByRole('cell',{name:'Internal SSH',exact:true}).count(),0);
  assert.deepEqual(errors,[]);await page.close();
 }
 console.log('PASS 8 application tabs, unified search, creation routing and protocol icon combinations');
}finally{await browser.close();}})().catch(e=>{console.error(e);process.exit(1)});
