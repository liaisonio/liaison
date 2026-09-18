import {LLM_PROTOCOL_OPTIONS,protocolFamily,uniqueProtocolFamilies,llmBase,llmLabel,clientProtocols} from '@/constants/llmProtocols';
import {forwardRef, useEffect, useImperativeHandle, useRef, useState} from 'react';
import {request} from '@/api/client';
import {Button, Field, Input, Notice, Select} from '@/components/ui';
import {useI18n} from '@/i18n';

type Upstream = {application_type?:string;protocol:string;base_path:string;tls:boolean;api_key?:string;has_api_key?:boolean};
type Access = {enabled:boolean;models:Record<string,string>;external_protocol:string};
export type LLMConnectionHandle = {saveUpstream:()=>Promise<void>;saveAccess:(id:number)=>Promise<void>};
async function api<T>(path:string, data?:unknown):Promise<T> {
  const r=await request<API.Response<T>>(path,data===undefined?{}:{method:'PUT',data});
  if(r.code!==200||!r.data)throw Error('llm configuration');
  return r.data;
}

export default forwardRef<LLMConnectionHandle,{applicationId:string;accessId?:number;disabled:boolean;applicationOnly?:boolean}>(function LLMConnection({applicationId,accessId,disabled,applicationOnly=false},ref){
  const {tr}=useI18n();
  const [upstream,setUpstream]=useState<Upstream>();
  const [pairs,setPairs]=useState<[string,string][]>([['','']]);
  const [enabled,setEnabled]=useState(true),[failed,setFailed]=useState(false),[retry,setRetry]=useState(0);
  const [loaded,setLoaded]=useState(false),[dirty,setDirty]=useState(false);
  const [probing,setProbing]=useState(false),[models,setModels]=useState<string[]>(),[probeError,setProbeError]=useState('');
  const probeRequest=useRef<AbortController>();
  useEffect(()=>()=>probeRequest.current?.abort(),[]);
  useEffect(()=>{
    let alive=true;probeRequest.current?.abort();setProbing(false);setModels(undefined);setProbeError('');setLoaded(false);setFailed(false);setUpstream(undefined);setDirty(false);
    (async()=>{
      const u=await api<Upstream>(`/api/v1/ai/applications/${applicationId}`);
      const a=accessId?await api<Access>(`/api/v1/ai/accesses/${accessId}`):undefined;
      if(!alive)return;
      setUpstream(u);setPairs(a&&Object.keys(a.models).length?Object.entries(a.models):[['','']]);
      setEnabled(a&&Object.keys(a.models).length?a.enabled:true);setLoaded(true);
    })().catch(()=>{if(alive)setFailed(true);});
    return()=>{alive=false;};
  },[applicationId,accessId,retry]);
  function validate(){
    if(!loaded||!upstream||probing||(!applicationOnly&&(!pairs.length||pairs.some(([a,b])=>!a.trim()||!b.trim())||new Set(pairs.map(([a])=>a.trim())).size!==pairs.length)))throw Error('llm configuration');
  }
  async function fetchModels(){
    if(!loaded||!upstream||probing)return;
    const controller=new AbortController();probeRequest.current=controller;
    setProbing(true);setModels(undefined);setProbeError('');
    try{
      const saved=await request<API.Response<Upstream>>(`/api/v1/ai/applications/${applicationId}`,{method:'PUT',data:upstream,signal:controller.signal});
      if(saved.code!==200||!saved.data)throw Error('save');
      if(controller.signal.aborted)return;
      setUpstream(saved.data);setDirty(false);
      const result=await request<API.Response<{state:string;models?:string[]}>>(`/api/v1/ai/applications/${applicationId}/probe`,{method:'POST',signal:controller.signal});
      if(controller.signal.aborted)return;
      if(result.code!==200||!result.data)throw Error('probe');
      if(result.data.state==='auth_required')setProbeError(tr('上游认证失败，请检查上游 API 密钥。','Upstream authentication failed. Check the upstream API key.'));
      else if(result.data.state==='unsupported')setProbeError(tr('此协议暂不支持自动列举，请手动填写上游模型 ID。','Automatic listing is unavailable for this protocol. Enter upstream model IDs manually.'));
      else if(result.data.state==='unreachable')setProbeError(tr('无法连接上游，请检查连接器和目标地址。','Cannot reach upstream. Check the connector and target address.'));
      else if(result.data.state!=='compatible')setProbeError(tr('未获得模型列表，请检查协议、接口路径，或手动填写模型。','No model list returned. Check the protocol and base path, or enter models manually.'));
      else setModels([...new Set(result.data.models||[])]);
    }catch{if(!controller.signal.aborted)setProbeError(tr('获取模型失败，请检查配置和权限后重试，也可手动填写。','Could not fetch models. Check settings and permissions, retry or enter models manually.'));}
    finally{if(!controller.signal.aborted)setProbing(false);}
  }
  useImperativeHandle(ref,()=>({
    async saveUpstream(){
      validate();
      // A plain read may return a default for an unconfigured application.
      // Explicit submission persists it; existing shared settings are preserved.
      if(!accessId||dirty||!upstream?.has_api_key){
        const saved=await api<Upstream>(`/api/v1/ai/applications/${applicationId}`,upstream);
        setUpstream(saved);setDirty(false);
      }
    },
    async saveAccess(id){validate();await api(`/api/v1/ai/accesses/${id}`,{enabled,external_protocol:clientProtocols(upstream!.protocol)[0],models:Object.fromEntries(pairs.map(([a,b])=>[a.trim(),b.trim()]))});},
  }));
  if(failed)return <Notice tone="danger">{tr('无法读取模型配置，请检查应用配置权限。','Cannot load model configuration. Check application configuration permissions.')} <Button onClick={()=>setRetry(n=>n+1)}>{tr('重试','Retry')}</Button></Notice>;
  if(!loaded||!upstream)return <p role="status">{tr('正在加载模型配置…','Loading model configuration…')}</p>;
  const change=(next:Upstream)=>{setDirty(true);setUpstream(next);setModels(undefined);setProbeError('');};
  return <fieldset className="is-full liaison-initial-connection liaison-llm-connection" disabled={disabled||probing}>
    <legend>{tr('模型连接','Model connection')}</legend>
    <p>{applicationOnly?tr('获取列表会保存上游配置，不会创建访问或自动开放模型。','Fetching saves upstream settings. It does not create access or expose models.'):tr('上游配置属于应用，修改会影响该应用的其他访问；模型映射仅属于本访问。','Upstream settings are shared by this application’s accesses. Model mappings apply only to this access.')}</p>
    <div className="liaison-initial-connection-fields">
      <Field label={tr('上游协议','Upstream protocol')}><Select disabled={!!upstream.application_type&&upstream.application_type!=='llm'} value={protocolFamily(upstream.protocol)} onChange={e=>change({...upstream,protocol:e.target.value==='openai'?'openai-compatible':e.target.value,base_path:llmBase(e.target.value)})}>{LLM_PROTOCOL_OPTIONS.map(p=><option key={p.value} value={p.value}>{p.label}</option>)}</Select></Field>
      {protocolFamily(upstream.protocol)==='openai'&&<Field label={tr('API 能力','API capabilities')} hint={tr('仅在上游支持时开启 Responses，不会自动探测或切换。','Enable Responses only if supported by the upstream. No automatic detection or fallback.')}><Select value={upstream.protocol} onChange={e=>change({...upstream,protocol:e.target.value})}><option value="openai-compatible">Chat Completions</option><option value="openai">Chat Completions + Responses</option></Select></Field>}
      <Field label={tr('接口路径','Base path')}><Input required value={upstream.base_path} onChange={e=>change({...upstream,base_path:e.target.value})}/></Field>
      <Field label="TLS"><Select value={String(upstream.tls)} onChange={e=>change({...upstream,tls:e.target.value==='true'})}><option value="true">HTTPS</option><option value="false">HTTP</option></Select></Field>
      <Field label={tr('上游 API 密钥','Upstream API key')} hint={upstream.has_api_key?tr('留空保留已保存密钥。','Leave blank to keep the saved key.'):tr('上游无需认证时留空，不是客户端调用密钥。','Leave blank if upstream needs no authentication. Not a client API key.')}><Input type="password" autoComplete="new-password" placeholder={upstream.has_api_key?'••••••••':undefined} value={upstream.api_key||''} onChange={e=>change({...upstream,api_key:e.target.value})}/></Field>
      <div className="is-full"><Button loading={probing} onClick={()=>void fetchModels()}>{tr('保存上游并获取模型列表','Save upstream & fetch models')}</Button></div>
      {probeError&&<div className="is-full"><Notice tone="danger">{probeError}</Notice></div>}
      {models!==undefined&&<div className="is-full liaison-llm-models" role="group" aria-label={tr('发现的模型','Discovered models')}>
        {!models.length?<p>{tr('上游返回空列表。可检查服务后重试，或在访问中手动填写模型。','The upstream returned no models. Check the service and retry, or enter models manually when configuring access.')}</p>:<>
          <p>{applicationOnly?tr('已发现的模型；在创建访问时选择要开放的模型。','Discovered models. Choose which to expose when creating access.'):tr('勾选模型加入本访问，可继续修改对外名称。','Select models for this access. Public names remain editable.')}</p>
          {models.map(model=>applicationOnly?<div key={model}><code>{model}</code></div>:<label className="liaison-checkbox" key={model}><input type="checkbox" checked={pairs.some(([,m])=>m===model)} disabled={!pairs.some(([,m])=>m===model)&&pairs.filter(([a,m])=>a||m).length>=100} onChange={e=>setPairs(old=>e.target.checked?[...old.filter(([a,m])=>a||m),[model,model]]:old.filter(([,m])=>m!==model))}/><code>{model}</code></label>)}
        </>}
      </div>}
      {!applicationOnly&&<>
      <div className="is-full liaison-llm-access-options"><span>{tr('调用协议','Client protocol')} · {uniqueProtocolFamilies(clientProtocols(upstream.protocol)).map(llmLabel).join(' / ')}</span><label className="liaison-checkbox"><input type="checkbox" checked={enabled} onChange={e=>setEnabled(e.target.checked)}/>{tr('允许 API 调用','Enable API calls')}</label></div>
      <details className="is-full liaison-llm-mappings" open={!models?.length}>
      <summary>{tr('对外名称与手动映射','Public names & manual mappings')}</summary>
      {pairs.map(([alias,model],i)=><div className="is-full liaison-llm-mapping" key={i}>
        <Field label={`${tr('可调用模型','Available model')} ${i+1}`}><Input required value={alias} onChange={e=>setPairs(old=>old.map((p,n)=>n===i?[e.target.value,p[1]]:p))}/></Field>
        <Field label={`${tr('上游模型','Upstream model')} ${i+1}`}><Input required value={model} onChange={e=>setPairs(old=>old.map((p,n)=>n===i?[p[0],e.target.value]:p))}/></Field>
        <Button disabled={pairs.length===1} aria-label={`${tr('移除映射','Remove mapping')} ${i+1}`} onClick={()=>setPairs(old=>old.filter((_,n)=>n!==i))}>{tr('移除','Remove')}</Button>
      </div>)}
      <Button disabled={pairs.length>=100} onClick={()=>setPairs(old=>[...old,['','']])}>{tr('添加映射','Add mapping')}</Button>
      </details>
      </>}
    </div>
  </fieldset>;
});
