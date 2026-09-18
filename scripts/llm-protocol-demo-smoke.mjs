// Opt-in real inference smoke test against an already-running local simulator.
import assert from 'node:assert/strict';
const base=process.env.LLM_DEMO_URL||'http://127.0.0.1:18082';
const model=process.env.LLM_DEMO_MODEL||'qwen3-0.6b';
const messages=[{role:'user',content:'你好，请用一句中文问候。'}];
for(const stream of [false,true]){
  const cases=[
    ['/v1/chat/completions',{model,messages,max_tokens:64,stream},stream?'[DONE]':'completion_tokens'],
    ['/v1/responses',{model,input:messages,store:false,max_output_tokens:64,stream},stream?'response.completed':'output_tokens'],
    ['/api/v3/chat/completions',{model,messages,max_tokens:64,stream},stream?'[DONE]':'completion_tokens'],
    ['/api/v3/responses',{model,input:'你好，请用一句中文问候。',store:false,max_output_tokens:64,stream},stream?'response.completed':'output_tokens'],
    ['/api/v1/services/aigc/text-generation/generation',{model,input:{messages},parameters:{max_tokens:64,result_format:'message',incremental_output:true,enable_thinking:false}},'output_tokens'],
    ['/v1/messages',{model,messages,max_tokens:64,stream},stream?'message_stop':'output_tokens'],
    [`/v1beta/models/${model}:${stream?'streamGenerateContent?alt=sse':'generateContent'}`,{contents:[{role:'user',parts:[{text:messages[0].content}]}],generationConfig:{maxOutputTokens:64}},'usageMetadata'],
    ['/api/chat',{model,messages,stream,options:{num_predict:64}},'"done":true'],
  ];
  for(const [path,body,terminal] of cases){
    const r=await fetch(base+path,{method:'POST',headers:{'content-type':'application/json',...(stream&&path.includes('text-generation')&&{'X-DashScope-SSE':'enable'})},body:JSON.stringify(body),signal:AbortSignal.timeout(100000)});
    const text=await r.text();assert.equal(r.status,200,text);assert(text.includes(terminal),text);assert(/[\u4e00-\u9fff]/.test(text),text);
    if(stream)assert.match(r.headers.get('content-type'),/event-stream|ndjson/);
    console.log('PASS',stream?'stream':'JSON',path);
  }
}
for(const path of ['/v1/models','/v1beta/models','/api/tags']){const r=await fetch(base+path);assert.equal(r.status,200);assert((await r.text()).includes(model));console.log('PASS discovery',path);}
console.log('19 real-model checks passed; not cloud-vendor certification or full SDK coverage.');
