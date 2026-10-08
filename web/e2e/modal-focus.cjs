const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const assert=require('node:assert/strict');
(async()=>{
 const browser=await chromium.launch();
 try{for(const fullscreen of [false,true]){
  const page=await browser.newPage();
  await page.goto(`${process.env.E2E_UI_URL}/e2e/modal-focus.html`);
  if(fullscreen){await page.getByRole('button',{name:'Fullscreen',exact:true}).click();await page.waitForFunction(()=>document.fullscreenElement);}
  const opener=page.getByRole('button',{name:'Open dialog',exact:true});
  await opener.click();
  const dialog=page.getByRole('dialog').first();await dialog.waitFor();
  assert(await dialog.evaluate(el=>el.contains(document.activeElement)),'initial focus');
  if(fullscreen)assert(await dialog.evaluate(el=>document.fullscreenElement.contains(el)),'portal stays inside fullscreen host');
  for(const key of ['Tab','Tab','Tab','Tab','Shift+Tab','Shift+Tab','Shift+Tab','Shift+Tab']){
   await page.keyboard.press(key);assert(await dialog.evaluate(el=>el.contains(document.activeElement)),'focus stays inside dialog');
  }
  const input=dialog.getByRole('textbox',{name:'Project name',exact:true});
  await input.fill('workspace');assert.equal(await input.inputValue(),'workspace');
  const nestedOpener=dialog.getByRole('button',{name:'Nested dialog',exact:true});
  await nestedOpener.click();await page.getByRole('textbox',{name:'Nested name',exact:true}).fill('nested');
  await page.keyboard.press('Escape');assert.equal(await page.getByRole('dialog').count(),1);
  assert(await nestedOpener.evaluate(el=>el===document.activeElement),'nested close restores parent focus');
  await page.keyboard.press('Escape');assert.equal(await page.getByRole('dialog').count(),0);
  assert(await opener.evaluate(el=>el===document.activeElement),'close restores trigger focus');
  if(fullscreen){assert(await page.evaluate(()=>!!document.fullscreenElement));await page.evaluate(()=>document.exitFullscreen());}
  await page.close();console.log('PASS shared modal focus, input, nested Escape and restore',fullscreen?'native fullscreen':'normal');
 }}finally{await browser.close();}
})().catch(e=>{console.error(e);process.exitCode=1;});
