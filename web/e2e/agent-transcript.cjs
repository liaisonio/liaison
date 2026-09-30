const assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
(async()=>{
 const browser=await chromium.launch(process.env.CHROME_EXECUTABLE?{executablePath:process.env.CHROME_EXECUTABLE}:undefined);
 try{
  for(const locale of ['zh-CN','en-US'])for(const dark of [false,true])for(const width of [1280,390]){
   const page=await browser.newPage({viewport:{width,height:900}});const errors=[];const calls=[];let fail=false,delay=0,byteLimit=20;
   page.on('pageerror',e=>errors.push(e.message));
   await page.addInitScript(({locale,dark})=>{localStorage.setItem('liaison-locale',locale);document.addEventListener('DOMContentLoaded',()=>{document.documentElement.className=dark?'dark':'light';});},{locale,dark});
   await page.route('**/api/v1/edge-agents',async route=>{
    const body=route.request().postDataJSON();calls.push(body);
    if(delay)await new Promise(resolve=>setTimeout(resolve,delay));
    if(fail){fail=false;return route.fulfill({status:503,json:{message:'Unavailable'}});}
    const pages=[];for(let i=Number(body.history_before)-1;i>=0&&pages.length<Math.min(byteLimit,Number(body.history_limit));i--)pages.push({version:1,status:'ok',session_id:body.session_id,window:i,closed:true,running:false,messages:[{role:'user',text:`Round ${i}`},{role:'assistant',text:`Answer ${i}\n\nA completed round with **Markdown** and a safe command: \`go test ./...\`.`}]});
    await route.fulfill({json:{data:{version:1,status:'ok',session_id:body.session_id,history_pages:pages,history_before:pages.at(-1)?.window?String(pages.at(-1).window):''}}});
   });
   const zh=locale==='zh-CN';const earlier=()=>page.getByRole('button',{name:zh?'加载更早记录':'Load earlier messages',exact:true});
   const count=async n=>page.waitForFunction(n=>document.querySelectorAll('.agent-message.is-user').length===n,n);
   await page.goto((process.env.E2E_UI_URL||'http://127.0.0.1:5298')+'/e2e/agent-transcript.html');
   await count(20);assert.equal(calls.length,1);assert.equal(calls[0].history_limit,'19');
   const transcript=page.locator('.edge-agent-messages');
   assert(await transcript.evaluate(e=>e.scrollWidth<=e.clientWidth+1),'Running long command cannot widen transcript');
   assert(await page.locator('.edge-agent-activity li.has-detail > details').last().evaluate(el=>el.open),'Running command details are automatically visible');
   assert(await transcript.evaluate(e=>e.scrollWidth<=e.clientWidth+1),'Expanded command output cannot widen transcript');
   await page.screenshot({path:`/tmp/agent-running-width-${locale}-${dark}-${width}.png`,fullPage:true});
   assert.equal((await page.locator('.agent-message.is-user').first().innerText()).split('\n').at(-1),'Round 26');
   await page.locator('.edge-agent-messages').evaluate(el=>{el.scrollTop=0;});
   const anchor=page.locator('.agent-message.is-user').filter({hasText:'Round 26'});const y=(await anchor.boundingBox()).y;
   await earlier().click();await count(40);assert.equal(calls.at(-1).history_limit,'20');
   assert(Math.abs((await anchor.boundingBox()).y-y)<4,'Prepending history must preserve the reading position');
   await page.locator('.edge-agent-messages').evaluate(el=>{el.scrollTop=0;});
   fail=true;await earlier().click();await page.getByRole('button',{name:zh?'重试':'Retry',exact:true}).waitFor();assert.equal(await page.locator('.agent-message.is-user').count(),40);
   await page.getByRole('button',{name:zh?'重试':'Retry',exact:true}).click();await count(46);assert.equal(await earlier().count(),0);
   await page.getByRole('button',{name:'Next round',exact:true}).click();await count(47);
   await page.waitForTimeout(250);assert.equal(await page.locator('.agent-message.is-user').count(),47,'Live rollover must not replace earlier messages');
   assert.equal(await earlier().count(),0);
   await page.screenshot({path:`/tmp/agent-continuous-${locale}-${dark}-${width}.png`,fullPage:true});
   assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));
   await page.getByRole('button',{name:'Remote rounds',exact:true}).click();
   await page.getByRole('button',{name:zh?'加载中间记录':'Load missing messages',exact:true}).click();await count(72);
   assert.equal(await page.getByRole('button',{name:zh?'加载中间记录':'Load missing messages',exact:true}).count(),0);
   await page.reload();await count(20);assert.equal(calls.at(-1).history_limit,'19');
   byteLimit=4;await page.reload();await count(5);byteLimit=20;
   await page.locator('.edge-agent-messages').evaluate(el=>{el.scrollTop=0;});await earlier().click();await count(25);
   // Ignore a delayed history response after navigating to another session.
   delay=400;await page.locator('.edge-agent-messages').evaluate(el=>{el.scrollTop=0;});await earlier().click();
   await page.getByRole('button',{name:'Switch session',exact:true}).click();await count(1);await page.waitForTimeout(600);await count(1);
   assert.deepEqual(errors,[]);await page.close();
  }
  console.log('PASS continuous history: 20-round initial load, cursor batches, stable scroll, retry, rollover, reload and session isolation across 8 UI combinations');
 }finally{await browser.close();}
})().catch(e=>{console.error(e);process.exitCode=1;});
