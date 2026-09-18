import {test} from 'node:test';
import assert from 'node:assert/strict';
import http from 'node:http';
import {once} from 'node:events';
import {createDemo,normalize} from './llm-protocol-demo.mjs';

test('text boundary rejects unsupported features and invalid requests',()=>{
  const body={model:'demo',messages:[{role:'user',content:'Hello'}]};
  assert.equal(normalize('/v1/messages',body,'demo').family,'anthropic');
  assert.equal(normalize('/v1/messages',{...body,max_tokens:1024},'demo').max_tokens,1024);
  assert.throws(()=>normalize('/v1/messages',{...body,max_tokens:1025},'demo'));
  assert.equal(normalize('/api/v1/services/aigc/text-generation/generation',{model:'demo',input:{messages:body.messages},parameters:{max_tokens:1024}},'demo').max_tokens,1024);
  for(const value of [{...body,tools:[]},{...body,model:'other'},{...body,max_tokens:-1},{...body,messages:[{role:'user',content:[{type:'image',source:{}}]}]}])assert.throws(()=>normalize('/v1/messages',value,'demo'));
  assert.throws(()=>createDemo({upstream:'http://example.com/v1'}));
  const response={model:'demo',input:'hello',store:false};
  assert.equal(normalize('/v1/responses',response,'demo').family,'responses');
  for(const extra of [{store:true},{background:true},{previous_response_id:'other'},{tools:[]},{input:[{type:'item_reference',id:'other'}]}])assert.throws(()=>normalize('/v1/responses',{...response,...extra},'demo'));
  assert.throws(()=>normalize('/api/v1/services/aigc/text-generation/generation',{model:'demo',input:{messages:body.messages},parameters:{enable_thinking:true}},'demo'));
});
test('translated protocols preserve upstream content and usage, with terminal stream events',async()=>{
  let missingUsage=false;
  const backend=http.createServer(async(req,res)=>{
    let body='';for await(const c of req)body+=c;
    assert.equal(JSON.parse(body).model,'demo');
    res.setHeader('content-type','application/json');
    res.end(JSON.stringify({choices:[{message:{content:'upstream answer'},finish_reason:'stop'}],...(!missingUsage&&{usage:{prompt_tokens:3,completion_tokens:2}})}));
  }).listen(0,'127.0.0.1');await once(backend,'listening');
  const demo=createDemo({upstream:`http://127.0.0.1:${backend.address().port}/v1`,model:'demo'}).listen(0,'127.0.0.1');await once(demo,'listening');
  const base=`http://127.0.0.1:${demo.address().port}`;
  try{
    const message={model:'demo',messages:[{role:'user',content:'Hello'}]};
    for(const [path,body,terminal] of [
      ['/v1/messages',{...message,stream:true},'message_stop'],
      ['/v1beta/models/demo:streamGenerateContent',{contents:[{parts:[{text:'Hello'}]}]},'"totalTokenCount":5'],
      ['/api/chat',{...message,stream:true},'"done":true'],
      ['/api/v1/services/aigc/text-generation/generation',{model:'demo',input:{messages:message.messages},parameters:{result_format:'message'}},'"input_tokens":3'],
    ]){
      const r=await fetch(base+path,{method:'POST',body:JSON.stringify(body)});assert.equal(r.status,200);
      const text=await r.text();assert(text.includes('upstream answer'));assert(text.includes(terminal));
    }
    missingUsage=true;
    const r=await fetch(base+'/v1/messages',{method:'POST',body:JSON.stringify(message)});assert.equal(r.status,502);
    assert.equal((await fetch(base+'/api/pull',{method:'POST',body:'{}'})).status,404);
  }finally{demo.closeAllConnections();backend.closeAllConnections();await Promise.all([new Promise(r=>demo.close(r)),new Promise(r=>backend.close(r))]);}
});
test('OpenAI and Ark relay validated stateless requests to vLLM without buffering',async()=>{
  let calls=0;
  const backend=http.createServer(async(req,res)=>{
    let raw='';for await(const chunk of req)raw+=chunk;
    const body=JSON.parse(raw);calls++;
    assert.equal(body.chat_template_kwargs.enable_thinking,false);
    if(req.url==='/v1/responses'){assert.equal(body.store,false);assert.equal(body.background,false);assert.equal(body.input[0].content,'Hello');}
    else assert.equal(req.url,'/v1/chat/completions');
    if(body.stream){res.setHeader('content-type','text/event-stream');res.end('data: {"upstream":true}\n\n');}
    else {res.setHeader('content-type','application/json');res.end('{"upstream":true}');}
  }).listen(0,'127.0.0.1');await once(backend,'listening');
  const demo=createDemo({upstream:`http://127.0.0.1:${backend.address().port}/v1`,model:'demo'}).listen(0,'127.0.0.1');await once(demo,'listening');
  const base=`http://127.0.0.1:${demo.address().port}`;
  try{
    for(const prefix of ['/v1','/api/v3'])for(const api of ['/responses','/chat/completions'])for(const stream of [false,true]){
      const r=await fetch(base+prefix+api,{method:'POST',body:JSON.stringify({model:'demo',stream,...(api==='/responses'?{input:'Hello'}:{messages:[{role:'user',content:'Hello'}]})})});
      assert.equal(r.status,200);assert.equal(r.headers.get('x-liaison-simulator'),'text-only; upstream-stream');assert((await r.text()).includes('"upstream":true'));
    }
    assert.equal(calls,8);
    const rejected=await fetch(base+'/api/v3/responses',{method:'POST',body:JSON.stringify({model:'demo',input:'Hello',store:true})});assert.equal(rejected.status,400);assert.equal(calls,8);
    assert.equal((await fetch(base+'/v1/responses/secret')).status,404);
  }finally{demo.closeAllConnections();backend.closeAllConnections();await Promise.all([new Promise(r=>demo.close(r)),new Promise(r=>backend.close(r))]);}
});
