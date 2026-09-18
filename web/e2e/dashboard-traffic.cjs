const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright'),assert=require('node:assert/strict');
(async()=>{const browser=await chromium.launch();try{for(const locale of ['zh-CN','en-US'])for(const theme of ['dark','light']){
const context=await browser.newContext({viewport:{width:1440,height:1000}});await context.addInitScript(({locale,theme})=>{localStorage.setItem('liaison-locale',locale);localStorage.setItem('liaison-theme-preference',theme);},{locale,theme});
let trafficCalls=0;
const sampledAt=Math.floor(Date.now()/60000)*60000;
await context.route('**/api/v1/**',route=>{const path=new URL(route.request().url()).pathname;if(path.includes('traffic'))trafficCalls++;let data={};if(path.includes('applications'))data={applications:[{id:1,name:'Peak',application_type:'http'},{id:2,name:'Steady',application_type:'http'}]};else if(path.includes('devices'))data={devices:[]};else if(path.includes('edges'))data={edges:[]};else if(path.includes('traffic'))data={metrics:[{application_id:1,timestamp:new Date(sampledAt-2*3600000).toISOString(),bytes_in:60000000,bytes_out:0},...[5,4,3,1].map((minute,i)=>({application_id:2,timestamp:new Date(sampledAt-minute*60000).toISOString(),bytes_in:((i===0||i===2?0:i+1))*6000,bytes_out:0}))]};return route.fulfill({json:{code:200,data}});});
const page=await context.newPage();page.on('pageerror',e=>console.log(e.message));await page.goto(process.env.E2E_UI_URL+'/e2e/dashboard-traffic.html');await page.screenshot({path:'/tmp/traffic-debug.png'});await page.locator('.overview-chart-legend button').first().waitFor();
assert.equal(trafficCalls,1,'Default range must issue only one traffic query');
const mix=page.locator('.overview-traffic-composition-list');assert((await mix.innerText()).includes('Steady'));assert(!(await mix.innerText()).includes('Peak'));
const chart=page.locator('.overview-chart');assert.equal(await chart.getByRole('button',{name:locale==='zh-CN'?'1 小时':'1h',exact:true}).getAttribute('aria-pressed'),'true');assert.equal(await chart.locator('.overview-chart-controls > span').count(),0);await chart.getByRole('button',{name:locale==='zh-CN'?'24 小时':'24h',exact:true}).click();await chart.getByRole('button',{name:'Peak',exact:true}).waitFor();assert.equal(await page.getByText('60.0 MB',{exact:true}).count(),1);assert((await mix.innerText()).startsWith('Peak'));assert.equal(await chart.locator('path').count(),3,'Gap splits steady series; isolated peak remains separate');assert((await chart.locator('path').allTextContents()).length>0);
await chart.getByRole('button',{name:'Peak',exact:true}).click();assert.equal(await chart.locator('circle').count(),1);await chart.getByRole('button',{name:locale==='zh-CN'?'1 小时':'1h',exact:true}).click();
await chart.getByRole('button',{name:'Peak',exact:true}).waitFor({state:'hidden'});assert.equal(trafficCalls,25,'Returning to fresh cached range must not refetch');assert(!(await mix.innerText()).includes('Peak'));
for(const width of [1440,390]){
 await page.setViewportSize({width,height:1000});
 const legend=await chart.locator('.overview-chart-legend').boundingBox();
 const controls=await chart.locator('.overview-chart-controls').boundingBox();
 const toolbar=await chart.locator('.overview-chart-toolbar').boundingBox();
 assert(Math.abs(legend.x-toolbar.x-8)<2,'Legend must align left');
 assert(Math.abs(controls.x+controls.width-(toolbar.x+toolbar.width-8))<2,'Time range must align right');
 if(width===1440)assert(legend.x+legend.width<=controls.x,'Desktop controls must follow legend');
 await page.screenshot({path:`/tmp/traffic-${locale}-${theme}-${width}.png`,fullPage:true});assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));
}
await chart.getByRole('button',{name:'Steady',exact:true}).click();await chart.getByText(locale==='zh-CN'?'所选范围暂无可见采样':'No visible samples in this range').waitFor();console.log('PASS time ranges, legends, gaps, responsive',locale,theme);await context.close();
}}finally{await browser.close();}})().catch(e=>{console.error(e);process.exitCode=1;});
