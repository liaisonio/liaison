// Start Vite separately, then set E2E_UI_URL. Live-service suites are not run here.
const {spawnSync}=require('node:child_process');
const fs=require('node:fs'),os=require('node:os'),path=require('node:path');
if(!process.env.E2E_UI_URL)throw Error('Set E2E_UI_URL to the local Vite server');
const root=path.resolve(__dirname,'../..');
const report=fs.mkdtempSync(path.join(os.tmpdir(),'liaison-fixture-e2e-'));
const suites=[
 'access-auth','access-header','access-ui','agent-session-isolation',
 'anthropic-workspace','brand-select','data-context','data-draft-review',
 'data-editor-handoff','data-result-share','desktop-layout','home-toolbar',
 'llm-access-create','llm-application','llm-playground-ui','markdown','ollama',
 'protocol-navigation','shell-agent-ui','sql-protocols','ssh-handoff',
 'terminal-completion','webcache','webs3','websftp-ui','webssh-files-ui',
 'native-llm','llm-access-types','llm-insights','llm-table-lines',
 'token-trend-zero','token-trend-feedback','agent-handoff','application-protocol-labels',
 'web-entry','web-entry-sources','websmb-ui','dameng','device-language',
 'dashboard-traffic','confirm-ui',
];
const results=[];
for(const suite of suites){
 const start=Date.now();
 const result=spawnSync(process.execPath,[path.join(__dirname,suite+'.cjs')],{
  cwd:root,env:{...process.env,UI_URL:process.env.E2E_UI_URL},encoding:'utf8',
  timeout:120000,maxBuffer:4*1024*1024,
 });
 fs.writeFileSync(path.join(report,suite+'.log'),(result.stdout||'')+(result.stderr||''));
 results.push({suite,status:result.status,error:result.error?.message,seconds:Math.round((Date.now()-start)/1000)});
 console.log(result.status===0?'PASS':'FAIL',suite);
 fs.writeFileSync(path.join(report,'results.json'),JSON.stringify(results,null,2));
}
console.log('Fixture report:',report);
process.exitCode=results.some(result=>result.status!==0)?1:0;
