import {useEffect,useRef,useState} from 'react';
import {Button,Notice} from '@/components/ui';
import {MessageContent} from '@/components/AgentWorkspace/MessageContent';
import {getToken} from '@/store/session';
import {useI18n} from '@/i18n';

type Result={model:string;text:string;state:'running'|'done'|'error'|'stopped';elapsed?:number;first?:number;id?:string};
export default function Compare({base,models,enabled}:{base:string;models:string[];enabled:boolean}){
 const {tr}=useI18n();const [selected,setSelected]=useState<string[]>(()=>models.slice(0,2));const [prompt,setPrompt]=useState('');const [results,setResults]=useState<Result[]>([]);const [running,setRunning]=useState(false);
 const active=useRef<AbortController[]>([]);const generation=useRef(0);const input=useRef<HTMLTextAreaElement>(null);
 useEffect(()=>()=>{generation.current++;active.current.forEach(c=>c.abort());},[base]);
 const choices=selected.filter(m=>models.includes(m));
 const run=async()=>{
  if(running||!enabled||choices.length<2||!prompt.trim())return;
  input.current?.focus({preventScroll:true});const turn=++generation.current;setRunning(true);setResults(choices.map(model=>({model,text:'',state:'running'})));
  const controllers=choices.map(()=>new AbortController());active.current=controllers;
  await Promise.all(choices.map(async(model,index)=>{
   const start=performance.now();let text='',first:number|undefined,id:string|undefined;
   const update=(state:Result['state'])=>{if(turn===generation.current)setResults(rows=>rows.map(r=>r.model===model?{model,text,first,id,state,elapsed:Math.round(performance.now()-start)}:r));};
   try{
    const response=await fetch(`${base}/test`,{method:'POST',headers:{'Content-Type':'application/json',Authorization:`Bearer ${getToken()}`},body:JSON.stringify({model,messages:[{role:'user',content:prompt}],max_tokens:1024,stream:true}),signal:controllers[index].signal});
    id=response.headers.get('X-Request-ID')||undefined;
    if(!response.ok||!response.body)throw Error();
    const reader=response.body.getReader(),decoder=new TextDecoder();let buffer='',done=false;
    try{while(true){const chunk=await reader.read();if(chunk.done)break;buffer+=decoder.decode(chunk.value,{stream:true});let end;while((end=buffer.indexOf('\n\n'))>=0){const block=buffer.slice(0,end);buffer=buffer.slice(end+2);for(const line of block.split('\n')){if(!line.startsWith('data:'))continue;const value=line.slice(5).trim();if(value==='[DONE]'){done=true;continue;}const data=JSON.parse(value);if(data.error)throw Error();const delta=data.choices?.[0]?.delta?.content;if(delta){first??=Math.round(performance.now()-start);text+=delta;update('running');}}}}}finally{reader.releaseLock();}
    if(!done)throw Error();update('done');
   }catch{controllers[index].abort();update(turn===generation.current&&active.current[index]?.signal.reason==='user-stop'?'stopped':'error');}
  }));
  if(turn===generation.current){setRunning(false);active.current=[];}
 };
 return <section className="ai-api-card ai-compare">
 <h2>{tr('模型对比','Model comparison')}</h2><p>{tr('选择 2–3 个授权模型发送同一问题。每个模型独立计量，可能产生多次上游费用。','Send the same prompt to 2–3 authorized models. Each call is metered separately and may incur upstream costs.')}</p>
 {models.length<2&&<Notice>{tr('当前访问只有一个可调用模型；添加第二个模型映射后可使用对比。','This access has only one model. Add a second model mapping to compare.')}</Notice>}
 <div className="ai-compare-models">{models.map(model=><label key={model}><input type="checkbox" checked={choices.includes(model)} disabled={running||!choices.includes(model)&&choices.length>=3} onChange={e=>setSelected(v=>e.target.checked?[...v,model]:v.filter(m=>m!==model))}/><code>{model}</code></label>)}</div>
 <textarea ref={input} className="liaison-input" aria-label={tr('对比问题','Comparison prompt')} placeholder={tr('输入同一个问题…','Ask the same question…')} rows={2} readOnly={running} value={prompt} onChange={e=>setPrompt(e.target.value)} onKeyDown={e=>{if(e.key==='Enter'&&!e.shiftKey&&!e.nativeEvent.isComposing){e.preventDefault();void run();}}}/>
 <div className="ai-compare-actions">{running?<Button onClick={()=>active.current.forEach(c=>c.abort('user-stop'))}>{tr('停止全部','Stop all')}</Button>:<Button variant="primary" disabled={!enabled||choices.length<2||!prompt.trim()} onClick={()=>void run()}>{tr('发送对比','Send comparison')}</Button>}</div>
 <div className="ai-compare-results">{results.map(r=><article key={r.model}><h2><code>{r.model}</code></h2><p role="status">{r.state==='running'?tr('正在回复…','Responding…'):r.state==='done'?tr('已完成','Complete'):r.state==='stopped'?tr('已停止','Stopped'):tr('调用失败，可重新发送','Request failed; send again to retry')}</p>
 <MessageContent text={r.text}/><dl><dt>{tr('首字到达 · 浏览器测量','First token · browser measured')}</dt><dd>{r.first==null?'—':`${r.first} ms`}</dd><dt>{tr('总耗时','Total duration')}</dt><dd>{r.elapsed==null?'—':`${r.elapsed} ms`}</dd></dl>{r.id&&<details><summary>{tr('请求 ID','Request ID')}</summary><code>{r.id}</code></details>}
 </article>)}</div>
 </section>;
}
