// Exercise the real AppLayout, including authentication and access-loading gaps.
const assert = require('node:assert/strict');
const {chromium} = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const origin = process.env.E2E_UI_URL || 'http://127.0.0.1:5303';
const access = {id:'a'.repeat(32),name:'Layout verification',kind:'codex',edge_id:1,installation_id:'b'.repeat(32),project:'/workspace/demo'};
const answer = 'Layout ready.\n\n' + '检查文件与模型的对应关系。 Verify the project configuration and request handling. '.repeat(12) + '\n\n```text\n' + 'long-output-'.repeat(100) + '\n```\n\n| File | Description |\n| --- | --- |\n| `src/example.ts` | Responsive preview |';
const session = {version:1,status:'ok',session_id:'c'.repeat(32),thread_id:'test-thread',project:access.project,title:'Layout verification',closed:true,running:false,messages:[{role:'user',text:'Check the layout. 检查布局。'},{role:'assistant',text:answer}],history_persistent:true};
const delay = ms => new Promise(resolve => setTimeout(resolve,ms));
(async()=>{
 const browser = await chromium.launch(process.env.CHROME_EXECUTABLE?{executablePath:process.env.CHROME_EXECUTABLE}:undefined);
 try {
  for(const locale of ['zh-CN','en-US'])for(const theme of ['light','dark'])for(const width of [1280,390]){
   const page=await browser.newPage({viewport:{width,height:900},ignoreHTTPSErrors:process.env.E2E_IGNORE_HTTPS_ERRORS==='1'});
   let failAccess=false,deny=false;const errors=[];
   page.on('pageerror',()=>errors.push('browser error'));
   await page.addInitScript(({locale,theme})=>{
    localStorage.setItem('token','synthetic-layout-test');
    localStorage.setItem('liaison-locale',locale);
    localStorage.setItem('liaison-theme-preference',theme);
    window.shellFlashes=0;
    window.observeFullPage=true;
    const sample=()=>{
     const query=new URLSearchParams(location.search);
     if(window.observeFullPage&&location.pathname==='/access/agents'&&query.get('access')&&query.get('view')==='full'){
      if([...document.querySelectorAll('.liaison-global-header,.liaison-sidebar,.liaison-page-header')].some(el=>el.getClientRects().length&&getComputedStyle(el).visibility!=='hidden'))window.shellFlashes++;
     }
     requestAnimationFrame(sample);
    };requestAnimationFrame(sample);
   },{locale,theme});
   await page.route('**/api/v1/**',async route=>{
    const path=new URL(route.request().url()).pathname;
    let data={};
    if(path==='/api/v1/iam/account'){await delay(450);data={id:1,name:'Layout tester'};}
    else if(path==='/api/v1/iam/permissions'){await delay(750);data={'ai.access.use':!deny};}
    else if(path.startsWith('/api/v1/agent-accesses/')){
     await delay(450);if(failAccess)return route.fulfill({status:503,json:{code:503,message:'Unavailable'}});data=access;
    }else if(path==='/api/v1/agent-accesses')data={items:[],total:0};
    else if(path==='/api/v1/edge-agents/connectors')data=[{id:1,name:'Test connector',device:'Test device',online:true}];
    else if(path==='/api/v1/edge-agents'){
     const {action}=route.request().postDataJSON();
     assert(['sessions','poll','transcript'].includes(action),'layout tests must not start or modify a session');
     data=action==='sessions'?{version:1,status:'ok',sessions_available:true,sessions:[{...session,updated_at:'2026-01-01T00:00:00Z'}]}:action==='transcript'?{version:1,status:'ok',history_pages:[]}:session;
    }
    await route.fulfill({json:{code:200,data}});
   });
   const url=`${origin}/access/agents?access=${access.id}&session=${session.session_id}&view=full`;
   const hidden=async()=>{
    await page.locator('.liaison-global-header').waitFor({state:'hidden'});
    assert.equal(await page.locator('.liaison-global-header').isVisible(),false);
    assert.equal(await page.locator('.liaison-sidebar').isVisible(),false);
    assert.equal(await page.evaluate(()=>window.shellFlashes),0,'product shell was visible during full-page initialization');
   };
   await page.goto(url);
   await page.locator('.liaison-app-frame').waitFor();
   await hidden();
   await page.getByText('Layout ready.',{exact:true}).waitFor();
   await hidden();
   const responsive=async()=>{
    const sizes=await page.locator('.edge-agent-messages').evaluate(el=>{
     const css=getComputedStyle(el),message=el.querySelector('.agent-message:not(.is-user)');
     return {available:el.clientWidth-parseFloat(css.paddingLeft)-parseFloat(css.paddingRight),message:message.getBoundingClientRect().width,overflow:el.scrollWidth-el.clientWidth,pageOverflow:document.documentElement.scrollWidth-innerWidth};
    });
    assert(Math.abs(sizes.message-sizes.available)<2,JSON.stringify(sizes));
    assert(sizes.overflow<=1&&sizes.pageOverflow<=1,'conversation must not overflow horizontally');
    return sizes.message;
   };
   await responsive();
   if(width===1280){
    await page.setViewportSize({width:1920,height:900});
    const wide=await responsive();
    await page.screenshot({path:`/tmp/agent-wide-${locale}-${theme}.png`});
    await page.setViewportSize({width:1280,height:900});
    assert(wide>await responsive()+500,'messages should grow with the conversation panel');
   }
   await page.screenshot({path:`/tmp/agent-full-page-${locale}-${theme}-${width}.png`});
   const inputMounted=await page.locator('.edge-agent-workspace').evaluate(el=>{el.dataset.mountCheck='preserved';return true;});assert(inputMounted);
   // This recorder covers document bootstrap, not the intentional normal view
   // before React commits a user-initiated transition. Reload resets recording.
   await page.evaluate(()=>{window.observeFullPage=false;});
   await page.locator('[data-page-expand]').click();
   await page.locator('.liaison-global-header').waitFor({state:'visible'});
   await responsive();
   assert(await page.locator('.liaison-global-header').isVisible());
   assert.equal(await page.locator('.edge-agent-workspace').getAttribute('data-mount-check'),'preserved');
   await page.screenshot({path:`/tmp/agent-normal-page-${locale}-${theme}-${width}.png`});
   await page.locator('[data-page-expand]').click();await hidden();
   await page.reload();await page.getByText('Layout ready.',{exact:true}).waitFor();await hidden();
   failAccess=true;await page.reload();
   await page.getByRole('button',{name:locale==='zh-CN'?'重试':'Retry',exact:true}).waitFor();await hidden();
   await page.screenshot({path:`/tmp/agent-full-page-error-${locale}-${theme}-${width}.png`});
   failAccess=false;await page.getByRole('button',{name:locale==='zh-CN'?'重试':'Retry',exact:true}).click();await page.getByText('Layout ready.',{exact:true}).waitFor();await hidden();
   deny=true;await page.reload();await page.getByRole('alert').waitFor();await hidden();
   deny=false;await page.goto(`${origin}/access/agents?view=full`);await page.locator('.liaison-global-header').waitFor();assert(await page.locator('.liaison-sidebar').isVisible());
   assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));assert.deepEqual(errors,[]);
   console.log(`PASS full-page bootstrap/reload/error/retry/permission/list/exit ${locale} ${theme} ${width}`);
   await page.close();
   if(process.env.E2E_FULLPAGE_QUICK==='1')return;
  }
 }finally{await browser.close();}
})().catch(error=>{console.error(error);process.exitCode=1;});
