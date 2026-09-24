import {forwardRef, useEffect, useImperativeHandle, useRef, useState} from 'react';
import {Button, Field, Input, Notice} from '@/components/ui';
import {request, RequestError} from '@/api/client';
import {llmBase, protocolFamily} from '@/constants/llmProtocols';
import {useI18n} from '@/i18n';

export type LLMDraftHandle = {create:(name:string, description:string)=>Promise<void>};
export function parseServiceURL(value:string, protocol:string) {
  const url=new URL(value.trim());
  if(!['http:','https:'].includes(url.protocol)||url.username||url.password||url.search||url.hash||/[?#]/.test(value)||!url.hostname)throw Error('address');
  return {host:url.hostname.toLowerCase(),port:Number(url.port||(url.protocol==='https:'?443:80)),path:url.pathname==='/'?llmBase(protocol):url.pathname.replace(/\/$/,''),tls:url.protocol==='https:'};
}

export default forwardRef<LLMDraftHandle,{edge:string;protocol:string;applications:API.Application[];disabled:boolean;onUse:(id:string)=>void;onReady?:(ready:boolean)=>void}>(function LLMApplicationDraft({edge,protocol,applications,disabled,onUse,onReady},ref){
  const {tr}=useI18n();
  const [address,setAddress]=useState(''),[apiKey,setAPIKey]=useState(''),[appName,setAppName]=useState('');
  const [responses,setResponses]=useState(false);
  const [models,setModels]=useState<string[]>(),[selected,setSelected]=useState<string[]>([]),[manual,setManual]=useState('');
  const [probing,setProbing]=useState(false),[error,setError]=useState('');
  const controller=useRef<AbortController>();
  const profile=protocol==='openai'&&!responses?'openai-compatible':protocol;
  useEffect(()=>{controller.current?.abort();setProbing(false);setModels(undefined);setSelected([]);setError('');},[address,edge,apiKey,profile]);
  useEffect(()=>()=>controller.current?.abort(),[]);
  let target:ReturnType<typeof parseServiceURL>|undefined;
  try{target=parseServiceURL(address,protocol);}catch{/* Incomplete input is validated on action. */}
  const existing=target&&applications.find(a=>String(a.edge_id)===edge&&a.ip.toLowerCase()===target!.host&&a.port===target!.port&&protocolFamily(a.application_type)===protocol);
  const chosenModels=[...new Set([...selected,...manual.split(/[\n,]/).map(v=>v.trim()).filter(Boolean)])];
  const ready=!!target&&!!edge&&!probing&&!existing&&chosenModels.length>0&&chosenModels.length<=100;
  useEffect(()=>{onReady?.(ready);return()=>onReady?.(false);},[ready,onReady]);
  const payload=()=>({edge_id:Number(edge),service_url:address.trim(),protocol:profile,api_key:apiKey});
  async function probe(){
    if(!target||!edge)return;
    const active=new AbortController();controller.current=active;setProbing(true);setError('');setModels(undefined);setSelected([]);
    try{
      const r=await request<API.Response<{state:string;models?:string[]}>>('/api/v1/ai/setup/probe',{method:'POST',data:payload(),signal:active.signal,skipErrorHandler:true});
      if(active.signal.aborted)return;
      if(r.code!==200||!r.data)throw Error('probe');
      const messages:Record<string,string>={auth_required:tr('认证失败，请检查上游 API 密钥。','Authentication failed. Check the upstream API key.'),unreachable:tr('无法连接，请检查连接器和服务地址。','Cannot connect. Check the connector and service address.'),unsupported:tr('此协议不支持自动列举，请手动填写模型 ID。','This protocol does not support discovery. Enter model IDs manually.'),busy:tr('检测繁忙，请稍后重试。','Discovery is busy. Try again shortly.')};
      if(r.data.state==='compatible')setModels([...new Set(r.data.models||[])]);
      else setError(messages[r.data.state]||tr('接口不兼容，请检查协议和接口路径。','Incompatible endpoint. Check the protocol and base path.'));
    }catch{if(!active.signal.aborted)setError(tr('检测失败，请检查权限或稍后重试。','Discovery failed. Check permissions or try again later.'));}
    finally{if(!active.signal.aborted)setProbing(false);}
  }
  useImperativeHandle(ref,()=>({async create(name,description){
    const chosen=[...new Set([...selected,...manual.split(/[\n,]/).map(v=>v.trim()).filter(Boolean)])];
    if(!target||!edge||probing||!chosen.length||chosen.length>100||existing)throw Error(tr('请填写有效地址、连接器并选择模型；已有应用请直接复用。','Enter a valid address, select a connector and models. Reuse an existing application if available.'));
    try{
      const r=await request<API.Response<{id:number}>>('/api/v1/ai/setup',{method:'POST',data:{...payload(),name:name||appName.trim()||`${target.host}:${target.port}`,description,application_name:appName.trim(),models:Object.fromEntries(chosen.map(m=>[m,m]))},skipErrorHandler:true});
      if(r.code!==200||!r.data?.id)throw Error('create');
    }catch(e){throw Error(e instanceof RequestError&&e.response?.status===409?tr('该应用已存在，请切换到“选择已有应用”复用。','This application already exists. Switch to “Choose existing application”.'):tr('创建未完成，请刷新列表确认结果后重试。','Creation could not be confirmed. Refresh the list before retrying.'));}
  }}));
  return <fieldset className="liaison-llm-draft" disabled={disabled}>
    <Field label={tr('应用名称','Application name')}><Input value={appName} onChange={e=>setAppName(e.target.value)} placeholder={target?`${target.host}:${target.port}`:tr('按应用地址自动生成','Generated from the application address')}/></Field>
    <Field label={tr('应用地址','Application address')} required hint={tr('从连接器所在机器访问；127.0.0.1 指该机器。','Reached from the connector’s machine; 127.0.0.1 refers to that machine.')}>
      <Input type="url" value={address} onChange={e=>setAddress(e.target.value)} placeholder={`http://127.0.0.1:${protocol==='ollama'?'11434':'8000'}${llmBase(protocol)}`}/>
    </Field>
    {existing&&<Notice>{tr('此目标已登记为应用：','This target is already registered: ')}{existing.name} <Button onClick={()=>onUse(String(existing.id))}>{tr('使用已有应用','Use existing application')}</Button></Notice>}
    <div><Button onClick={()=>void probe()} loading={probing} disabled={!target||!edge||!!existing}>{tr('检测连接并获取模型','Check connection & fetch models')}</Button></div>
    {error&&<Notice tone="danger">{error}</Notice>}
    {models!==undefined&&<div className="liaison-llm-models" role="group" aria-label={tr('可调用模型','Available models')}>
      <p role="status">{models.length?tr('连接成功，选择要开放的模型。','Connected. Select models to expose.'):tr('连接成功，但模型列表为空，可手动填写。','Connected, but the model list is empty. Enter IDs manually.')}</p>
      {models.map(m=><label key={m} className="liaison-checkbox"><input type="checkbox" checked={selected.includes(m)} disabled={!selected.includes(m)&&selected.length>=100} onChange={e=>setSelected(old=>e.target.checked?[...old,m]:old.filter(v=>v!==m))}/><code>{m}</code></label>)}
    </div>}
    <details open={!!error||models?.length===0||undefined}><summary>{tr('高级设置','Advanced settings')}</summary>
    <Field label={tr('上游 API 密钥（选填）','Upstream API key (optional)')} hint={tr('用于连接模型服务，不是客户端调用密钥。','Authenticates with the model service, not the client access.')}>
      <Input type="password" autoComplete="new-password" value={apiKey} onChange={e=>setAPIKey(e.target.value)}/>
    </Field>
      <Field label={tr('手动输入模型 ID','Enter model IDs manually')} hint={tr('多个模型用逗号分隔，最多 100 个。','Separate model IDs with commas, up to 100.')}><Input value={manual} onChange={e=>setManual(e.target.value)} placeholder="qwen3, deepseek-r1"/></Field>
      {protocol==='openai'&&<label className="liaison-checkbox"><input type="checkbox" checked={responses} onChange={e=>setResponses(e.target.checked)}/>{tr('上游支持 Responses API','Upstream supports the Responses API')}</label>}
    </details>
  </fieldset>;
});
