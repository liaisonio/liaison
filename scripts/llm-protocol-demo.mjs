// Local text-only protocol simulator backed by real OpenAI chat inference.
// OpenAI/Ark relay upstream streams; translated protocols are deliberately buffered.
import http from 'node:http';
import {randomUUID} from 'node:crypto';
import {pathToFileURL} from 'node:url';
import {Readable} from 'node:stream';
import {pipeline} from 'node:stream/promises';

const responsesPaths=['/v1/responses','/api/v3/responses'];
const chatPaths=['/v1/chat/completions','/api/v3/chat/completions'];
const qwenPath='/api/v1/services/aigc/text-generation/generation';
function textMessages(value){
  check(Array.isArray(value));
  return value.map(m=>{only(m,['role','content']);check(['system','user','assistant','developer'].includes(m.role)&&typeof m.content==='string');return m;});
}

function check(ok) { if (!ok) throw Object.assign(new Error('Unsupported or invalid text request'), {status:400}); }
function only(value, keys) { check(value && typeof value==='object' && !Array.isArray(value)); check(Object.keys(value).every(k=>keys.includes(k))); }
function parts(value, anthropic=false) {
  if (typeof value==='string') return value;
  check(Array.isArray(value) && value.length>0);
  return value.map(p=>{only(p,anthropic?['type','text']:['text']);check(typeof p.text==='string'&&(!anthropic||p.type==='text'));return p.text;}).join('\n');
}
export function normalize(path, body, model) {
  let family, messages=[], limit, stream;
  if(responsesPaths.includes(path)){
    family='responses';only(body,['model','input','instructions','max_output_tokens','stream','store','background']);check(body.model===model);
    check((body.store===undefined||body.store===false)&&(body.background===undefined||body.background===false));
    if(body.instructions!==undefined){check(typeof body.instructions==='string');messages.push({role:'system',content:body.instructions});}
    if(typeof body.input==='string')messages.push({role:'user',content:body.input});
    else {check(Array.isArray(body.input));for(const item of body.input){
      only(item,['type','role','content']);check(item.type===undefined||item.type==='message');check(['user','assistant','system','developer'].includes(item.role));
      let content=item.content;
      if(Array.isArray(content))content=content.map(p=>{only(p,['type','text']);check(p.type==='input_text'&&typeof p.text==='string');return p.text;}).join('\n');
      check(typeof content==='string');messages.push({role:item.role,content});
    }}
    limit=body.max_output_tokens;stream=body.stream===true;
  }else if(chatPaths.includes(path)){
    family='chat';only(body,['model','messages','max_tokens','stream']);check(body.model===model);
    messages=textMessages(body.messages);limit=body.max_tokens;stream=body.stream===true;
  }else if(path===qwenPath){
    family='qwen';only(body,['model','input','parameters']);check(body.model===model);only(body.input,['messages']);
    messages=textMessages(body.input.messages);
    const p=body.parameters??{};only(p,['result_format','incremental_output','max_tokens','enable_thinking']);
    check(p.result_format===undefined||['message','text'].includes(p.result_format));
    check(p.incremental_output===undefined||typeof p.incremental_output==='boolean');
    check(p.enable_thinking===undefined||p.enable_thinking===false);limit=p.max_tokens;
  }else if(path==='/v1/messages'){
    family='anthropic';only(body,['model','messages','system','max_tokens','stream']);
    check(body.model===model);limit=body.max_tokens;stream=body.stream===true;
    if(body.system!==undefined)messages.push({role:'system',content:parts(body.system,true)});
    check(Array.isArray(body.messages));
    for(const m of body.messages){only(m,['role','content']);check(['user','assistant'].includes(m.role));messages.push({role:m.role,content:parts(m.content,true)});}
  }else if(path==='/api/chat'){
    family='ollama';only(body,['model','messages','stream','options']);check(body.model===model);
    if(body.options!==undefined)only(body.options,['num_predict']);
    limit=body.options?.num_predict;stream=body.stream!==false;check(Array.isArray(body.messages));
    for(const m of body.messages){only(m,['role','content']);check(['system','user','assistant'].includes(m.role)&&typeof m.content==='string');messages.push(m);}
  }else{
    const match=path.match(/^\/v1beta\/models\/([^/]+):(generateContent|streamGenerateContent)$/);
    check(match&&decodeURIComponent(match[1])===model);family='gemini';stream=match[2]==='streamGenerateContent';
    only(body,['contents','systemInstruction','generationConfig']);
    if(body.generationConfig!==undefined)only(body.generationConfig,['maxOutputTokens']);
    limit=body.generationConfig?.maxOutputTokens;
    if(body.systemInstruction){only(body.systemInstruction,['parts']);messages.push({role:'system',content:parts(body.systemInstruction.parts)});}
    check(Array.isArray(body.contents));
    for(const m of body.contents){only(m,['role','parts']);check(m.role===undefined||['user','model'].includes(m.role));messages.push({role:m.role==='model'?'assistant':'user',content:parts(m.parts)});}
  }
  check(body.stream===undefined||typeof body.stream==='boolean');
  check(messages.some(m=>m.role==='user')&&messages.length<=32);
  check(limit===undefined||(Number.isInteger(limit)&&limit>0&&limit<=1024));
  return {family,messages,max_tokens:limit??128,stream};
}
function sendJSON(res, status, data) {res.writeHead(status,{'content-type':'application/json'});res.end(JSON.stringify(data));}
export function createDemo({upstream='http://127.0.0.1:18081/v1',model='qwen3-0.6b'}={}) {
  const target=new URL(upstream);
  if(!['127.0.0.1','localhost','[::1]'].includes(target.hostname)||target.protocol!=='http:'||target.username||target.password||target.search||target.hash)throw Error('Demo upstream must be a local HTTP service');
  return http.createServer(async(req,res)=>{
    res.setHeader('X-Liaison-Simulator','text-only; buffered-stream');
    const abort=new AbortController();res.on('close',()=>abort.abort());
    try{
      const path=new URL(req.url,'http://localhost').pathname;
      if(req.method==='GET'){
        if(path==='/health')return sendJSON(res,200,{simulator:true,model,streaming:{openai:'upstream',ark:'upstream',qwen:'buffered',anthropic:'buffered',gemini:'buffered',ollama:'buffered'},protocols:['openai','ark','qwen','anthropic','gemini','ollama']});
        if(path==='/v1/models')return sendJSON(res,200,{object:'list',data:[{id:model,object:'model',type:'model',display_name:model,created_at:'2026-01-01T00:00:00Z'}],has_more:false,first_id:model,last_id:model});
        if(path==='/v1beta/models')return sendJSON(res,200,{models:[{name:`models/${model}`,displayName:model,supportedGenerationMethods:['generateContent']}]});
        if(path==='/api/tags')return sendJSON(res,200,{models:[{name:model,model}]});
        return sendJSON(res,404,{error:{message:'Unknown simulator endpoint'}});
      }
      if(req.method!=='POST')return sendJSON(res,405,{error:{message:'Method not supported'}});
      if(![...responsesPaths,...chatPaths,qwenPath,'/v1/messages','/api/chat'].includes(path)&&!/^\/v1beta\/models\/[^/]+:(generateContent|streamGenerateContent)$/.test(path))return sendJSON(res,404,{error:{message:'Unknown simulator endpoint'}});
      let body='',size=0;
      req.setTimeout(10000,()=>req.destroy());
      for await(const chunk of req){size+=chunk.length;if(size>65536)throw Object.assign(Error('Request too large'),{status:413});body+=chunk;}
      let parsed;try{parsed=JSON.parse(body);}catch{throw Object.assign(Error('Invalid JSON'),{status:400});}
      const input=normalize(path,parsed,model);
      if(input.family==='qwen'){
        check(req.headers['x-dashscope-sse']===undefined||req.headers['x-dashscope-sse']==='enable');
        input.stream=req.headers['x-dashscope-sse']==='enable';
      }
      if(['responses','chat'].includes(input.family)){
        const isResponses=input.family==='responses';
        const payload=isResponses?{model,input:input.messages,max_output_tokens:input.max_tokens,store:false,background:false,stream:input.stream}:{model,messages:input.messages,max_tokens:input.max_tokens,stream:input.stream,...(input.stream&&{stream_options:{include_usage:true}})};
        const response=await fetch(`${upstream.replace(/\/$/,'')}/${isResponses?'responses':'chat/completions'}`,{
          method:'POST',headers:{'content-type':'application/json'},redirect:'error',signal:AbortSignal.any([abort.signal,AbortSignal.timeout(90000)]),
          body:JSON.stringify({...payload,temperature:0,chat_template_kwargs:{enable_thinking:false}}),
        });
        if(!response.ok||!response.body)throw Error('Upstream inference failed');
        res.setHeader('X-Liaison-Simulator','text-only; upstream-stream');
        res.writeHead(200,{'content-type':input.stream?'text/event-stream':'application/json','cache-control':'no-cache'});
        await pipeline(Readable.fromWeb(response.body),res);return;
      }
      const response=await fetch(`${upstream.replace(/\/$/,'')}/chat/completions`,{
        method:'POST',headers:{'content-type':'application/json'},redirect:'error',signal:AbortSignal.any([abort.signal,AbortSignal.timeout(90000)]),
        body:JSON.stringify({model,messages:input.messages,max_tokens:input.max_tokens,stream:false,temperature:0,chat_template_kwargs:{enable_thinking:false}}),
      });
      if(!response.ok)throw Error('Upstream inference failed');
      const completion=await response.json(),choice=completion.choices?.[0],text=choice?.message?.content,u=completion.usage;
      if(typeof text!=='string'||!['stop','length'].includes(choice.finish_reason)||!Number.isSafeInteger(u?.prompt_tokens)||!Number.isSafeInteger(u?.completion_tokens)||u.prompt_tokens<0||u.completion_tokens<0)throw Error('Incomplete upstream response or usage');
      const id=`sim_${randomUUID()}`,limited=choice.finish_reason==='length';
      const sse=(data,event)=>res.write(`${event?`event: ${event}\n`:''}data: ${JSON.stringify(data)}\n\n`);
      if(input.family==='qwen'){
        const output=parsed.parameters?.result_format==='text'?{text,finish_reason:limited?'length':'stop'}:{choices:[{finish_reason:limited?'length':'stop',message:{role:'assistant',content:text}}]};
        const result={request_id:id,output,usage:{input_tokens:u.prompt_tokens,output_tokens:u.completion_tokens,total_tokens:u.prompt_tokens+u.completion_tokens}};
        if(!input.stream)return sendJSON(res,200,result);
        res.writeHead(200,{'content-type':'text/event-stream','cache-control':'no-cache'});sse(result,'result');res.end();
      }else if(input.family==='anthropic'){
        const result={id,type:'message',role:'assistant',model,content:[{type:'text',text}],stop_reason:limited?'max_tokens':'end_turn',stop_sequence:null,usage:{input_tokens:u.prompt_tokens,output_tokens:u.completion_tokens}};
        if(!input.stream)return sendJSON(res,200,result);
        res.writeHead(200,{'content-type':'text/event-stream','cache-control':'no-cache'});
        sse({type:'message_start',message:{...result,content:[],stop_reason:null,usage:{input_tokens:u.prompt_tokens,output_tokens:0}}},'message_start');
        sse({type:'content_block_start',index:0,content_block:{type:'text',text:''}},'content_block_start');
        sse({type:'content_block_delta',index:0,delta:{type:'text_delta',text}},'content_block_delta');
        sse({type:'content_block_stop',index:0},'content_block_stop');
        sse({type:'message_delta',delta:{stop_reason:result.stop_reason,stop_sequence:null},usage:{output_tokens:u.completion_tokens}},'message_delta');
        sse({type:'message_stop'},'message_stop');res.end();
      }else if(input.family==='gemini'){
        const result={modelVersion:model,candidates:[{index:0,content:{role:'model',parts:[{text}]},finishReason:limited?'MAX_TOKENS':'STOP'}],usageMetadata:{promptTokenCount:u.prompt_tokens,candidatesTokenCount:u.completion_tokens,totalTokenCount:u.prompt_tokens+u.completion_tokens}};
        if(!input.stream)return sendJSON(res,200,result);
        res.writeHead(200,{'content-type':'text/event-stream','cache-control':'no-cache'});sse(result);res.end();
      }else{
        const result={model,created_at:new Date().toISOString(),message:{role:'assistant',content:text},done:true,done_reason:limited?'length':'stop',prompt_eval_count:u.prompt_tokens,eval_count:u.completion_tokens};
        if(!input.stream)return sendJSON(res,200,result);
        res.writeHead(200,{'content-type':'application/x-ndjson'});
        res.write(JSON.stringify({model,created_at:result.created_at,message:result.message,done:false})+'\n');
        res.end(JSON.stringify({...result,message:{role:'assistant',content:''}})+'\n');
      }
    }catch(error){
      if(!res.destroyed&&!res.headersSent)sendJSON(res,error.status||502,{error:{message:error.status?error.message:'Local model inference failed'}});
      else if(!res.destroyed)res.destroy();
    }
  });
}
if(process.argv[1]&&import.meta.url===pathToFileURL(process.argv[1]).href){
  const server=createDemo({upstream:process.env.LLM_DEMO_UPSTREAM,model:process.env.LLM_DEMO_MODEL});
  server.requestTimeout=15000;server.headersTimeout=10000;
  server.listen(Number(process.env.LLM_DEMO_PORT||18082),'127.0.0.1',()=>console.log('LLM text protocol simulator listening on 127.0.0.1:'+server.address().port));
  for(const signal of ['SIGINT','SIGTERM'])process.on(signal,()=>{server.close();server.closeAllConnections();});
}
