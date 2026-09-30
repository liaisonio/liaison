const assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
(async()=>{const browser=await chromium.launch();try{
 for(const locale of ['zh-CN','en-US'])for(const dark of [false,true])for(const width of [1280,390]){
  const page=await browser.newPage({viewport:{width,height:900}}),errors=[];let apps=[],accesses=[],discoveries=0;
  const zh=locale==='zh-CN',tr=(a,b)=>zh?a:b;
  page.on('pageerror',e=>errors.push(e.message));
  await page.addInitScript(({locale,dark})=>{localStorage.setItem('liaison-locale',locale);document.addEventListener('DOMContentLoaded',()=>{document.documentElement.classList.toggle('dark',dark);document.documentElement.classList.toggle('light',!dark);});},{locale,dark});
  await page.route('**/api/v1/edge-agents**',route=>{if(route.request().method()==='GET')return route.fulfill({json:{data:[{id:6,name:'Mac mini',device:'Mac-mini.local',online:true}]}});assert.equal(route.request().postDataJSON().action,'discover','Registration never starts a session');discoveries++;return route.fulfill({json:{data:{version:1,status:'ok',installations:[{id:'a'.repeat(32),kind:'codex',path:'/opt/homebrew/bin/codex'}]}}});});
  await page.route('**/api/v1/agent-applications**',route=>{const req=route.request();let data;
   if(req.method()==='POST'){data={...req.postDataJSON(),id:'b'.repeat(32),access_count:0};apps=[data];}
   else if(req.method()==='PUT'){apps[0].name=req.postDataJSON().name;data=apps[0];}
   else if(req.method()==='DELETE'){assert.equal(apps[0].access_count,0);apps=[];data=null;}
   else data={items:apps,total:apps.length};return route.fulfill({json:{data}});
  });
  await page.route('**/api/v1/agent-accesses**',route=>{assert.equal(route.request().method(),'POST');const data={...route.request().postDataJSON(),id:'c'.repeat(32)};assert.equal(data.application_id,apps[0].id);accesses.push(data);apps[0].access_count++;return route.fulfill({json:{data}});});
  await page.goto((process.env.E2E_UI_URL||'http://127.0.0.1:5301')+'/e2e/agent-applications.html');
  await page.getByRole('button',{name:tr('新建应用','Create application'),exact:true}).click();
  await page.getByRole('textbox',{name:tr('应用名称','Application name')}).fill('Mac Codex');
  await page.getByRole('button',{name:tr('发现已安装的 Agent','Discover installed Agents')}).click();
  await page.getByRole('combobox',{name:tr('Agent 安装','Agent installation')}).waitFor();
  await page.screenshot({path:`/tmp/agent-app-registration-${locale}-${dark}-${width}.png`,fullPage:true});
  await page.getByRole('button',{name:tr('保存','Save'),exact:true}).click();
  await page.getByRole('cell',{name:'Mac Codex',exact:true}).waitFor();assert.equal(discoveries,1);
  for(let i=0;i<2;i++){
   await page.getByRole('button',{name:tr('创建访问','Create access'),exact:true}).click();
   const dialog=page.getByRole('dialog');
   assert.equal(await dialog.getByRole('combobox',{name:tr('应用来源','Application source')}).inputValue(),'existing');
   await dialog.getByRole('option',{name:'Mac Codex',exact:true}).waitFor({state:'attached'});
   await dialog.getByRole('textbox',{name:tr('项目目录','Project directory')}).fill('/Users/local/project');
   await dialog.getByRole('button',{name:tr('创建访问','Create access'),exact:true}).click();await dialog.waitFor({state:'hidden'});
  }
  assert.equal(discoveries,1,'Reusing an application does not discover or launch');assert.equal(accesses.length,2);
  assert(await page.getByRole('button',{name:tr('删除','Delete'),exact:true}).isDisabled());
  await page.getByRole('button',{name:tr('编辑','Edit'),exact:true}).click();
  await page.getByRole('textbox',{name:tr('应用名称','Application name')}).fill('Renamed Codex');
  await page.getByRole('button',{name:tr('保存','Save'),exact:true}).click();
  await page.getByRole('cell',{name:'Renamed Codex',exact:true}).waitFor();
  assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),'No document overflow');
  await page.screenshot({path:`/tmp/agent-app-list-${locale}-${dark}-${width}.png`,fullPage:true});
  assert.deepEqual(errors,[]);await page.close();
 }
 console.log('PASS 8 Agent application registration/reuse/rename/guard combinations');
}finally{await browser.close();}})().catch(e=>{console.error(e);process.exit(1)});
