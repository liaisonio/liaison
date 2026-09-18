import {Field, Select} from '@/components/ui';
import {requestAPILabel} from '@/constants/llmProtocols';
import {useI18n} from '@/i18n';
import {useState} from 'react';

export default function RequestExample({base,model,protocols=['openai-compatible']}:{base:string;model:string;protocols?:string[]}) {
  const {tr}=useI18n();
  const [selected,setSelected]=useState('');
  const protocol=protocols.includes(selected)?selected:protocols[0]||'openai-compatible';
  let path='/v1/chat/completions';
  let payload:unknown={model,messages:[{role:'user',content:'Hello'}],stream:true};
  let headers=['Authorization: Bearer $LIAISON_API_KEY'];
  if(protocol==='openai'||protocol==='ark'){
    path=protocol==='ark'?'/api/v3/responses':'/v1/responses';
    payload={model,input:'Hello',store:false,stream:true};
  }else if(protocol==='anthropic'){
    path='/v1/messages';payload={model,messages:[{role:'user',content:'Hello'}],max_tokens:1024,stream:true};
    headers=['x-api-key: $LIAISON_API_KEY','anthropic-version: 2023-06-01'];
  }else if(protocol==='ollama'){
    path='/api/chat';
  }else if(protocol==='qwen'){
    path='/api/v1/services/aigc/text-generation/generation';
    payload={model,input:{messages:[{role:'user',content:'Hello'}]},parameters:{result_format:'message',incremental_output:true}};
    headers.push('X-DashScope-SSE: enable');
  }else if(protocol==='gemini'){
    path='/v1beta/models/'+encodeURIComponent(model)+':streamGenerateContent?alt=sse';
    payload={contents:[{role:'user',parts:[{text:'Hello'}]}]};
    headers=['x-goog-api-key: $LIAISON_API_KEY'];
  }
  const quote=(text:string)=>"'"+text.replace(/'/g,"'\\''")+"'";
  const lines=['curl '+quote(window.location.origin+base+path),...headers.map(h=>'  -H "'+h+'"'),"  -H 'Content-Type: application/json'",'  -d '+quote(JSON.stringify(payload))];
  return <>
    {protocols.length>1&&<Field label={tr('调用 API','Request API')}><Select value={protocol} onChange={e=>setSelected(e.target.value)}>{protocols.map(p=><option key={p} value={p}>{requestAPILabel(p)}</option>)}</Select></Field>}
    <pre className="ai-api-example">{lines.join(' \\\n')}</pre>
  </>;
}
